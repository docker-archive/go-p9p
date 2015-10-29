package main

import (
	"bytes"
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

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	log.SetFlags(0)

	// addr := os.Args[1]
	ctx := context.Background()
	// TODO(stevvooe): Use a dialer once we have the server session working
	// and running.

	session := newSimpleSession()

	sconn, cconn := net.Pipe()

	go p9pnew.Serve(ctx, sconn, session)

	log.Println("new session")
	csession, err := p9pnew.NewSession(ctx, cconn)
	if err != nil {
		log.Fatalln(err)
	}

	// session, err := p9pnew.Dial(ctx, addr)
	// if err != nil {
	// 	log.Fatalln(err)
	// }

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
		// readline.PcItem("stat"),
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

	log.Println("attach root")
	// attach root
	commander.nextfid = 1
	if _, err := commander.session.Attach(commander.ctx, commander.nextfid, p9pnew.NOFID, "anyone", "/"); err != nil {
		log.Fatalln(err)
	}
	commander.rootfid = commander.nextfid
	commander.nextfid++

	log.Println("clone root")
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
		default:
			cmd = func(ctx context.Context, args ...string) error {
				return fmt.Errorf("command not implemented")
			}
		}

		ctx, _ = context.WithTimeout(commander.ctx, time.Second)
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

		if _, _, err := c.session.Open(ctx, targetfid, p9pnew.OREAD); err != nil {
			return err
		}

		p := make([]byte, 4<<20)

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
	if _, err := c.session.Walk(c.ctx, c.rootfid, targetfid, components...); err != nil {
		return err
	}
	defer c.session.Clunk(c.ctx, c.pwdfid)

	_, msize, err := c.session.Open(c.ctx, targetfid, p9pnew.OREAD)
	if err != nil {
		return err
	}

	b := make([]byte, msize)

	n, err := c.session.Read(c.ctx, targetfid, b, 0)
	if err != nil {
		return err
	}

	if _, err := os.Stdout.Write(b[:n]); err != nil {
		return err
	}

	os.Stdout.Write([]byte("\n"))

	return nil
}

type simpleSession struct {
	root   *fidinfo
	fids   map[p9pnew.Fid]*fidinfo
	opened map[p9pnew.Fid]struct{}
}

type fidinfo struct {
	name string
	dir  p9pnew.Dir

	children map[string]*fidinfo
	data     []byte
}

func (fi *fidinfo) add(child *fidinfo) {
	log.Println("add", fi.name, child.name)
	fi.children[child.name] = child
}

var _ p9pnew.Session = &simpleSession{}

var pathnum uint64

func mkdir(name string) *fidinfo {
	log.Println("mkdir", name)
	pathnum++
	return &fidinfo{
		name: name,
		dir: p9pnew.Dir{
			Name:       name,
			Qid:        p9pnew.Qid{Type: p9pnew.QTDIR, Path: pathnum, Version: 1},
			Mode:       p9pnew.DMDIR | p9pnew.DMREAD | p9pnew.DMEXEC,
			ModTime:    time.Now().UTC(),
			AccessTime: time.Now().UTC(),
		},
		children: make(map[string]*fidinfo),
	}
}

func mkfile(name string, p []byte) *fidinfo {
	log.Println("mkfile", name)
	pathnum++
	return &fidinfo{
		name: name,
		dir: p9pnew.Dir{
			Name:       name,
			Qid:        p9pnew.Qid{Type: p9pnew.QTFILE, Path: pathnum, Version: 1},
			Mode:       p9pnew.DMREAD,
			ModTime:    time.Now().UTC(),
			AccessTime: time.Now().UTC(),
		},
		data: p,
	}
}

func newSimpleSession() p9pnew.Session {
	root := mkdir("/")
	a := mkdir("a")
	ab := mkfile("b", []byte("the b child"))
	a.add(ab)
	b := mkdir("b")
	bc := mkfile("c", []byte("the c child"))
	b.add(bc)
	root.add(a)
	root.add(b)

	return &simpleSession{
		root:   root,
		fids:   make(map[p9pnew.Fid]*fidinfo),
		opened: make(map[p9pnew.Fid]struct{}),
	}
}

