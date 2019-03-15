package chromedp

import (
	"context"
	"io"
	"net"
	"strings"

	"github.com/chromedp/cdproto"
	"github.com/gorilla/websocket"
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
}

// Read reads the next message.
func (c *Conn) Read() (*cdproto.Message, error) {
	msg := new(cdproto.Message)
	if err := c.ReadJSON(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// Write writes a message.
func (c *Conn) Write(msg *cdproto.Message) error {
	return c.WriteJSON(msg)
}

// Dial dials the specified websocket URL using gorilla/websocket.
func DialContext(ctx context.Context, urlstr string) (*Conn, error) {
	d := &websocket.Dialer{
		ReadBufferSize:  DefaultReadBufferSize,
		WriteBufferSize: DefaultWriteBufferSize,
	}

	// connect
	conn, _, err := d.DialContext(ctx, urlstr, nil)
	if err != nil {
		return nil, err
	}

	return &Conn{conn}, nil
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
