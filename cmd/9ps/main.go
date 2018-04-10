package main

import (
	"flag"
	"log"
	"net"
	"strings"

	"github.com/docker/go-p9p"
	"github.com/docker/go-p9p/ufs"
	"golang.org/x/net/context"
)

var (
	root string
	addr string
)

func init() {
	flag.StringVar(&root, "root", "~/", "root of filesystem to serve over 9p")
	flag.StringVar(&addr, "addr", ":5640", "bind addr for 9p server, prefix with unix: for unix socket")
}

func main() {
	ctx := context.Background()
	log.SetFlags(0)
	flag.Parse()

	proto := "tcp"
	if strings.HasPrefix(addr, "unix:") {
		proto = "unix"
		addr = addr[5:]
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		log.Fatalln("error listening:", err)
	}
	defer listener.Close()

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Fatalln("error accepting:", err)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			ctx := context.WithValue(ctx, "conn", conn)
			log.Println("connected", conn.RemoteAddr())
			session, err := ufs.NewSession(ctx, root)
			if err != nil {
				log.Println("error creating session")
				return
			}

			if err := p9p.ServeConn(ctx, conn, p9p.Dispatch(session)); err != nil {
				log.Printf("serving conn: %v", err)
			}
		}(c)
	}
}
