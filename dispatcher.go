package p9pnew

import (
	"fmt"

	"golang.org/x/net/context"
)

type handler interface {
	handle(ctx context.Context, req *Fcall) (*Fcall, error)
}

// dispatcher routes fcalls to a Session.
type dispatcher struct {
	session Session
}

// handle responds to an fcall using the session. An error is only returned if
// the handler cannot proceed. All session errors are returned as Rerror.
func (d *dispatcher) handle(ctx context.Context, req *Fcall) (*Fcall, error) {
	var resp *Fcall
	switch req.Type {
	case Tauth:
		reqmsg, ok := req.Message.(MessageTauth)
		if !ok {
			return nil, fmt.Errorf("incorrect message for type: %v message=%v", req, req.Message)
		}

		qid, err := d.session.Auth(ctx, reqmsg.Afid, reqmsg.Uname, reqmsg.Aname)
		if err != nil {
			return nil, err
		}

		resp = newFcall(MessageRauth{Qid: qid})
	case Tattach:
		reqmsg, ok := req.Message.(*MessageTattach)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%#v", req, req.Message)
		}

		qid, err := d.session.Attach(ctx, reqmsg.Fid, reqmsg.Afid, reqmsg.Uname, reqmsg.Aname)
		if err != nil {
			return nil, err
		}

		resp = newFcall(MessageRattach{
			Qid: qid,
		})
	case Twalk:
		reqmsg, ok := req.Message.(*MessageTwalk)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%#v", req, req.Message)
		}

		// TODO(stevvooe): This is one of the places where we need to manage
		// fid allocation lifecycle. We need to reserve the fid, then, if this
		// call succeeds, we should alloc the fid for future uses. Also need
		// to interact correctly with concurrent clunk and the flush of this
		// walk message.
		qids, err := d.session.Walk(ctx, reqmsg.Fid, reqmsg.Newfid, reqmsg.Wnames...)
		if err != nil {
			return nil, err
		}

		resp = newFcall(&MessageRwalk{
			Qids: qids,
		})
	case Topen:
		reqmsg, ok := req.Message.(*MessageTopen)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%v", req, req.Message)
		}

		qid, iounit, err := d.session.Open(ctx, reqmsg.Fid, reqmsg.Mode)
		if err != nil {
			return nil, err
		}

		resp = newFcall(&MessageRopen{
			Qid:    qid,
			IOUnit: iounit,
		})
	case Tcreate:
		reqmsg, ok := req.Message.(*MessageTcreate)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%v", req, req.Message)
		}

		qid, iounit, err := d.session.Create(ctx, reqmsg.Fid, reqmsg.Name, reqmsg.Perm, uint32(reqmsg.Mode))
		if err != nil {
			return nil, err
		}

		resp = newFcall(&MessageRcreate{
			Qid:    qid,
			IOUnit: iounit,
		})

	case Tread:
		reqmsg, ok := req.Message.(*MessageTread)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%v", req, req.Message)
		}

		p := make([]byte, int(reqmsg.Count))
		n, err := d.session.Read(ctx, reqmsg.Fid, p, int64(reqmsg.Offset))
		if err != nil {
			return nil, err
		}

		resp = newFcall(&MessageRread{
			Data: p[:n],
		})
	case Twrite:
		reqmsg, ok := req.Message.(*MessageTwrite)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%v", req, req.Message)
		}

		n, err := d.session.Write(ctx, reqmsg.Fid, reqmsg.Data, int64(reqmsg.Offset))
		if err != nil {
			return nil, err
		}

		resp = newFcall(&MessageRwrite{
			Count: uint32(n),
		})
	case Tclunk:
		reqmsg, ok := req.Message.(*MessageTclunk)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%v", req, req.Message)
		}

		// TODO(stevvooe): Manage the clunking of file descriptors based on
		// walk and attach call progression.
		if err := d.session.Clunk(ctx, reqmsg.Fid); err != nil {
			return nil, err
		}

		resp = newFcall(&MessageRclunk{})
	case Tremove:
		reqmsg, ok := req.Message.(*MessageTremove)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%v", req, req.Message)
		}

		if err := d.session.Remove(ctx, reqmsg.Fid); err != nil {
			return nil, err
		}

		resp = newFcall(&MessageRremove{})
	case Tstat:
		reqmsg, ok := req.Message.(*MessageTstat)
		if !ok {
			return nil, fmt.Errorf("bad message: %v message=%v", req, req.Message)
		}

		dir, err := d.session.Stat(ctx, reqmsg.Fid)
		if err != nil {
			return nil, err
		}

		resp = newFcall(&MessageRstat{
			Stat: dir,
		})
	case Twstat:
		panic("not implemented")
	default:
		return nil, ErrUnknownMsg
	}

	return resp, nil
}
