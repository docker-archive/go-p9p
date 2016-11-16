package p9p

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// TestTwriteOverflow ensures that a Twrite message will have the data field
// truncated if the msize would be exceeded.
func TestTwriteOverflow(t *testing.T) {
	const (
		msize = 512

		// size[4] Twrite tag[2] fid[4] offset[8] count[4] data[count] | count = 0
		overhead = 4 + 1 + 2 + 4 + 8 + 4
	)

	var (
		ctx  = context.Background()
		conn = &mockConn{}
		ch   = NewChannel(conn, msize)
	)

	for _, testcase := range []struct {
		name     string
		overflow int // amount to overflow the message by.
	}{
		{
			name:     "BoundedOverflow",
			overflow: msize / 2,
		},
		{
			name:     "LargeOverflow",
			overflow: msize * 3,
		},
		{
			name:     "HeaderOverflow",
			overflow: overhead,
		},
		{
			name:     "HeaderOffsetOverflow",
			overflow: overhead - 1,
		},
		{
			name:     "OverflowByOne",
			overflow: 1,
		},
	} {

		t.Run(testcase.name, func(t *testing.T) {
			var (
				fcall = overflowMessage(ch.(*channel).codec, msize, testcase.overflow)
				data  = fcall.Message.(MessageTwrite).Data
				size  uint32
			)

			t.Logf("overflow: %v, len(data): %v, expected overflow: %v", testcase.overflow, len(data), overhead+len(data)-msize)
			conn.buf.Reset()
			if err := ch.WriteFcall(ctx, fcall); err != nil {
				t.Fatal(err)
			}

			if err := binary.Read(bytes.NewReader(conn.buf.Bytes()), binary.LittleEndian, &size); err != nil {
				t.Fatal(err)
			}

			if size != msize {
				t.Fatalf("should have truncated size header: %d != %d", size, msize)
			}

			if conn.buf.Len() != msize {
				t.Fatalf("should have truncated message: conn.buf.Len(%v) != msize(%v)", conn.buf.Len(), msize)
			}
		})
	}

}

// TestWriteOverflowError ensures that we return an error in cases when there
// will certainly be an overflow and it cannot be resolved.
func TestWriteOverflowError(t *testing.T) {
	const (
		msize         = 4
		overflowMSize = msize + 1
	)

	var (
		ctx   = context.Background()
		conn  = &mockConn{}
		ch    = NewChannel(conn, msize)
		data  = bytes.Repeat([]byte{'A'}, 4)
		fcall = newFcall(1, MessageTwrite{
			Data: data,
		})
		messageSize = 4 + ch.(*channel).codec.Size(fcall)
	)

	err := ch.WriteFcall(ctx, fcall)
	if err == nil {
		t.Fatal("error expected when overflowing message")
	}

	if Overflow(err) != messageSize-msize {
		t.Fatalf("overflow should reflect messageSize and msize, %d != %d", Overflow(err), messageSize-msize)
	}
}

// TestReadOverflow ensures that messages coming over a network connection do
// not overflow the msize. Invalid messages will cause `ReadFcall` to return an
// Overflow error.
func TestReadFcallOverflow(t *testing.T) {
	const (
		msize = 256
	)

	var (
		ctx   = context.Background()
		conn  = &mockConn{}
		ch    = NewChannel(conn, msize)
		codec = ch.(*channel).codec
	)

	for _, testcase := range []struct {
		name     string
		overflow int
	}{
		{
			name:     "OverflowByOne",
			overflow: 1,
		},
		{
			name:     "HeaderOverflow",
			overflow: overheadMessage(codec, MessageTwrite{}),
		},
		{
			name:     "HeaderOffsetOverflow",
			overflow: overheadMessage(codec, MessageTwrite{}) - 1,
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			fcall := overflowMessage(codec, msize, testcase.overflow)

			// prepare the raw message
			p, err := ch.(*channel).codec.Marshal(fcall)
			if err != nil {
				t.Fatal(err)
			}

			// "send" the message into the buffer
			// this message is crafted to overflow the read buffer.
			if err := sendmsg(&conn.buf, p); err != nil {
				t.Fatal(err)
			}

			var incoming Fcall
			err = ch.ReadFcall(ctx, &incoming)
			if err == nil {
				t.Fatal("expected error on fcall")
			}

			// sanity check to ensure our test code has the right overflow
			if testcase.overflow != ch.(*channel).msgmsize(fcall)-msize {
				t.Fatalf("overflow calculation incorrect: %v != %v", testcase.overflow, ch.(*channel).msgmsize(fcall)-msize)
			}

			if Overflow(err) != testcase.overflow {
				t.Fatalf("unexpected overflow on error: %v !=%v", Overflow(err), testcase.overflow)
			}
		})
	}
}

// TestTreadRewrite ensures that messages that whose response would overflow
// the msize will have be adjusted before sending.
func TestTreadRewrite(t *testing.T) {
	const (
		msize         = 256
		overflowMSize = msize + 1
	)

	var (
		ctx  = context.Background()
		conn = &mockConn{}
		ch   = NewChannel(conn, msize)
		buf  = make([]byte, overflowMSize)
		// data  = bytes.Repeat([]byte{'A'}, overflowMSize)
		fcall = newFcall(1, MessageTread{
			Count: overflowMSize,
		})
		responseMSize = ch.(*channel).msgmsize(newFcall(1, MessageRread{
			Data: buf,
		}))
	)

	if err := ch.WriteFcall(ctx, fcall); err != nil {
		t.Fatal(err)
	}

	// just read the message off the buffer
	n, err := readmsg(&conn.buf, buf)
	if err != nil {
		t.Fatal(err)
	}

	*fcall = Fcall{}
	if err := ch.(*channel).codec.Unmarshal(buf[:n], fcall); err != nil {
		t.Fatal(err)
	}

	tread, ok := fcall.Message.(MessageTread)
	if !ok {
		t.Fatalf("unexpected message: %v", fcall)
	}

	if tread.Count != overflowMSize-(uint32(responseMSize)-msize) {
		t.Fatalf("count not rewritten: %v != %v", tread.Count, overflowMSize-(uint32(responseMSize)-msize))
	}
}

type mockConn struct {
	net.Conn
	buf bytes.Buffer
}

func (m mockConn) SetWriteDeadline(t time.Time) error { return nil }
func (m mockConn) SetReadDeadline(t time.Time) error  { return nil }

func (m *mockConn) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func (m *mockConn) Read(p []byte) (int, error) {
	return m.buf.Read(p)
}

func overheadMessage(codec Codec, msg Message) int {
	return 4 + codec.Size(newFcall(1, msg))
}

// overflowMessage returns message that overflows the msize by overflow bytes,
// returning the message size and the fcall.
func overflowMessage(codec Codec, msize, overflow int) *Fcall {
	var (
		overhead = overheadMessage(codec, MessageTwrite{})
		data     = bytes.Repeat([]byte{'A'}, (msize-overhead)+overflow)
		fcall    = newFcall(1, MessageTwrite{
			Data: data,
		})
	)

	return fcall
}
