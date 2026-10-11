// Package wstest is a minimal WebSocket server for tests. It follows RFC 6455
// and shares no code with the client in the package websocket, so that a test
// does not only check the client against itself. It builds each frame by hand
// and lets a test send frames that a correct server never sends.
//
// Only tests import this package.
package wstest

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Opcodes of RFC 6455 section 5.2.
const (
	OpContinuation = 0x0
	OpText         = 0x1
	OpBinary       = 0x2
	OpClose        = 0x8
	OpPing         = 0x9
	OpPong         = 0xa
)

// Conn is the server end of a connection after the handshake.
type Conn struct {
	net.Conn

	// R reads from the connection. It can hold bytes that came with the
	// request.
	R *bufio.Reader
}

// Frame is a frame that the server read. The payload is unmasked.
type Frame struct {
	Fin     bool
	Opcode  byte
	Payload []byte
}

// AcceptKey returns the Sec-WebSocket-Accept value for a key, as RFC 6455
// section 4.2.2 describes.
func AcceptKey(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// Upgrade checks that the request is a WebSocket handshake, takes over the
// connection and writes the 101 response. The extra headers go in the response.
// An Accept header in extra replaces the correct value. It returns an error and
// writes a 400 response for a request that is not a valid handshake.
func Upgrade(w http.ResponseWriter, r *http.Request, extra http.Header) (*Conn, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	raw, err := base64.StdEncoding.DecodeString(key)
	switch {
	case r.Method != http.MethodGet:
		err = fmt.Errorf("the method is %s", r.Method)
	case !strings.EqualFold(r.Header.Get("Upgrade"), "websocket"):
		err = fmt.Errorf("the Upgrade header is %q", r.Header.Get("Upgrade"))
	case !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade"):
		err = fmt.Errorf("the Connection header is %q", r.Header.Get("Connection"))
	case r.Header.Get("Sec-WebSocket-Version") != "13":
		err = fmt.Errorf("the version is %q", r.Header.Get("Sec-WebSocket-Version"))
	case err != nil || len(raw) != 16:
		err = fmt.Errorf("the key %q is not 16 bytes in base64", key)
	default:
		err = nil
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, err
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("the response writer cannot hijack")
	}
	nc, brw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"
	accept := AcceptKey(key)
	for k, v := range extra {
		if strings.EqualFold(k, "Sec-WebSocket-Accept") {
			accept = v[0]
			continue
		}
		for _, s := range v {
			resp += k + ": " + s + "\r\n"
		}
	}
	resp += "Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := io.WriteString(nc, resp); err != nil {
		nc.Close()
		return nil, err
	}
	return &Conn{Conn: nc, R: brw.Reader}, nil
}

// NewServer starts a test server that upgrades each request and calls handle
// with the request and the connection. It closes the connection when handle
// returns. The server closes when the test ends. The test fails when an upgrade
// fails.
func NewServer(t testing.TB, handle func(r *http.Request, c *Conn)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(Handler(t, handle))
	t.Cleanup(srv.Close)
	return srv
}

// Handler returns the handler of NewServer, for a test that makes its own
// server.
func Handler(t testing.TB, handle func(r *http.Request, c *Conn)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrading the request: %v", err)
			return
		}
		defer c.Close()
		handle(r, c)
	})
}

// URL returns the ws address of an HTTP test server.
func URL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// Encode returns an unmasked frame. It writes the shortest length encoding.
func Encode(fin bool, op byte, payload []byte) []byte {
	b0 := op
	if fin {
		b0 |= 0x80
	}
	b := []byte{b0}
	switch n := len(payload); {
	case n <= 125:
		b = append(b, byte(n))
	case n <= 0xffff:
		b = append(b, 126)
		b = binary.BigEndian.AppendUint16(b, uint16(n))
	default:
		b = append(b, 127)
		b = binary.BigEndian.AppendUint64(b, uint64(n))
	}
	return append(b, payload...)
}

// WriteFrame sends an unmasked frame with the FIN bit set.
func (c *Conn) WriteFrame(op byte, payload []byte) error {
	return c.WriteRaw(Encode(true, op, payload))
}

// WriteText sends an unmasked text frame.
func (c *Conn) WriteText(s string) error {
	return c.WriteFrame(OpText, []byte(s))
}

// WriteRaw writes bytes as they are.
func (c *Conn) WriteRaw(b []byte) error {
	_, err := c.Conn.Write(b)
	return err
}

// ReadFrame reads one frame from the client and unmasks it. It gives up after
// five seconds. It fails when the client does not mask the frame, as RFC 6455
// section 5.1 requires.
func (c *Conn) ReadFrame() (Frame, error) {
	var f Frame
	c.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var h [2]byte
	if _, err := io.ReadFull(c.R, h[:]); err != nil {
		return f, err
	}
	f.Fin = h[0]&0x80 != 0
	f.Opcode = h[0] & 0x0f
	if h[0]&0x70 != 0 {
		return f, fmt.Errorf("the client set a reserved bit")
	}
	if h[1]&0x80 == 0 {
		return f, fmt.Errorf("the client did not mask a frame")
	}
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var e [2]byte
		if _, err := io.ReadFull(c.R, e[:]); err != nil {
			return f, err
		}
		n = uint64(binary.BigEndian.Uint16(e[:]))
	case 127:
		var e [8]byte
		if _, err := io.ReadFull(c.R, e[:]); err != nil {
			return f, err
		}
		n = binary.BigEndian.Uint64(e[:])
	}
	var key [4]byte
	if _, err := io.ReadFull(c.R, key[:]); err != nil {
		return f, err
	}
	f.Payload = make([]byte, n)
	if _, err := io.ReadFull(c.R, f.Payload); err != nil {
		return f, err
	}
	for i := range f.Payload {
		f.Payload[i] ^= key[i%4]
	}
	return f, nil
}
