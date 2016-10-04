package p9p

import (
	"context"
	"io"
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
	n, err = r.session.Read(r.ctx, r.fid, p, r.offset)
	r.offset += int64(n)
	// additional error handling for compliying with io.Reader
	if n == 0 && (err == nil || err == io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	if n < len(p) && err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return
}
