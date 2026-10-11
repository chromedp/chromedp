package websocket

import (
	"bytes"
	"errors"
	"testing"
)

// FuzzReadFrame feeds arbitrary bytes to the reader. The reader must not
// panic, must not hang, and must return one of the documented errors. It
// must never return more bytes than the limit. The seeds are the examples of RFC
// 6455 section 5.7 and the error cases.
func FuzzReadFrame(f *testing.F) {
	seeds := [][]byte{
		mustHex(f, "81 05 48656c6c6f"),
		mustHex(f, "81 85 37fa213d 7f9f4d5158"),
		mustHex(f, "01 03 48656c 80 02 6c6f"),
		mustHex(f, "89 05 48656c6c6f"),
		mustHex(f, "8a 05 48656c6c6f"),
		cat(mustHex(f, "82 7e 0100"), bytes.Repeat([]byte{1}, 256)),
		cat(mustHex(f, "82 7f 0000000000010000"), bytes.Repeat([]byte{2}, 65536)),
		mustHex(f, "82 7f 7fffffffffffffff"),
		mustHex(f, "82 7f 8000000000000000"),
		mustHex(f, "88 02 03e8"),
		mustHex(f, "88 01 03"),
		mustHex(f, "c1 00"),
		mustHex(f, "81 02 ffff"),
		mustHex(f, "80 00"),
		mustHex(f, "01 01 61 01 01 62"),
		mustHex(f, "09 00"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		fc := &fakeConn{in: bytes.NewReader(data)}
		c, err := newConn(fc, nil)
		if err != nil {
			t.Fatal(err)
		}
		c.maxMessage = 1 << 20
		for range 64 {
			op, msg, err := c.ReadMessage()
			if err != nil {
				var ce *CloseError
				switch {
				case errors.As(err, &ce), errors.Is(err, ErrProtocol),
					errors.Is(err, ErrInvalidPayload), errors.Is(err, ErrMessageTooBig),
					err.Error() == "EOF", err.Error() == "unexpected EOF":
				default:
					t.Fatalf("unexpected error %v", err)
				}
				return
			}
			if op != OpText && op != OpBinary {
				t.Fatalf("got the opcode %s", op)
			}
			if len(msg) > 1<<20 {
				t.Fatalf("a message of %d bytes", len(msg))
			}
		}
	})
}
