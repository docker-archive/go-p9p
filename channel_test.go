package p9p

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

type fakeAddr struct{}

func (a fakeAddr) Network() string {
	return ""
}
func (a fakeAddr) String() string {
	return "fake address"
}

type writeConnMock struct {
	data []byte
}

func (c *writeConnMock) Read(b []byte) (n int, err error) {
	return 0, errors.New("not implemented")
}

func (c *writeConnMock) Write(b []byte) (n int, err error) {
	c.data = append(c.data, b...)

	n = len(b)
	return n, nil
}

func (c *writeConnMock) Close() error {
	return nil
}

func (c *writeConnMock) LocalAddr() net.Addr {
	return fakeAddr{}
}

func (c *writeConnMock) RemoteAddr() net.Addr {
	return fakeAddr{}
}

func (c *writeConnMock) SetDeadline(t time.Time) error {
	return nil
}

func (c *writeConnMock) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *writeConnMock) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestReadOverflow(t *testing.T) {
	const testMsize = 500
	conn := writeConnMock{}
	channel := newChannel(&conn, NewCodec(), testMsize)
	readRequest := MessageTread{Fid: 1, Offset: 0, Count: testMsize}
	ctx := context.Background()
	fcall := newFcall(Tag(1), readRequest)
	channel.WriteFcall(ctx, fcall)
	// writing the fCall should check for read overflows. so fCall.Message should now have the correct size for a truncated read according to channel's msize
	rewrittenRequest := fcall.Message.(MessageTread)
	// according to http://man.cat-v.org/plan_9/5/read, read reply has the following layout:
	// size[4] Rread[1] tag[2] count[4] data[count]
	// so max read size should be `msize-4-1-2-4`
	expectedSize := testMsize - 4 - 1 - 2 - 4
	if int(rewrittenRequest.Count) != expectedSize {
		t.Errorf("expected truncated read count: %v. got %v", expectedSize, rewrittenRequest.Count)
	}

}
