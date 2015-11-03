package p9pnew

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"time"

	"golang.org/x/net/context"
)

// Channel defines the operations necessary to implement a 9p message channel
// interface. Typically, message channels do no protocol processing except to
// send and receive message frames.
type Channel interface {
	// ReadFcall reads one fcall frame into the provided fcall structure. The
	// Fcall may be cleared whether there is an error or not. If the operation
	// is successful, the contents of the fcall will be populated in the
	// argument. ReadFcall cannot be called concurrently with other calls to
	// ReadFcall. This both to preserve message ordering and to allow lockless
	// buffer reusage.
	ReadFcall(ctx context.Context, fcall *Fcall) error

	// WriteFcall writes the provided fcall to the channel. WriteFcall cannot
	// be called concurrently with other calls to WriteFcall.
	WriteFcall(ctx context.Context, fcall *Fcall) error

	// SetMSize sets the maximum message size for the channel. This must never
	// be called currently with ReadFcall or WriteFcall.
	SetMSize(msize int)
}

const (
	defaultRWTimeout = 1 * time.Second // default read/write timeout if not set in context
)

// channel provides bidirectional protocol framing for 9p over net.Conn.
// Operations are not thread-safe but reads and writes may be carried out
// concurrently, supporting separate read and write loops.
//
// Lifecyle
//
// A connection, or message channel abstraction, has a lifecycle delineated by
// Tversion/Rversion request response cycles. For now, this is part of the
// channel itself but doesn't necessarily influence the channels state, except
// the msize. Visually, it might look something like this:
//
// 	[Established] -> [Version] -> [Session] -> [Version]---+
//	                     ^                                 |
// 	                     |_________________________________|
//
// The connection is established, then we negotiate a version, run a session,
// then negotiate a version and so on. For most purposes, we are likely going
// to terminate the connection after the session but we may want to support
// connection pooling. Pooling may result in possible security leaks if the
// connections are shared among contexts, since the version is negotiated at
// the start of the session. To avoid this, we can actually use a "tombstone"
// version message which clears the server's session state without starting a
// new session. The next version message would then prepare the session
// without leaking any Fid's.
type channel struct {
	conn   net.Conn
	codec  Codec
	brd    *bufio.Reader
	bwr    *bufio.Writer
	closed chan struct{}
	msize  int
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
		msize:  msize,
		rdbuf:  make([]byte, msize),
		wrbuf:  make([]byte, msize),
	}
}

// setmsize resizes the buffers for use with a separate msize. This call must
// be protected by a mutex or made before passing to other goroutines.
func (ch *channel) setmsize(msize int) {
	// NOTE(stevvooe): We cannot safely resize the buffered reader and writer.
	// Proceed assuming that original size is sufficient.

	ch.msize = msize
	if msize < len(ch.rdbuf) {
		// just change the cap
		ch.rdbuf = ch.rdbuf[:msize]
		ch.wrbuf = ch.wrbuf[:msize]
		return
	}

	ch.rdbuf = make([]byte, msize)
	ch.wrbuf = make([]byte, msize)
}

// version negiotiates the protocol version using channel, blocking until a
// response is received. The received values can be used to set msize for the
// channel or assist in client setup.
func (ch *channel) version(ctx context.Context, msize uint32, version string) (uint32, string, error) {
	req := newFcall(MessageTversion{
		MSize:   uint32(msize),
		Version: version,
	})

	if err := ch.writeFcall(ctx, req); err != nil {
		return 0, "", err
	}

	resp := new(Fcall)
	if err := ch.readFcall(ctx, resp); err != nil {
		return 0, "", err
	}

	mv, ok := resp.Message.(*MessageRversion)
	if !ok {
		return 0, "", fmt.Errorf("invalid rpc response for version message: %v", resp)
	}

	return mv.MSize, mv.Version, nil
}

// negotiate blocks until a version message is received or a timeout occurs.
// The msize for the tranport will be set from the negotiation. If negotiate
// returns nil, a server may proceed with the connection.
//
// In the future, it might be better to handle the version messages in a
// separate object that manages the session. Each set of version requests
// effectively "reset" a connection, meaning all fids get clunked and all
// outstanding IO is aborted. This is probably slightly racy, in practice with
// a misbehaved client. The main issue is that we cannot tell which session
// messages belong to.
func (ch *channel) negotiate(ctx context.Context, version string) error {
	// wait for the version message over the transport.
	req := new(Fcall)
	if err := ch.readFcall(ctx, req); err != nil {
		return err
	}

	mv, ok := req.Message.(*MessageTversion)
	if !ok {
		return fmt.Errorf("expected version message: %v", mv)
	}

	respmsg := MessageRversion{
		Version: version,
	}

	if mv.Version != version {
		// TODO(stevvooe): Not the best place to do version handling. We need
		// to have a way to pass supported versions into this method then have
		// it return the actual version. For now, respond with unknown for
		// anything that doesn't match the provided version string.
		respmsg.Version = "unknown"
	}

	if int(mv.MSize) < ch.msize {
		// if the server msize is too large, use the client's suggested msize.
		ch.setmsize(int(mv.MSize))
		respmsg.MSize = mv.MSize
	} else {
		respmsg.MSize = uint32(ch.msize)
	}

	resp := newFcall(respmsg)
	if err := ch.writeFcall(ctx, resp); err != nil {
		return err
	}

	if respmsg.Version == "unknown" {
		return fmt.Errorf("bad version negotiation")
	}

	return nil
}

// ReadFcall reads the next message from the channel into fcall.
func (ch *channel) readFcall(ctx context.Context, fcall *Fcall) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch.closed:
		return ErrClosed
	default:
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultRWTimeout)
	}

	if err := ch.conn.SetReadDeadline(deadline); err != nil {
		log.Printf("transport: error setting read deadline on %v: %v", ch.conn.RemoteAddr(), err)
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

	// clear out the fcall
	*fcall = Fcall{}
	if err := ch.codec.Unmarshal(ch.rdbuf[:n], fcall); err != nil {
		return err
	}
	log.Println("channel: recv", fcall)
	return nil
}

func (ch *channel) writeFcall(ctx context.Context, fcall *Fcall) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch.closed:
		return ErrClosed
	default:
	}
	log.Println("channel: send", fcall)

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultRWTimeout)
	}

	if err := ch.conn.SetWriteDeadline(deadline); err != nil {
		log.Printf("transport: error setting read deadline on %v: %v", ch.conn.RemoteAddr(), err)
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
