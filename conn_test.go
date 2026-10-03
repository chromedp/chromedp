package chromedp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/gobwas/ws"
)

// newWebsocketServer starts a server that upgrades each request to a
// websocket and passes the connection to handle. It returns the ws address.
func newWebsocketServer(t *testing.T, handle func(r *http.Request, c *serverConn)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("upgrading the request: %v", err)
			return
		}
		defer conn.Close()
		handle(r, &serverConn{conn})
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// serverConn is the server end of a websocket connection in a test.
type serverConn struct {
	conn interface {
		Read([]byte) (int, error)
		Write([]byte) (int, error)
		SetDeadline(time.Time) error
	}
}

func (s *serverConn) write(t *testing.T, f ws.Frame) {
	t.Helper()
	if err := ws.WriteFrame(s.conn, f); err != nil {
		t.Errorf("writing a frame: %v", err)
	}
}

// read reads one frame from the client. It gives up after a few seconds.
func (s *serverConn) read(t *testing.T) (ws.Frame, bool) {
	t.Helper()
	s.conn.SetDeadline(time.Now().Add(5 * time.Second))
	f, err := ws.ReadFrame(s.conn)
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
	pongs := make(chan ws.Frame, 2)
	url := newWebsocketServer(t, func(_ *http.Request, c *serverConn) {
		time.Sleep(delay)
		// The ping frames come before the message. A ping with a payload, an
		// empty ping and a pong that nobody asked for must not stop the read.
		c.write(t, ws.NewPingFrame([]byte("are you there")))
		c.write(t, ws.NewPongFrame([]byte("unasked")))
		c.write(t, ws.NewPingFrame(nil))
		c.write(t, ws.NewTextFrame([]byte(`{"id":5,"result":{}}`)))
		for range 2 {
			f, ok := c.read(t)
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
			if f.Header.OpCode != ws.OpPong {
				t.Fatalf("want a pong frame, got %v", f.Header.OpCode)
			}
			if !f.Header.Masked {
				t.Fatal("a client must mask the pong frame")
			}
			// The server reads the frame with the mask still applied.
			ws.Cipher(f.Payload, f.Header.Mask, 0)
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
	url := newWebsocketServer(t, func(_ *http.Request, c *serverConn) {
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			for {
				select {
				case <-stop:
					return
				case <-time.After(time.Millisecond):
					// The connection can close while this runs, so ignore the error.
					_ = ws.WriteFrame(c.conn, ws.NewPingFrame([]byte("ping")))
				}
			}
		}()
		n := 0
		for n < writes {
			f, ok := c.read(t)
			if !ok {
				return
			}
			if f.Header.OpCode == ws.OpText {
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
