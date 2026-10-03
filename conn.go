package chromedp

import (
	"bytes"
	"context"
	"io"
	"net"

	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"github.com/chromedp/cdproto"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// Transport is the common interface to send and receive the protocol messages
// of a browser. Conn and PipeConn implement it. Browser reads and writes its
// messages through it, and NewBrowserTransport accepts any Transport.
type Transport interface {
	Read(context.Context, *cdproto.Message) error
	Write(context.Context, *cdproto.Message) error
	io.Closer
}

// Conn implements Transport with a gobwas/ws websocket connection.
type Conn struct {
	conn net.Conn

	// reuse the websocket reader and writer to avoid an alloc per
	// Read/Write.
	reader wsutil.Reader
	writer wsutil.Writer

	// reuse the easyjson structs to avoid allocs per Read/Write.
	decoder jsontext.Decoder
	encoder jsontext.Encoder

	debugf func(string, ...any)
}

// DialContext dials the websocket URL with gobwas/ws.
func DialContext(ctx context.Context, urlstr string, opts ...DialOption) (*Conn, error) {
	// connect
	conn, br, _, err := ws.Dial(ctx, urlstr)
	if err != nil {
		return nil, err
	}
	if br != nil {
		panic("br should be nil")
	}

	// apply opts
	c := &Conn{
		conn: conn,
		// pass 0 to use the default initial buffer size (4KiB).
		// github.com/gobwas/ws will grow the buffer size if needed.
		writer: *wsutil.NewWriterBufferSize(conn, ws.StateClientSide, ws.OpText, 0),
	}
	for _, o := range opts {
		o(c)
	}

	return c, nil
}

// Close satisfies the io.Closer interface.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Read reads the next message.
func (c *Conn) Read(_ context.Context, msg *cdproto.Message) error {
	// get websocket reader
	c.reader = wsutil.Reader{Source: c.conn, State: ws.StateClientSide}
	h, err := c.reader.NextFrame()
	if err != nil {
		return err
	}

	if h.OpCode == ws.OpPing { // ping
		if c.debugf != nil {
			c.debugf("received ping frame, ignoring...")
		}
		return nil
	} else if h.OpCode == ws.OpClose { // close
		if c.debugf != nil {
			c.debugf("received close frame")
		}
		return io.EOF
	} else if h.OpCode != ws.OpText {
		if c.debugf != nil {
			c.debugf("unknown OpCode: %s", h.OpCode)
		}
		return ErrInvalidWebsocketMessage
	}

	var b bytes.Buffer
	if _, err := b.ReadFrom(&c.reader); err != nil {
		return err
	}

	if c.debugf != nil {
		c.debugf("<- %s", b.Bytes())
	}

	// unmarshal, and reuse the decoder
	c.decoder.Reset(&b, DefaultUnmarshalOptions)
	return jsonv2.UnmarshalDecode(&c.decoder, msg, DefaultUnmarshalOptions)
}

// Write writes a message.
func (c *Conn) Write(_ context.Context, msg *cdproto.Message) error {
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
	c.encoder.Reset(&b, DefaultMarshalOptions)
	if err := jsonv2.MarshalEncode(&c.encoder, msg, DefaultMarshalOptions); err != nil {
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

// WithConnDebugf is a dial option to set a protocol logger.
func WithConnDebugf(f func(string, ...any)) DialOption {
	return func(c *Conn) {
		c.debugf = f
	}
}
