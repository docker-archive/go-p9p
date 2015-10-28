package p9pnew

import (
	"bufio"
	"fmt"
	"log"
	"time"

	"golang.org/x/net/context"

	"net"
)

type client struct {
	ctx      context.Context
	conn     net.Conn
	tags     *tagPool
	requests chan fcallRequest
	closed   chan struct{}
}

// NewSession returns a session using the connection. The Context ctx provides
// a context for out of bad messages, such as flushes, that may be sent by the
// session. The session can effectively shutdown with this context.
func NewSession(ctx context.Context, conn net.Conn) (Session, error) {
	return &client{
		ctx:  ctx,
		conn: conn,
	}, nil
}

var _ Session = &client{}

func (c *client) Auth(ctx context.Context, afid Fid, uname, aname string) (Qid, error) {
	panic("not implemented")
}

func (c *client) Attach(ctx context.Context, fid, afid Fid, uname, aname string) (Qid, error) {
	fcall := &Fcall{
		Type: Tattach,
		Message: &MessageTattach{
			Fid:   fid,
			Afid:  afid,
			Uname: uname,
			Aname: aname,
		},
	}

	resp, err := c.send(ctx, fcall)
	if err != nil {
		return Qid{}, err
	}

	mrr, ok := resp.Message.(*MessageRattach)
	if !ok {
		return Qid{}, fmt.Errorf("invalid rpc response for attach message: %v", resp)
	}

	return mrr.Qid, nil
}

func (c *client) Clunk(ctx context.Context, fid Fid) error {
	panic("not implemented")
}

func (c *client) Remove(ctx context.Context, fid Fid) error {
	panic("not implemented")
}

func (c *client) Walk(ctx context.Context, fid Fid, newfid Fid, names ...string) ([]Qid, error) {
	if len(names) > 16 {
		// TODO(stevvooe): Implement multi-message handling for more than 16
		// wnames. May want to actually force caller to implement this since
		// we'll need a new fid for each RPC.
		panic("more than 16 components not implemented")
	}

	fcall := &Fcall{
		Type: Twalk,
		Message: &MessageTwalk{
			Fid:    fid,
			Newfid: newfid,
			Wname:  names,
		},
	}

	resp, err := c.send(ctx, fcall)
	if err != nil {
		return nil, err
	}

	mrr, ok := resp.Message.(*MessageRwalk)
	if !ok {
		return nil, fmt.Errorf("invalid rpc response for walk message: %v", resp)
	}

	return mrr.Qids, nil
}

func (c *client) Read(ctx context.Context, fid Fid, p []byte, offset int64) (n int, err error) {
	// TODO(stevvooe): Split up reads into multiple messages based on iounit.
	// For now, we just support full blast. I mean, why not?
	fcall := &Fcall{
		Type: Tread,
		Message: &MessageTread{
			Fid:    fid,
			Offset: uint64(offset),
			Count:  uint32(len(p)),
		},
	}

	resp, err := c.send(ctx, fcall)
	if err != nil {
		return 0, err
	}

	mrr, ok := resp.Message.(*MessageRread)
	if !ok {
		return 0, fmt.Errorf("invalid rpc response for read message: %v", resp)
	}

	return copy(p, mrr.Data), nil
}

func (c *client) Write(ctx context.Context, fid Fid, p []byte, offset int64) (n int, err error) {
	// TODO(stevvooe): Split up writes into multiple messages based on iounit.
	// For now, we just support full blast. I mean, why not?
	fcall := &Fcall{
		Type: Twrite,
		Message: &MessageTwrite{
			Fid:    fid,
			Offset: uint64(offset),
			Data:   p,
		},
	}

	resp, err := c.send(ctx, fcall)
	if err != nil {
		return 0, err
	}

	mrr, ok := resp.Message.(MessageRwrite)
	if !ok {
		return 0, fmt.Errorf("invalid rpc response for read message: %v", resp)
	}

	return int(mrr.Count), nil
}

