// Package chromedp is a high level Chrome DevTools Protocol client that
// simplifies driving browsers for scraping, unit testing, or profiling web
// pages using the CDP.
//
// chromedp requires no third-party dependencies, implementing the async Chrome
// DevTools Protocol entirely in Go.
//
// This package includes a number of simple examples. Additionally,
// https://github.com/chromedp/examples contains more complex examples.
package chromedp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
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

	// targetID is set up by WithTargetID. If nil, Run will pick the only
	// unused page target, or create a new one.
	targetID target.ID

	browserListeners []cancelableListener
	targetListeners  []cancelableListener

	// browserOpts holds the browser options passed to NewContext via
	// WithBrowserOption, so that they can later be used when allocating a
	// browser in Run.
	browserOpts []BrowserOption

	// cancel simply cancels the context that was used to start Browser.
	// This is useful to stop all activity and avoid deadlocks if we detect
	// that the browser was closed or happened to crash. Note that this
	// cancel function doesn't do any waiting.
	cancel func()

	// first records whether this context created a brand new Chrome
	// process. This is important, because its cancellation should stop the
	// entire browser and its handler, and not just a portion of its pages.
	first bool

	// closedTarget allows waiting for a target's page to be closed on
	// cancellation.
	closedTarget sync.WaitGroup

	// allocated is closed when an allocated browser completely stops. If no
	// browser needs to be allocated, the channel is simply not initialised
	// and remains nil.
	allocated chan struct{}

	// cancelErr is the first error encountered when cancelling this
	// context, for example if a browser's temporary user data directory
	// couldn't be deleted.
	cancelErr error
}

// NewContext creates a chromedp context from the parent context. The parent
// context's Allocator is inherited, defaulting to an ExecAllocator with
// DefaultExecAllocatorOptions.
//
// If the parent context contains an allocated Browser, the child context
// inherits it, and its first Run creates a new tab on that browser. Otherwise,
// its first Run will allocate a new browser.
//
// Cancelling the returned context will close a tab or an entire browser,
// depending on the logic described above. To cancel a context while checking
// for errors, see Cancel.
//
// Note that NewContext doesn't allocate nor start a browser; that happens the
// first time Run is used on the context.
func NewContext(parent context.Context, opts ...ContextOption) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	c := &Context{cancel: cancel, first: true}
	if pc := FromContext(parent); pc != nil {
		c.Allocator = pc.Allocator
		c.Browser = pc.Browser
		// don't inherit Target, so that NewContext can be used to
		// create a new tab on the same browser.

		c.first = c.Browser == nil

		// TODO: make this more generic somehow.
		if _, ok := c.Allocator.(*RemoteAllocator); ok {
			c.first = false
		}
	}
	if c.Browser == nil {
		// set up the semaphore for Allocator.Allocate
		c.allocated = make(chan struct{}, 1)
		c.allocated <- struct{}{}
	}

	for _, o := range opts {
		o(c)
	}
	if c.Allocator == nil {
		c.Allocator = setupExecAllocator(DefaultExecAllocatorOptions[:]...)
	}

	ctx = context.WithValue(ctx, contextKey{}, c)
	c.closedTarget.Add(1)
	go func() {
		<-ctx.Done()
		defer c.closedTarget.Done()
		if c.first {
			// This is the original browser tab, so the entire
			// browser will already be cleaned up elsewhere.
			return
		}

		if c.Target == nil {
			// This is a new tab, but we didn't create it and attach
			// to it yet. Nothing to do.
			return
		}

		// Not the original browser tab; simply detach and close it.
		// We need a new context, as ctx is cancelled; use a 1s timeout.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if id := c.Target.SessionID; id != "" {
			action := target.DetachFromTarget().WithSessionID(id)
			if err := action.Do(cdp.WithExecutor(ctx, c.Browser)); c.cancelErr == nil {
				c.cancelErr = err
			}
		}
		if id := c.Target.TargetID; id != "" {
			action := target.CloseTarget(id)
			if ok, err := action.Do(cdp.WithExecutor(ctx, c.Browser)); c.cancelErr == nil {
				if !ok && err == nil {
					err = fmt.Errorf("could not close target %q", id)
				}
				c.cancelErr = err
			}
		}
	}()
	cancelWait := func() {
		cancel()
		c.closedTarget.Wait()
		// If we allocated, wait for the browser to stop.
		if c.allocated != nil {
			<-c.allocated
		}
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
// If the context allocated a browser, the browser will be closed gracefully by
// Cancel.
//
// Usually a "defer cancel()" will be enough for most use cases. However, Cancel
// is the better option if one wants to gracefully close a browser, or catch
// underlying errors happening during cancellation.
func Cancel(ctx context.Context) error {
	c := FromContext(ctx)
	if c == nil {
		return ErrInvalidContext
	}
	if c.first && c.Browser != nil {
		if err := c.Browser.execute(ctx, browser.CommandClose, nil, nil); err != nil {
			return err
		}
	} else {
		c.cancel()
		c.closedTarget.Wait()
	}
	// If we allocated, wait for the browser to stop.
	if c.allocated != nil {
		<-c.allocated
	}
	return c.cancelErr
}

// Run runs an action against context. The provided context must be a valid
// chromedp context, typically created via NewContext.
//
// Note that the first time Run is called on a context, a browser will be
// allocated via Allocator. Thus, it's generally a bad idea to use a context
// timeout on the first Run call, as it will stop the entire browser.
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
		c.Browser.listeners = append(c.Browser.listeners, c.browserListeners...)
	}
	if c.Target == nil {
		if err := c.newTarget(ctx); err != nil {
			return err
		}
	}
	return Tasks(actions).Do(cdp.WithExecutor(ctx, c.Target))
}