func (s *simpleSession) Auth(ctx context.Context, afid p9pnew.Fid, uname, aname string) (p9pnew.Qid, error) {
	panic("not implemented")
}

func (s *simpleSession) Attach(ctx context.Context, fid, afid p9pnew.Fid, uname, aname string) (p9pnew.Qid, error) {
	log.Println("attach", fid, aname)
	if aname != "/" {
		return p9pnew.Qid{}, p9pnew.ErrBadattach
	}

	s.fids[fid] = s.root
	log.Println("root", s.root, fid)

	return s.fids[fid].dir.Qid, nil
}

func (s *simpleSession) Clunk(ctx context.Context, fid p9pnew.Fid) error {
	log.Println("clunk", fid)
	delete(s.fids, fid)
	return nil
}

func (s *simpleSession) Remove(ctx context.Context, fid p9pnew.Fid) error {
	panic("not implemented")
}

func (s *simpleSession) Walk(ctx context.Context, fid, newfid p9pnew.Fid, names ...string) ([]p9pnew.Qid, error) {
	log.Println("walk", fid, newfid, names)
	fi, ok := s.fids[fid]
	if !ok {
		log.Println("source fid not found")
		return nil, p9pnew.ErrUnknownfid
	}

	if len(names) == 0 {
		// clone the fid
		s.fids[newfid] = fi
		return []p9pnew.Qid{fi.dir.Qid}, nil
	}

	var qids []p9pnew.Qid
	for i, name := range names {
		var chfi *fidinfo

		if name == "" || name == "." {
			chfi = fi
		} else {
			var ok bool
			chfi, ok = fi.children[name]
			if !ok {
				if i == 0 {
					return nil, p9pnew.ErrUnknownfid
				}

				return qids, nil
			}
		}

		qids = append(qids, chfi.dir.Qid)
		fi = chfi
	}

	// assign the fid
	s.fids[newfid] = fi

	return qids, nil
}

func (s *simpleSession) Read(ctx context.Context, fid p9pnew.Fid, p []byte, offset int64) (n int, err error) {
	fi, ok := s.fids[fid]
	if !ok {
		return 0, p9pnew.ErrUnknownfid
	}

	if _, ok := s.opened[fid]; !ok {
		return 0, p9pnew.ErrClosed
	}

	if fi.dir.Qid.Type == p9pnew.QTDIR {
		var b bytes.Buffer
		for _, child := range fi.children {
			if err := p9pnew.EncodeDir(&b, &child.dir); err != nil {
				return 0, err
			}
		}
		n := copy(p, b.Bytes()) // just truncate for now.

		return n, nil
	}

	return copy(p, fi.data[int(offset):]), nil
}

func (s *simpleSession) Write(ctx context.Context, fid p9pnew.Fid, p []byte, offset int64) (n int, err error) {
	panic("not implemented")
}

func (s *simpleSession) Open(ctx context.Context, fid p9pnew.Fid, mode uint8) (p9pnew.Qid, uint32, error) {
	fi, ok := s.fids[fid]
	if !ok {
		return p9pnew.Qid{}, 0, p9pnew.ErrUnknownfid
	}

	s.opened[fid] = struct{}{}

	return fi.dir.Qid, 4 << 20, nil
}

func (s *simpleSession) Create(ctx context.Context, parent p9pnew.Fid, name string, perm uint32, mode uint32) (p9pnew.Qid, error) {
	panic("not implemented")
}

func (s *simpleSession) Stat(context.Context, p9pnew.Fid) (p9pnew.Dir, error) {
	panic("not implemented")
}

func (s *simpleSession) WStat(context.Context, p9pnew.Fid, p9pnew.Dir) error {
	panic("not implemented")
}

// TODO(stevvooe): The version message affects a lot of protocol behavior.
// Consider hiding it behind the implementation, letting the version get
// negotiated. The API user should still be able to query it.
func (s *simpleSession) Version(ctx context.Context, msize uint32, version string) (uint32, string, error) {
	return 4096, "9P2000", nil
}
