package websocket

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/chromedp/chromedp/remote/internal/wstest"
)

// BenchmarkRoundTrip sends a text message to an echo server and reads the
// echo. The server is the independent one of the package wstest.
func BenchmarkRoundTrip(b *testing.B) {
	for _, size := range []struct {
		name string
		n    int
	}{{"1KiB", 1 << 10}, {"1MiB", 1 << 20}} {
		b.Run(size.name, func(b *testing.B) {
			srv := wstest.NewServer(b, func(_ *http.Request, c *wstest.Conn) {
				for {
					f, err := c.ReadFrame()
					if err != nil || f.Opcode == wstest.OpClose {
						return
					}
					if c.WriteFrame(f.Opcode, f.Payload) != nil {
						return
					}
				}
			})
			c, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
			if err != nil {
				b.Fatal(err)
			}
			defer c.Close()
			msg := bytes.Repeat([]byte("a"), size.n)
			b.SetBytes(int64(2 * size.n))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := c.WriteText(msg); err != nil {
					b.Fatal(err)
				}
				if _, got, err := c.ReadMessage(); err != nil || len(got) != size.n {
					b.Fatalf("got %d bytes and %v", len(got), err)
				}
			}
		})
	}
}
