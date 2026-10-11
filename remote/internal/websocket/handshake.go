package websocket

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// guid is the constant that RFC 6455 section 1.3 adds to the key to make the
// accept value.
const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const (
	// defaultDialTimeout is how long the TCP connect of a dial can take. The
	// context of the caller wins when its deadline is shorter.
	defaultDialTimeout = 30 * time.Second

	// defaultTLSHandshakeTimeout is how long the TLS handshake of a wss dial
	// can take. The context of the caller wins when its deadline is shorter.
	defaultTLSHandshakeTimeout = 10 * time.Second
)

// maxErrorBody is how many bytes of a failed response the error shows.
const maxErrorBody = 512

// ErrHandshake is the error of a handshake that the server did not accept. The
// errors of the dial wrap it when the response is not a valid answer.
var ErrHandshake = errors.New("websocket handshake failed")

// Dialer holds the settings of a dial. The zero value is ready to use.
type Dialer struct {
	// Header holds more headers for the handshake request. The dialer sets
	// Upgrade, Connection, Sec-WebSocket-Key and Sec-WebSocket-Version itself,
	// and removes Sec-WebSocket-Extensions, because the client supports no
	// extension. A Host header sets the host of the request. A
	// Sec-WebSocket-Protocol header lists the subprotocols that the client
	// accepts.
	Header http.Header

	// TLSConfig is the TLS configuration for a wss address. A nil value
	// means the defaults.
	TLSConfig *tls.Config

	// DialTimeout limits the TCP connect. Zero means 30 seconds. The
	// deadline of the context of Dial wins when it is shorter.
	DialTimeout time.Duration

	// TLSHandshakeTimeout limits the TLS handshake. Zero means 10 seconds.
	// The deadline of the context of Dial wins when it is shorter.
	TLSHandshakeTimeout time.Duration

	// MaxMessageSize is the size of the largest message that the connection
	// reads. Zero means [MaxMessageSize].
	MaxMessageSize uint64
}

// Dial connects to a ws or wss address and runs the opening handshake of RFC
// 6455 section 4.1. It uses net/http for the request, so it handles the
// default ports, IPv6 hosts, user information in the address and TLS. It uses
// no proxy, and it does not follow a redirect. The context limits the dial.
// When Dial returns, the context has no effect on the connection.
func (d *Dialer) Dial(ctx context.Context, address string) (*Conn, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("parsing the websocket address: %w", err)
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	default:
		return nil, fmt.Errorf("%w: the scheme of %q is not ws or wss", ErrHandshake, address)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("%w: the address %q has no host", ErrHandshake, address)
	}
	u.Fragment = ""

	key, err := newKey()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("making the handshake request: %w", err)
	}
	// Add canonicalizes each key, so that a key such as "x-test" or
	// "Sec-WebSocket-Key" cannot hide from the checks below.
	for k, vs := range d.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if h := req.Header.Get("Host"); h != "" {
		req.Host = h
		req.Header.Del("Host")
	}
	req.Header.Del("Sec-WebSocket-Extensions")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", key)

	nc := new(connRecorder)
	tr := newTransport(d, nc)
	// The transport has no idle connection to keep. This call releases what
	// a failed dial left, on every path out of Dial.
	defer tr.CloseIdleConnections()
	client := &http.Client{
		Transport: tr,
		// RFC 6455 section 4.2.2 lets a client follow a redirect, but a
		// DevTools address does not redirect, so the dial reports it.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		// A server can close the connection before it answers. The transport
		// reports that as io.EOF when it sees the close while it reads the
		// response. When the close arrives first, it reports "server closed
		// idle connection", which is the same event. The dialer gives both
		// the same error value.
		if strings.Contains(err.Error(), "server closed idle connection") {
			err = fmt.Errorf("%w: %w", io.EOF, err)
		}
		return nil, fmt.Errorf("sending the handshake request: %w", err)
	}
	protocol, err := checkResponse(resp, key, req.Header.Values("Sec-WebSocket-Protocol"))
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	// The transport gives the stream of a 101 response as a body that can
	// write too.
	rwc, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		resp.Body.Close()
		return nil, fmt.Errorf("%w: the response body cannot write", ErrHandshake)
	}
	var dl deadliner
	if c := nc.get(); c != nil {
		dl = c
	}
	c, err := newConn(rwc, dl)
	if err != nil {
		rwc.Close()
		return nil, err
	}
	c.subprotocol = protocol
	if d.MaxMessageSize > 0 {
		c.maxMessage = d.MaxMessageSize
	}
	return c, nil
}