func (c *Context) newTarget(ctx context.Context) error {
	if c.targetID != "" {
		if err := c.attachTarget(ctx, c.targetID); err != nil {
			return err
		}
		// This new page might have already loaded its top-level frame
		// already, in which case we wouldn't see the frameNavigated and
		// documentUpdated events. Load them here.
		tree, err := page.GetFrameTree().Do(cdp.WithExecutor(ctx, c.Target))
		if err != nil {
			return err
		}
		c.Target.cur = tree.Frame
		c.Target.documentUpdated(ctx)
		return nil
	}
	if !c.first {
		var err error
		c.targetID, err = target.CreateTarget("about:blank").Do(cdp.WithExecutor(ctx, c.Browser))
		if err != nil {
			return err
		}
		return c.attachTarget(ctx, c.targetID)
	}

	// This is like WaitNewTarget, but for the entire browser.
	ch := make(chan target.ID, 1)
	lctx, cancel := context.WithCancel(ctx)
	ListenBrowser(lctx, func(ev interface{}) {
		var info *target.Info
		switch ev := ev.(type) {
		case *target.EventTargetCreated:
			info = ev.TargetInfo
		case *target.EventTargetInfoChanged:
			info = ev.TargetInfo
		default:
			return
		}
		if info.Type == "page" && info.URL == "about:blank" {
			select {
			case <-lctx.Done():
			case ch <- info.TargetID:
			}
			cancel()
		}
	})

	// wait for the first blank tab to appear
	action := target.SetDiscoverTargets(true)
	if err := action.Do(cdp.WithExecutor(ctx, c.Browser)); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.targetID = <-ch:
	}
	return c.attachTarget(ctx, c.targetID)
}

func (c *Context) attachTarget(ctx context.Context, targetID target.ID) error {
	sessionID, err := target.AttachToTarget(targetID).WithFlatten(true).Do(cdp.WithExecutor(ctx, c.Browser))
	if err != nil {
		return err
	}

	c.Target, err = c.Browser.newExecutorForTarget(ctx, targetID, sessionID)
	if err != nil {
		return err
	}

	c.Target.listeners = append(c.Target.listeners, c.targetListeners...)
	go c.Target.run(ctx)

	for _, action := range []Action{
		// enable domains
		log.Enable(),
		runtime.Enable(),
		inspector.Enable(),
		page.Enable(),
		dom.Enable(),
		css.Enable(),

		// enable target discovery
		target.SetDiscoverTargets(true),
		target.SetAutoAttach(true, false).WithFlatten(true),
	} {
		if err := action.Do(cdp.WithExecutor(ctx, c.Target)); err != nil {
			return fmt.Errorf("unable to execute %T: %v", action, err)
		}
	}
	return nil
}

