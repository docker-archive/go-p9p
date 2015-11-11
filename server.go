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

// activeRequest includes information about the active request.
type activeRequest struct {
	ctx     context.Context
	request *Fcall
	cancel  context.CancelFunc
}

func (s *server) run() {
	tags := map[Tag]*activeRequest{} // active requests

	requests := make(chan *Fcall) // sync, read-limited
	responses := make(chan *Fcall)
	completed := make(chan *Fcall, 1)

	// read loop
	go func() {
		for {
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

			select {
			case requests <- req:
			case <-s.ctx.Done():
				log.Println("server: context done")
				return
			case <-s.closed:
				log.Println("server: shutdown")
				return
			}
		}
	}()

	// write loop
	go func() {
		for {
			select {
			case resp := <-responses:
				if err := s.ch.WriteFcall(s.ctx, resp); err != nil {
					log.Println("server: error writing fcall:", err)
				}
			case <-s.ctx.Done():
				log.Println("server: context done")
				return
			case <-s.closed:
				log.Println("server: shutdown")
				return
			}
		}
	}()

	log.Println("server.run()")
	for {
		log.Println("server:", "wait")
		select {
		case req := <-requests:
			log.Println("request", req)
			if _, ok := tags[req.Tag]; ok {
				select {
				case responses <- newErrorFcall(req.Tag, ErrDuptag):
					// Send to responses, bypass tag management.
				case <-s.ctx.Done():
					return
				case <-s.closed:
					return
				}
				continue
			}

			switch msg := req.Message.(type) {
			case MessageTflush:
				log.Println("server: flushing message", msg.Oldtag)

				var resp *Fcall
				// check if we have actually know about the requested flush
				active, ok := tags[msg.Oldtag]
				if ok {
					active.cancel() // cancel the context of oldtag
					resp = newFcall(req.Tag, MessageRflush{})
				} else {
					resp = newErrorFcall(req.Tag, ErrUnknownTag)
				}

				select {
				case responses <- resp:
					// bypass tag management in completed.
				case <-s.ctx.Done():
					return
				case <-s.closed:
					return
				}
			default:
				// Allows us to session handlers to cancel processing of the fcall
				// through context.
				ctx, cancel := context.WithCancel(s.ctx)

				// The contents of these instances are only writable in the main
				// server loop. The value of tag will not change.
				tags[req.Tag] = &activeRequest{
					ctx:     ctx,
					request: req,
					cancel:  cancel,
				}

				go func(ctx context.Context, req *Fcall) {
					var resp *Fcall
					msg, err := s.handler.Handle(ctx, req.Message)
					if err != nil {
						// all handler errors are forwarded as protocol errors.
						resp = newErrorFcall(req.Tag, err)
					} else {
						resp = newFcall(req.Tag, msg)
					}

					select {
					case completed <- resp:
					case <-ctx.Done():
						return
					case <-s.closed:
						return
					}
				}(ctx, req)
			}
		case resp := <-completed:
			log.Println("completed", resp)
			// only responses that flip the tag state traverse this section.
			active, ok := tags[resp.Tag]
			if !ok {
				panic("BUG: unbalanced tag")
			}

			select {
			case responses <- resp:
			case <-active.ctx.Done():
				// the context was canceled for some reason, perhaps timeout or
				// due to a flush call. We treat this as a condition where a
				// response should not be sent.
				log.Println("canceled", resp, active.ctx.Err())
			}
			delete(tags, resp.Tag)
		case <-s.ctx.Done():
			log.Println("server: context done")
			return
		case <-s.closed:
			log.Println("server: shutdown")
			return
		}
	}
}
