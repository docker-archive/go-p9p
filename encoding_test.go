package p9pnew

import (
	"bytes"
	"reflect"
	"testing"
	"time"
)

func TestEncodeDecode(t *testing.T) {
	for _, testcase := range []struct {
		description string
		target      interface{}
		marshaled   []byte
	}{
		{
			description: "string",
			target:      "asdf",
			marshaled:   []byte{0x4, 0x0, 0x61, 0x73, 0x64, 0x66},
		},
		{
			description: "[]string",
			target:      []string{"asdf", "qwer", "zxcv"},
			marshaled: []byte{
				0x3, 0x0, // len(target)
				0x4, 0x0, 0x61, 0x73, 0x64, 0x66,
				0x4, 0x0, 0x71, 0x77, 0x65, 0x72,
				0x4, 0x0, 0x7a, 0x78, 0x63, 0x76},
		},
		// Dir
		// Qid
		{
			description: "Tversion fcall",
			target: &Fcall{
				Type: Tversion,
				Tag:  2255,
				Message: &MessageTversion{
					MSize:   uint32(1024),
					Version: "9PTEST",
				},
			},
			marshaled: []byte{
				0x13, 0x0, 0x0, 0x0,
				0x64, 0xcf, 0x8, 0x0, 0x4, 0x0, 0x0,
				0x6, 0x0, 0x39, 0x50, 0x54, 0x45, 0x53, 0x54},
		},
		{
			description: "Rversion fcall",
			target: &Fcall{
				Type: Rversion,
				Tag:  2255,
				Message: &MessageRversion{
					MSize:   uint32(1024),
					Version: "9PTEST",
				},
			},
			marshaled: []byte{
				0x13, 0x0, 0x0, 0x0,
				0x65, 0xcf, 0x8, 0x0, 0x4, 0x0, 0x0,
				0x6, 0x0, 0x39, 0x50, 0x54, 0x45, 0x53, 0x54},
		},
		{
			description: "Twalk fcall",
			target: &Fcall{
				Type: Twalk,
				Tag:  5666,
				Message: &MessageTwalk{
					Fid:    1010,
					Newfid: 1011,
					Wnames: []string{"a", "b", "c"},
				},
			},
			marshaled: []byte{
				0x1a, 0x0, 0x0, 0x0,
				0x6e, 0x22, 0x16, 0xf2, 0x3, 0x0, 0x0, 0xf3, 0x3, 0x0, 0x0,
				0x3, 0x0, // len(wnames)
				0x1, 0x0, 0x61, // "a"
				0x1, 0x0, 0x62, // "b"
				0x1, 0x0, 0x63}, // "c"
		},
		{
			description: "Rwalk call",
			target: &Fcall{
				Type: Rwalk,
				Tag:  5556,
				Message: &MessageRwalk{
					Qids: []Qid{
						Qid{
							Type:    QTDIR,
							Path:    1111,
							Version: 11112,
						},
						Qid{Type: QTFILE,
							Version: 1112,
							Path:    11114},
					},
				},
			},
			marshaled: []byte{
				0x23, 0x0, 0x0, 0x0,
				0x6f, 0xb4, 0x15,
				0x2, 0x0,
				0x80, 0x68, 0x2b, 0x0, 0x0, 0x57, 0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x58, 0x4, 0x0, 0x0, 0x6a, 0x2b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		},
		{
			description: "Rread fcall",
			target: &Fcall{
				Type: Rread,
				Tag:  5556,
				Message: &MessageRread{
					Data: []byte("a lot of byte data"),
				},
			},
			marshaled: []byte{
				0x1b, 0x0, 0x0, 0x0,
				0x75, 0xb4, 0x15,
				0x12, 0x0,
				0x61, 0x20, 0x6c, 0x6f, 0x74, 0x20, 0x6f, 0x66, 0x20, 0x62, 0x79, 0x74, 0x65, 0x20, 0x64, 0x61, 0x74, 0x61},
		},
		{
			description: "",
			target: &Fcall{
				Type: Rstat,
				Tag:  5556,
				Message: &MessageRstat{
					Stat: Dir{
						Type: ^uint16(0),
						Dev:  ^uint32(0),
						Qid: Qid{
							Type:    QTDIR,
							Version: ^uint32(0),
							Path:    ^uint64(0),
						},
						Mode:       DMDIR | DMREAD,
						AccessTime: time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
						ModTime:    time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
						Length:     ^uint64(0),
						Name:       "somedir",
						UID:        "uid",
						GID:        "gid",
						MUID:       "muid",
					},
				},
			},
			marshaled: []byte{
				0x47, 0x0, 0x0, 0x0,
				0x7d, 0xb4, 0x15,
				// 0x40, 0x0, // TODO(stevvooe): Include Dir size. Not straightforward.
				0xff, 0xff, // type
				0xff, 0xff, 0xff, 0xff, // dev
				0x80, 0xff, 0xff, 0xff, 0xff, // qid.type, qid.version
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // qid.path
				0x4, 0x0, 0x0, 0x80, // mode
				0x25, 0x98, 0xb8, 0x43, // atime
				0x25, 0x98, 0xb8, 0x43, // mtime
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // length
				0x7, 0x0, 0x73, 0x6f, 0x6d, 0x65, 0x64, 0x69, 0x72,
				0x3, 0x0, 0x75, 0x69, 0x64, // uid
				0x3, 0x0, 0x67, 0x69, 0x64, // gid
				0x4, 0x0, 0x6d, 0x75, 0x69, 0x64}, // muid
		},
	} {
		t.Logf("target under test: %v", testcase.target)
		fatalf := func(format string, args ...interface{}) {
			t.Fatalf(testcase.description+": "+format, args...)
		}

		t.Logf("expecting message of %v bytes", len(testcase.marshaled))

		var b bytes.Buffer

		enc := &encoder{&b}
		dec := &decoder{&b}

		if err := enc.encode(testcase.target); err != nil {
			fatalf("error writing fcall: %v", err)
		}

		if !bytes.Equal(b.Bytes(), testcase.marshaled) {
			fatalf("unexpected bytes for fcall: \n%#v != \n%#v", b.Bytes(), testcase.marshaled)
		}

		// check that size9p is working correctly
		if int(size9p(testcase.target)) != len(testcase.marshaled) {
			fatalf("size not correct: %v != %v", int(size9p(testcase.target)), len(testcase.marshaled))
		}

		var v interface{}
		targetType := reflect.TypeOf(testcase.target)

		if targetType.Kind() == reflect.Ptr {
			v = reflect.New(targetType.Elem()).Interface()
		} else {
			v = reflect.New(targetType).Interface()
		}

		if err := dec.decode(v); err != nil {
			fatalf("error reading: %v", err)
		}

		if targetType.Kind() != reflect.Ptr {
			v = reflect.Indirect(reflect.ValueOf(v)).Interface()
		}

		if !reflect.DeepEqual(v, testcase.target) {
			fatalf("not equal: %v != %v", v, testcase.target)
		}

		t.Logf("%#v", v)

	}
}
