package chromedp

import (
	"bytes"
	"context"
	"io"
	"net"

	"github.com/chromedp/cdproto"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
)

// Transport is the common interface to send/receive messages to a target.
type Transport interface {
	Read(context.Context, *cdproto.Message) error
	Write(context.Context, *cdproto.Message) error
	io.Closer
}

// Conn implements Transport with a gobwas/ws websocket connection.
type Conn struct {
	conn net.Conn

	// buf helps us reuse space when reading from the websocket.
	buf bytes.Buffer

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

func (c *Conn) bufReadAll(r io.Reader) ([]byte, error) {
	c.buf.Reset()
	_, err := c.buf.ReadFrom(r)
	return c.buf.Bytes(), err
}

func unmarshal(lex *jlexer.Lexer, data []byte, v easyjson.Unmarshaler) error {
	*lex = jlexer.Lexer{Data: data}
	v.UnmarshalEasyJSON(lex)
	return lex.Error()
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

	// Unmarshal via a bytes.Buffer. Don't use UnmarshalFromReader, as that
	// uses ioutil.ReadAll, which uses a brand new bytes.Buffer each time.
	// That doesn't reuse any space.
	buf, err := c.bufReadAll(&c.reader)
	if err != nil {
		return err
	}
	if c.dbgf != nil {
		c.dbgf("<- %s", buf)
	}

	// Reuse the easyjson lexer.
	if err := unmarshal(&c.decoder, buf, msg); err != nil {
		return err
	}

	// bufReadAll uses the buffer space directly, and msg.Result is an
	// easyjson.RawMessage, so we must make a copy of those bytes to prevent
	// data races. This still allocates much less than using a new buffer
	// each time.
	msg.Result = append([]byte{}, msg.Result...)
	return nil
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
type DialOption func(*Conn)

// WithConnDebugf is a dial option to set a protocol logger.
func WithConnDebugf(f func(string, ...interface{})) DialOption {
	return func(c *Conn) {
		c.dbgf = f
	}
}
