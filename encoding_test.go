package p9p

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestEncodeDecode(t *testing.T) {
	codec := NewCodec()
	for _, testcase := range []struct {
		description string
		target      interface{}
		marshaled   []byte
	}{
		{
			description: "uint8",
			target:      uint8('U'),
			marshaled:   []byte{0x55},
		},
		{
			description: "uint16",
			target:      uint16(0x5544),
			marshaled:   []byte{0x44, 0x55},
		},
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
		{
			description: "Qid",
			target: Qid{
				Type:    QTDIR,
				Version: 0x10203040,
				Path:    0x1020304050607080},
			marshaled: []byte{
				byte(QTDIR),            // qtype
				0x40, 0x30, 0x20, 0x10, // version
				0x80, 0x70, 0x60, 0x50, 0x40, 0x30, 0x20, 0x10, // path
			},
		},
		// Dir
		{
			description: "Tversion fcall",
			target: &Fcall{
				Type: Tversion,
				Tag:  2255,
				Message: MessageTversion{
					MSize:   uint32(1024),
					Version: "9PTEST",
				},
			},
			marshaled: []byte{
				0x64, 0xcf, 0x8, 0x0, 0x4, 0x0, 0x0,
				0x6, 0x0, 0x39, 0x50, 0x54, 0x45, 0x53, 0x54},
		},
		{
			description: "Rversion fcall",
			target: &Fcall{
				Type: Rversion,
				Tag:  2255,
				Message: MessageRversion{
					MSize:   uint32(1024),
					Version: "9PTEST",
				},
			},
			marshaled: []byte{
				0x65, 0xcf, 0x8, 0x0, 0x4, 0x0, 0x0,
				0x6, 0x0, 0x39, 0x50, 0x54, 0x45, 0x53, 0x54},
		},
		{
			description: "Twalk fcall",
			target: &Fcall{
				Type: Twalk,
				Tag:  5666,
				Message: MessageTwalk{
					Fid:    1010,
					Newfid: 1011,
					Wnames: []string{"a", "b", "c"},
				},
			},
			marshaled: []byte{
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
				Message: MessageRwalk{
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
				Message: MessageRread{
					Data: []byte("a lot of byte data"),
				},
			},
			marshaled: []byte{
				0x75, 0xb4, 0x15,
				0x12, 0x0, 0x0, 0x0,
				0x61, 0x20, 0x6c, 0x6f, 0x74, 0x20, 0x6f, 0x66, 0x20, 0x62, 0x79, 0x74, 0x65, 0x20, 0x64, 0x61, 0x74, 0x61},
		},
		{
			description: "",
			target: &Fcall{
				Type: Rstat,
				Tag:  5556,
				Message: MessageRstat{
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
				0x7d, 0xb4, 0x15,
				0x42, 0x0, // TODO(stevvooe): Include Dir size. Not straightforward.
				0x40, 0x0, // TODO(stevvooe): Include Dir size. Not straightforward.
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
		{
			description: "Dir[]",
			target: []Dir{
				{
					Type: uint16(0),
					Dev:  uint32(0),
					Qid: Qid{
						Type:    QTDIR,
						Version: uint32(0),
						Path:    ^uint64(0),
					},
					Mode:       DMDIR | DMREAD,
					AccessTime: time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
					ModTime:    time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
					Length:     0x88,
					Name:       ".",
					UID:        "501",
					GID:        "20",
					MUID:       "none",
				},
				{
					Type: uint16(0),
					Dev:  uint32(0),
					Qid: Qid{
						Type:    QTDIR,
						Version: uint32(0),
						Path:    ^uint64(0),
					},
					Mode:       DMDIR | DMREAD,
					AccessTime: time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
					ModTime:    time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
					Length:     0x63e,
					Name:       "..",
					UID:        "501",
					GID:        "20",
					MUID:       "none",
				},
				{
					Type: uint16(0),
					Dev:  uint32(0),
					Qid: Qid{
						Type:    QTDIR,
						Version: uint32(0),
						Path:    ^uint64(0),
					},
					Mode:       DMDIR | DMREAD,
					AccessTime: time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
					ModTime:    time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
					Length:     0x44,
					Name:       "hello",
					UID:        "501",
					GID:        "20",
					MUID:       "none",
				},
				{
					Type: uint16(0),
					Dev:  uint32(0),
					Qid: Qid{
						Type:    QTDIR,
						Version: uint32(0),
						Path:    ^uint64(0),
					},
					Mode:       DMDIR | DMREAD,
					AccessTime: time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
					ModTime:    time.Date(2006, 01, 02, 03, 04, 05, 0, time.UTC),
					Length:     0x44,
					Name:       "there",
					UID:        "501",
					GID:        "20",
					MUID:       "none",
				},
			},
			marshaled: []byte{
				0x39, 0x0, // size
				0x0, 0x0, // type
				0x0, 0x0, 0x0, 0x0, // dev
				0x80,               // qid.type == QTDIR
				0x0, 0x0, 0x0, 0x0, // qid.vers
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // qid.path
				0x4, 0x0, 0x0, 0x80, // mode
				0x25, 0x98, 0xb8, 0x43, // atime
				0x25, 0x98, 0xb8, 0x43, // mtime
				0x88, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // length
				0x1, 0x0,
				0x2e, // .
				0x3, 0x0,
				0x35, 0x30, 0x31, // 501
				0x2, 0x0,
				0x32, 0x30, // 20
				0x4, 0x0,
				0x6e, 0x6f, 0x6e, 0x65, // none

				0x3a, 0x0,
				0x0, 0x0, // type
				0x0, 0x0, 0x0, 0x0, // dev
				0x80,               // qid.type == QTDIR
				0x0, 0x0, 0x0, 0x0, // qid.vers
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // qid.path
				0x4, 0x0, 0x0, 0x80, // mode
				0x25, 0x98, 0xb8, 0x43, // atime
				0x25, 0x98, 0xb8, 0x43, // mtime
				0x3e, 0x6, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // length
				0x2, 0x0,
				0x2e, 0x2e, // ..
				0x3, 0x0,
				0x35, 0x30, 0x31, // 501
				0x2, 0x0,
				0x32, 0x30, // 20
				0x4, 0x0,
				0x6e, 0x6f, 0x6e, 0x65, // none

				0x3d, 0x0,
				0x0, 0x0, // type
				0x0, 0x0, 0x0, 0x0, // dev
				0x80,               // qid.type == QTDIR
				0x0, 0x0, 0x0, 0x0, // qid.vers
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // qid.Path
				0x4, 0x0, 0x0, 0x80, // mode
				0x25, 0x98, 0xb8, 0x43, // atime
				0x25, 0x98, 0xb8, 0x43, // mtime
				0x44, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // length
				0x5, 0x0,
				0x68, 0x65, 0x6c, 0x6c, 0x6f, // hello
				0x3, 0x0,
				0x35, 0x30, 0x31, // 501
				0x2, 0x0,
				0x32, 0x30, // 20
				0x4, 0x0,
				0x6e, 0x6f, 0x6e, 0x65, // none

				0x3d, 0x0,
				0x0, 0x0, // type
				0x0, 0x0, 0x0, 0x0, // dev
				0x80,               // qid.type == QTDIR
				0x0, 0x0, 0x0, 0x0, //qid.vers
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // qid.path
				0x4, 0x0, 0x0, 0x80, // mode
				0x25, 0x98, 0xb8, 0x43, // atime
				0x25, 0x98, 0xb8, 0x43, // mtime
				0x44, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // length
				0x5, 0x0,
				0x74, 0x68, 0x65, 0x72, 0x65, // there
				0x3, 0x0,
				0x35, 0x30, 0x31, // 501
				0x2, 0x0,
				0x32, 0x30, // 20
				0x4, 0x0,
				0x6e, 0x6f, 0x6e, 0x65, // none
			},
		},
		{
			description: "Rerror fcall",
			target:      newErrorFcall(5556, errors.New("A serious error")),
			marshaled: []byte{
				0x6b,       // Rerror
				0xb4, 0x15, // Tag
				0xf, 0x0, // String size.
				0x41, 0x20, 0x73, 0x65, 0x72, 0x69, 0x6f, 0x75, 0x73, 0x20, 0x65, 0x72, 0x72, 0x6f, 0x72},
		},
	} {
		t.Logf("target under test: %#v %T", testcase.target, testcase.target)
		fatalf := func(format string, args ...interface{}) {
			t.Fatalf(testcase.description+": "+format, args...)
		}

		p, err := codec.Marshal(testcase.target)
		if err != nil {
			fatalf("error writing fcall: %v", err)
		}

		if !bytes.Equal(p, testcase.marshaled) {
			fatalf("unexpected bytes for fcall: \n%#v != \n%#v", p, testcase.marshaled)
		}

		if size9p(testcase.target) == 0 {
			fatalf("size of target should never be zero")
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

		if err := codec.Unmarshal(p, v); err != nil {
			fatalf("error reading: %v", err)
		}

		if targetType.Kind() != reflect.Ptr {
			v = reflect.Indirect(reflect.ValueOf(v)).Interface()
		}

		if !reflect.DeepEqual(v, testcase.target) {
			fatalf("not equal: %v != %v (\n%#v\n%#v\n)",
				v, testcase.target,
				v, testcase.target)
		}

		t.Logf("%#v", v)

	}
}
