package chromedp

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/inspector"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
)

// Context is attached to any context.Context which is valid for use with Run.
type Context struct {
	Allocator Allocator

	Browser *Browser

	Target *Target
}

// NewContext creates a browser context using the parent context.
func NewContext(parent context.Context, opts ...ContextOption) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	c := &Context{}
	if pc := FromContext(parent); pc != nil {
		c.Allocator = pc.Allocator
		c.Browser = pc.Browser
		// don't inherit SessionID, so that NewContext can be used to
		// create a new tab on the same browser.
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

// FromContext extracts the Context data stored inside a context.Context.
func FromContext(ctx context.Context) *Context {
	c, _ := ctx.Value(contextKey{}).(*Context)
	return c
}

// Run runs an action against the provided context. The provided context must
// contain a valid Allocator; typically, that will be created via NewContext or
// NewAllocator.
func Run(ctx context.Context, actions ...Action) error {
	c := FromContext(ctx)
	if c == nil || c.Allocator == nil {
		return ErrInvalidContext
	}
	if c.Browser == nil {
		browser, err := c.Allocator.Allocate(ctx)
		if err != nil {
			return err
		}
		c.Browser = browser
	}
	if c.Target == nil {
		if err := c.newSession(ctx); err != nil {
			return err
		}
	}
	return Tasks(actions).Do(ctx, c.Target)
}

func (c *Context) newSession(ctx context.Context) error {
	create := target.CreateTarget("about:blank")
	targetID, err := create.Do(ctx, c.Browser)
	if err != nil {
		return err
	}

	attach := target.AttachToTarget(targetID)
	sessionID, err := attach.Do(ctx, c.Browser)
	if err != nil {
		return err
	}

	c.Target = c.Browser.newExecutorForTarget(ctx, sessionID)

	// enable domains
	for _, enable := range []Action{
		log.Enable(),
		runtime.Enable(),
		//network.Enable(),
		inspector.Enable(),
		page.Enable(),
		dom.Enable(),
		css.Enable(),
	} {
		if err := enable.Do(ctx, c.Target); err != nil {
			return fmt.Errorf("unable to execute %T: %v", enable, err)
		}
	}
	return nil
}

type ContextOption func(*Context)
