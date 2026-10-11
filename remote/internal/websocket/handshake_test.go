package websocket

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/chromedp/chromedp/remote/internal/wstest"
)

// upgradeServer starts a server that upgrades with the extra response
// headers, and sends each request to the channel.
func upgradeServer(t *testing.T, extra http.Header) (*httptest.Server, <-chan *http.Request) {
	t.Helper()
	reqs := make(chan *http.Request, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs <- r.Clone(context.Background())
		c, err := wstest.Upgrade(w, r, extra)
		if err != nil {
			return
		}
		defer c.Close()
		c.WriteText("hello")
		// Wait for the client to close.
		c.ReadFrame()
	}))
	t.Cleanup(srv.Close)
	return srv, reqs
}

func TestDialSendsHandshake(t *testing.T) {
	t.Parallel()

	srv, reqs := upgradeServer(t, nil)
	d := Dialer{Header: http.Header{
		"Authorization":            {"Bearer secret"},
		"X-Test":                   {"one", "two"},
		"Origin":                   {"http://example.test"},
		"Sec-Websocket-Key":        {"caller"},
		"Sec-Websocket-Version":    {"8"},
		"Sec-Websocket-Extensions": {"permessage-deflate"},
	}}
	c, err := d.Dial(context.Background(), wstest.URL(srv)+"/devtools/browser/abc?x=1")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	r := <-reqs
	if r.Method != "GET" || r.URL.RequestURI() != "/devtools/browser/abc?x=1" {
		t.Errorf("got the request %s %s", r.Method, r.URL.RequestURI())
	}
	if got := r.Header.Get("Upgrade"); got != "websocket" {
		t.Errorf("Upgrade is %q", got)
	}
	if got := r.Header.Get("Connection"); !strings.EqualFold(got, "upgrade") {
		t.Errorf("Connection is %q", got)
	}
	if got := r.Header.Get("Sec-WebSocket-Version"); got != "13" {
		t.Errorf("the version is %q, the dialer must replace the one of the caller", got)
	}
	if got := r.Header.Get("Sec-WebSocket-Key"); got == "caller" || len(got) != 24 {
		t.Errorf("the key is %q", got)
	}
	if got := r.Header.Values("Sec-WebSocket-Extensions"); len(got) != 0 {
		t.Errorf("the request asks for the extensions %q", got)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer secret" {
		t.Errorf("Authorization is %q", got)
	}
	if got := r.Header.Values("X-Test"); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("X-Test is %q", got)
	}
	if got := r.Header.Get("Origin"); got != "http://example.test" {
		t.Errorf("Origin is %q", got)
	}
	if d.Header.Get("Sec-WebSocket-Key") != "caller" {
		t.Error("the dialer changed the header of the caller")
	}

	op, msg, err := c.ReadMessage()
	if err != nil || op != OpText || string(msg) != "hello" {
		t.Fatalf("got %s %q and %v after the handshake", op, msg, err)
	}
}

// TestDialKeysDiffer checks that each dial uses a new key.
func TestDialKeysDiffer(t *testing.T) {
	t.Parallel()

	srv, reqs := upgradeServer(t, nil)
	keys := map[string]bool{}
	for range 5 {
		c, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
		if err != nil {
			t.Fatal(err)
		}
		c.Close()
		keys[(<-reqs).Header.Get("Sec-WebSocket-Key")] = true
	}
	if len(keys) != 5 {
		t.Fatalf("got %d different keys in 5 dials", len(keys))
	}
}

