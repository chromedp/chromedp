// Package chromedp is a high level Chrome DevTools Protocol client that
// simplifies driving browsers for scraping, unit testing, or profiling web
// pages using the CDP.
//
// chromedp requires no third-party dependencies, implementing the async Chrome
// DevTools Protocol entirely in Go.
package chromedp

import (
	"context"
	"log"
	"sync/atomic"

	"github.com/chromedp/cdproto"
	"github.com/mailru/easyjson"
)

// Browser is the high-level Chrome DevTools Protocol browser manager, handling
// the browser process runner, WebSocket clients, associated targets, and
// network, page, and DOM events.
type Browser struct {
	UserDataDir string

	conn Transport

	// next is the next message id.
	next int64

	// logging funcs
	logf func(string, ...interface{})
	errf func(string, ...interface{})
}

// NewBrowser creates a new browser.
func NewBrowser(urlstr string, opts ...BrowserOption) (*Browser, error) {
	conn, err := Dial(ForceIP(urlstr))
	if err != nil {
		return nil, err
	}

	b := &Browser{
		conn: conn,
		logf: log.Printf,
	}

	// apply options
	for _, o := range opts {
		if err := o(b); err != nil {
			return nil, err
		}
	}

	// ensure errf is set
	if b.errf == nil {
		b.errf = func(s string, v ...interface{}) { b.logf("ERROR: "+s, v...) }
	}

	return b, nil
}

// Shutdown shuts down the browser.
func (b *Browser) Shutdown() error {
	if b.conn != nil {
		if err := b.send(cdproto.CommandBrowserClose, nil); err != nil {
			b.errf("could not close browser: %v", err)
		}
		return b.conn.Close()
	}
	return nil
}

// send writes the supplied message and params.
func (b *Browser) send(method cdproto.MethodType, params easyjson.RawMessage) error {
	msg := &cdproto.Message{
		Method: method,
		ID:     atomic.AddInt64(&b.next, 1),
		Params: params,
	}
	buf, err := msg.MarshalJSON()
	if err != nil {
		return err
	}
	return b.conn.Write(buf)
}

// sendToTarget writes the supplied message to the target.
func (b *Browser) sendToTarget(targetID string, method cdproto.MethodType, params easyjson.RawMessage) error {
	return nil
}

// CreateContext creates a new browser context.
func (b *Browser) CreateContext() (context.Context, error) {
	return nil, nil
}

// BrowserOption is a browser option.
type BrowserOption func(*Browser) error

// WithLogf is a browser option to specify a func to receive general logging.
func WithLogf(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) error {
		b.logf = f
		return nil
	}
}

// WithErrorf is a browser option to specify a func to receive error logging.
func WithErrorf(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) error {
		b.errf = f
		return nil
	}
}

// WithConsolef is a browser option to specify a func to receive chrome log events.
//
// Note: NOT YET IMPLEMENTED.
func WithConsolef(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) error {
		return nil
	}
}
