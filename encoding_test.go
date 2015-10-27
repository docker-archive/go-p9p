package p9pnew

import (
	"bytes"
	"reflect"
	"testing"
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
		{
			description: "Tversion fcall",
			target: &Fcall{
				Type: Tversion,
				Tag:  2255,
				Message: &MessageVersion{
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
			description: "Twalk fcall",
			target: &Fcall{
				Type: Twalk,
				Tag:  5666,
				Message: &MessageTwalk{
					Fid:    1010,
					Newfid: 1011,
					Wname:  []string{"a", "b", "c"},
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
	} {
		t.Logf("target under test: %v", testcase.target)
		fatalf := func(format string, args ...interface{}) {
			t.Fatalf(testcase.description+": "+format, args...)
		}

		// check that size9p is working correctly
		if int(size9p(testcase.target)) != len(testcase.marshaled) {
			fatalf("size not correct: %v != %v", int(size9p(testcase.target)), len(testcase.marshaled))
		}

		t.Logf("expecting message of %v bytes", len(testcase.marshaled))

		var b bytes.Buffer

		enc := &encoder{&b}
		dec := &decoder{&b}

		if err := enc.encode(testcase.target); err != nil {
			fatalf("error writing fcall: %v", err)
		}

		if !bytes.Equal(b.Bytes(), testcase.marshaled) {
			fatalf("unexpected bytes for fcall: %#v != %#v", b.Bytes(), testcase.marshaled)
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
			fatalf("not equal: %#v != %#v", v, testcase.target)
		}
		t.Logf("%#v", v)

	}
}