func TestDialHostHeader(t *testing.T) {
	t.Parallel()

	srv, reqs := upgradeServer(t, nil)
	c, err := (&Dialer{Header: http.Header{"Host": {"browser.internal:9222"}}}).Dial(context.Background(), wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if r := <-reqs; r.Host != "browser.internal:9222" {
		t.Fatalf("the host is %q", r.Host)
	}

	// Without the header, the host is the one of the address.
	c, err = new(Dialer).Dial(context.Background(), wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if r := <-reqs; r.Host != strings.TrimPrefix(srv.URL, "http://") {
		t.Fatalf("the host is %q", r.Host)
	}
}

func TestDialUserInfo(t *testing.T) {
	t.Parallel()

	srv, reqs := upgradeServer(t, nil)
	c, err := new(Dialer).Dial(context.Background(), "ws://user:pa%20ss@"+strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	user, pass, ok := (<-reqs).BasicAuth()
	if !ok || user != "user" || pass != "pa ss" {
		t.Fatalf("got the basic auth %q %q %v", user, pass, ok)
	}
}

func TestDialIPv6(t *testing.T) {
	t.Parallel()

	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback: %v", err)
	}
	srv := httptest.NewUnstartedServer(wstest.Handler(t, func(*http.Request, *wstest.Conn) {}))
	srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	defer srv.Close()

	c, err := new(Dialer).Dial(context.Background(), "ws://"+l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestDialDefaultPort(t *testing.T) {
	t.Parallel()

	// The dial fails when nothing listens on the default port, and the
	// error names the address that the dial used.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := new(Dialer).Dial(ctx, "ws://127.0.0.1")
	if err == nil || somethingListens(err) {
		t.Skip("something listens on the port 80")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:80") {
		t.Fatalf("the error does not name the port 80: %v", err)
	}
	_, err = new(Dialer).Dial(ctx, "wss://127.0.0.1")
	if err == nil || somethingListens(err) {
		t.Skip("something listens on the port 443")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:443") {
		t.Fatalf("the error does not name the port 443: %v", err)
	}
}

// somethingListens reports whether the error comes from a server that answered
// on the port, and not from the failed connection. A runner can have a web
// server on the port 80 or the port 443, and its answer is not a websocket
// handshake.
func somethingListens(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return false
	}
	return !strings.Contains(err.Error(), "127.0.0.1:")
}

func TestDialErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		extra   http.Header
		handler http.HandlerFunc
		want    string
	}{
		{name: "wrong accept", extra: http.Header{"Sec-WebSocket-Accept": {"AAAAAAAAAAAAAAAAAAAAAAAAAAA="}}, want: "Sec-WebSocket-Accept"},
		{name: "accept of another key", extra: http.Header{"Sec-WebSocket-Accept": {wstest.AcceptKey("other")}}, want: "Sec-WebSocket-Accept"},
		{name: "extension", extra: http.Header{"Sec-WebSocket-Extensions": {"permessage-deflate"}}, want: "extensions"},
		{name: "subprotocol", extra: http.Header{"Sec-WebSocket-Protocol": {"chat"}}, want: "subprotocol"},
		{
			name: "403 with a body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "origin not allowed", http.StatusForbidden)
			},
			want: `"403 Forbidden", body "origin not allowed"`,
		},
		{
			name: "long body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				io.WriteString(w, strings.Repeat("x", 100000))
			},
			want: "502 Bad Gateway",
		},
		{
			name: "200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, "plain page")
			},
			want: "200 OK",
		},
		{
			name: "no upgrade header",
			handler: func(w http.ResponseWriter, r *http.Request) {
				hj := w.(http.Hijacker)
				nc, _, _ := hj.Hijack()
				defer nc.Close()
				io.WriteString(nc, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: "+wstest.AcceptKey(r.Header.Get("Sec-WebSocket-Key"))+"\r\n\r\n")
			},
			want: "Upgrade header",
		},
		{
			name: "upgrade of another protocol",
			handler: func(w http.ResponseWriter, r *http.Request) {
				hj := w.(http.Hijacker)
				nc, _, _ := hj.Hijack()
				defer nc.Close()
				io.WriteString(nc, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: h2c\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: "+wstest.AcceptKey(r.Header.Get("Sec-WebSocket-Key"))+"\r\n\r\n")
			},
			want: "Upgrade header",
		},
		{
			name: "no connection header",
			handler: func(w http.ResponseWriter, r *http.Request) {
				hj := w.(http.Hijacker)
				nc, _, _ := hj.Hijack()
				defer nc.Close()
				io.WriteString(nc, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: "+wstest.AcceptKey(r.Header.Get("Sec-WebSocket-Key"))+"\r\n\r\n")
			},
			want: "Connection header",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := tt.handler
			if h == nil {
				h = func(w http.ResponseWriter, r *http.Request) {
					c, err := wstest.Upgrade(w, r, tt.extra)
					if err == nil {
						c.Close()
					}
				}
			}
			srv := httptest.NewServer(h)
			defer srv.Close()
			c, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
			if err == nil {
				c.Close()
				t.Fatal("the dial succeeded")
			}
			if !errors.Is(err, ErrHandshake) {
				t.Errorf("the error does not wrap ErrHandshake: %v", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("the error %q does not contain %q", err, tt.want)
			}
			if len(err.Error()) > 1000 {
				t.Errorf("the error has %d bytes", len(err.Error()))
			}
		})
	}
}

// TestDialAcceptsServerHeaders checks the cases that the client must accept:
// the case of the header values, and a list in Connection.
func TestDialAcceptsServerHeaders(t *testing.T) {
	t.Parallel()

	for name, resp := range map[string]string{
		"upper case":        "Upgrade: WEBSOCKET\r\nConnection: UPGRADE\r\n",
		"connection list":   "Upgrade: websocket\r\nConnection: keep-alive, Upgrade\r\n",
		"lower case header": "upgrade: websocket\r\nconnection: upgrade\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nc, _, _ := w.(http.Hijacker).Hijack()
				defer nc.Close()
				io.WriteString(nc, "HTTP/1.1 101 Switching Protocols\r\n"+resp+"Sec-WebSocket-Accept: "+wstest.AcceptKey(r.Header.Get("Sec-WebSocket-Key"))+"\r\n\r\n")
				io.Copy(io.Discard, nc)
			}))
			defer srv.Close()
			c, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
			if err != nil {
				t.Fatal(err)
			}
			c.Close()
		})
	}
}

