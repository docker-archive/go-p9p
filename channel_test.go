package p9p

import (
	"bytes"
	"context"
	"encoding/binary"
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

func TestWriteOverflow(t *testing.T) {
	const testMsize = 500
	conn := writeConnMock{}
	channel := newChannel(&conn, NewCodec(), testMsize)
	writeRequest := MessageTwrite{1, 0, make([]byte, 2*testMsize)}
	ctx := context.Background()
	channel.WriteFcall(ctx, newFcall(Tag(1), writeRequest))
	reader := bytes.NewReader(conn.data)
	var writtenSize uint32
	err := binary.Read(reader, binary.LittleEndian, &writtenSize)
	if err != nil {
		t.Errorf("error reading result: %v", err)
	}
	// as there is an overflow, written size should have been truncated such that the message size is equal to channel's msize
	if int(writtenSize) != testMsize {
		t.Errorf("message should have been truncated to size %v. written message has size %v", testMsize, writtenSize)
	}
}
