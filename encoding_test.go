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
			description: "Tversion fcall",
			target: &Fcall{
				Type: Tversion,
				Tag:  2255,
				Message: &MessageVersion{
					MSize:   uint32(1024),
					Version: "9PTEST",
				},
			},
			marshaled: []byte{0xf, 0x0, 0x0, 0x0, 0x64, 0xcf, 0x8, 0x0, 0x4, 0x0, 0x0, 0x6, 0x0, 0x39, 0x50, 0x54, 0x45, 0x53, 0x54},
		},
	} {
		t.Logf("target under test: %v", testcase.target)
		fatalf := func(format string, args ...interface{}) {
			t.Fatalf(testcase.description+": "+format, args...)
		}

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
			fatalf("error reading fcall: %v", err)
		}

		if targetType.Kind() != reflect.Ptr {
			v = reflect.Indirect(reflect.ValueOf(v)).Interface()
		}

		if !reflect.DeepEqual(v, testcase.target) {
			fatalf("not equal: %#v != %#v", v, testcase.target)
		}
	}
}
