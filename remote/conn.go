// Package remote connects chromedp to a browser with a websocket. It holds the
// websocket connection, the allocator for a browser that runs already, the
// websocket mode of the exec allocator and the option that keeps a browser
// open. The core module github.com/chromedp/chromedp has none of this code, so
// that it needs no library other than the standard library and cdproto.
//
// Use [NewAllocator] to attach to a browser that runs already, for example a
// hosted browser service or a container. Use [WebSocket] to make the exec
// allocator of chromedp connect with a websocket, and [WithKeepOpen] to leave
// the browser open when the program ends.
package remote

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"

	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"github.com/chromedp/cdproto"
	"github.com/chromedp/chromedp"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// ErrInvalidMessage is the error of a websocket frame that is neither a text
// frame nor a control frame.
var ErrInvalidMessage = errors.New("invalid websocket message")

// Conn implements [chromedp.Transport] with a gobwas/ws websocket connection.
type Conn struct {
	conn net.Conn

	// reuse the websocket reader and writer to avoid an alloc per
	// Read/Write.
	reader wsutil.Reader
	writer wsutil.Writer

	// writeMu makes Write and the pong frames of Read write one frame at a
	// time. The two run on different goroutines.
	writeMu sync.Mutex

	// reuse the easyjson structs to avoid allocs per Read/Write.
	decoder jsontext.Decoder
	encoder jsontext.Encoder

	// header is the set of HTTP headers that the dial sends with the
	// handshake request.
	header http.Header

	debugf func(string, ...any)
}

// DialContext dials the websocket URL with gobwas/ws.
func DialContext(ctx context.Context, urlstr string, opts ...DialOption) (*Conn, error) {
	// apply opts, as the dial needs the header
	c := &Conn{}
	for _, o := range opts {
		o(c)
	}

	// connect
	dialer := ws.Dialer{}
	if len(c.header) > 0 {
		dialer.Header = ws.HandshakeHeaderHTTP(c.header)
	}
	conn, br, _, err := dialer.Dial(ctx, urlstr)
	if err != nil {
		return nil, err
	}
	if br != nil {
		// The server sent frames right after the handshake, and the dialer
		// read them with the handshake. Read them first.
		conn = &bufferedConn{Conn: conn, r: br}
	}
	c.conn = conn
	// pass 0 to use the default initial buffer size (4KiB).
	// github.com/gobwas/ws will grow the buffer size if needed.
	c.writer = *wsutil.NewWriterBufferSize(conn, ws.StateClientSide, ws.OpText, 0)
	return c, nil
}

// bufferedConn is a net.Conn that reads from a buffer first. It holds the bytes
// that the handshake read after the end of the response.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

// Read reads from the buffer, and then from the connection.
func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

// SetDebugf sets the protocol logger. chromedp.NewBrowserTransport calls it
// with the logger of the browser.
func (c *Conn) SetDebugf(f func(string, ...any)) {
	c.debugf = f
}

// Close satisfies the io.Closer interface.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Read reads the next message. It answers a ping frame with a pong frame and
// skips a pong frame, as RFC 6455 section 5.5 requires. A server can send a
// ping at any time to check the connection. The method returns only when it
// has a text message or an error.
func (c *Conn) Read(_ context.Context, msg *cdproto.Message) error {
	// get websocket reader
	c.reader = wsutil.Reader{Source: c.conn, State: ws.StateClientSide}
	for {
		h, err := c.reader.NextFrame()
		if err != nil {
			return err
		}

		switch h.OpCode {
		case ws.OpPing:
			// The payload of a control frame has 125 bytes at most, so the
			// reader has checked its length.
			payload := make([]byte, h.Length)
			if _, err := io.ReadFull(&c.reader, payload); err != nil {
				return err
			}
			if c.debugf != nil {
				c.debugf("received ping frame, sending pong")
			}
			if err := c.writePong(payload); err != nil {
				return err
			}
			continue
		case ws.OpPong:
			// A pong answers a ping that this client did not send. Read its
			// payload, so that the next frame header is at the right place.
			if _, err := io.Copy(io.Discard, &c.reader); err != nil {
				return err
			}
			if c.debugf != nil {
				c.debugf("received pong frame, ignoring")
			}
			continue
		case ws.OpClose:
			if c.debugf != nil {
				c.debugf("received close frame")
			}
			return io.EOF
		case ws.OpText:
		default:
			if c.debugf != nil {
				c.debugf("unknown OpCode: %s", h.OpCode)
			}
			return ErrInvalidMessage
		}

		var b bytes.Buffer
		if _, err := b.ReadFrom(&c.reader); err != nil {
			return err
		}

		if c.debugf != nil {
			c.debugf("<- %s", b.Bytes())
		}

		// unmarshal, and reuse the decoder
		c.decoder.Reset(&b, chromedp.DefaultUnmarshalOptions)
		return jsonv2.UnmarshalDecode(&c.decoder, msg, chromedp.DefaultUnmarshalOptions)
	}
}

// writePong sends a pong frame with the payload of a ping frame. A client must
// mask every frame that it sends. The write lock keeps the frame apart from
// the frames of Write.
func (c *Conn) writePong(payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return ws.WriteFrame(c.conn, ws.MaskFrameInPlace(ws.NewPongFrame(payload)))
}

// Write writes a message.
func (c *Conn) Write(_ context.Context, msg *cdproto.Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.writer.Reset(c.conn, ws.StateClientSide, ws.OpText)
	// Chrome does not support fragmentation of incoming websocket messages.
	// Instead, it supports single-fragment messages of up to 100MiB.
	//
	// See https://github.com/ChromeDevTools/devtools-protocol/issues/175.
	//
	// According to https://bugs.chromium.org/p/chromium/issues/detail?id=1069431,
	// Chrome will probably not support fragmentation soon.
	// Luckily, github.com/gobwas/ws now grows the buffer when it needs to.
	// The func name DisableFlush is a little misleading,
	// but it does make the buffer grow when needed.
	c.writer.DisableFlush()

	// Marshal the value, and reuse the encoder
	var b bytes.Buffer
	c.encoder.Reset(&b, chromedp.DefaultMarshalOptions)
	if err := jsonv2.MarshalEncode(&c.encoder, msg, chromedp.DefaultMarshalOptions); err != nil {
		return err
	}

	// Write the bytes to the websocket.
	if c.debugf != nil {
		c.debugf("-> %s", b.Bytes())
	}
	if _, err := b.WriteTo(&c.writer); err != nil {
		return err
	}
	return c.writer.Flush()
}

// DialOption is a dial option.
type DialOption = func(*Conn)

// WithConnHTTPHeader is a dial option that sets HTTP headers on the websocket
// handshake request. For example, a hosted browser service can need an
// Authorization header. The option copies the header, so a later change of h
// has no effect.
func WithConnHTTPHeader(h http.Header) DialOption {
	return func(c *Conn) {
		c.header = h.Clone()
	}
}

// WithConnDebugf is a dial option to set a protocol logger.
func WithConnDebugf(f func(string, ...any)) DialOption {
	return func(c *Conn) {
		c.debugf = f
	}
}
