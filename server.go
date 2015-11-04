package p9pnew

import (
	"log"
	"net"
	"time"

	"golang.org/x/net/context"
)

// Serve the 9p session over the provided network connection.
func Serve(ctx context.Context, conn net.Conn, session Session) {
	const msize = 64 << 10
	const vers = "9P2000"

	ch := newChannel(conn, codec9p{}, msize)

	negctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// TODO(stevvooe): For now, we negotiate here. It probably makes sense to
	// do this outside of this function and then pass in a ready made channel.
	// We are not really ready to export the channel type yet.

	if err := servernegotiate(negctx, ch, vers); err != nil {
		// TODO(stevvooe): Need better error handling and retry support here.
		// For now, we silently ignore the failure.
		log.Println("error negotiating version:", err)
		return
	}

	s := &server{
		ctx:     ctx,
		ch:      ch,
		handler: &dispatcher{session: session},
		closed:  make(chan struct{}),
	}

	s.run()
}

type server struct {
	ctx     context.Context
	session Session
	ch      Channel
	handler handler
	closed  chan struct{}
}

type activeTag struct {
	ctx       context.Context
	request   *Fcall
	cancel    context.CancelFunc
	responded bool // true, if some response was sent (Response or Rflush/Rerror)
}

func (s *server) run() {
	tags := map[Tag]*activeTag{} // active requests

	log.Println("server.run()")
	for {
		select {
		case <-s.ctx.Done():
			log.Println("server: shutdown")
			return
		case <-s.closed:
		default:
		}

		// BUG(stevvooe): This server blocks on reads, calls to handlers and
		// write, effectively single tracking fcalls through a target
		// dispatcher. There is no reason we couldn't parallelize these
		// requests out to the dispatcher to get massive performance
		// improvements.

		log.Println("server:", "wait")
		req := new(Fcall)
		if err := s.ch.ReadFcall(s.ctx, req); err != nil {
			log.Println("server: error reading fcall", err)
			continue
		}

		if _, ok := tags[req.Tag]; ok {
			resp := newErrorFcall(req.Tag, ErrDuptag)
			if err := s.ch.WriteFcall(s.ctx, resp); err != nil {
				log.Printf("error sending duplicate tag response: %v", err)
			}
			continue
		}

		// handle flush calls. The tag never makes it into active from here.
		if mf, ok := req.Message.(MessageTflush); ok {
			log.Println("flushing message", mf.Oldtag)

			// check if we have actually know about the requested flush
			active, ok := tags[mf.Oldtag]
			if ok {
				active.cancel() // cancel the context

				resp := newFcall(MessageRflush{})
				resp.Tag = req.Tag
				if err := s.ch.WriteFcall(s.ctx, resp); err != nil {
					log.Printf("error responding to flush: %v", err)
				}
				active.responded = true
			} else {
				resp := newErrorFcall(req.Tag, ErrUnknownTag)
				if err := s.ch.WriteFcall(s.ctx, resp); err != nil {
					log.Printf("error responding to flush: %v", err)
				}
			}

			continue
		}

		// TODO(stevvooe): Add handler timeout here, as well, if we desire.

		// Allows us to signal handlers to cancel processing of the fcall
		// through context.
		ctx, cancel := context.WithCancel(s.ctx)

		tags[req.Tag] = &activeTag{
			ctx:     ctx,
			request: req,
			cancel:  cancel,
		}

		resp, err := s.handler.handle(ctx, req)
		if err != nil {
			// all handler errors are forwarded as protocol errors.
			resp = newErrorFcall(req.Tag, err)
		}
		resp.Tag = req.Tag

		if err := ctx.Err(); err != nil {
			// NOTE(stevvooe): We aren't really getting our moneys worth for
			// how this is being handled. We really need to dispatch each
			// request handler to a separate thread.

			// the context was canceled for some reason, perhaps timeout or
			// due to a flush call. We treat this as a condition where a
			// response should not be sent.
			log.Println("context error:", err)
			continue
		}

		if !tags[req.Tag].responded {
			if err := s.ch.WriteFcall(ctx, resp); err != nil {
				log.Println("server: error writing fcall:", err)
				continue
			}
		}

		delete(tags, req.Tag)
	}
}
