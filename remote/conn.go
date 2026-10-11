// Package remote connects chromedp to a browser with a websocket. It holds the
// websocket connection, the allocator for a browser that runs already, the
// websocket mode of the exec allocator and the option that keeps a browser
// open. The package has its own WebSocket client, so it needs no library other
// than the standard library and cdproto. The pipe transport of the core does
// not import it.
//
// Use [NewAllocator] to attach to a browser that runs already, for example a
// hosted browser service or a container. Use [WebSocket] to make the exec
// allocator of chromedp connect with a websocket, and [WithKeepOpen] to leave
// the browser open when the program ends.
package remote

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/chromedp/cdproto"
	jsonv2 "github.com/chromedp/cdproto/cdp/jsonv2"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote/internal/websocket"
)

// ErrInvalidMessage is the error of a websocket frame that is neither a text
// frame nor a control frame.
var ErrInvalidMessage = errors.New("invalid websocket message")

// Conn implements [chromedp.Transport] with a websocket connection. The client
// is in the package remote/internal/websocket.
type Conn struct {
	ws *websocket.Conn

	// reuse the decoder to avoid allocs per Read. Only the reader goroutine
	// uses them.
	decoder jsonv2.Decoder
	reader  bytes.Reader

	// encMu makes Write use the encoder and its buffer one call at a time.
	// The websocket connection has its own lock for the frames, so a pong
	// frame of Read never splits a message.
	encMu   sync.Mutex
	encoder jsonv2.Encoder
	encBuf  bytes.Buffer

	// header is the set of HTTP headers that the dial sends with the
	// handshake request.
	header http.Header

	debugf func(string, ...any)
}

// DialContext dials the websocket URL.
func DialContext(ctx context.Context, urlstr string, opts ...DialOption) (*Conn, error) {
	// apply opts, as the dial needs the header
	c := &Conn{}
	for _, o := range opts {
		o(c)
	}

	// connect
	dialer := websocket.Dialer{Header: c.header}
	ws, err := dialer.Dial(ctx, urlstr)
	if err != nil {
		return nil, err
	}
	c.ws = ws
	ws.Logf = c.logf
	return c, nil
}

// logf sends a line to the protocol logger, if the connection has one.
func (c *Conn) logf(format string, args ...any) {
	if c.debugf != nil {
		c.debugf(format, args...)
	}
}

// SetDebugf sets the protocol logger. chromedp.NewBrowserTransport calls it
// with the logger of the browser.
func (c *Conn) SetDebugf(f func(string, ...any)) {
	c.debugf = f
}

// Close satisfies the io.Closer interface.
func (c *Conn) Close() error {
	return c.ws.Close()
}

// Read reads the next message. It answers a ping frame with a pong frame and
// skips a pong frame, as RFC 6455 section 5.5 requires. A server can send a
// ping at any time to check the connection. The method returns only when it
// has a text message or an error.
func (c *Conn) Read(_ context.Context, msg *cdproto.Message) error {
	op, data, err := c.ws.ReadMessage()
	if err != nil {
		return err
	}
	if op != websocket.OpText {
		c.logf("unknown OpCode: %s", op)
		return ErrInvalidMessage
	}
	c.logf("<- %s", data)

	// unmarshal, and reuse the decoder
	c.reader.Reset(data)
	c.decoder.Reset(&c.reader, chromedp.DefaultUnmarshalOptions)
	return jsonv2.UnmarshalDecode(&c.decoder, msg, chromedp.DefaultUnmarshalOptions)
}

// Write writes a message.
func (c *Conn) Write(_ context.Context, msg *cdproto.Message) error {
	c.encMu.Lock()
	defer c.encMu.Unlock()

	// Marshal the value, and reuse the encoder
	c.encBuf.Reset()
	c.encoder.Reset(&c.encBuf, chromedp.DefaultMarshalOptions)
	if err := jsonv2.MarshalEncode(&c.encoder, msg, chromedp.DefaultMarshalOptions); err != nil {
		return err
	}

	// Write the bytes to the websocket as one text frame. Chrome does not
	// support fragmentation of incoming websocket messages. Instead, it
	// supports single-fragment messages of up to 100MiB.
	//
	// See https://github.com/ChromeDevTools/devtools-protocol/issues/175.
	//
	// According to https://bugs.chromium.org/p/chromium/issues/detail?id=1069431,
	// Chrome will probably not support fragmentation soon.
	c.logf("-> %s", c.encBuf.Bytes())
	return c.ws.WriteText(c.encBuf.Bytes())
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
