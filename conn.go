package chromedp

import (
	"bytes"
	"context"
	"io"
	"net"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"

	"github.com/chromedp/cdproto"
)

// Transport is the common interface to send/receive messages to a target.
//
// This interface is currently used internally by Browser, but it is exposed as
// it will be useful as part of the public API in the future.
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
	decoder jlexer.Lexer
	encoder jwriter.Writer

	dbgf func(string, ...interface{})
}

// Chrome doesn't support fragmentation of incoming websocket messages. To
// compensate this, they support single-fragment messages of up to 100MiB.
//
// If our write buffer size is too small, large messages will get fragmented,
// and Chrome will silently crash. And if it's too large, chromedp will require
// more memory for all users.
//
// For now, make this a middle ground. 1MiB is large enough for practically any
// outgoing message, but small enough to not take too much meomry.
//
// See https://github.com/ChromeDevTools/devtools-protocol/issues/175.
const wsWriteBufferSize = 1 << 20

// DialContext dials the specified websocket URL using gobwas/ws.
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
		writer: *wsutil.NewWriterBufferSize(conn,
			ws.StateClientSide, ws.OpText, wsWriteBufferSize),
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
	if h.OpCode != ws.OpText {
		return ErrInvalidWebsocketMessage
	}

	var b bytes.Buffer
	if _, err := b.ReadFrom(&c.reader); err != nil {
		return err
	}
	buf := b.Bytes()
	if c.dbgf != nil {
		c.dbgf("<- %s", buf)
	}

	// unmarshal, reusing lexer
	c.decoder = jlexer.Lexer{Data: buf}
	msg.UnmarshalEasyJSON(&c.decoder)
	return c.decoder.Error()
}

// Write writes a message.
func (c *Conn) Write(_ context.Context, msg *cdproto.Message) error {
	c.writer.Reset(c.conn, ws.StateClientSide, ws.OpText)

	// Reuse the easyjson writer.
	c.encoder = jwriter.Writer{}

	// Perform the marshal.
	msg.MarshalEasyJSON(&c.encoder)
	if err := c.encoder.Error; err != nil {
		return err
	}

	// Write the bytes to the websocket.
	// BuildBytes consumes the buffer, so we can't use it as well as DumpTo.
	if c.dbgf != nil {
		buf, _ := c.encoder.BuildBytes()
		c.dbgf("-> %s", buf)
		if _, err := c.writer.Write(buf); err != nil {
			return err
		}
	} else {
		if _, err := c.encoder.DumpTo(&c.writer); err != nil {
			return err
		}
	}
	return c.writer.Flush()
}

// DialOption is a dial option.
type DialOption = func(*Conn)

// WithConnDebugf is a dial option to set a protocol logger.
func WithConnDebugf(f func(string, ...interface{})) DialOption {
	return func(c *Conn) {
		c.dbgf = f
	}
}