// newKey makes the Sec-WebSocket-Key value: 16 random bytes in base64, as RFC
// 6455 section 4.1 says.
func newKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("making the websocket key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b[:]), nil
}

// acceptValue returns the Sec-WebSocket-Accept value for a key. RFC 6455
// section 4.2.2 says it is the base64 of the SHA-1 of the key and the GUID.
func acceptValue(key string) string {
	sum := sha1.Sum([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// checkResponse checks the handshake response as RFC 6455 section 4.2.2 says.
// It returns the subprotocol that the server chose. The requested argument
// lists the Sec-WebSocket-Protocol values that the request had. For a response
// that is not a 101 response, the error has the status and the start of the
// body.
func checkResponse(resp *http.Response, key string, requested []string) (string, error) {
	if resp.StatusCode != http.StatusSwitchingProtocols {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return "", fmt.Errorf("%w: unexpected status %q, body %q",
			ErrHandshake, resp.Status, strings.TrimSpace(string(snippet)))
	}
	if !hasToken(resp.Header.Values("Upgrade"), "websocket") {
		return "", fmt.Errorf("%w: the Upgrade header is %q", ErrHandshake, resp.Header.Values("Upgrade"))
	}
	if !hasToken(resp.Header.Values("Connection"), "upgrade") {
		return "", fmt.Errorf("%w: the Connection header is %q", ErrHandshake, resp.Header.Values("Connection"))
	}
	accept := resp.Header.Values("Sec-WebSocket-Accept")
	if len(accept) != 1 || accept[0] != acceptValue(key) {
		return "", fmt.Errorf("%w: the Sec-WebSocket-Accept header is wrong", ErrHandshake)
	}
	// The client asks for no extension, so the server must not choose one.
	if v := resp.Header.Values("Sec-WebSocket-Extensions"); len(v) > 0 {
		return "", fmt.Errorf("%w: the server chose the extensions %q that the client did not ask for", ErrHandshake, v)
	}
	var protocol string
	if v := resp.Header.Values("Sec-WebSocket-Protocol"); len(v) > 0 {
		// RFC 6455 section 4.1 says that the server must echo a value that
		// the client sent. The comparison is exact, with no case folding.
		if len(v) > 1 || !hasExactToken(requested, strings.TrimSpace(v[0])) {
			return "", fmt.Errorf("%w: the server chose the subprotocol %q that the client did not ask for", ErrHandshake, v)
		}
		protocol = strings.TrimSpace(v[0])
	}
	return protocol, nil
}

// hasToken reports whether one of the header values has the token. The values
// can hold a list that a comma separates. The comparison ignores the case.
func hasToken(values []string, token string) bool {
	for _, v := range values {
		for t := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// hasExactToken is like hasToken, but it compares the case too.
func hasExactToken(values []string, token string) bool {
	for _, v := range values {
		for t := range strings.SplitSeq(v, ",") {
			if strings.TrimSpace(t) == token {
				return true
			}
		}
	}
	return false
}

// connRecorder remembers the network connection that the transport dialed, so
// that a connection can set a write deadline. The transport dials in another
// goroutine.
type connRecorder struct {
	mu   sync.Mutex
	conn net.Conn
}

func (r *connRecorder) set(c net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conn = c
}

func (r *connRecorder) get() net.Conn {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conn
}

// newTransport makes the transport for one dial. It has no proxy, so the
// environment variables HTTP_PROXY and HTTPS_PROXY have no effect. It does not
// offer HTTP/2, because an upgrade needs HTTP/1.1.
func newTransport(d *Dialer, rec *connRecorder) *http.Transport {
	nd := net.Dialer{Timeout: cmp.Or(d.DialTimeout, defaultDialTimeout)}
	return &http.Transport{
		Proxy:               nil,
		TLSClientConfig:     d.TLSConfig,
		TLSHandshakeTimeout: cmp.Or(d.TLSHandshakeTimeout, defaultTLSHandshakeTimeout),
		// A non-nil empty map turns HTTP/2 off.
		TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := nd.DialContext(ctx, network, addr)
			if err == nil {
				rec.set(c)
			}
			return c, err
		},
	}
}
