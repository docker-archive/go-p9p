// +build ignore

package p9pnew

import (
	"log"
	"net"
	"time"

	"golang.org/x/net/context"
)

// Serve the 9p session over the provided network connection.
func Serve(ctx context.Context, conn net.Conn, session Session) error {
	panic("not implemented")
}

type server struct {
	ctx     context.Context
	session Session
	conn    net.Conn
}

func (s *server) run() {
	dec := decoder{s.conn}

	fcall := new(Fcall)
	if err := dec.decode(fcall); err != nil {
		log.Println(err)
	}

}

// handle responds to an fcall using the session. An error is only returned if
// the handler cannot proceed. All session errors are returned as Rerror.
func (s *server) handle(f *Fcall) (*Fcall, error) {
	const timeout = 30 * time.Second // TODO(stevvooe): Allow this to be configured.
	ctx, cancel = context.WithTimeout(s.ctx, timeout)
	defer cancel()

	switch fcall.Type {
	case Tattach:
		atc, ok := fcall.Message.(*MessageTattach)
		if ok {
			log.Println("bad message")
			continue
		}

		qid, err := s.session.Attach(s.ctx, atc.Fid, atc.Afid, atc.Uname, atc.Aname)
		if err != nil {
			return
		}
	}
}
