package p9pnew

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"encoding"
)

type FcallType uint8

const (
	Tversion FcallType = iota + 100
	Rversion
	Tauth
	Rauth
	Tattach
	Rattach
	Terror
	Rerror
	Tflush
	Rflush
	Twalk
	Rwalk
	Topen
	Ropen
	Tcreate
	Rcreate
	Tread
	Rread
	Twrite
	Rwrite
	Tclunk
	Rclunk
	Tremove
	Rremove
	Tstat
	Rstat
	Twstat
	Rwstat
	Tmax
)

type Fcall struct {
	Type    Type
	Tag     Tag
	Message Message
}

const (
	fcallHeaderSize = 4 /*size*/ + 1 /*type*/
)

func (fc *Fcall) Size() int {
	return fcallHeaderSize + fc.Message.Size()
}

func (fc *Fcall) MarshalBinary() ([]byte, error) {
	mp, err := fc.Message.MarshalBinary()
	if err != nil {
		return nil, err
	}

	b := bytes.NewBuffer(make([]byte, 0, fc.Size()))
	if err := write9p(b, fc.Size(), fc.Tag, mp); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (fc *Fcall) UnmarshalBinary(p []data) error {
	var (
		r = bytes.NewReader(p)
	)

	if err := read9p(r, &fc.Type, &fc.Tag); err != nil {
		return err
	}

	switch fc.Type {
	case Tversion, Rversion:
		fc.Message = &MessageVersion{}
	case Tauth:

	case Rauth:

	case Tattach:

	case Rattach:

	case Terror:

	case Rerror:

	case Tflush:

	case Rflush:

	case Twalk:

	case Rwalk:

	case Topen:

	case Ropen:

	case Tcreate:

	case Rcreate:

	case Tread:

	case Rread:

	case Twrite:

	case Rwrite:

	case Tclunk:

	case Rclunk:

	case Tremove:

	case Rremove:

	case Tstat:

	case Rstat:

	case Twstat:

	case Rwstat:

	}

	return fc.Message.UnmarshalBinary(p[len(p)-r.Len():])
}

type Message interface {
	Size() int
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// MessageVersion encodes the message body for Tversion and Rversion RPC
// calls. The body is identical in both directions.
type MessageVersion struct {
	MSize   uint32
	Version string
}

func (mv MessageVersion) Size() int {
	return 4 + 2 + len(mv.Version)
}

func (mv MessageVersion) MarshalBinary() ([]byte, error) {
	b := bytes.NewBuffer(make([]byte, 0, mv.Size()))

	if err := write9p(b, mv.MSize, mv.Version); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// write9p implements serialization for base types.
func write9p(w io.Writer, vs ...interface{}) error {
	for _, v := range vs {
		switch v := v.(type) {
		case string:
			// implement string[s] encoding
			if err := binary.Write(w, binary.LittleEndian, uint16(len(v))); err != nil {
				return err
			}

			_, err := io.WriteString(w, s)
			if err != nil {

				return err
			}
		case *Fcall:
			if err := write9p(w, v.Size()); err != nil {
				return err
			}
			p, err := v.MarshalBinary()
			if err != nil {
				return err
			}

			n, err := w.Write(p)
			if err != nil {
				return err
			}

			if n != len(p) {
				return io.ErrShortWrite
			}

			return nil
		default:
			if err := binary.Write(w, binary.LittleEndian, v); err != nil {
				return err
			}
		}
	}

	return nil
}

// read9p extracts values from rd and unmarshals them to the targets of vs.
func read9p(rd io.Reader, vs ...interface{}) error {
	for _, v := range vs {
		switch v := v.(type) {
		case *string:
			var ll uint16

			// implement string[s] encoding
			if err := binary.Read(r, binary.LittleEndian, &ll); err != nil {
				return err
			}

			b := make([]byte, ll)

			n, err := io.ReadFull(b)
			if err != nil {
				return err
			}

			if n != int(ll) {
				return fmt.Errorf("unexpected string length")
			}

			*v = string(b)
		case *Fcall:
			var size uint32
			if err := read9p(buffered, &size); err != nil {
				return err
			}

			p := make([]byte, size)
			n, err := io.ReadFull(p)
			if err != nil {
				return err
			}

			if n != size {
				return fmt.Errorf("error reading fcall: short read")
			}

			return v.UnmarshalBinary(p)
		default:
			if err := binary.Read(r, binary.LittleEndian, v); err != nil {
				return err
			}
		}
	}
}
