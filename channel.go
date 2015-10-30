package p9pnew

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"

	"golang.org/x/net/context"
)

// channel provides bidirectional protocol framing for 9p over net.Conn.
// Operations are not thread-safe but reads and writes may be carried out
// concurrently, supporting separate read and write loops.
type channel struct {
	conn   net.Conn
	codec  Codec
	brd    *bufio.Reader
	bwr    *bufio.Writer
	closed chan struct{}
	rdbuf  []byte
	wrbuf  []byte
}

func newChannel(conn net.Conn, codec Codec, msize int) *channel {
	return &channel{
		conn:   conn,
		codec:  codec,
		brd:    bufio.NewReaderSize(conn, msize), // msize may not be optimal buffer size
		bwr:    bufio.NewWriterSize(conn, msize),
		closed: make(chan struct{}),
		rdbuf:  make([]byte, msize),
		wrbuf:  make([]byte, msize),
	}
}

// setmsize resizes the buffers for use with a separate msize. This call must
// be protected by a mutex or made before passing to other goroutines.
func (ch *channel) setmsize(msize int) {
	// NOTE(stevvooe): We cannot safely resize the buffered reader and writer.
	// Proceed assuming that original size is sufficient.

	if msize < len(ch.rdbuf) {
		// just change the cap
		ch.rdbuf = ch.rdbuf[:msize]
		ch.wrbuf = ch.wrbuf[:msize]
		return
	}

	ch.rdbuf = make([]byte, msize)
	ch.wrbuf = make([]byte, msize)
}

// ReadFcall reads the next message from the channel into fcall.
func (ch *channel) readFcall(ctx context.Context, fcall *Fcall) error {
	select {
	case <-ch.closed:
		return ErrClosed
	default:
	}
	log.Println("channel: readFcall", fcall)

	if deadline, ok := ctx.Deadline(); ok {
		if err := ch.conn.SetReadDeadline(deadline); err != nil {
			log.Printf("transport: error setting read deadline on %v: %v", ch.conn.RemoteAddr(), err)
		}
	}

	n, err := readmsg(ch.brd, ch.rdbuf)
	if err != nil {
		// TODO(stevvooe): There may be more we can do here to detect partial
		// reads. For now, we just propagate the error untouched.
		return err
	}

	if n > len(ch.rdbuf) {
		// TODO(stevvooe): Make this error detectable and respond with error
		// message.
		return fmt.Errorf("message large than buffer:", n)
	}

	return ch.codec.Unmarshal(ch.rdbuf[:n], fcall)
}

func (ch *channel) writeFcall(ctx context.Context, fcall *Fcall) error {
	select {
	case <-ch.closed:
		return ErrClosed
	default:
	}
	log.Println("channel: writeFcall", fcall)

	if deadline, ok := ctx.Deadline(); ok {
		if err := ch.conn.SetWriteDeadline(deadline); err != nil {
			log.Printf("transport: error setting read deadline on %v: %v", ch.conn.RemoteAddr(), err)
		}
	}

	n, err := ch.codec.Marshal(ch.wrbuf, fcall)
	if err != nil {
		return err
	}

	p := ch.wrbuf[:n]

	if err := sendmsg(ch.bwr, p); err != nil {
		return err
	}

	return ch.bwr.Flush()
}

// readmsg reads a 9p message into p from rd, ensuring that all bytes are
// consumed from the size header. If the size header indicates the message is
// larger than p, the entire message will be discarded, leaving a truncated
// portion in p. Any error should be treated as a framing error unless n is
// zero. The caller must check that n is less than or equal to len(p) to
// ensure that a valid message has been read.
func readmsg(rd io.Reader, p []byte) (n int, err error) {
	var msize uint32

	if err := binary.Read(rd, binary.LittleEndian, &msize); err != nil {
		return 0, err
	}

	n += binary.Size(msize)
	mbody := int(msize) - 4

	if mbody < len(p) {
		p = p[:mbody]
	}

	np, err := io.ReadFull(rd, p)
	if err != nil {
		return np + n, err
	}
	n += np

	if mbody > len(p) {
		// message has been read up to len(p) but we must consume the entire
		// message. This is an error condition but is non-fatal if we can
		// consume msize bytes.
		nn, err := io.CopyN(ioutil.Discard, rd, int64(mbody-len(p)))
		n += int(nn)
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

// sendmsg writes a message of len(p) to wr with a 9p size header. All errors
// should be considered terminal.
func sendmsg(wr io.Writer, p []byte) error {
	size := uint32(len(p) + 4) // message size plus 4-bytes for size.
	if err := binary.Write(wr, binary.LittleEndian, size); err != nil {
		return nil
	}

	// This assume partial writes to wr aren't possible. Not sure if this
	// valid. Matters during timeout retries.
	if n, err := wr.Write(p); err != nil {
		return err
	} else if n < len(p) {
		return io.ErrShortWrite
	}

	return nil
}
