package p9p

import (
	"io"

	"golang.org/x/net/context"
)

type fileReader struct {
	session Session
	ctx     context.Context
	fid     Fid
	offset  int64
}

// NewFileReader creates an io.Reader wrapper for reading a 9p session file
func NewFileReader(s Session, ctx context.Context, fid Fid, readAt int64) io.Reader {
	return &fileReader{s, ctx, fid, readAt}
}

func (r *fileReader) Read(p []byte) (n int, err error) {
	if len(p) > r.session.MaxReadSize() {
		p = p[:r.session.MaxReadSize()]
	}
	n, err = r.session.Read(r.ctx, r.fid, p, r.offset)
	r.offset += int64(n)
	// additional error handling for compliying with io.Reader
	if n < len(p) && err == nil {
		err = io.EOF
	}
	return
}
