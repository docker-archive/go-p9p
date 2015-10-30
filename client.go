package p9pnew

import (
	"fmt"
	"log"

	"golang.org/x/net/context"

	"net"
)

type client struct {
	version   string
	msize     uint32
	ctx       context.Context
	transport roundTripper
}

// NewSession returns a session using the connection. The Context ctx provides
// a context for out of bad messages, such as flushes, that may be sent by the
// session. The session can effectively shutdown with this context.
func NewSession(ctx context.Context, conn net.Conn) (Session, error) {
	const msize = 64 << 10
	const vers = "9P2000"

	ch := newChannel(conn, codec9p{}, msize) // sets msize, effectively.

	// negotiate the protocol version
	smsize, svers, err := ch.version(ctx, msize, vers)
	if err != nil {
		return nil, err
	}

	if svers != vers {
		// TODO(stevvooe): A stubborn client indeed!
		return nil, fmt.Errorf("unsupported server version: %v", vers)
	}

	if smsize > msize {
		// upgrade msize if server differs.
		ch.setmsize(int(smsize))
	}

	return &client{
		version:   vers,
		msize:     msize,
		ctx:       ctx,
		transport: newTransport(ctx, ch),
	}, nil
}

var _ Session = &client{}

func (c *client) Version() (uint32, string) {
	return c.msize, c.version
}

func (c *client) Auth(ctx context.Context, afid Fid, uname, aname string) (Qid, error) {
	panic("not implemented")
}

func (c *client) Attach(ctx context.Context, fid, afid Fid, uname, aname string) (Qid, error) {
	log.Println("client attach", fid, aname)
	fcall := &Fcall{
		Type: Tattach,
		Message: &MessageTattach{
			Fid:   fid,
			Afid:  afid,
			Uname: uname,
			Aname: aname,
		},
	}

	resp, err := c.transport.send(ctx, fcall)
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
	fcall := newFcall(&MessageTclunk{
		Fid: fid,
	})

	resp, err := c.transport.send(ctx, fcall)
	if err != nil {
		return err
	}

	if resp.Type != Rclunk {
		return fmt.Errorf("incorrect response type: %v", resp)
	}

	return nil
}

func (c *client) Remove(ctx context.Context, fid Fid) error {
	panic("not implemented")
}

func (c *client) Walk(ctx context.Context, fid Fid, newfid Fid, names ...string) ([]Qid, error) {
	if len(names) > 16 {
		return nil, fmt.Errorf("too many elements in wname")
	}

	fcall := &Fcall{
		Type: Twalk,
		Message: &MessageTwalk{
			Fid:    fid,
			Newfid: newfid,
			Wnames: names,
		},
	}

	resp, err := c.transport.send(ctx, fcall)
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

	resp, err := c.transport.send(ctx, fcall)
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

	resp, err := c.transport.send(ctx, fcall)
	if err != nil {
		return 0, err
	}

	mrr, ok := resp.Message.(*MessageRwrite)
	if !ok {
		return 0, fmt.Errorf("invalid rpc response for write message: %v", resp)
	}

	return int(mrr.Count), nil
}

func (c *client) Open(ctx context.Context, fid Fid, mode uint8) (Qid, uint32, error) {
	fcall := newFcall(&MessageTopen{
		Fid:  fid,
		Mode: mode,
	})

	resp, err := c.transport.send(ctx, fcall)
	if err != nil {
		return Qid{}, 0, err
	}

	respmsg, ok := resp.Message.(*MessageRopen)
	if !ok {
		return Qid{}, 0, fmt.Errorf("invalid rpc response for open message: %v", resp)
	}

	return respmsg.Qid, respmsg.IOUnit, nil
}

func (c *client) Create(ctx context.Context, parent Fid, name string, perm uint32, mode uint32) (Qid, uint32, error) {
	panic("not implemented")
}

func (c *client) Stat(context.Context, Fid) (Dir, error) {
	panic("not implemented")
}

func (c *client) WStat(context.Context, Fid, Dir) error {
	panic("not implemented")
}

func (c *client) flush(ctx context.Context, tag Tag) error {
	// TODO(stevvooe): We need to fire and forget flush messages when a call
	// context gets cancelled.

	panic("not implemented")
}
