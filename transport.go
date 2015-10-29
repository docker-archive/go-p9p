package p9pnew

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/net/context"
)

// roundTripper manages the request and response from the client-side. A
// roundTripper must abide by many of the rules of http.RoundTripper.
// Typically, the roundTripper will manage tag assignment and message
// serialization.
type roundTripper interface {
	send(ctx context.Context, fc *Fcall) (*Fcall, error)
}

type transport struct {
	ctx      context.Context
	conn     net.Conn
	requests chan *fcallRequest
	closed   chan struct{}

	tags uint16
}

func newTransport(ctx context.Context, conn net.Conn) roundTripper {
	t := &transport{
		ctx:      ctx,
		conn:     conn,
		requests: make(chan *fcallRequest),
		closed:   make(chan struct{}),
	}

	go t.handle()

	return t
}

type fcallRequest struct {
	ctx      context.Context
	fcall    *Fcall
	response chan *Fcall
	err      chan error
}

func newFcallRequest(ctx context.Context, fcall *Fcall) *fcallRequest {
	return &fcallRequest{
		ctx:      ctx,
		fcall:    fcall,
		response: make(chan *Fcall, 1),
		err:      make(chan error, 1),
	}
}

func (t *transport) send(ctx context.Context, fcall *Fcall) (*Fcall, error) {
	req := newFcallRequest(ctx, fcall)

	log.Println("dispatch", fcall)
	// dispatch the request.
	select {
	case <-t.closed:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case t.requests <- req:
	}

	log.Println("wait", fcall)
	// wait for the response.
	select {
	case <-t.closed:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-req.response:
		if resp.Type == Rerror {
			// pack the error into something useful
			respmesg, ok := resp.Message.(*MessageRerror)
			if !ok {
				return nil, fmt.Errorf("invalid error response: %v", resp)
			}

			return nil, new9pError(respmesg.Ename)
		}

		return resp, nil
	}
}

// handle takes messages off the wire and wakes up the waiting tag call.
func (t *transport) handle() {

	// the following variable block are protected components owned by this thread.
	var (
		responses = make(chan *Fcall)
		tags      Tag
		// outstanding provides a map of tags to outstanding requests.
		outstanding = map[Tag]*fcallRequest{}
		brd         = bufio.NewReader(t.conn)
		bwr         = bufio.NewWriter(t.conn)
		enc         = &encoder{bwr}
		dec         = &decoder{brd}
	)

	// loop to read messages off of the connection
	go func() {

	loop:
		for {
			const pump = time.Second

			// Continuously set the read dead line pump the loop below. We can
			// probably set a connection dead threshold that can count these.
			// Usually, this would only matter when there are actually
			// outstanding requests.
			deadline, ok := t.ctx.Deadline()
			if !ok {
				deadline = time.Now().Add(pump)
			} else {
				// if the deadline is before
				nd := time.Now().Add(pump)
				if nd.Before(deadline) {
					deadline = nd
				}
			}

			if err := t.conn.SetReadDeadline(deadline); err != nil {
				log.Printf("error setting read deadline: %v", err)
			}

			fc := new(Fcall)
			if err := dec.decode(fc); err != nil {
				switch err := err.(type) {
				case net.Error:
					if err.Timeout() || err.Temporary() {
						break loop
					}
				}

				panic(fmt.Sprintf("connection read error: %v", err))
			}

			select {
			case <-t.ctx.Done():
				return
			case <-t.closed:
				return
			case responses <- fc:
			}
		}
	}()

	for {
		select {
		case req := <-t.requests:
			tags++
			req.fcall.Tag = tags
			outstanding[req.fcall.Tag] = req

			// use deadline to set write deadline for this request.
			deadline, ok := req.ctx.Deadline()
			if !ok {
				deadline = time.Now().Add(time.Second)
			}

			if err := t.conn.SetWriteDeadline(deadline); err != nil {
				log.Printf("error setting write deadline: %v", err)
			}

			// TODO(stevvooe): Consider the case of requests that never
			// receive a response. We need to remove the fcall context from
			// the tag map and dealloc the tag. We may also want to send a
			// flush for the tag.

			log.Println("send", req.fcall)
			if err := enc.encode(req.fcall); err != nil {
				delete(outstanding, req.fcall.Tag)
				req.err <- err
			}
			if err := bwr.Flush(); err != nil {
				delete(outstanding, req.fcall.Tag)
				req.err <- err
			}

			log.Println("sent", req.fcall)
		case b := <-responses:
			req, ok := outstanding[b.Tag]
			if !ok {
				panic("unknown tag received")
			}
			delete(outstanding, req.fcall.Tag)

			req.response <- b

			// TODO(stevvooe): Reclaim tag id.
		case <-t.ctx.Done():
			return
		case <-t.closed:
			return
		}
	}
}

func (t *transport) Close() error {
	select {
	case <-t.closed:
		return ErrClosed
	case <-t.ctx.Done():
		return t.ctx.Err()
	default:
		close(t.closed)
	}

	return nil
}
