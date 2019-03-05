package chromedp

import (
	"io"
	"net"
	"strings"

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
	Read() ([]byte, error)
	Write([]byte) error
	io.Closer
}

// Conn wraps a gorilla/websocket.Conn connection.
type Conn struct {
	*websocket.Conn
}

// Read reads the next message.
func (c *Conn) Read() ([]byte, error) {
	_, buf, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// Write writes a message.
func (c *Conn) Write(buf []byte) error {
	return c.WriteMessage(websocket.TextMessage, buf)
}

// Dial dials the specified websocket URL using gorilla/websocket.
func Dial(urlstr string) (*Conn, error) {
	d := &websocket.Dialer{
		ReadBufferSize:  DefaultReadBufferSize,
		WriteBufferSize: DefaultWriteBufferSize,
	}

	// connect
	conn, _, err := d.Dial(urlstr, nil)
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
