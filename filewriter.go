package p9p

import (
	"context"
	"io"
)

type fileWriter struct {
	session Session
	ctx     context.Context
	fid     Fid
	offset  int64
}

// NewFileWriter creates an io.Writer wrapper for writing on a 9p session file
func NewFileWriter(s Session, ctx context.Context, fid Fid, writeAt int64) io.Writer {
	return &fileWriter{s, ctx, fid, writeAt}
}

func (w *fileWriter) Write(p []byte) (n int, err error) {
	for err == nil {
		var written int
		written, err = w.session.Write(w.ctx, w.fid, p, w.offset)
		p = p[written:]
		w.offset += int64(written)
		n += written
		if len(p) == 0 {
			break
		}
	}
	return
}
