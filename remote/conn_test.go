package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote/internal/wstest"
)

// newWebsocketServer starts a server that upgrades each request to a
// websocket and passes the connection to handle. It returns the ws address.
func newWebsocketServer(t *testing.T, handle func(r *http.Request, c *wstest.Conn)) string {
	t.Helper()
	return wstest.URL(wstest.NewServer(t, handle))
}

// mustWrite sends a frame from the server, and fails the test on an error.
func mustWrite(t *testing.T, c *wstest.Conn, op byte, payload []byte) {
	t.Helper()
	if err := c.WriteFrame(op, payload); err != nil {
		t.Errorf("writing a frame: %v", err)
	}
}

// mustRead reads one frame from the client. It gives up after a few seconds.
func mustRead(t *testing.T, c *wstest.Conn) (wstest.Frame, bool) {
	t.Helper()
	f, err := c.ReadFrame()
	if err != nil {
		t.Errorf("reading a frame: %v", err)
		return f, false
	}
	return f, true
}

func TestConnAnswersPing(t *testing.T) {
	t.Parallel()

	// A server that sends at once puts the frames in the same packet as the
	// handshake. A late server sends them in a packet of their own.
	for name, delay := range map[string]time.Duration{"at once": 0, "late": 100 * time.Millisecond} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testConnAnswersPing(t, delay)
		})
	}
}

