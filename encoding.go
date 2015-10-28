package p9pnew

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"reflect"
	"time"
)

// EncodeDir is just a helper for encoding directories until we export the
// encoder and decoder.
func EncodeDir(wr io.Writer, d *Dir) error {
	enc := &encoder{wr}

	return enc.encode(d)
}

// DecodeDir is just a helper for decoding directories until we export the
// encoder and decoder.
func DecodeDir(rd io.Reader, d *Dir) error {
	dec := &decoder{rd}
	return dec.decode(d)
}

// NOTE(stevvooe): This file covers 9p encoding and decoding (despite just
// being called encoding).

type encoder struct {
	wr io.Writer
}

func (e *encoder) encode(vs ...interface{}) error {
	for _, v := range vs {
		switch v := v.(type) {
		case *[]string:
			if err := e.encode(*v); err != nil {
				return err
			}
		case []string:
			if err := e.encode(uint16(len(v))); err != nil {
				return err
			}

			var elements []interface{}
			for _, e := range v {
				elements = append(elements, e)
			}

			if err := e.encode(elements...); err != nil {
				return err
			}
		case *[]byte:
			if err := e.encode(uint16(len(*v))); err != nil {
				return err
			}

			if err := e.encode(*v); err != nil {
				return err
			}
		case *string:
			if err := e.encode(*v); err != nil {
				return err
			}
		case string:
			// implement string[s] encoding
			if err := binary.Write(e.wr, binary.LittleEndian, uint16(len(v))); err != nil {
				return err
			}

			_, err := io.WriteString(e.wr, v)
			if err != nil {
				return err
			}
		case Message, *Qid, *Dir:
			// BUG(stevvooe): The encoding for Dir is incorrect. Under certain
			// cases, we need to include size field and in other cases, such
			// as Twstat, we need the size twice. See bugs in
			// http://man.cat-v.org/plan_9/5/stat to make sense of this.

			elements, err := fields9p(v)
			if err != nil {
				return err
			}

			if err := e.encode(elements...); err != nil {
				return err
			}
		case *[]Qid:
			if err := e.encode(*v); err != nil {
				return err
			}
		case []Qid:
			if err := e.encode(uint16(len(v))); err != nil {
				return err
			}

			elements := make([]interface{}, len(v))
			for i := range v {
				elements[i] = &v[i]
			}

			if err := e.encode(elements...); err != nil {
				return err
			}
		case time.Time:
			if err := e.encode(uint32(v.Unix())); err != nil {
				return err
			}
		case *time.Time:
			if err := e.encode(*v); err != nil {
				return err
			}
		case Fcall:
			if err := e.encode(&v); err != nil {
				return err
			}
		case *Fcall:
			if err := e.encode(size9p(v), v.Type, v.Tag, v.Message); err != nil {
				return err
			}
		default:
			if err := binary.Write(e.wr, binary.LittleEndian, v); err != nil {
				return err
			}
		}
	}

	return nil
}

type decoder struct {
	rd io.Reader
}

