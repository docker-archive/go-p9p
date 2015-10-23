package p9pfs

import (
	"fmt"
	"log"
	"path"
	"strings"
	"time"

	"github.com/docker/pinata/v1/fs"
)

type fileSystem struct {
	session Session

	fid  Fid
	root Fid
}

var _ fs.FileSystem = &fileSystem{}

func NewFileSystem(session Session) (fs.FileSystem, error) {
	f := &fileSystem{
		session: session,
	}

	f.fid++
	f.root = f.fid
	_, err := f.session.Attach(f.root, NOFID, "cell", "/")
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (f *fileSystem) Path(mdi fs.Metadata) (string, error) {
	md, ok := mdi.(metadata)
	if !ok {
		return "", fs.ErrUnknown
	}

	return md.path, nil
}

func (f *fileSystem) Stat(p string) (fs.Metadata, error) {
	if !path.IsAbs(p) {
		return nil, fs.ErrInvalidName
	}

	tp, fid, _, err := f.pwalk(p)
	if err != nil {
		return nil, err
	}
	defer f.clunk(fid)

	if p[1:] != tp {
		return nil, fs.ErrUnknown
	}

	dir, err := f.session.Stat(fid)
	if err != nil {
		return nil, err
	}

	return metadata{path: p, Dir: dir}, nil
}

func (f *fileSystem) WStat(p string, metadata fs.Metadata) error {
	if !path.IsAbs(p) {
		return fs.ErrInvalidName
	}

	dir := Dir{
		// TODO(stevvooe): It only really makes sense to be able to change the name.
		Name: metadata.Name(),
	}

	tp, fid, _, err := f.pwalk(p)
	if err != nil {
		return err
	}
	defer f.clunk(fid)

	if tp != p[1:] {
		return fs.ErrNotExists
	}

	return f.session.WStat(fid, dir)
}

func (f *fileSystem) Open(p string, flag fs.Flag) (fs.Resource, error) {
	tp, fid, _, err := f.pwalk(p)
	if err != nil {
		return nil, err
	}

	if tp != p[1:] {
		defer f.clunk(fid)
		return nil, fs.ErrUnknown
	}

	var mode int32
	switch flag {
	case fs.OpenRead:
		mode |= OREAD
	case fs.OpenWrite:
		mode |= OWRITE
	case fs.OpenReadWrite:
		mode |= ORDWR
	}

	qid, err := f.session.Open(fid, mode)
	if err != nil {
		defer f.clunk(fid)
		return nil, err
	}

	switch qid.Type {
	case QTDIR:
		return &directory{fs: f, qid: qid, fid: fid}, nil
	case QTFILE:
		return &file{fs: f, qid: qid, fid: fid}, nil
	}

	return nil, nil
}

func (f *fileSystem) Create(p string, kind fs.Kind, flags fs.Flag) (fs.Resource, error) {
	tp, fid, _, err := f.pwalk(p)
	if err != nil {
		return nil, err
	}
	defer f.clunk(fid) // this fid gets dropped no matter what.

	if tp == p[1:] {
		return nil, fs.ErrExists
	}

	if path.Dir(p[1:]) != tp {
		// cannot create without creating parents. consider making this recursive.
		return nil, fmt.Errorf("must create parents")
	}

	var mode uint32
	switch flags {
	case fs.OpenRead:
		mode |= OREAD
	case fs.OpenWrite:
		mode |= OWRITE
	case fs.OpenReadWrite:
		mode |= ORDWR
	}

	var perm uint32
	switch kind {
	case fs.KindDirectory:
		perm = DMDIR
	case fs.KindFile:
		perm = 0
		// TODO(stevvooe): handle other types here.
	}

	if _, err := f.session.Create(fid, path.Base(p), perm, mode); err != nil {
		return nil, err
	}

	return f.Open(p, flags) // Just recursively call into Open
}

func (f *fileSystem) Remove(p string) error {
	tp, fid, _, err := f.pwalk(p)
	if err != nil {
		return err
	}
	defer f.clunk(fid)

	if tp != p[1:] {
		return fs.ErrNotExists
	}

	return f.session.Remove(fid)
}

func (f *fileSystem) Close() error {
	return nil
}

// clunk provides a helper to clunk an fid and log an error. Typically used in
// a defer to manage resource cleanup for ephemeral fids.
func (f *fileSystem) clunk(fid Fid) {
	if err := f.session.Clunk(fid); err != nil {
		log.Println("error clunking: %v", err)
	}
}

// pwalk walks the path p from the root, returning an Fid and any intermediate
// Qids, if found. The discovered path prefix of p is also returned.
func (f *fileSystem) pwalk(p string) (string, Fid, []Qid, error) {
	const stride = 16
	var (
		fid        Fid
		qids       []Qid
		components = strings.Split(strings.Trim(p, "/"), "/")
		walked     []string
	)
	for i := 0; i < len(components); i += stride {
		var block []string
		if len(components)-i > stride {
			block = components[i : i+stride]
		} else {
			block = components
		}

		cqids, err := f.session.Walk(f.root, f.fid, block...)
		if err != nil {
			return "", 0, nil, err
		}

		if fid != 0 {
			// clunk the unused fid
			if err := f.session.Clunk(fid); err != nil {
				return "", 0, nil, err
			}
		}

		fid = f.fid
		f.fid++
		qids = append(qids, cqids...)
		walked = append(walked, block...)

		// terminate loop if we don't have all the children
		if len(qids) < len(block) {
			break
		}
	}

	return path.Join(walked...), fid, qids, nil
}

type metadata struct { // kind of like Fid struct in plan9
	path string
	Dir
}

var _ fs.Metadata = metadata{}

func (d metadata) Kind() fs.Kind {
	switch d.Qid.Type {
	case QTDIR:
		return fs.KindDirectory
	default:
		return fs.KindFile
	}
}

func (d metadata) Name() string {
	return d.Dir.Name
}

func (d metadata) Perm() fs.Permissions {
	return fs.Permissions(d.Dir.Mode)
}

func (d metadata) ModTime() time.Time {
	return d.Dir.ModificationTime
}

func (d metadata) Size() int64 {
	return int64(d.Dir.Length)
}

type directory struct {
	fs     *fileSystem
	fid    Fid
	qid    Qid
	offset int
	closed bool
}

var _ fs.Directory = &directory{}

func (d *directory) Readdir(n int) ([]fs.Metadata, error) {
	// TODO(stevvooe): Plan9 wire protocol specifics for reading Dir entries.
	return nil, fmt.Errorf("not implemented")
}

func (d *directory) Reset() error {
	if d.closed {
		return fs.ErrClosed
	}
	d.offset = 0
	return nil
}

func (d *directory) Walk(rel string) ([]fs.Metadata, error) {
	// NOTE(stevvooe): Walk may not actually make sense for our filesystem
	// API. For this to work, we need to re-walk from the directory path and
	// then use that as the walk target.
	panic("walk not implemented")
}

func (d *directory) Close() error {
	if d.closed {
		return fs.ErrClosed
	}
	d.closed = true

	return d.fs.session.Clunk(d.fid)
}

type file struct {
	fs     *fileSystem
	fid    Fid
	qid    Qid
	closed bool
}

func (f *file) ReadAt(p []byte, offset int64) (n int, err error) {
	return f.fs.session.Read(f.fid, p, offset)
}

func (f *file) WriteAt(p []byte, offset int64) (n int, err error) {
	return f.fs.session.Write(f.fid, p, offset)
}

func (f *file) Close() error {
	if f.closed {
		return fs.ErrClosed
	}
	f.closed = true

	return f.fs.session.Clunk(f.fid)
}