func testConnAnswersPing(t *testing.T, delay time.Duration) {
	pongs := make(chan wstest.Frame, 2)
	url := newWebsocketServer(t, func(_ *http.Request, c *wstest.Conn) {
		time.Sleep(delay)
		// The ping frames come before the message. A ping with a payload, an
		// empty ping and a pong that nobody asked for must not stop the read.
		mustWrite(t, c, wstest.OpPing, []byte("are you there"))
		mustWrite(t, c, wstest.OpPong, []byte("unasked"))
		mustWrite(t, c, wstest.OpPing, nil)
		mustWrite(t, c, wstest.OpText, []byte(`{"id":5,"result":{}}`))
		for range 2 {
			f, ok := mustRead(t, c)
			if !ok {
				return
			}
			pongs <- f
		}
	})

	ctx := context.Background()
	conn, err := DialContext(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var msg cdproto.Message
	if err := conn.Read(ctx, &msg); err != nil {
		t.Fatalf("reading after the ping frames: %v", err)
	}
	if msg.ID != 5 {
		t.Fatalf("want the message with the id 5, got %+v", msg)
	}

	for _, want := range []string{"are you there", ""} {
		select {
		case f := <-pongs:
			// ReadFrame fails when the frame is not masked.
			if f.Opcode != wstest.OpPong {
				t.Fatalf("want a pong frame, got %v", f.Opcode)
			}
			if got := string(f.Payload); got != want {
				t.Fatalf("want the payload %q, got %q", want, got)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the server got no pong frame")
		}
	}
}

// TestConnPingWhileWriting sends pings while the client writes messages. The
// race detector must find no data race, and each frame must arrive whole.
func TestConnPingWhileWriting(t *testing.T) {
	t.Parallel()

	const writes = 50
	got := make(chan int, 1)
	url := newWebsocketServer(t, func(_ *http.Request, c *wstest.Conn) {
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			for {
				select {
				case <-stop:
					return
				case <-time.After(time.Millisecond):
					// The connection can close while this runs, so ignore the error.
					_ = c.WriteFrame(wstest.OpPing, []byte("ping"))
				}
			}
		}()
		n := 0
		for n < writes {
			f, ok := mustRead(t, c)
			if !ok {
				return
			}
			if f.Opcode == wstest.OpText {
				n++
			}
		}
		got <- n
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := DialContext(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// A reader runs, as in a browser. It answers the pings.
	go func() {
		var msg cdproto.Message
		for conn.Read(ctx, &msg) == nil {
		}
	}()
	for i := range writes {
		if err := conn.Write(ctx, &cdproto.Message{ID: int64(i + 1), Method: "Test.method"}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case n := <-got:
		if n != writes {
			t.Fatalf("want %d messages, got %d", writes, n)
		}
	case <-ctx.Done():
		t.Fatal("the server did not get all messages")
	}
}

// headerServer starts a websocket server that sends the headers of each
// handshake request to the returned channel. It holds each connection open
// until the test ends.
func headerServer(t *testing.T) (string, <-chan http.Header) {
	t.Helper()
	headers := make(chan http.Header, 4)
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	url := newWebsocketServer(t, func(r *http.Request, _ *wstest.Conn) {
		headers <- r.Header.Clone()
		<-done
	})
	return url, headers
}

func receiveHeader(t *testing.T, headers <-chan http.Header) http.Header {
	t.Helper()
	select {
	case h := <-headers:
		return h
	case <-time.After(5 * time.Second):
		t.Fatal("the server got no handshake request")
		return nil
	}
}

func TestDialHTTPHeader(t *testing.T) {
	t.Parallel()

	const want = "Bearer secret-token"
	url, headers := headerServer(t)
	h := http.Header{"Authorization": {want}, "X-Test": {"one", "two"}}
	conn, err := DialContext(context.Background(), url, WithConnHTTPHeader(h))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// A later change of the caller header has no effect.
	h.Set("Authorization", "changed")

	got := receiveHeader(t, headers)
	if got.Get("Authorization") != want {
		t.Errorf("want the Authorization %q, got %q", want, got.Get("Authorization"))
	}
	if v := got.Values("X-Test"); len(v) != 2 || v[0] != "one" || v[1] != "two" {
		t.Errorf("want the X-Test values one and two, got %q", v)
	}
	// The handshake keeps its own headers.
	if got.Get("Upgrade") != "websocket" {
		t.Errorf("the handshake lost its Upgrade header: %v", got)
	}
}

func TestDialHTTPHeaderAbsent(t *testing.T) {
	t.Parallel()

	url, headers := headerServer(t)
	conn, err := DialContext(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if got := receiveHeader(t, headers).Get("Authorization"); got != "" {
		t.Fatalf("want no Authorization header, got %q", got)
	}
}

func TestAllocatorDialHTTPHeader(t *testing.T) {
	t.Parallel()

	url, headers := headerServer(t)
	allocCtx, cancel := NewAllocator(context.Background(), url, NoModifyURL,
		WithDialHTTPHeader(http.Header{"Authorization": {"Bearer remote"}}))
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// The server does not speak the protocol, so the run ends with an error
	// after the allocator dialed.
	tctx, tcancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer tcancel()
	_ = chromedp.Do(tctx)
	if got := receiveHeader(t, headers).Get("Authorization"); got != "Bearer remote" {
		t.Fatalf("want the header of the allocator option, got %q", got)
	}
}

// TestConnReadErrors checks the errors of Read for the frames that end a
// session.
func TestConnReadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		send  func(c *wstest.Conn)
		check func(t *testing.T, err error)
	}{
		{
			name: "normal close",
			send: func(c *wstest.Conn) { c.WriteFrame(wstest.OpClose, []byte{0x03, 0xe8}) },
			check: func(t *testing.T, err error) {
				if err != io.EOF {
					t.Fatalf("got %v, want io.EOF", err)
				}
			},
		},
		{
			name: "going away",
			send: func(c *wstest.Conn) { c.WriteFrame(wstest.OpClose, []byte{0x03, 0xe9}) },
			check: func(t *testing.T, err error) {
				if err != io.EOF {
					t.Fatalf("got %v, want io.EOF", err)
				}
			},
		},
		{
			name: "binary frame",
			send: func(c *wstest.Conn) { c.WriteFrame(wstest.OpBinary, []byte("x")) },
			check: func(t *testing.T, err error) {
				if !errors.Is(err, ErrInvalidMessage) {
					t.Fatalf("got %v, want ErrInvalidMessage", err)
				}
			},
		},
		{
			name: "connection dropped",
			send: func(c *wstest.Conn) { c.Close() },
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("want an error")
				}
			},
		},
		{
			name: "invalid JSON",
			send: func(c *wstest.Conn) { c.WriteFrame(wstest.OpText, []byte("{not json")) },
			check: func(t *testing.T, err error) {
				if err == nil || errors.Is(err, io.EOF) {
					t.Fatalf("got %v, want a JSON error", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			url := newWebsocketServer(t, func(_ *http.Request, c *wstest.Conn) {
				tt.send(c)
				// Let the client read before the server closes.
				c.ReadFrame()
			})
			conn, err := DialContext(context.Background(), url)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			var msg cdproto.Message
			tt.check(t, conn.Read(context.Background(), &msg))
		})
	}
}

// TestConnReadAfterClose checks that a read after Close, and a read that Close
// ends, give an error that wraps net.ErrClosed.
func TestConnReadAfterClose(t *testing.T) {
	t.Parallel()

	url := newWebsocketServer(t, func(_ *http.Request, c *wstest.Conn) {
		c.ReadFrame()
	})
	conn, err := DialContext(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		var msg cdproto.Message
		done <- conn.Read(context.Background(), &msg)
	}()
	time.Sleep(50 * time.Millisecond)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("got %v, want net.ErrClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not end the read")
	}
	var msg cdproto.Message
	if err := conn.Read(context.Background(), &msg); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("read after Close: got %v", err)
	}
}

// TestConnDebugf checks the lines for the protocol logger.
func TestConnDebugf(t *testing.T) {
	t.Parallel()

	url := newWebsocketServer(t, func(_ *http.Request, c *wstest.Conn) {
		mustWrite(t, c, wstest.OpPing, nil)
		mustWrite(t, c, wstest.OpText, []byte(`{"id":1}`))
		mustRead(t, c)
		mustRead(t, c)
	})
	var mu sync.Mutex
	var lines []string
	conn, err := DialContext(context.Background(), url, WithConnDebugf(func(f string, a ...any) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, fmt.Sprintf(f, a...))
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var msg cdproto.Message
	if err := conn.Read(context.Background(), &msg); err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(context.Background(), &cdproto.Message{ID: 2}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	got := strings.Join(lines, "\n")
	for _, want := range []string{"received ping frame, sending pong", `<- {"id":1}`, "-> "} {
		if !strings.Contains(got, want) {
			t.Errorf("the log %q has no %q", got, want)
		}
	}
}