func TestDialSubprotocol(t *testing.T) {
	t.Parallel()

	srv, _ := upgradeServer(t, http.Header{"Sec-WebSocket-Protocol": {"chat"}})
	c, err := (&Dialer{Header: http.Header{"Sec-WebSocket-Protocol": {"super, chat"}}}).Dial(context.Background(), wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if got := c.Subprotocol(); got != "chat" {
		t.Fatalf("the subprotocol is %q", got)
	}
}

func TestDialRedirect(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()

	_, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
	if err == nil || !errors.Is(err, ErrHandshake) || !strings.Contains(err.Error(), "302") {
		t.Fatalf("got %v, want a handshake error with the status 302", err)
	}
	if hits.Load() != 0 {
		t.Fatal("the dialer followed the redirect")
	}
}

func TestDialBadAddress(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{"http://127.0.0.1:1", "127.0.0.1:9222", "ws://", "ws://[::1", "", "ftp://x"} {
		if c, err := new(Dialer).Dial(context.Background(), addr); err == nil {
			c.Close()
			t.Errorf("Dial(%q) succeeded", addr)
		}
	}
}

func TestDialTLS(t *testing.T) {
	t.Parallel()

	srv := httptest.NewUnstartedServer(wstest.Handler(t, func(_ *http.Request, c *wstest.Conn) {
		c.WriteText("secure")
		c.ReadFrame()
	}))
	srv.StartTLS()
	defer srv.Close()
	addr := "wss" + strings.TrimPrefix(srv.URL, "https")

	t.Run("trusted", func(t *testing.T) {
		pool := x509.NewCertPool()
		pool.AddCert(srv.Certificate())
		c, err := (&Dialer{TLSConfig: &tls.Config{RootCAs: pool}}).Dial(context.Background(), addr)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		if _, msg, err := c.ReadMessage(); err != nil || string(msg) != "secure" {
			t.Fatalf("got %q and %v", msg, err)
		}
	})
	t.Run("insecure skip verify", func(t *testing.T) {
		c, err := (&Dialer{TLSConfig: &tls.Config{InsecureSkipVerify: true}}).Dial(context.Background(), addr)
		if err != nil {
			t.Fatal(err)
		}
		c.Close()
	})
	t.Run("unknown authority", func(t *testing.T) {
		c, err := new(Dialer).Dial(context.Background(), addr)
		if err == nil {
			c.Close()
			t.Fatal("the dial trusted the test certificate")
		}
		var ua x509.UnknownAuthorityError
		if !errors.As(err, &ua) {
			t.Fatalf("got %v, want an unknown authority error", err)
		}
	})
}

func TestDialContext(t *testing.T) {
	t.Parallel()

	// A server that accepts the connection and never answers.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	addr := "ws://" + l.Addr().String()

	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := new(Dialer).Dial(ctx, addr)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v, want a deadline error", err)
		}
		if d := time.Since(start); d > 3*time.Second {
			t.Fatalf("the dial took %v", d)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(100*time.Millisecond, cancel)
		_, err := new(Dialer).Dial(ctx, addr)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want a cancel error", err)
		}
	})
	t.Run("already canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := new(Dialer).Dial(ctx, addr); !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want a cancel error", err)
		}
	})
}