// read9p extracts values from rd and unmarshals them to the targets of vs.
func (d *decoder) decode(vs ...interface{}) error {
	for _, v := range vs {
		switch v := v.(type) {
		case *string:
			var ll uint16

			// implement string[s] encoding
			if err := d.decode(&ll); err != nil {
				return err
			}

			b := make([]byte, ll)

			n, err := io.ReadFull(d.rd, b)
			if err != nil {
				log.Println("readfull failed:", err)
				return err
			}

			if n != int(ll) {
				return fmt.Errorf("unexpected string length")
			}

			*v = string(b)
		case *[]string:
			var ll uint16

			if err := d.decode(&ll); err != nil {
				return err
			}

			elements := make([]interface{}, int(ll))
			*v = make([]string, int(ll))
			for i := range elements {
				elements[i] = &(*v)[i]
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		case *[]byte:
			var ll uint16

			if err := d.decode(&ll); err != nil {
				return err
			}

			*v = make([]byte, int(ll))

			if err := binary.Read(d.rd, binary.LittleEndian, v); err != nil {
				return err
			}
		case *Fcall:
			var size uint32
			if err := d.decode(&size, &v.Type, &v.Tag); err != nil {
				return err
			}

			var err error
			v.Message, err = newMessage(v.Type)
			if err != nil {
				return err
			}

			if err := d.decode(v.Message); err != nil {
				return err
			}
		case *[]Qid:
			var ll uint16

			if err := d.decode(&ll); err != nil {
				return err
			}

			elements := make([]interface{}, int(ll))
			*v = make([]Qid, int(ll))
			for i := range elements {
				elements[i] = &(*v)[i]
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		case *time.Time:
			var epoch uint32
			if err := d.decode(&epoch); err != nil {
				return err
			}

			*v = time.Unix(int64(epoch), 0).UTC()
		case Message, *Qid, *Dir:
			elements, err := fields9p(v)
			if err != nil {
				return err
			}

			if err := d.decode(elements...); err != nil {
				return err
			}
		default:
			if err := binary.Read(d.rd, binary.LittleEndian, v); err != nil {
				return err
			}
		}
	}

	return nil
}

// size9p calculates the projected size of the values in vs when encoded into
// 9p binary protocol. If an element or elements are not valid for 9p encoded,
// the value 0 will be used for the size. The error will be detected when
// encoding.
func size9p(vs ...interface{}) uint32 {
	var s uint32
	for _, v := range vs {
		if v == nil {
			continue
		}

		switch v := v.(type) {
		case *string:
			s += uint32(binary.Size(uint16(0)) + len(*v))
		case string:
			s += uint32(binary.Size(uint16(0)) + len(v))
		case *[]string:
			s += size9p(*v)
		case []string:
			s += size9p(uint16(0))
			elements := make([]interface{}, len(v))
			for i := range elements {
				elements[i] = v[i]
			}

			s += size9p(elements...)
		case *[]byte:
			s += size9p(uint16(0), *v)
		case *[]Qid:
			s += size9p(*v)
		case []Qid:
			s += size9p(uint16(0))
			elements := make([]interface{}, len(v))
			for i := range elements {
				elements[i] = &v[i]
			}
			s += size9p(elements...)
		case time.Time, *time.Time:
			s += size9p(uint32(0))
		case Message, *Qid, *Dir:
			// walk the fields of the message to get the total size. we just
			// use the field order from the message struct. We may add tag
			// ignoring if needed.
			elements, err := fields9p(v)
			if err != nil {
				// BUG(stevvooe): The options here are to return 0, panic or
				// make this return an error. Ideally, we make it safe to
				// return 0 and have the rest of the package do the right
				// thing. For now, we do this, but may want to panic until
				// things are stable.
				panic(err)
			}

			s += size9p(elements...)
		case Fcall:
			s += size9p(uint32(0), v.Type, v.Tag, v.Message)
		case *Fcall:
			s += size9p(*v)
		default:
			s += uint32(binary.Size(v))
		}
	}

	return s
}

// fields9p lists the settable fields from a struct type for reading and
// writing. We are using a lot of reflection here for fairly static
// serialization but we can replace this in the future with generated code if
// performance is an issue.
func fields9p(v interface{}) ([]interface{}, error) {
	rv := reflect.Indirect(reflect.ValueOf(v))

	if rv.Kind() != reflect.Struct {
		panic("asdf")
		return nil, fmt.Errorf("cannot extract fields from non-struct: %v", rv)
	}

	var elements []interface{}
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)

		if !f.CanInterface() {
			return nil, fmt.Errorf("can't interface: %v", f)
		}

		if !f.CanSet() {
			panic("asdf")
			return nil, fmt.Errorf("cannot set %v", f)
		}

		if f.CanAddr() {
			f = f.Addr()
		}

		elements = append(elements, f.Interface())
	}

	return elements, nil
}