func (c *client) Open(ctx context.Context, fid Fid, mode int32) (Qid, error) {
	panic("not implemented")
}

func (c *client) Create(ctx context.Context, parent Fid, name string, perm uint32, mode uint32) (Qid, error) {
	panic("not implemented")
}

func (c *client) Stat(context.Context, Fid) (Dir, error) {
	panic("not implemented")
}

func (c *client) WStat(context.Context, Fid, Dir) error {
	panic("not implemented")
}

func (c *client) Version(ctx context.Context, msize uint32, version string) (uint32, string, error) {
	fcall := &Fcall{
		Type: Tversion,
		Message: MessageVersion{
			MSize:   uint32(msize),
			Version: version,
		},
	}

	resp, err := c.send(ctx, fcall)
	if err != nil {
		return 0, "", err
	}

	mv, ok := resp.Message.(*MessageVersion)
	if !ok {
		return 0, "", fmt.Errorf("invalid rpc response for version message: %v", resp)
	}

	// TODO(stevvooe): Use this response to set iounit and version on this
	// client instance.

	return mv.MSize, mv.Version, nil
}

func (c *client) flush(ctx context.Context, tag Tag) error {
	// TODO(stevvooe): We need to fire and forget flush messages when a call
	// context gets cancelled.
	panic("not implemented")
}

// send dispatches the fcall.
func (c *client) send(ctx context.Context, fc *Fcall) (*Fcall, error) {
	fc.Tag = c.tags.Get()
	defer c.tags.Put(fc.Tag)

	fcreq := newFcallRequest(ctx, fc)

	// dispatch the request.
	select {
	case <-c.closed:
		return nil, ErrClosed
	case c.requests <- fcreq:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// wait for the response.
	select {
	case <-c.closed:
		return nil, ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-fcreq.response:
		return resp, nil
	}
}

type fcallRequest struct {
	ctx      context.Context
	fcall    *Fcall
	response chan *Fcall
	err      chan error
}

func newFcallRequest(ctx context.Context, fc *Fcall) fcallRequest {
	return fcallRequest{
		ctx:      ctx,
		fcall:    fc,
		response: make(chan *Fcall, 1),
		err:      make(chan error, 1),
	}
}

// handle takes messages off the wire and wakes up the waiting tag call.
func (c *client) handle() {

	var (
		responses = make(chan *Fcall)
		// outstanding provides a map of tags to outstanding requests.
		outstanding = map[Tag]*fcallRequest{}
	)

	// loop to read messages off of the connection
	go func() {
		dec := &decoder{bufio.NewReader(c.conn)}

	loop:
		for {
			const pump = time.Second

			// Continuously set the read dead line pump the loop below. We can
			// probably set a connection dead threshold that can count these.
			// Usually, this would only matter when there are actually
			// outstanding requests.
			deadline, ok := c.ctx.Deadline()
			if !ok {
				deadline = time.Now().Add(pump)
			} else {
				// if the deadline is before
				nd := time.Now().Add(pump)
				if nd.Before(deadline) {
					deadline = nd
				}
			}

			if err := c.conn.SetReadDeadline(deadline); err != nil {
				panic(fmt.Sprintf("error setting read deadline: %v", err))
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
			case <-c.ctx.Done():
				return
			case <-c.closed:
				return
			case responses <- fc:
			}
		}

	}()

	enc := &encoder{bufio.NewWriter(c.conn)}

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.closed:
			return
		case req := <-c.requests:
			outstanding[req.fcall.Tag] = &req

			// use deadline to set write deadline for this request.
			deadline, ok := req.ctx.Deadline()
			if !ok {
				deadline = time.Now().Add(time.Second)
			}

			if err := c.conn.SetWriteDeadline(deadline); err != nil {
				log.Println("error setting write deadline: %v", err)
			}

			if err := enc.encode(req.fcall); err != nil {
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
		}
	}
}
