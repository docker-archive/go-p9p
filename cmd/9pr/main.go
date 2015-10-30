package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/chzyer/readline"
	"github.com/docker/pinata/v1/fs/p9p/new"
	"golang.org/x/net/context"
)

var addr string

func init() {
	flag.StringVar(&addr, "addr", ":5640", "addr of 9p service")
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	ctx := context.Background()
	log.SetFlags(0)
	flag.Parse()

	log.Println("dialing", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	csession, err := p9pnew.NewSession(ctx, conn)
	if err != nil {
		log.Fatalln(err)
	}

	commander := &fsCommander{
		ctx:     context.Background(),
		session: csession,
		pwd:     "/",
		stdout:  os.Stdout,
		stderr:  os.Stderr,
	}

	completer := readline.NewPrefixCompleter(
		readline.PcItem("ls"),
		// readline.PcItem("find"),
		readline.PcItem("stat"),
		readline.PcItem("cat"),
		readline.PcItem("cd"),
		readline.PcItem("pwd"),
	)

	rl, err := readline.NewEx(&readline.Config{
		HistoryFile:  ".history",
		AutoComplete: completer,
	})
	if err != nil {
		log.Fatalln(err)
	}
	commander.readline = rl

	msize, version := commander.session.Version()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("9p version", version, msize)

	// attach root
	commander.nextfid = 1
	if _, err := commander.session.Attach(commander.ctx, commander.nextfid, p9pnew.NOFID, "anyone", "/"); err != nil {
		log.Fatalln(err)
	}
	commander.rootfid = commander.nextfid
	commander.nextfid++

	// clone the pwd fid so we can clunk it
	if _, err := commander.session.Walk(commander.ctx, commander.rootfid, commander.nextfid); err != nil {
		log.Fatalln(err)
	}
	commander.pwdfid = commander.nextfid
	commander.nextfid++

	for {
		commander.readline.SetPrompt(fmt.Sprintf("%s 🐳 > ", commander.pwd))

		line, err := rl.Readline()
		if err != nil {
			log.Fatalln("error: ", err)
		}

		if line == "" {
			continue
		}

		args := strings.Fields(line)

		name := args[0]
		var cmd func(ctx context.Context, args ...string) error

		switch name {
		case "ls":
			cmd = commander.cmdls
		case "cd":
			cmd = commander.cmdcd
		case "pwd":
			cmd = commander.cmdpwd
		case "cat":
			cmd = commander.cmdcat
		case "stat":
			cmd = commander.cmdstat
		default:
			cmd = func(ctx context.Context, args ...string) error {
				return fmt.Errorf("command not implemented")
			}
		}

		ctx, _ = context.WithTimeout(commander.ctx, 5*time.Second)
		if err := cmd(ctx, args[1:]...); err != nil {
			log.Printf("👹 %s: %v", name, err)
		}
	}
}

type fsCommander struct {
	ctx     context.Context
	session p9pnew.Session
	pwd     string
	pwdfid  p9pnew.Fid
	rootfid p9pnew.Fid

	nextfid p9pnew.Fid

	readline *readline.Instance
	stdout   io.Writer
	stderr   io.Writer
}

func (c *fsCommander) cmdls(ctx context.Context, args ...string) error {
	ps := []string{c.pwd}
	if len(args) > 0 {
		ps = args
	}

	wr := tabwriter.NewWriter(c.stdout, 0, 8, 8, ' ', 0)

	for _, p := range ps {
		// create a header if have more than one path.
		if len(ps) > 1 {
			fmt.Fprintln(wr, p+":")
		}

		if !path.IsAbs(p) {
			p = path.Join(c.pwd, p)
		}

		targetfid := c.nextfid
		c.nextfid++
		components := strings.Split(strings.Trim(p, "/"), "/")
		if _, err := c.session.Walk(ctx, c.rootfid, targetfid, components...); err != nil {
			return err
		}
		defer c.session.Clunk(ctx, targetfid)

		_, iounit, err := c.session.Open(ctx, targetfid, p9pnew.OREAD)
		if err != nil {
			return err
		}

		if iounit < 1 {
			msize, _ := c.session.Version()
			iounit = msize - 24 // size of message max minus fcall io header (Rread)
		}

		p := make([]byte, iounit)

		n, err := c.session.Read(ctx, targetfid, p, 0)
		if err != nil {
			return err
		}

		rd := bytes.NewReader(p[:n])

		for {
			var d p9pnew.Dir
			if err := p9pnew.DecodeDir(rd, &d); err != nil {
				if err == io.EOF {
					break
				}

				return err
			}

			fmt.Fprintf(wr, "%v\t%v\t%v\t%s\n", os.FileMode(d.Mode), d.Length, d.ModTime, d.Name)
		}

		if len(ps) > 1 {
			fmt.Fprintln(wr, "")
		}
	}

	// all output is dumped only after success.
	return wr.Flush()
}

func (c *fsCommander) cmdcd(ctx context.Context, args ...string) error {
	var p string
	switch len(args) {
	case 0:
		p = "/"
	case 1:
		p = args[0]
	default:
		return fmt.Errorf("cd: invalid args: %v", args)
	}

	if !path.IsAbs(p) {
		p = path.Join(c.pwd, p)
	}

	targetfid := c.nextfid
	c.nextfid++
	components := strings.Split(strings.TrimSpace(strings.Trim(p, "/")), "/")
	if _, err := c.session.Walk(c.ctx, c.rootfid, targetfid, components...); err != nil {
		return err
	}
	defer c.session.Clunk(c.ctx, c.pwdfid)

	log.Println("cd", p, targetfid)
	c.pwd = p
	c.pwdfid = targetfid

	return nil
}

func (c *fsCommander) cmdstat(ctx context.Context, args ...string) error {
	ps := []string{c.pwd}
	if len(args) > 0 {
		ps = args
	}

	wr := tabwriter.NewWriter(c.stdout, 0, 8, 8, ' ', 0)

	for _, p := range ps {
		targetfid := c.nextfid
		c.nextfid++
		components := strings.Split(strings.Trim(p, "/"), "/")
		if _, err := c.session.Walk(ctx, c.rootfid, targetfid, components...); err != nil {
			return err
		}
		defer c.session.Clunk(ctx, targetfid)

		d, err := c.session.Stat(ctx, targetfid)
		if err != nil {
			return err
		}

		fmt.Fprintf(wr, "%v\t%v\t%v\t%s\n", os.FileMode(d.Mode), d.Length, d.ModTime, d.Name)
	}

	return wr.Flush()
}

func (c *fsCommander) cmdpwd(ctx context.Context, args ...string) error {
	if len(args) != 0 {
		return fmt.Errorf("pwd takes no arguments")
	}

	fmt.Println(c.pwd)
	return nil
}

func (c *fsCommander) cmdcat(ctx context.Context, args ...string) error {
	var p string
	switch len(args) {
	case 0:
		p = "/"
	case 1:
		p = args[0]
	default:
		return fmt.Errorf("cd: invalid args: %v", args)
	}

	if !path.IsAbs(p) {
		p = path.Join(c.pwd, p)
	}

	targetfid := c.nextfid
	c.nextfid++
	components := strings.Split(strings.TrimSpace(strings.Trim(p, "/")), "/")
	if _, err := c.session.Walk(ctx, c.rootfid, targetfid, components...); err != nil {
		return err
	}
	defer c.session.Clunk(ctx, c.pwdfid)

	_, msize, err := c.session.Open(ctx, targetfid, p9pnew.OREAD)
	if err != nil {
		return err
	}

	b := make([]byte, msize)

	n, err := c.session.Read(ctx, targetfid, b, 0)
	if err != nil {
		return err
	}

	if _, err := os.Stdout.Write(b[:n]); err != nil {
		return err
	}

	os.Stdout.Write([]byte("\n"))

	return nil
}
