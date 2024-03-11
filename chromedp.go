// Package chromedp is a high level Chrome DevTools Protocol client that
// simplifies driving browsers for scraping, unit testing, or profiling web
// pages using the CDP.
//
// chromedp requires no third-party dependencies, implementing the async Chrome
// DevTools Protocol entirely in Go.
//
// This package includes a number of simple examples. Additionally,
// [chromedp/examples] contains more complex examples.
//
// [chromedp/examples]: https://github.com/chromedp/examples
package chromedp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/inspector"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
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

	// createBrowserContextParams is set up by WithNewBrowserContext. It is used
	// to create a new BrowserContext.
	createBrowserContextParams *target.CreateBrowserContextParams

	// browserContextOwner indicates whether this context is a BrowserContext
	// owner. The owner is responsible for disposing the BrowserContext once
	// the context is done.
	browserContextOwner bool

	// BrowserContextID is set up by WithExistingBrowserContext.
	//
	// Otherwise, BrowserContextID holds a non-empty value in the following cases:
	//
	// 1. if the context is created with the WithNewBrowserContext option, a new
	// BrowserContext is created on its first run, and BrowserContextID holds
	// the id of that new BrowserContext;
	//
	// 2. if the context is not created with the WithTargetID option, and its
	// parent context has a non-empty BrowserContextID, this context's
	// BrowserContextID is copied from the parent context.
	BrowserContextID cdp.BrowserContextID

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
	// browser needs to be allocated, the channel is simply not initialized
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
// for errors, see [Cancel].
//
// Note that NewContext doesn't allocate nor start a browser; that happens the
// first time Run is used on the context.
func NewContext(parent context.Context, opts ...ContextOption) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	c := &Context{cancel: cancel, first: true}
	var parentBrowserContextID cdp.BrowserContextID
	if pc := FromContext(parent); pc != nil {
		c.Allocator = pc.Allocator
		c.Browser = pc.Browser
		parentBrowserContextID = pc.BrowserContextID
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
	if c.createBrowserContextParams != nil && c.BrowserContextID != "" {
		panic("WithExistingBrowserContext can not be used when WithNewBrowserContext is specified")
	}
	if c.targetID == "" {
		if c.BrowserContextID == "" {
			// Inherit BrowserContextID from its parent context.
			c.BrowserContextID = parentBrowserContextID
		}
	} else {
		if c.createBrowserContextParams != nil {
			panic("WithNewBrowserContext can not be used when WithTargetID is specified")
		}
		if c.BrowserContextID != "" {
			panic("WithExistingBrowserContext can not be used when WithTargetID is specified")
		}
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
		browserExecutor := cdp.WithExecutor(ctx, c.Browser)
		if id := c.Target.SessionID; id != "" {
			action := target.DetachFromTarget().WithSessionID(id)
			if err := action.Do(browserExecutor); c.cancelErr == nil && err != nil {
				c.cancelErr = err
			}
		}
		if id := c.Target.TargetID; id != "" {
			action := target.CloseTarget(id)
			if err := action.Do(browserExecutor); c.cancelErr == nil && err != nil {
				c.cancelErr = err
			}
		}
		if c.browserContextOwner {
			action := target.DisposeBrowserContext(c.BrowserContextID)
			if err := action.Do(browserExecutor); c.cancelErr == nil && err != nil {
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
// Cancel. A timeout can be attached to this context to determine how long to
// wait for the browser to close itself:
//
//	tctx, tcancel := context.WithTimeout(ctx, 10 * time.Second)
//	defer tcancel()
//	chromedp.Cancel(tctx)
//
// Usually a "defer cancel()" will be enough for most use cases. However, Cancel
// is the better option if one wants to gracefully close a browser, or catch
// underlying errors happening during cancellation.
func Cancel(ctx context.Context) error {
	c := FromContext(ctx)
	// c.cancel is nil when Cancel is wrongly called with a context returned
	// by chromedp.NewExecAllocator or chromedp.NewRemoteAllocator.
	if c == nil || c.cancel == nil {
		return ErrInvalidContext
	}
	graceful := c.first && c.Browser != nil
	if graceful {
		close(c.Browser.closingGracefully)
		if err := c.Browser.execute(ctx, browser.CommandClose, nil, nil); err != nil {
			return err
		}
	} else {
		c.cancel()
		c.closedTarget.Wait()
	}
	// If we allocated, wait for the browser to stop, up to any possible
	// deadline set in this ctx.
	ready := false
	if c.allocated != nil {
		select {
		case <-c.allocated:
			ready = true
		case <-ctx.Done():
		}
	}
	// If this was a graceful close, cancel the entire context, in case any
	// goroutines or resources are left, or if we hit the timeout above and
	// the browser hasn't finished yet. Note that, in the non-graceful path,
	// we already called c.cancel above.
	if graceful {
		c.cancel()
	}

	// If we allocated and we hit ctx.Done earlier, we can't rely on
	// cancelErr being ready until the allocated channel is closed, as that
	// is racy. If we didn't hit ctx.Done earlier, then c.allocated was
	// already cancelled then, so this will be a no-op.
	if !ready && c.allocated != nil {
		<-c.allocated
	}
	return c.cancelErr
}

func initContextBrowser(ctx context.Context) (*Context, error) {
	c := FromContext(ctx)
	// If c is nil, it's not a chromedp context.
	// If c.Allocator is nil, NewContext wasn't used properly.
	// If c.cancel is nil, Run is being called directly with an allocator
	// context.
	if c == nil || c.Allocator == nil || c.cancel == nil {
		return nil, ErrInvalidContext
	}
	if c.Browser == nil {
		b, err := c.Allocator.Allocate(ctx, c.browserOpts...)
		if err != nil {
			return nil, err
		}
		c.Browser = b
		c.Browser.listeners = append(c.Browser.listeners, c.browserListeners...)
	}
	return c, nil
}

// Run runs an action against context. The provided context must be a valid
// chromedp context, typically created via NewContext.
//
// Note that the first time Run is called on a context, a browser will be
// allocated via Allocator. Thus, it's generally a bad idea to use a context
// timeout on the first Run call, as it will stop the entire browser.
//
// Also note that the actions are run with the Target executor. In the case that
// a Browser executor is required, the action can be written like this:
//
//	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
//		c := chromedp.FromContext(ctx)
//		id, err := target.CreateBrowserContext().Do(cdp.WithExecutor(ctx, c.Browser))
//		return err
//	}))
func Run(ctx context.Context, actions ...Action) error {
	c, err := initContextBrowser(ctx)
	if err != nil {
		return err
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
		// Since at the time of writing this (2020-1-27), Page.* CDP methods are
		// not implemented in worker targets, we need to skip this step when we
		// attach to workers.
		if !c.Target.isWorker {
			tree, err := page.GetFrameTree().Do(cdp.WithExecutor(ctx, c.Target))
			if err != nil {
				return err
			}

			c.Target.frameMu.Lock()
			c.Target.frames[tree.Frame.ID] = tree.Frame
			c.Target.cur = tree.Frame.ID
			c.Target.frameMu.Unlock()

			c.Target.documentUpdated(ctx)
		}
		return nil
	}
	if !c.first {
		var err error
		browserExecutor := cdp.WithExecutor(ctx, c.Browser)
		if c.createBrowserContextParams != nil {
			c.BrowserContextID, err = c.createBrowserContextParams.Do(browserExecutor)
			if err != nil {
				return err
			}
			c.browserContextOwner = true
			c.createBrowserContextParams = nil
		}
		c.targetID, err = target.CreateTarget("about:blank").WithBrowserContextID(c.BrowserContextID).Do(browserExecutor)
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
		// In the following cases, the browser will start with a non-blank tab:
		// 1. with the "--app" option (should disable headless mode);
		// 2. URL other than "about:blank" is placed in the command line arguments.
		// So we should not require that the URL to be "about:blank".
		// See issue https://github.com/chromedp/chromedp/issues/1076
		// In any cases that the browser starts with multiple tabs open,
		// it should be okay to attach to any one of them (no matter whether it
		// is blank).
		if info.Type == "page" {
			select {
			case <-lctx.Done():
			case ch <- info.TargetID:
			}
			cancel()
		}
	})

	// wait for the first tab to appear
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

	// Check if this is a worker target. We cannot use Target.getTargetInfo or
	// Target.getTargets in a worker, so we check if "self" refers to a
	// WorkerGlobalScope or ServiceWorkerGlobalScope.
	if err := runtime.Enable().Do(cdp.WithExecutor(ctx, c.Target)); err != nil {
		return err
	}
	res, _, err := runtime.Evaluate("self").Do(cdp.WithExecutor(ctx, c.Target))
	if err != nil {
		return err
	}
	c.Target.isWorker = strings.Contains(res.ClassName, "WorkerGlobalScope")

	// Enable available domains and discover targets.
	actions := []Action{
		log.Enable(),
		network.Enable(),
	}
	// These actions are not available on a worker target.
	if !c.Target.isWorker {
		actions = append(actions, []Action{
			inspector.Enable(),
			page.Enable(),
			dom.Enable(),
			css.Enable(),
			target.SetDiscoverTargets(true),
			target.SetAutoAttach(true, false).WithFlatten(true),
			page.SetLifecycleEventsEnabled(true),
		}...)
	}

	for _, action := range actions {
		if err := action.Do(cdp.WithExecutor(ctx, c.Target)); err != nil {
			return fmt.Errorf("unable to execute %T: %w", action, err)
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

// CreateBrowserContextOption is a BrowserContext creation options.
type CreateBrowserContextOption = func(*target.CreateBrowserContextParams) *target.CreateBrowserContextParams

// WithNewBrowserContext sets up a context to create a new BrowserContext, and
// create a new target in this BrowserContext. A child context will create its
// target in this BrowserContext too, unless it's set up with other options.
// The new BrowserContext will be disposed when the context is done.
func WithNewBrowserContext(options ...CreateBrowserContextOption) ContextOption {
	return func(c *Context) {
		if c.first {
			panic("WithNewBrowserContext can not be used before Browser is initialized")
		}

		params := target.CreateBrowserContext().WithDisposeOnDetach(true)
		for _, o := range options {
			params = o(params)
		}
		c.createBrowserContextParams = params
	}
}

// WithExistingBrowserContext sets up a context to create a new target in the
// specified browser context.
func WithExistingBrowserContext(id cdp.BrowserContextID) ContextOption {
	return func(c *Context) {
		if c.first {
			panic("WithExistingBrowserContext can not be used before Browser is initialized")
		}
		c.BrowserContextID = id
	}
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
		if c.Browser != nil {
			panic("WithBrowserOption can only be used when allocating a new browser")
		}
		c.browserOpts = append(c.browserOpts, opts...)
	}
}

// RunResponse is an alternative to Run which can be used with a list of actions
// that trigger a page navigation, such as clicking on a link or button.
//
// RunResponse will run the actions and block until a page loads, returning the
// HTTP response information for its HTML document. This can be useful to wait
// for the page to be ready, or to catch 404 status codes, for example.
//
// Note that if the actions trigger multiple navigations, only the first is
// used. And if the actions trigger no navigations at all, RunResponse will
// block until the context is cancelled.
func RunResponse(ctx context.Context, actions ...Action) (*network.Response, error) {
	var resp *network.Response
	if err := Run(ctx, responseAction(&resp, actions...)); err != nil {
		return nil, err
	}
	return resp, nil
}

func responseAction(resp **network.Response, actions ...Action) Action {
	return ActionFunc(func(ctx context.Context) error {
		// loaderID lets us filter the requests from the currently
		// loading navigation.
		var loaderID cdp.LoaderID

		// reqID is the request we're currently looking at. This can
		// go through multiple values, e.g. if the page redirects.
		var reqID network.RequestID

		// frameID corresponds to the target's root frame.
		var frameID cdp.FrameID

		var loadErr error
		hasInit := false
		finished := false

		// First, set up the function to handle events.
		// We are listening for lifecycle events, so we will use those to
		// make sure we grab the response for a request initiated by the
		// loaderID that we want.

		lctx, lcancel := context.WithCancel(ctx)
		defer lcancel()
		handleEvent := func(ev interface{}) {
			switch ev := ev.(type) {
			case *network.EventRequestWillBeSent:
				if ev.LoaderID == loaderID && ev.Type == network.ResourceTypeDocument {
					reqID = ev.RequestID
				}
			case *network.EventLoadingFailed:
				if ev.RequestID == reqID {
					loadErr = fmt.Errorf("page load error %s", ev.ErrorText)
					// If Canceled is true, we won't receive a
					// loadEventFired at all.
					if ev.Canceled {
						finished = true
						lcancel()
					}
				}
			case *network.EventResponseReceived:
				if ev.RequestID == reqID && resp != nil {
					*resp = ev.Response
				}
			case *page.EventLifecycleEvent:
				if ev.FrameID == frameID && ev.Name == "init" {
					hasInit = true
				}
			case *page.EventLoadEventFired:
				// Ignore load events before the "init"
				// lifecycle event, as those are old.
				if hasInit {
					finished = true
					lcancel()
				}
			}
		}
		// earlyEvents is a buffered list of events which happened
		// before we knew what loaderID to look for.
		var earlyEvents []interface{}

		// Obtain frameID from the target.
		c := FromContext(ctx)
		c.Target.frameMu.RLock()
		frameID = c.Target.cur
		c.Target.frameMu.RUnlock()

		ListenTarget(lctx, func(ev interface{}) {
			if loaderID != "" {
				handleEvent(ev)
				return
			}
			earlyEvents = append(earlyEvents, ev)
			switch ev := ev.(type) {
			case *page.EventFrameNavigated:
				// Make sure we keep frameID up to date.
				if ev.Frame.ParentID == "" {
					frameID = ev.Frame.ID
				}
			case *network.EventRequestWillBeSent:
				// Under some circumstances like ERR_TOO_MANY_REDIRECTS, we never
				// see the "init" lifecycle event we want. Those "lone" requests
				// also tend to have a loaderID that matches their requestID, for
				// some reason. If such a request is seen, use it.
				// TODO: research this some more when we have the time.
				if ev.FrameID == frameID && string(ev.LoaderID) == string(ev.RequestID) {
					loaderID = ev.LoaderID
				}
			case *page.EventLifecycleEvent:
				if ev.FrameID == frameID && ev.Name == "init" {
					loaderID = ev.LoaderID
				}
			case *page.EventNavigatedWithinDocument:
				// A fragment navigation doesn't need extra steps.
				finished = true
				lcancel()
			}
			if loaderID != "" {
				for _, ev := range earlyEvents {
					handleEvent(ev)
				}
				earlyEvents = nil
			}
		})

		// Second, run the actions.
		if err := Run(ctx, actions...); err != nil {
			return err
		}

		// Third, block until we have finished loading.
		select {
		case <-lctx.Done():
			if loadErr != nil {
				return loadErr
			}

			// If the ctx parameter was cancelled by the caller (or
			// by a timeout etc.) the select will race between
			// lctx.Done and ctx.Done, since lctx is a sub-context
			// of ctx. So we can't return nil here, as otherwise
			// that race would mean that we would drop 50% of the
			// parent context cancellation errors.
			if !finished {
				return ctx.Err()
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// Targets lists all the targets in the browser attached to the given context.
func Targets(ctx context.Context) ([]*target.Info, error) {
	c, err := initContextBrowser(ctx)
	if err != nil {
		return nil, err
	}
	// TODO: If this is a new browser, the initial target (tab) might not be
	// ready yet. Should we block until at least one target is available?
	// Right now, the caller has to add retries with a timeout.
	return target.GetTargets().Do(cdp.WithExecutor(ctx, c.Browser))
}

// Action is the common interface for an action that will be executed against a
// context and frame handler.
type Action interface {
	// Do executes the action using the provided context and frame handler.
	Do(context.Context) error
}

// ActionFunc is an adapter to allow the use of ordinary func's as an Action.
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
		return sleepContext(ctx, d)
	})
}

// sleepContext sleeps for the specified duration. It returns ctx.Err() immediately
// if the context is cancelled.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// retryWithSleep reties the execution of the specified func until the func returns
// true (means to stop) or a non-nil error.
func retryWithSleep(ctx context.Context, d time.Duration, f func(ctx context.Context) (bool, error)) error {
	for {
		toStop, err := f(ctx)
		if toStop || err != nil {
			return err
		}
		err = sleepContext(ctx, d)
		if err != nil {
			return err
		}
	}
}

type cancelableListener struct {
	ctx context.Context
	fn  func(ev interface{})
}

// ListenBrowser adds a function which will be called whenever a browser event
// is received on the chromedp context. Note that this only includes browser
// events; command responses and target events are not included. Cancelling ctx
// stops the listener from receiving any more events.
//
// Note that the function is called synchronously when handling events. The
// function should avoid blocking at all costs. For example, any Actions must be
// run via a separate goroutine (otherwise, it could result in a deadlock if the
// action sends CDP messages).
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
// received on the chromedp context. Cancelling ctx stops the listener from
// receiving any more events.
//
// Note that the function is called synchronously when handling events. The
// function should avoid blocking at all costs. For example, any Actions must be
// run via a separate goroutine (otherwise, it could result in a deadlock if the
// action sends CDP messages).
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
