package chromedp

import (
	"context"
	"encoding/json"
	"net/http"
)

// Executor
type Executor interface {
	Execute(context.Context, string, json.Marshaler, json.Unmarshaler) error
}

// Context
type Context struct {
	Allocator Allocator

	browser *Browser
	handler *TargetHandler

	logf func(string, ...interface{})
	errf func(string, ...interface{})
}

// Wait can be called after cancelling the context containing Context, to block
// until all the underlying resources have been cleaned up.
func (c *Context) Wait() {
	if c.Allocator != nil {
		c.Allocator.Wait()
	}
}

// NewContext creates a browser context using the parent context.
func NewContext(parent context.Context, opts ...ContextOption) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	c := &Context{}
	if pc := FromContext(parent); pc != nil {
		c.Allocator = pc.Allocator
	}

	for _, o := range opts {
		o(c)
	}
	if c.Allocator == nil {
		WithExecAllocator(
			NoFirstRun,
			NoDefaultBrowserCheck,
			Headless,
		)(&c.Allocator)
	}

	ctx = context.WithValue(ctx, contextKey{}, c)
	return ctx, cancel
}

type contextKey struct{}

// FromContext creates a new browser context from the provided context.
func FromContext(ctx context.Context) *Context {
	c, _ := ctx.Value(contextKey{}).(*Context)
	return c
}

// Run runs the action against the provided browser context.
func Run(ctx context.Context, action Action) error {
	c := FromContext(ctx)
	if c == nil || c.Allocator == nil {
		return ErrInvalidContext
	}
	if c.browser == nil {
		browser, err := c.Allocator.Allocate(ctx)
		if err != nil {
			return err
		}
		c.browser = browser
	}
	if c.handler == nil {
		if err := c.newHandler(ctx); err != nil {
			return err
		}
	}
	return action.Do(ctx, c.handler)
}

func (c *Context) newHandler(ctx context.Context) error {
	// TODO: add RemoteAddr() to the Transport interface?
	conn := c.browser.conn.(*Conn).Conn
	addr := conn.RemoteAddr()
	url := "http://" + addr.String() + "/json/new"
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var wurl withWebsocketURL
	if err := json.NewDecoder(resp.Body).Decode(&wurl); err != nil {
		return err
	}
	c.handler, err = NewTargetHandler(wurl.WebsocketURL)
	if err != nil {
		return err
	}
	if err := c.handler.Run(ctx); err != nil {
		return err
	}
	return nil
}

type withWebsocketURL struct {
	WebsocketURL string `json:"webSocketDebuggerUrl"`
}

// ContextOption
type ContextOption func(*Context)
