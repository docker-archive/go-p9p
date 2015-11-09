package p9p

import (
	"log"
	"net"
	"time"

	"golang.org/x/net/context"
)

// Serve the 9p session over the provided network connection.
func Serve(ctx context.Context, conn net.Conn, handler Handler) {

	// TODO(stevvooe): It would be nice if the handler could declare the
	// supported version. Before we had handler, we used the session to get
	// the version (msize, version := session.Version()). We must decided if
	// we want to proxy version and message size decisions all the back to the
	// origin server or make those decisions at each link of a proxy chain.

	ch := newChannel(conn, codec9p{}, DefaultMSize)
	negctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// TODO(stevvooe): For now, we negotiate here. It probably makes sense to
	// do this outside of this function and then pass in a ready made channel.
	// We are not really ready to export the channel type yet.

	if err := servernegotiate(negctx, ch, DefaultVersion); err != nil {
		// TODO(stevvooe): Need better error handling and retry support here.
		// For now, we silently ignore the failure.
		log.Println("error negotiating version:", err)
		return
	}

	ctx = withVersion(ctx, DefaultVersion)

	s := &server{
		ctx:     ctx,
		ch:      ch,
		handler: handler,
		closed:  make(chan struct{}),
	}

	s.run()
}

type server struct {
	ctx     context.Context
	session Session
	ch      Channel
	handler Handler
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
			log.Println("server: context done")
			return
		case <-s.closed:
			log.Println("server: shutdown")
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
			if err, ok := err.(net.Error); ok {
				if err.Timeout() || err.Temporary() {
					continue
				}
			}

			log.Println("server: error reading fcall", err)
			return
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

		var resp *Fcall
		msg, err := s.handler.Handle(ctx, req.Message)
		if err != nil {
			// all handler errors are forwarded as protocol errors.
			resp = newErrorFcall(req.Tag, err)
		} else {
			resp = newFcall(msg)
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
