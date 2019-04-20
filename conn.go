package chromedp

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"

	"github.com/chromedp/cdproto"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
)

var (
	// DefaultReadBufferSize is the default maximum read buffer size.
	DefaultReadBufferSize = 25 * 1024 * 1024

	// DefaultWriteBufferSize is the default maximum write buffer size.
	DefaultWriteBufferSize = 10 * 1024 * 1024
)

// Transport is the common interface to send/receive messages to a target.
type Transport interface {
	Read(*cdproto.Message) error
	Write(*cdproto.Message) error
	io.Closer
}

// Conn wraps a gorilla/websocket.Conn connection.
type Conn struct {
	*websocket.Conn

	// buf helps us reuse space when reading from the websocket.
	buf bytes.Buffer

	// reuse the easyjson structs to avoid allocs per Read/Write.
	lexer  jlexer.Lexer
	writer jwriter.Writer

	dbgf func(string, ...interface{})
}

// DialContext dials the specified websocket URL using gorilla/websocket.
func DialContext(ctx context.Context, urlstr string, opts ...DialOption) (*Conn, error) {
	d := &websocket.Dialer{
		ReadBufferSize:  DefaultReadBufferSize,
		WriteBufferSize: DefaultWriteBufferSize,
	}

	// connect
	conn, _, err := d.DialContext(ctx, urlstr, nil)
	if err != nil {
		return nil, err
	}

	// apply opts
	c := &Conn{
		Conn: conn,
	}
	for _, o := range opts {
		o(c)
	}

	return c, nil
}

func (c *Conn) bufReadAll(r io.Reader) ([]byte, error) {
	c.buf.Reset()
	_, err := c.buf.ReadFrom(r)
	return c.buf.Bytes(), err
}

// Read reads the next message.
func (c *Conn) Read(msg *cdproto.Message) error {
	// get websocket reader
	typ, r, err := c.NextReader()
	if err != nil {
		return err
	}
	if typ != websocket.TextMessage {
		return ErrInvalidWebsocketMessage
	}

	// Unmarshal via a bytes.Buffer. Don't use UnmarshalFromReader, as that
	// uses ioutil.ReadAll, which uses a brand new bytes.Buffer each time.
	// That doesn't reuse any space.
	buf, err := c.bufReadAll(r)
	if err != nil {
		return err
	}
	if c.dbgf != nil {
		c.dbgf("<- %s", buf)
	}

	// Reuse the easyjson lexer.
	c.lexer = jlexer.Lexer{Data: buf}
	msg.UnmarshalEasyJSON(&c.lexer)
	if err := c.lexer.Error(); err != nil {
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
func (c *Conn) Write(msg *cdproto.Message) error {
	w, err := c.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	defer w.Close()

	// Reuse the easyjson writer.
	c.writer = jwriter.Writer{}

	// Perform the marshal.
	msg.MarshalEasyJSON(&c.writer)
	if err := c.writer.Error; err != nil {
		return err
	}

	// Write the bytes to the websocket.
	// BuildBytes consumes the buffer, so we can't use it as well as DumpTo.
	if c.dbgf != nil {
		buf, _ := c.writer.BuildBytes()
		c.dbgf("-> %s", buf)
		if _, err := w.Write(buf); err != nil {
			return err
		}
	} else {
		if _, err := c.writer.DumpTo(w); err != nil {
			return err
		}
	}
	return w.Close()
}

// ForceIP forces the host component in urlstr to be an IP address.
//
// Since Chrome 66+, Chrome DevTools Protocol clients connecting to a browser
// must send the "Host:" header as either an IP address, or "localhost".
func ForceIP(urlstr string) string {
	if i := strings.Index(urlstr, "://"); i != -1 {
		scheme := urlstr[:i+3]
		host, port, path := urlstr[len(scheme)+3:], "", ""
		if i := strings.Index(host, "/"); i != -1 {
			host, path = host[:i], host[i:]
		}
		if i := strings.Index(host, ":"); i != -1 {
			host, port = host[:i], host[i:]
		}
		if addr, err := net.ResolveIPAddr("ip", host); err == nil {
			urlstr = scheme + addr.IP.String() + port + path
		}
	}
	return urlstr
}

// DialOption is a dial option.
type DialOption func(*Conn)

// WithConnDebugf is a dial option to set a protocol logger.
func WithConnDebugf(f func(string, ...interface{})) DialOption {
	return func(c *Conn) {
		c.dbgf = f
	}
}
