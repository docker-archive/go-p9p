package p9pnew

import (
	"fmt"
	"log"
	"net"

	"golang.org/x/net/context"
)

// roundTripper manages the request and response from the client-side. A
// roundTripper must abide by similar rules to the http.RoundTripper.
// Typically, the roundTripper will manage tag assignment and message
// serialization.
type roundTripper interface {
	send(ctx context.Context, msg Message) (Message, error)
}

// transport plays the role of being a client channel manager. It multiplexes
// function calls onto the wire and dispatches responses to blocking calls to
// send. On the whole, transport is thread-safe for calling send
type transport struct {
	ctx      context.Context
	ch       Channel
	requests chan *fcallRequest
	closed   chan struct{}

	tags uint16
}

var _ roundTripper = &transport{}

func newTransport(ctx context.Context, ch *channel) roundTripper {
	t := &transport{
		ctx:      ctx,
		ch:       ch,
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

func (t *transport) send(ctx context.Context, msg Message) (Message, error) {
	fcall := newFcall(msg)
	req := newFcallRequest(ctx, fcall)

	// dispatch the request.
	select {
	case <-t.closed:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case t.requests <- req:
	}

	// wait for the response.
	select {
	case <-t.closed:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-req.err:
		return nil, err
	case resp := <-req.response:
		if resp.Type == Rerror {
			// pack the error into something useful
			respmesg, ok := resp.Message.(MessageRerror)
			if !ok {
				return nil, fmt.Errorf("invalid error response: %v", resp)
			}

			return nil, respmesg
		}

		return resp.Message, nil
	}
}

// handle takes messages off the wire and wakes up the waiting tag call.
func (t *transport) handle() {
	defer func() {
		log.Println("exited handle loop")
		close(t.closed)
	}()
	// the following variable block are protected components owned by this thread.
	var (
		responses = make(chan *Fcall)
		tags      Tag
		// outstanding provides a map of tags to outstanding requests.
		outstanding = map[Tag]*fcallRequest{}
	)

	// loop to read messages off of the connection
	go func() {
		defer func() {
			log.Println("exited read loop")
			close(t.closed)
		}()
	loop:
		for {
			fcall := new(Fcall)
			if err := t.ch.ReadFcall(t.ctx, fcall); err != nil {
				switch err := err.(type) {
				case net.Error:
					if err.Timeout() || err.Temporary() {
						// BUG(stevvooe): There may be partial reads under
						// timeout errors where this is actually fatal.

						// can only retry if we haven't offset the frame.
						continue loop
					}
				}

				log.Println("fatal error reading msg:", err)
				t.Close()
				return
			}

			select {
			case <-t.ctx.Done():
				log.Println("ctx done")
				return
			case <-t.closed:
				log.Println("transport closed")
				return
			case responses <- fcall:
			}
		}
	}()

	for {
		log.Println("wait...")
		select {
		case req := <-t.requests:
			if req.fcall.Tag == NOTAG {
				// NOTE(stevvooe): We disallow fcalls with NOTAG to come
				// through this path since we can't join the tagged response
				// with the waiting caller. This is typically used for the
				// Tversion/Rversion round trip to setup a session.
				//
				// It may be better to allow these through but block all
				// requests until a notag message has a response.

				req.err <- fmt.Errorf("disallowed tag through transport")
				continue
			}

			// BUG(stevvooe): This is an awful tag allocation procedure.
			// Replace this with something that let's us allocate tags and
			// associate data with them, returning to them to a pool when
			// complete. Such a system would provide a lot of information
			// about outstanding requests.
			tags++
			req.fcall.Tag = tags
			outstanding[req.fcall.Tag] = req

			// TODO(stevvooe): Consider the case of requests that never
			// receive a response. We need to remove the fcall context from
			// the tag map and dealloc the tag. We may also want to send a
			// flush for the tag.
			if err := t.ch.WriteFcall(req.ctx, req.fcall); err != nil {
				delete(outstanding, req.fcall.Tag)
				req.err <- err
			}
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

func (t *transport) flush(ctx context.Context, tag Tag) error {
	// TODO(stevvooe): We need to fire and forget flush messages when a call
	// context gets cancelled.
	panic("not implemented")
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
