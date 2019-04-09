package chromedp

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"strings"

	"github.com/chromedp/cdproto"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
)

var (
	// DefaultReadBufferSize is the default maximum read buffer size.
	DefaultReadBufferSize = 25 * 1024 * 1024

	// DefaultWriteBufferSize is the default maximum write buffer size.
	DefaultWriteBufferSize = 10 * 1024 * 1024
)

// Transport is the common interface to send/receive messages to a target.
type Transport interface {
	Read() (*cdproto.Message, error)
	Write(*cdproto.Message) error
	io.Closer
}

// Conn wraps a gorilla/websocket.Conn connection.
type Conn struct {
	*websocket.Conn
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

// Read reads the next message.
func (c *Conn) Read() (*cdproto.Message, error) {
	// get websocket reader
	typ, r, err := c.NextReader()
	if err != nil {
		return nil, err
	}
	if typ != websocket.TextMessage {
		return nil, ErrInvalidWebsocketMessage
	}

	// when dbgf defined, buffer, log, unmarshal
	if c.dbgf != nil {
		// buffer output
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		c.dbgf("<- %s", string(buf))
		msg := new(cdproto.Message)
		if err = easyjson.Unmarshal(buf, msg); err != nil {
			return nil, err
		}
		return msg, nil
	}

	// unmarshal direct from reader
	msg := new(cdproto.Message)
	if err = easyjson.UnmarshalFromReader(r, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// Write writes a message.
func (c *Conn) Write(msg *cdproto.Message) error {
	w, err := c.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if c.dbgf != nil {
		var buf []byte
		buf, err = easyjson.Marshal(msg)
		if err != nil {
			return err
		}
		c.dbgf("-> %s", string(buf))
		_, err = w.Write(buf)
		if err != nil {
			return err
		}
	} else {
		// direct marshal
		_, err = easyjson.MarshalToWriter(msg, w)
		if err != nil {
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
