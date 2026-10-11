package websocket

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
)

// mustHex decodes a hex string. A space is allowed.
func mustHex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestMaskBytes checks the masking of RFC 6455 section 5.7. The unmasked text
// frame "Hello" has the key 37 fa 21 3d.
func TestMaskBytes(t *testing.T) {
	t.Parallel()

	key := [4]byte{0x37, 0xfa, 0x21, 0x3d}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"RFC example", "Hello", "7f9f4d5158"},
		{"empty", "", ""},
		{"eight bytes", "01234567", "07cb13 0e 35cc12 0a"},
		{"nine bytes", "012345678", "07cb130e35cc120a0f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := []byte(tt.in)
			maskBytes(key, b)
			// A byte by byte mask gives the expected result too.
			slow := []byte(tt.in)
			for i := range slow {
				slow[i] ^= key[i%4]
			}
			if !bytes.Equal(b, slow) {
				t.Fatalf("got %x, the byte loop gives %x", b, slow)
			}
			if tt.name == "RFC example" && !bytes.Equal(b, mustHex(t, tt.want)) {
				t.Fatalf("got %x, want %s", b, tt.want)
			}
			maskBytes(key, b)
			if string(b) != tt.in {
				t.Fatalf("masking twice gives %q, want %q", b, tt.in)
			}
		})
	}
}

// TestAppendHeader checks the header of a client frame. RFC 6455 section 5.7
// has the masked "Hello" frame 81 85 37 fa 21 3d 7f 9f 4d 51 58.
func TestAppendHeader(t *testing.T) {
	t.Parallel()

	key := [4]byte{0x37, 0xfa, 0x21, 0x3d}
	tests := []struct {
		name string
		op   Opcode
		n    int
		want string
	}{
		{"Hello", OpText, 5, "81 85 37fa213d"},
		{"empty pong", OpPong, 0, "8a 80 37fa213d"},
		{"125 bytes", OpText, 125, "81 fd 37fa213d"},
		{"126 bytes", OpText, 126, "81 fe 007e 37fa213d"},
		{"256 bytes", OpBinary, 256, "82 fe 0100 37fa213d"},
		{"65535 bytes", OpText, 65535, "81 fe ffff 37fa213d"},
		{"64 KiB", OpBinary, 65536, "82 ff 0000000000010000 37fa213d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := appendHeader(nil, tt.op, tt.n, key)
			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Fatalf("got %x, want %x", got, want)
			}
		})
	}

	// The whole masked "Hello" frame of the RFC.
	b := appendHeader(nil, OpText, 5, key)
	hdr := len(b)
	b = append(b, "Hello"...)
	maskBytes(key, b[hdr:])
	if want := mustHex(t, "81 85 37fa213d 7f9f4d5158"); !bytes.Equal(b, want) {
		t.Fatalf("got %x, want %x", b, want)
	}
}

func newReader(b []byte) *bufio.Reader {
	return bufio.NewReader(bytes.NewReader(b))
}

func TestReadFrameHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    frameHeader
		wantErr error
	}{
		{"Hello", "81 05", frameHeader{true, OpText, 5}, nil},
		{"first fragment", "01 03", frameHeader{false, OpText, 3}, nil},
		{"last fragment", "80 02", frameHeader{true, OpContinuation, 2}, nil},
		{"ping", "89 05", frameHeader{true, OpPing, 5}, nil},
		{"pong", "8a 05", frameHeader{true, OpPong, 5}, nil},
		{"256 bytes", "82 7e 0100", frameHeader{true, OpBinary, 256}, nil},
		{"64 KiB", "82 7f 0000000000010000", frameHeader{true, OpBinary, 65536}, nil},
		{"long encoding of a short length", "81 7e 0005", frameHeader{true, OpText, 5}, nil},
		{"long 64 bit encoding of a short length", "81 7f 0000000000000005", frameHeader{true, OpText, 5}, nil},
		{"control frame with a long encoding", "89 7e 0005", frameHeader{true, OpPing, 5}, nil},
		{"empty stream", "", frameHeader{}, io.EOF},
		{"one byte", "81", frameHeader{}, io.ErrUnexpectedEOF},
		{"cut 16 bit length", "81 7e 01", frameHeader{}, io.ErrUnexpectedEOF},
		{"cut 64 bit length", "81 7f 00000000", frameHeader{}, io.ErrUnexpectedEOF},
		{"RSV1", "c1 05", frameHeader{}, ErrProtocol},
		{"RSV2", "a1 05", frameHeader{}, ErrProtocol},
		{"RSV3", "91 05", frameHeader{}, ErrProtocol},
		{"reserved data opcode", "83 05", frameHeader{}, ErrProtocol},
		{"reserved control opcode", "8b 05", frameHeader{}, ErrProtocol},
		{"masked frame", "81 85 37fa213d", frameHeader{}, ErrProtocol},
		{"64 bit length with the top bit", "82 7f 8000000000000000", frameHeader{}, ErrProtocol},
		{"fragmented ping", "09 00", frameHeader{}, ErrProtocol},
		{"fragmented close", "08 00", frameHeader{}, ErrProtocol},
		{"ping of 126 bytes", "89 7e 007e", frameHeader{}, ErrProtocol},
		{"close of 126 bytes", "88 7e 007e", frameHeader{}, ErrProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := readFrameHeader(newReader(mustHex(t, tt.in)))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) && err != tt.wantErr {
					t.Fatalf("got the error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
