// Package chromedp is a high level Chrome DevTools Protocol client that
// simplifies driving browsers for scraping, unit testing, or profiling web
// pages using the CDP.
//
// chromedp requires no third-party dependencies, implementing the async Chrome
// DevTools Protocol entirely in Go.
package chromedp

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	// Allocator is used to create new browsers. It is inherited from the
	// parent context when using NewContext.
	Allocator Allocator

	// Browser is the browser being used in the context. It is inherited
	// from the parent context when using NewContext.
	Browser *Browser

	// Target is the target to run actions (commands) against. It is not
	// inherited from the parent context, and typically each context will
	// have its own unique Target pointing to a separate browser tab (page).
	Target *Target

	// browserOpts holds the browser options passed to NewContext via
	// WithBrowserOption, so that they can later be used when allocating a
	// browser in Run.
	browserOpts []BrowserOption

	// cancel simply cancels the context that was used to start Browser.
	// This is useful to stop all activity and avoid deadlocks if we detect
	// that the browser was closed or happened to crash. Note that this
	// cancel function doesn't do any waiting.
	cancel func()

	// first records whether this context was the one that allocated
	// Browser. This is important, because its cancellation will stop the
	// entire browser handler, meaning that no further actions can be
	// executed.
	first bool

	// wg allows waiting for a target to be closed on cancellation.
	wg sync.WaitGroup

	// cancelErr is the first error encountered when cancelling this
	// context, for example if a browser's temporary user data directory
	// couldn't be deleted.
	cancelErr error
}

// NewContext creates a browser context using the parent context.
func NewContext(parent context.Context, opts ...ContextOption) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	c := &Context{cancel: cancel, first: true}
	if pc := FromContext(parent); pc != nil {
		c.Allocator = pc.Allocator
		c.Browser = pc.Browser
		// don't inherit SessionID, so that NewContext can be used to
		// create a new tab on the same browser.

		c.first = c.Browser == nil
	}

	for _, o := range opts {
		o(c)
	}
	if c.Allocator == nil {
		c.Allocator = setupExecAllocator(
			NoFirstRun,
			NoDefaultBrowserCheck,
			Headless,
		)
	}

	ctx = context.WithValue(ctx, contextKey{}, c)
	go func() {
		<-ctx.Done()
		if c.first {
			// This is the original browser tab, so the entire
			// browser will already be cleaned up elsewhere.
			c.wg.Done()
			return
		}

		// Not the original browser tab; simply detach and close it.
		// We need a new context, as ctx is cancelled; use a 1s timeout.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if id := c.Target.SessionID; id != "" {
			action := target.DetachFromTarget().WithSessionID(id)
			if err := action.Do(ctx, c.Browser); c.cancelErr == nil {
				c.cancelErr = err
			}
		}
		if id := c.Target.TargetID; id != "" {
			action := target.CloseTarget(id)
			if ok, err := action.Do(ctx, c.Browser); c.cancelErr == nil {
				if !ok && err == nil {
					err = fmt.Errorf("could not close target %q", id)
				}
				c.cancelErr = err
			}
		}
		c.wg.Done()
	}()
	cancelWait := func() {
		cancel()
		c.wg.Wait()
	}
	return ctx, cancelWait
}

type contextKey struct{}

// FromContext extracts the Context data stored inside a context.Context.
func FromContext(ctx context.Context) *Context {
	c, _ := ctx.Value(contextKey{}).(*Context)
	return c
}

// Cancel cancels a chromedp context, waits for its resources to be cleaned up,
// and returns any error encountered during that process.
//
// Usually a "defer cancel()" will be enough for most use cases. This API is
// useful if you want to catch underlying cancel errors, such as when a
// temporary directory cannot be deleted.
func Cancel(ctx context.Context) error {
	c := FromContext(ctx)
	if c == nil {
		return ErrInvalidContext
	}
	c.cancel()
	c.wg.Wait()
	return c.cancelErr
}

// Run runs an action against context. The provided context must be a valid
// chromedp context, typically created via NewContext.
func Run(ctx context.Context, actions ...Action) error {
	c := FromContext(ctx)
	// If c is nil, it's not a chromedp context.
	// If c.Allocator is nil, NewContext wasn't used properly.
	// If c.cancel is nil, Run is being called directly with an allocator
	// context.
	if c == nil || c.Allocator == nil || c.cancel == nil {
		return ErrInvalidContext
	}
	if c.Browser == nil {
		browser, err := c.Allocator.Allocate(ctx, c.browserOpts...)
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
	var targetID target.ID
	if c.first {
		// If we just allocated this browser, and it has a single page
		// that's blank and not attached, use it.
		infos, err := target.GetTargets().Do(ctx, c.Browser)
		if err != nil {
			return err
		}
		pages := 0
		for _, info := range infos {
			if info.Type == "page" && info.URL == "about:blank" && !info.Attached {
				targetID = info.TargetID
				pages++
			}
		}
		if pages > 1 {
			// Multiple blank pages; just in case, don't use any.
			targetID = ""
		}
	}

	if targetID == "" {
		var err error
		targetID, err = target.CreateTarget("about:blank").Do(ctx, c.Browser)
		if err != nil {
			return err
		}
	}

	sessionID, err := target.AttachToTarget(targetID).Do(ctx, c.Browser)
	if err != nil {
		return err
	}
	c.wg.Add(1)

	c.Target = c.Browser.newExecutorForTarget(ctx, targetID, sessionID)

	// enable domains
	for _, enable := range []Action{
		log.Enable(),
		runtime.Enable(),
		// network.Enable(),
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

// ContextOption is a context option.
type ContextOption func(*Context)

// WithLogf is a shortcut for WithBrowserOption(WithBrowserLogf(f)).
func WithLogf(f func(string, ...interface{})) ContextOption {
	return WithBrowserOption(WithBrowserLogf(f))
}

// WithErrorf is a shortcut for WithBrowserOption(WithBrowserErrorf(f)).
func WithErrorf(f func(string, ...interface{})) ContextOption {
	return WithBrowserOption(WithBrowserErrorf(f))
}

// WithDebugf is a shortcut for WithBrowserOption(WithBrowserDebugf(f)).
func WithDebugf(f func(string, ...interface{})) ContextOption {
	return WithBrowserOption(WithBrowserDebugf(f))
}

// WithBrowserOption allows passing a number of browser options to the allocator
// when allocating a new browser. As such, this context option can only be used
// when NewContext is allocating a new browser.
func WithBrowserOption(opts ...BrowserOption) ContextOption {
	return func(c *Context) {
		if !c.first {
			panic("WithBrowserOption can only be used when allocating a new browser")
		}
		c.browserOpts = append(c.browserOpts, opts...)
	}
}

// Targets lists all the targets in the browser attached to the given context.
func Targets(ctx context.Context) ([]*target.Info, error) {
	// Don't rely on Run, as that needs to be able to call Targets, and we
	// don't want cyclic func calls.
	c := FromContext(ctx)
	if c == nil || c.Allocator == nil {
		return nil, ErrInvalidContext
	}
	if c.Browser == nil {
		browser, err := c.Allocator.Allocate(ctx, c.browserOpts...)
		if err != nil {
			return nil, err
		}
		c.Browser = browser
	}
	return target.GetTargets().Do(ctx, c.Browser)
}