// TestContextEndsAfterDial checks that the connection works after the context
// of the dial is done. The allocator cancels its dial context at once.
func TestContextEndsAfterDial(t *testing.T) {
	t.Parallel()

	srv := wstest.NewServer(t, func(_ *http.Request, c *wstest.Conn) {
		time.Sleep(200 * time.Millisecond)
		c.WriteText("late")
		c.ReadFrame()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	c, err := new(Dialer).Dial(ctx, wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	cancel()
	_, msg, err := c.ReadMessage()
	if err != nil || string(msg) != "late" {
		t.Fatalf("got %q and %v after the context ended", msg, err)
	}
	if err := c.WriteText([]byte("x")); err != nil {
		t.Fatal(err)
	}
}

// TestFramesWithHandshake checks that a frame that the server sends in the
// same packet as the 101 response is not lost.
func TestFramesWithHandshake(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nc, _, _ := w.(http.Hijacker).Hijack()
		defer nc.Close()
		resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " +
			wstest.AcceptKey(r.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n"
		nc.Write(append([]byte(resp), wstest.Encode(true, wstest.OpText, []byte("early"))...))
		io.Copy(io.Discard, nc)
	}))
	defer srv.Close()
	c, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, msg, err := c.ReadMessage(); err != nil || string(msg) != "early" {
		t.Fatalf("got %q and %v", msg, err)
	}
}

// TestTransportHasNoProxy checks that the dial ignores the proxy variables.
func TestTransportHasNoProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("NO_PROXY", "")
	tr := newTransport(new(Dialer), new(connRecorder))
	if tr.Proxy != nil {
		t.Fatal("the transport has a proxy func")
	}
	// With a proxy func, a non-local name goes to the proxy. The
	// dial must fail at the name lookup or the connection, never at 127.0.0.1:1.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := new(Dialer).Dial(ctx, "ws://chromedp-no-proxy.invalid:9222")
	if err == nil || strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("got %v", err)
	}
}

// TestDialServerClosesAtOnce checks that a server that closes the connection
// before it answers gives an error that wraps io.EOF, whatever the timing.
func TestDialServerClosesAtOnce(t *testing.T) {
	t.Parallel()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	for range 50 {
		c, err := new(Dialer).Dial(context.Background(), "ws://"+l.Addr().String())
		if err == nil {
			c.Close()
			t.Fatal("the dial succeeded")
		}
		if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.ECONNRESET) && !isWindowsReset(err) {
			t.Fatalf("got %v, want io.EOF or a reset", err)
		}
	}
}

// isWindowsReset reports whether the error is a connection reset or abort of
// Windows. Its error numbers are not the ones of syscall.ECONNRESET.
func isWindowsReset(err error) bool {
	const wsaeconnaborted, wsaeconnreset = 10053, 10054
	return errors.Is(err, syscall.Errno(wsaeconnaborted)) || errors.Is(err, syscall.Errno(wsaeconnreset))
}

// serverLog is what a recording server saw.
type serverLog struct {
	mu      sync.Mutex
	opcodes []byte
}

func (l *serverLog) add(op byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.opcodes = append(l.opcodes, op)
}

func (l *serverLog) get() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return bytes.Clone(l.opcodes)
}

// TestNoDataFrameAfterClose checks that a Write that runs while Close runs
// never puts a data frame on the wire after the close frame.
func TestNoDataFrameAfterClose(t *testing.T) {
	t.Parallel()

	for i := range 20 {
		var log serverLog
		finished := make(chan struct{})
		srv := wstest.NewServer(t, func(_ *http.Request, c *wstest.Conn) {
			defer close(finished)
			for {
				f, err := c.ReadFrame()
				if err != nil {
					return
				}
				log.add(f.Opcode)
				if f.Opcode == wstest.OpClose {
					c.WriteFrame(wstest.OpClose, f.Payload)
					return
				}
			}
		})
		c, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}()
		var wg sync.WaitGroup
		stop := make(chan struct{})
		for range 4 {
			wg.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
					}
					if c.WriteText([]byte("data")) != nil {
						return
					}
				}
			})
		}
		time.Sleep(time.Duration(i) * time.Millisecond)
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
		close(stop)
		wg.Wait()
		<-finished
		ops := log.get()
		closeAt := bytes.IndexByte(ops, wstest.OpClose)
		if closeAt < 0 {
			t.Fatalf("round %d: the server saw no close frame", i)
		}
		if closeAt != len(ops)-1 {
			t.Fatalf("round %d: frames follow the close frame: %v", i, ops[closeAt:])
		}
		if err := c.WriteText([]byte("x")); !errors.Is(err, net.ErrClosed) {
			t.Fatalf("round %d: a write after Close gave %v, want net.ErrClosed", i, err)
		}
	}
}