// ContextOption is a context option.
type ContextOption = func(*Context)

// WithTargetID sets up a context to be attached to an existing target, instead
// of creating a new one.
func WithTargetID(id target.ID) ContextOption {
	return func(c *Context) { c.targetID = id }
}

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
	if err := Run(ctx); err != nil {
		return nil, err
	}
	c := FromContext(ctx)
	return target.GetTargets().Do(cdp.WithExecutor(ctx, c.Browser))
}

// Action is the common interface for an action that will be executed against a
// context and frame handler.
type Action interface {
	// Do executes the action using the provided context and frame handler.
	Do(context.Context) error
}

// ActionFunc is a adapter to allow the use of ordinary func's as an Action.
type ActionFunc func(context.Context) error

// Do executes the func f using the provided context and frame handler.
func (f ActionFunc) Do(ctx context.Context) error {
	return f(ctx)
}

// Tasks is a sequential list of Actions that can be used as a single Action.
type Tasks []Action

// Do executes the list of Actions sequentially, using the provided context and
// frame handler.
func (t Tasks) Do(ctx context.Context) error {
	for _, a := range t {
		if err := a.Do(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Sleep is an empty action that calls time.Sleep with the specified duration.
//
// Note: this is a temporary action definition for convenience, and will likely
// be marked for deprecation in the future, after the remaining Actions have
// been able to be written/tested.
func Sleep(d time.Duration) Action {
	return ActionFunc(func(ctx context.Context) error {
		// Don't use time.After, to avoid a temporary goroutine leak if
		// ctx is cancelled before the timer fires.
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		return nil
	})
}

type cancelableListener struct {
	ctx context.Context
	fn  func(ev interface{})
}

// ListenBrowser adds a function which will be called whenever a browser event
// is received on the chromedp context. Cancelling ctx stops the listener from
// receiving any more events.
//
// Note that the function is called synchronously when handling events. The
// function should avoid blocking at all costs. For example, any Actions must be
// run via a separate goroutine.
func ListenBrowser(ctx context.Context, fn func(ev interface{})) {
	c := FromContext(ctx)
	if c == nil {
		panic(ErrInvalidContext)
	}
	cl := cancelableListener{ctx, fn}
	if c.Browser != nil {
		c.Browser.listenersMu.Lock()
		c.Browser.listeners = append(c.Browser.listeners, cl)
		c.Browser.listenersMu.Unlock()
	} else {
		c.browserListeners = append(c.browserListeners, cl)
	}
}

// ListenTarget adds a function which will be called whenever a target event is
// received on the chromedp context. Note that this only includes browser
// events; command responses and target events are not included. Cancelling ctx
// stops the listener from receiving any more events.
//
// Note that the function is called synchronously when handling events. The
// function should avoid blocking at all costs. For example, any Actions must be
// run via a separate goroutine.
func ListenTarget(ctx context.Context, fn func(ev interface{})) {
	c := FromContext(ctx)
	if c == nil {
		panic(ErrInvalidContext)
	}
	cl := cancelableListener{ctx, fn}
	if c.Target != nil {
		c.Target.listenersMu.Lock()
		c.Target.listeners = append(c.Target.listeners, cl)
		c.Target.listenersMu.Unlock()
	} else {
		c.targetListeners = append(c.targetListeners, cl)
	}
}

// WaitNewTarget can be used to wait for the current target to open a new
// target. Once fn matches a new unattached target, its target ID is sent via
// the returned channel.
func WaitNewTarget(ctx context.Context, fn func(*target.Info) bool) <-chan target.ID {
	ch := make(chan target.ID, 1)
	lctx, cancel := context.WithCancel(ctx)
	ListenTarget(lctx, func(ev interface{}) {
		var info *target.Info
		switch ev := ev.(type) {
		case *target.EventTargetCreated:
			info = ev.TargetInfo
		case *target.EventTargetInfoChanged:
			info = ev.TargetInfo
		default:
			return
		}
		if info.OpenerID == "" {
			return // not a child target
		}
		if info.Attached {
			return // already attached; not a new target
		}
		if fn(info) {
			select {
			case <-lctx.Done():
			case ch <- info.TargetID:
			}
			close(ch)
			cancel()
		}
	})
	return ch
}
