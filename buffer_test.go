package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"testing"
)

type TestBufferItem struct {
	label    string
	src, dst net.UDPAddr
	payload  []byte
	result   []byte
}

var tests []TestBufferItem

func TestMain(m *testing.M) {
	tests = []TestBufferItem{
		{
			label:   "Test IPv4",
			src:     net.UDPAddr{IP: net.ParseIP("10.1.2.3"), Port: 3000},
			dst:     net.UDPAddr{IP: net.ParseIP("10.100.101.102"), Port: 3123},
			payload: []byte("payload 1"),
			result: []byte{
				0x45, 0x00, 0x00, 0x25, 0x12, 0x34, 0x00, 0x00,
				0x10, 0x11, 0x00, 0x00, 0x0a, 0x01, 0x02, 0x3,
				0x0a, 0x64, 0x65, 0x66, 0x0b, 0xb8, 0x0c, 0x33,
				0x00, 0x11, 0x7d, 0xc3, 0x70, 0x61, 0x79, 0x6c,
				0x6f, 0x61, 0x64, 0x20, 0x31,
			},
		},
	}
	os.Exit(m.Run())
}

func TestBuffer(t *testing.T) {
	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			var buf Buffer
			n := copy(buf.Slice(), test.payload)
			result := buf.SpoofUDP(test.src, test.dst, n)
			if !bytes.Equal(result, test.result) {
				fmt.Printf("Test %s: got %#v\n", test.label, result)
				t.Fail()
			}
		})
	}
}

func BenchmarkBuffer(b *testing.B) {
	// run the Fib function b.N times
	var buf Buffer
	test := tests[0]
	for n := 0; n < b.N; n++ {
		buf.SpoofUDP(test.src, test.dst, len(test.payload))
	}
}