// TestCloseWaitIsBounded checks that Close returns after about one second when
// the server reads the close frame and never answers.
func TestCloseWaitIsBounded(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	srv := wstest.NewServer(t, func(_ *http.Request, c *wstest.Conn) {
		for {
			if _, err := c.ReadFrame(); err != nil {
				return
			}
			<-release
		}
	})
	c, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < 900*time.Millisecond || d > 3*time.Second {
		t.Fatalf("Close took %v, want about one second", d)
	}
}

// TestCloseEndsWithServerClose checks that Close does not wait when the server
// answers with its close frame.
func TestCloseEndsWithServerClose(t *testing.T) {
	t.Parallel()

	srv := wstest.NewServer(t, func(_ *http.Request, c *wstest.Conn) {
		f, err := c.ReadFrame()
		if err == nil && f.Opcode == wstest.OpClose {
			c.WriteFrame(wstest.OpClose, f.Payload)
			// Keep the connection open. The client must close it.
			io.Copy(io.Discard, c.Conn)
		}
	})
	c, err := new(Dialer).Dial(context.Background(), wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("Close took %v, the server answered at once", d)
	}
}

// TestDialTLSHandshakeTimeout checks that a server that accepts the TCP
// connection and never answers the TLS handshake ends the dial.
func TestDialTLSHandshakeTimeout(t *testing.T) {
	t.Parallel()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	start := time.Now()
	d := Dialer{TLSHandshakeTimeout: 200 * time.Millisecond}
	_, err = d.Dial(context.Background(), "wss://"+l.Addr().String())
	if err == nil || !strings.Contains(err.Error(), "TLS handshake timeout") {
		t.Fatalf("got %v, want a TLS handshake timeout", err)
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("the dial took %v", el)
	}

	// A shorter deadline of the context wins.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	d.TLSHandshakeTimeout = time.Minute
	start = time.Now()
	if _, err := d.Dial(ctx, "wss://"+l.Addr().String()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want a deadline error", err)
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("the dial took %v", el)
	}
}

func TestTransportTimeouts(t *testing.T) {
	t.Parallel()

	tr := newTransport(new(Dialer), new(connRecorder))
	if tr.TLSHandshakeTimeout != 10*time.Second || defaultDialTimeout != 30*time.Second {
		t.Errorf("got the TLS timeout %v and the dial timeout %v", tr.TLSHandshakeTimeout, defaultDialTimeout)
	}
	if tr.Proxy != nil || tr.ForceAttemptHTTP2 || len(tr.TLSNextProto) != 0 {
		t.Error("the transport must have no proxy and no HTTP/2")
	}
	tr = newTransport(&Dialer{TLSHandshakeTimeout: time.Second}, new(connRecorder))
	if tr.TLSHandshakeTimeout != time.Second {
		t.Errorf("got the TLS timeout %v, want one second", tr.TLSHandshakeTimeout)
	}
}

// TestDialSubprotocolIsCaseSensitive checks that the dial compares the
// subprotocol as it was given.
func TestDialSubprotocolIsCaseSensitive(t *testing.T) {
	t.Parallel()

	srv, _ := upgradeServer(t, http.Header{"Sec-WebSocket-Protocol": {"Chat"}})
	c, err := (&Dialer{Header: http.Header{"Sec-WebSocket-Protocol": {"chat"}}}).Dial(context.Background(), wstest.URL(srv))
	if err == nil {
		c.Close()
		t.Fatal("the dial accepted Chat for chat")
	}
	if !errors.Is(err, ErrHandshake) || !strings.Contains(err.Error(), "subprotocol") {
		t.Fatalf("got %v", err)
	}
	c, err = (&Dialer{Header: http.Header{"Sec-WebSocket-Protocol": {"x, Chat"}}}).Dial(context.Background(), wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

// TestDialMaxMessageSize checks the limit that the dialer sets.
func TestDialMaxMessageSize(t *testing.T) {
	t.Parallel()

	srv := wstest.NewServer(t, func(_ *http.Request, c *wstest.Conn) {
		c.WriteText(strings.Repeat("a", 100))
		c.ReadFrame()
	})
	c, err := (&Dialer{MaxMessageSize: 10}).Dial(context.Background(), wstest.URL(srv))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, _, err := c.ReadMessage(); !errors.Is(err, ErrMessageTooBig) {
		t.Fatalf("got %v, want ErrMessageTooBig", err)
	}
}
