package p9pnew

import (
	"net"

	"golang.org/x/net/context"
)

// Session provides the central abstraction for a 9p connection. Clients
// implement sessions and servers serve sessions. Sessions can be proxied by
// serving up a client session.
//
// The interface is also wired up with full context support to manage timeouts
// and resource clean up.
//
// Session represents the operations covered in section 5 of the plan 9 manual
// (http://man.cat-v.org/plan_9/5/). Requests are managed internally, so the
// Flush method is handled by the internal implementation. Consider preceeding
// these all with context to control request timeout.
type Session interface {
	Auth(ctx context.Context, afid Fid, uname, aname string) (Qid, error)
	Attach(ctx context.Context, fid, afid Fid, uname, aname string) (Qid, error)
	Clunk(ctx context.Context, fid Fid) error
	Remove(ctx context.Context, fid Fid) error
	Walk(ctx context.Context, fid Fid, newfid Fid, names ...string) ([]Qid, error)
	Read(ctx context.Context, fid Fid, p []byte, offset int64) (n int, err error)
	Write(ctx context.Context, fid Fid, p []byte, offset int64) (n int, err error)
	Open(ctx context.Context, fid Fid, mode uint8) (Qid, uint32, error)
	Create(ctx context.Context, parent Fid, name string, perm uint32, mode uint32) (Qid, uint32, error)
	Stat(context.Context, Fid) (Dir, error)
	WStat(context.Context, Fid, Dir) error

	// Version returns the supported version and msize of the session. This
	// can be affected by negotiating or the level of support provided by the
	// session implementation.
	Version() (msize int, version string)
}

func Dial(ctx context.Context, addr string) (Session, error) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	// BUG(stevvooe): Session doesn't actually close connection. Dial might
	// not be the right interface.

	return NewSession(ctx, c)
}
