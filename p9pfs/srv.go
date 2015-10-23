package p9pnew

import (
	"crypto/sha512"
	"encoding/binary"
	"fmt"

	"github.com/docker/pinata/v1/fs"
)

type state struct {
	Qid  Qid
	Path string
}

type sessionServer struct {
	fs     fs.FileSystem
	states map[Fid]*state
}

func Serve(f fs.FileSystem) (Session, error) {
	return &sessionServer{fs: f, states: make(map[Fid]*state)}, nil
}

func (s *sessionServer) Auth(afid Fid, uname, aname string) (Qid, error) {
	return Qid{}, Error{Name: "unsupported"}
}

func (s *sessionServer) Attach(fid, afid Fid, uname, aname string) (Qid, error) {
	if afid != NOFID {
		return Qid{}, Error{Name: "auth fid not supported"}
	}

	// for now, we only support attaching to "/"
	if aname != "/" {
		return Qid{}, Error{Name: "can only attach to root"}
	}

	st, ok := s.states[fid]
	if !ok {
		// TODO(stevvooe): Need to be slightly careful here: this assumes that the
		// root of the filesystem is always a directory which we may not want to
		// maintain.
		qid := Qid{
			Type:    QTDIR,
			Path:    hashpath(aname),
			Version: 0,
		}

		st = &state{
			Qid:  qid,
			Path: aname,
		}

		s.states[fid] = st
	} else {
		// check paths match here
		if aname != st.Path {
			return Qid{}, Error{Name: fmt.Sprintf("already attached to %q", st.Path)}
		}
	}

	return st.Qid, nil
}

func (s *sessionServer) Clunk(fid Fid) error {
	panic("not implemented")
}

func (s *sessionServer) Remove(fid Fid) error {
	panic("not implemented")
}

func (s *sessionServer) Walk(fid Fid, newfid Fid, names ...string) ([]Qid, error) {
	st, ok := s.states[fid]
	if !ok {
		return Qid{}, ErrUnknownFid
	}

}

// walk1 steps one name from the provided fid, assigning the target to qid.
func (s *sessionServer) walk1(fid Fid, name string, qid Qid) error {

}

func (s *sessionServer) Read(fid Fid, p []byte, offset int64) (n int, err error) {
	panic("not implemented")

}

func (s *sessionServer) Write(fid Fid, p []byte, offset int64) (n int, err error) {
	panic("not implemented")

}

func (s *sessionServer) Open(fid Fid, mode int32) (Qid, error) {
	panic("not implemented")

}

func (s *sessionServer) Create(parent Fid, name string, perm uint32, mode uint32) (Qid, error) {
	panic("not implemented")

}

func (s *sessionServer) Stat(Fid) (Dir, error) {
	panic("not implemented")

}

func (s *sessionServer) WStat(Fid, Dir) error {
	panic("not implemented")

}

func (s *sessionServer) Version(msize int32, version string) (int32, string, error) {
	panic("not implemented")

}

func hashpath(p string) uint64 {
	b := sha512.Sum512_224([]byte(p))
	return binary.LittleEndian.Uint64(b[:8])
}
