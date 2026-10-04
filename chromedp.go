// Package chromedp is a high level Chrome DevTools Protocol client. It drives
// browsers to scrape web pages, to test them, and to profile them.
//
// chromedp needs no external driver. It implements the asynchronous protocol
// in Go.
//
// This package includes a number of simple examples. The module
// [chromedp/examples] has more complex examples. The module [chromedp/termcast]
// draws the screen of a page in a terminal that shows images.
//
// [chromedp/examples]: https://github.com/chromedp/examples
// [chromedp/termcast]: https://github.com/chromedp/termcast
package chromedp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto"
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

// closeTargetTimeout is how long the cancellation of a tab waits for the
// browser to detach from the tab and to close it.
const closeTargetTimeout = 5 * time.Second

// Context is the data that NewContext stores in a context.Context. Run needs
// it.
type Context struct {
	// Allocator creates new browsers. A child context inherits it from the
	// parent context.
	Allocator Allocator

	// Browser is the browser of the context. A child context inherits it from
	// the parent context.
	Browser *Browser

	// Target is the target that actions run against. A child context does not
	// inherit it. Typically each context has its own Target, which points to a
	// separate browser tab (page).
	Target *Target

	// targetMu protects the write of Target in attachTarget and the read of
	// Target in the cancellation watcher of NewContext. The watcher runs on
	// its own goroutine, and the context can end while attachTarget runs. The
	// goroutine that runs the actions needs no lock to read Target, because it
	// is the only one that writes it.
	targetMu sync.Mutex

	// targetID is set by WithTargetID. If it is nil, Run uses the only unused
	// page target, or creates a new one.
	targetID target.ID

	// createBrowserContextParams is set by WithNewBrowserContext. Run uses it
	// to create a new BrowserContext.
	createBrowserContextParams *target.CreateBrowserContextParams

	// sharedWindow is set by WithNewWindow(false). A new tab then opens in
	// the window of the browser and not in a new window. A child context
	// inherits it.
	sharedWindow bool

	// detachOnCancel is set by WithDetachOnCancel. The cancellation then
	// detaches from the target and leaves the tab and its BrowserContext.
	detachOnCancel bool

	// browserContextOwner is true when this context owns its BrowserContext.
	// The owner disposes the BrowserContext when the context is done.
	browserContextOwner bool

	// BrowserContextID is set by WithExistingBrowserContext.
	//
	// Otherwise, BrowserContextID is not empty in these cases:
	//
	// 1. The context was made with WithNewBrowserContext. Its first run creates
	// a new BrowserContext, and BrowserContextID holds the id of that
	// BrowserContext.
	//
	// 2. The context was not made with WithTargetID, and the parent context
	// has a BrowserContextID that is not empty. The context copies it from the
	// parent context.
	BrowserContextID cdp.BrowserContextID

	// browserOpts holds the browser options that NewContext receives from
	// WithBrowserOption. Run uses them when it allocates a browser.
	browserOpts []BrowserOption

	// cancel cancels the context that started Browser. Use it to stop all
	// activity and to prevent deadlocks when the browser closes or crashes. It
	// does not wait for anything.
	cancel func()

	// first is true when this context started a new Chrome process. Its
	// cancellation then stops the whole browser and its handler, and not only
	// some of its pages.
	first bool

	// allocatorOptions is set by WithAllocatorOptions. NewContext adds them to
	// the default allocator in the same way as visibleWindow.
	allocatorOptions []ExecAllocatorOption

	// attachReleased is true after initContextBrowser closed allocated for an
	// allocator that attaches to a running browser.
	attachReleased bool

	// visibleWindow is set by WithVisibleWindow. NewContext uses it to build
	// the default allocator. It has no effect on an allocator that the caller
	// made.
	visibleWindow bool

	// closedTarget lets a cancellation wait until the page of a target is
	// closed.
	closedTarget sync.WaitGroup

	// allocated is closed when an allocated browser stops completely. It is
	// nil when no browser needs allocation.
	allocated chan struct{}
	// cancelErr is the first error that the cancellation of this context met.
	// For example, the context failed to delete the temporary user data
	// directory of a browser.
	cancelErr error
}

// NewContext creates a chromedp context from the parent context. The context
// inherits the Allocator of the parent. By default, that is an ExecAllocator
// with DefaultExecAllocatorOptions.
//
// If the parent context holds an allocated Browser, the child context inherits
// it, and its first Run creates a new tab on that browser. Otherwise, its
// first Run allocates a new browser.
//
// Canceling the returned context closes a tab or a whole browser, as the
// rules above describe. To cancel a context and read the errors, see [Cancel].
//
// NewContext does not allocate or start a browser. That happens the first time
// that you call Run on the context.
//
// # The lifetime of the browser
//
// The first call of [Run] on a context starts the browser, and it binds the life
// of the browser to the context that you pass to that call. When that context
// ends, the browser stops. So a context from [context.WithTimeout] that you
// use for the first Run closes the browser when the timeout ends, and every
// later call on the parent context fails with context.Canceled.
//
// To limit one action, start the browser first with a context that has no
// timeout. Then run the action with a context that you derive from it:
//
//	if err := chromedp.Do(ctx); err != nil { // starts the browser
//		return err
//	}
//	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
//	defer cancel()
//	err := chromedp.Do(tctx, chromedp.Navigate(url))
//
// When the timeout ends, Run returns an error that wraps
// context.DeadlineExceeded. The tab and the browser stay open, and ctx still
// works. Run has no option for the timeout of one action.
//
// # Use from several goroutines
//
// Contexts that share one browser run in separate tabs, and they are safe to
// use in parallel. To get them, call [Run] once on a parent context so that it
// has a browser, and then make a child context for each goroutine with
// NewContext. A child of a context that has no browser yet starts a browser of
// its own.
//
// Do not share one context between goroutines. The actions of one context run
// in one tab, so they can race. For example, two [Navigate] actions on the same
// tab can fail. The first call of [Run] on a context starts the
// browser, and it must not run at the same time as another call on that
// context.
func NewContext(parent context.Context, opts ...ContextOption) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	c := &Context{cancel: cancel, first: true}
	var parentBrowserContextID cdp.BrowserContextID
	if pc := FromContext(parent); pc != nil {
		c.Allocator = pc.Allocator
		c.Browser = pc.Browser
		parentBrowserContextID = pc.BrowserContextID
		c.sharedWindow = pc.sharedWindow
		// do not inherit Target, so that NewContext can be used to
		// create a new tab on the same browser.

		c.first = c.Browser == nil

		if isAttacher(c.Allocator) {
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
		c.Allocator = setupExecAllocator(defaultExecAllocatorOptions(
			c.visibleWindow || visibleWindowFromEnv(), c.allocatorOptions)...)
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

		c.targetMu.Lock()
		tgt := c.Target
		c.targetMu.Unlock()
		if tgt == nil {
			// This is a new tab, but we did not create it and attach
			// to it yet. Nothing to do.
			return
		}

		// This is not the original browser tab. Detach and close it.
		// The context ctx is canceled, so make a new context with a timeout.
		// A busy machine, such as a CI runner, can need more than a second
		// to answer.
		ctx, cancel := context.WithTimeout(context.Background(), closeTargetTimeout)
		defer cancel()
		if id := tgt.SessionID; id != "" {
			_, err := cdp.Call(ctx, c.Browser, target.DetachFromTarget, target.DetachFromTargetParams{SessionID: id})
			if c.cancelErr == nil && err != nil {
				c.cancelErr = fmt.Errorf("detaching from the target %s: %w", tgt.TargetID, err)
			}
		}
		if c.detachOnCancel {
			// Leave the tab open. A BrowserContext that this context owns
			// stays too, because disposing of it closes its tabs.
			return
		}
		if id := tgt.TargetID; id != "" {
			if _, err := cdp.Call(ctx, c.Browser, target.CloseTarget, target.CloseTargetParams{TargetID: id}); err != nil {
				if c.cancelErr == nil {
					c.cancelErr = fmt.Errorf("closing the target %s: %w", id, err)
				}
			} else {
				// Current Chrome answers CloseTarget before the
				// target is gone. Wait until it is not listed.
				// A destroyed event is not enough, because a
				// browser connection can miss it.
				// A tab with a request that never ends can stay
				// for a long time, so give up after a short time.
				wctx, wcancel := context.WithTimeout(ctx, 500*time.Millisecond)
				err := waitTargetGone(wctx, c.Browser, id)
				wcancel()
				if err != nil && !errors.Is(err, context.DeadlineExceeded) && c.cancelErr == nil {
					c.cancelErr = err
				}
			}
		}
		if c.browserContextOwner {
			_, err := cdp.Call(ctx, c.Browser, target.DisposeBrowserContext, target.DisposeBrowserContextParams{BrowserContextID: c.BrowserContextID})
			if c.cancelErr == nil && err != nil {
				c.cancelErr = fmt.Errorf("disposing of the browser context %s: %w", c.BrowserContextID, err)
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

// withExitError adds the exit error of the browser process to err. See
// [Browser.withExitError]. It accepts a nil Context.
func (c *Context) withExitError(err error) error {
	if c == nil {
		return err
	}
	return c.Browser.withExitError(err)
}

type contextKey struct{}

// FromContext returns the Context that a context.Context holds, or nil.
func FromContext(ctx context.Context) *Context {
	c, _ := ctx.Value(contextKey{}).(*Context)
	return c
}

// Cancel cancels a chromedp context, waits until the context frees its
// resources, and returns any error that occurred.
//
// If the context allocated a browser, Cancel closes the browser gracefully. To
// limit how long Cancel waits for the browser to close, give the context a
// timeout:
//
//	tctx, tcancel := context.WithTimeout(ctx, 10 * time.Second)
//	defer tcancel()
//	chromedp.Cancel(tctx)
//
// A "defer cancel()" is enough in most cases. Use Cancel to close a browser
// gracefully, or to get the errors that occur during the cancellation.
func Cancel(ctx context.Context) error {
	c := FromContext(ctx)
	// c.cancel is nil when the caller passes to Cancel a context from
	// NewExecAllocator or by an allocator of another module.
	if c == nil || c.cancel == nil {
		return ErrInvalidContext
	}
	// A browser that was kept open must stay open, so Cancel does not close it.
	graceful := c.first && c.Browser != nil && !c.Browser.keptOpen
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
	// After a graceful close, cancel the whole context. This frees any
	// goroutines or resources that are left, and it stops a browser that did
	// not finish before the timeout above. The non-graceful path already
	// called c.cancel above.
	if graceful {
		c.cancel()
	}

	// If we allocated and hit ctx.Done earlier, cancelErr is not ready until
	// the allocated channel is closed, because the two race. If we did not hit
	// ctx.Done earlier, c.allocated was already closed, so this does nothing.
	if !ready && c.allocated != nil {
		<-c.allocated
	}
	return c.cancelErr
}

func initContextBrowser(ctx context.Context) (*Context, error) {
	c := FromContext(ctx)
	// If c is nil, it is not a chromedp context.
	// If c.Allocator is nil, NewContext was not used properly.
	// If c.cancel is nil, the caller passed an allocator context to Run
	// directly.
	if c == nil || c.Allocator == nil || c.cancel == nil {
		return nil, ErrInvalidContext
	}
	if c.Browser == nil {
		attach := isAttacher(c.Allocator)
		if attach && !c.attachReleased {
			// The allocator starts no process, so nothing waits for the
			// context to stop a browser. See Cancel.
			c.attachReleased = true
			close(c.allocated)
		}
		b, err := c.Allocator.Allocate(ctx, c.browserOpts...)
		if err != nil {
			return nil, err
		}
		c.Browser = b
		if attach {
			go func() {
				// If the browser loses the connection, stop the context at
				// once. Do not stop it in the middle of a graceful close.
				<-b.LostConnection
				select {
				case <-b.closingGracefully:
				default:
					b.noteLost(ctx)
					c.cancel()
				}
			}()
		}
	}
	return c, nil
}

// isAttacher reports whether a attaches to a browser that runs already. See
// [Attacher].
func isAttacher(a Allocator) bool {
	at, ok := a.(Attacher)
	return ok && at.Attaches()
}

// initContextTarget is like initContextBrowser, and also attaches the target
// of the context when it has none yet.
func initContextTarget(ctx context.Context) (*Context, error) {
	c, err := initContextBrowser(ctx)
	if err != nil {
		return nil, err
	}
	if c.Target == nil {
		if err := c.newTarget(ctx); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Context) newTarget(ctx context.Context) error {
	if c.targetID != "" {
		if err := c.attachTarget(ctx, c.targetID); err != nil {
			return err
		}
		// A new page can load its top-level frame before we listen. Then we do
		// not see the frameNavigated and documentUpdated events, so load them
		// here. As of 2020-1-27, worker targets do not implement the Page.*
		// methods, so skip this step for workers.
		if !c.Target.isWorker {
			tree, err := cdp.Call(ctx, c.Target, page.GetFrameTree, cdp.Empty{})
			if err != nil {
				return err
			}

			c.Target.frameMu.Lock()
			c.Target.frames[tree.FrameTree.Frame.ID] = &Frame{Frame: tree.FrameTree.Frame}
			c.Target.cur = tree.FrameTree.Frame.ID
			c.Target.frameMu.Unlock()

			c.Target.documentUpdated(ctx)
		}
		return nil
	}
	if !c.first {
		newWindow := !c.sharedWindow
		if c.createBrowserContextParams != nil {
			params := *c.createBrowserContextParams
			if c.detachOnCancel {
				// Chrome disposes of a browser context that disposeOnDetach
				// marks when the context detaches, and that closes the tab.
				params.DisposeOnDetach = nil
			}
			res, err := cdp.Call(ctx, c.Browser, target.CreateBrowserContext, params)
			if err != nil {
				return err
			}
			c.BrowserContextID = res.BrowserContextID
			c.browserContextOwner = true
			c.createBrowserContextParams = nil
			// A new browser context has no window yet, so its first tab
			// needs a window of its own.
			newWindow = true
		}
		create := func(newWindow bool) (target.CreateTargetResult, error) {
			return cdp.Call(ctx, c.Browser, target.CreateTarget, target.CreateTargetParams{
				URL:              "about:blank",
				BrowserContextID: c.BrowserContextID,
				// A tab in a shared window is hidden when another tab is
				// active. A hidden page gets no animation frames, so Poll
				// never returns. By default, each target gets its own
				// window. See WithNewWindow.
				NewWindow: &newWindow,
			})
		}
		res, err := create(newWindow)
		var cerr *cdproto.Error
		if err != nil && !newWindow && errors.As(err, &cerr) && strings.Contains(cerr.Message, "no browser is open") {
			// The browser context has no window to put a tab in. This is
			// the case of a context that WithExistingBrowserContext names.
			res, err = create(true)
		}
		if err != nil {
			return err
		}
		c.targetID = res.TargetID
		return c.attachTarget(ctx, c.targetID)
	}

	// This is like WaitNewTarget, but for the entire browser.
	ch := make(chan target.ID, 1)
	lctx, cancel := context.WithCancel(ctx)
	c.Browser.listen(lctx, func(ev any) {
		var info *target.Info
		switch ev := ev.(type) {
		case *target.EventTargetCreated:
			info = ev.TargetInfo
		case *target.EventTargetInfoChanged:
			info = ev.TargetInfo
		default:
			return
		}
		// The browser starts with a tab that is not blank in these cases:
		// 1. The "--app" option is used (this disables headless mode).
		// 2. The command line arguments hold a URL other than "about:blank".
		// So do not require that the URL is "about:blank".
		// See issue https://github.com/chromedp/chromedp/issues/1076
		// When the browser starts with several tabs, attach to any one of them,
		// blank or not.
		if info.Type == "page" {
			select {
			case <-lctx.Done():
			case ch <- info.TargetID:
			}
			cancel()
		}
	})

	// wait for the first tab to appear
	if _, err := cdp.Call(ctx, c.Browser, target.SetDiscoverTargets, target.SetDiscoverTargetsParams{Discover: true}); err != nil {
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
	res, err := cdp.Call(ctx, c.Browser, target.AttachToTarget, target.AttachToTargetParams{
		TargetID: targetID,
		Flatten:  new(true),
	})
	if err != nil {
		return err
	}

	t, err := c.Browser.newTarget(ctx, targetID, res.SessionID)
	if err != nil {
		return err
	}
	c.targetMu.Lock()
	c.Target = t
	c.targetMu.Unlock()

	go c.Target.run(ctx)

	// Find out whether this is a worker target. A worker cannot use
	// Target.getTargetInfo or Target.getTargets, so find out whether "self"
	// refers to a WorkerGlobalScope or a ServiceWorkerGlobalScope.
	if _, err := cdp.Call(ctx, c.Target, runtime.Enable, cdp.Empty{}); err != nil {
		return err
	}
	self, err := cdp.Call(ctx, c.Target, runtime.Evaluate, runtime.EvaluateParams{Expression: "self"})
	if err != nil {
		return err
	}
	c.Target.isWorker = strings.Contains(self.Result.ClassName, "WorkerGlobalScope")

	// Enable available domains and discover targets.
	type step struct {
		method string
		params any
	}
	steps := []step{
		{log.Enable.Method, cdp.Empty{}},
		{network.Enable.Method, network.EnableParams{}},
	}
	// These steps are not available on a worker target.
	if !c.Target.isWorker {
		steps = append(steps,
			step{inspector.Enable.Method, cdp.Empty{}},
			step{page.Enable.Method, page.EnableParams{}},
			step{dom.Enable.Method, dom.EnableParams{}},
			step{css.Enable.Method, cdp.Empty{}},
			step{target.SetDiscoverTargets.Method, target.SetDiscoverTargetsParams{Discover: true}},
			step{target.SetAutoAttach.Method, target.SetAutoAttachParams{AutoAttach: true, Flatten: new(true)}},
			step{page.SetLifecycleEventsEnabled.Method, page.SetLifecycleEventsEnabledParams{Enabled: true}},
		)
	}

	for _, s := range steps {
		if err := c.Target.Call(ctx, s.method, s.params, nil); err != nil {
			return fmt.Errorf("unable to execute %s: %w", s.method, err)
		}
	}
	return nil
}

// ContextOption is a context option.
type ContextOption = func(*Context)

// WithVisibleWindow makes NewContext build the default allocator with
// [VisibleWindow] and without Headless, so that the browser opens a visible
// window. The window stays open until the context ends. See also [KeepOpen]
// and [WaitClosed].
//
// The environment variable CHROMEDP_VISIBLEWINDOW has the same effect with no
// change in the code. Any value other than the empty string, "false" and "0"
// turns it on.
//
// The option only applies when the parent context has no allocator, because
// then NewContext builds the allocator. It has no effect on an allocator that
// the caller made, and it has no effect on a context that inherits a browser. For an allocator that you make,
// add [VisibleWindow] to the options of NewExecAllocator instead.
//
// On Linux, the first Run returns [ErrNoDisplay] when the environment has no
// display.
func WithVisibleWindow() ContextOption {
	return func(c *Context) { c.visibleWindow = true }
}

// WithAllocatorOptions makes NewContext add opts to the options of the default
// allocator, after [DefaultExecAllocatorOptions] and after [VisibleWindow]. The
// module github.com/chromedp/chromedp/remote uses it for its own WithKeepOpen.
//
// The option only applies when the parent context has no allocator. It has no
// effect on an allocator that the caller made, and it has no effect on a
// context that inherits a browser. For an allocator that you make, add the
// options to NewExecAllocator instead.
func WithAllocatorOptions(opts ...ExecAllocatorOption) ContextOption {
	return func(c *Context) {
		c.allocatorOptions = append(c.allocatorOptions, opts...)
	}
}

// WithNewWindow chooses where a new tab opens. By default, the first Run of a
// context that creates a tab opens it in a new window, so that each tab is
// visible and gets animation frames. WithNewWindow(false) opens the tab in the
// window of the browser, as a real tab. A child context inherits the choice.
//
// Chrome hides every tab of a window except the active one, and a hidden page
// gets no animation frames. So an action that waits for a frame, such as
// [Poll] and the actions that use it, can wait for ever on a tab that is not
// active. Make a tab active with the command [target.ActivateTarget], or with
// [page.BringToFront] in the tab, before you run such an action on it.
//
// WithNewWindow has no effect on the first context of a browser, which uses the
// tab that the browser opens, or on a context that attaches to an existing
// target with WithTargetID.
func WithNewWindow(newWindow bool) ContextOption {
	return func(c *Context) {
		c.sharedWindow = !newWindow
	}
}

// WithDetachOnCancel makes the cancellation of the context detach from its tab
// and leave the tab open. By default, the cancellation closes the tab that the
// context created or attached to with [WithTargetID], and it disposes of a
// BrowserContext that [WithNewBrowserContext] made.
//
// Use it with a remote browser that keeps its tabs when a client leaves, so
// that a later client can attach to the tab again with [WithTargetID]. Read the
// ID of the tab from the Target field of [FromContext] after the first Run.
// With this option, the context also keeps a BrowserContext that it owns,
// because disposing of it closes the tab. It creates that BrowserContext
// without the setting disposeOnDetach. The program must dispose of it with the
// command [target.DisposeBrowserContext] when it no longer needs it.
//
// The option has no effect on the first context of a browser that the exec
// allocator starts, because the cancellation stops that browser. It is not
// inherited by a child context.
func WithDetachOnCancel() ContextOption {
	return func(c *Context) { c.detachOnCancel = true }
}

// WithTargetID makes a context attach to an existing target, and not create a
// new one.
//
// It also works for an iframe from another site (a cross-site iframe). Chrome
// runs such an iframe in its own process, and lists it as a target of the type
// "iframe" in [Targets]. [FromNode] and the DOM tree of the page do not reach
// it, and the network events of the iframe do not arrive in the context of the
// page, because chromedp does not attach to the iframe by itself. A context
// that WithTargetID attaches to the iframe runs actions in the iframe and gets
// its events. The parent context must have run already, so that it has a
// browser. For example:
//
//	infos, _ := chromedp.Targets(ctx)
//	for _, info := range infos {
//		if info.Type == "iframe" && strings.HasPrefix(info.URL, "https://other.example/") {
//			frameCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(info.TargetID))
//			defer cancel()
//			text, err := chromedp.Run(frameCtx, chromedp.Text(chromedp.CSS("#inner")))
//		}
//	}
//
// The other way is to turn off site isolation in the browser. Give the exec
// allocator Flag("disable-features", "SitePerProcess,IsolateOrigins") and
// Flag("disable-site-isolation-trials", true). The iframe then stays in the page
// process, and [FromNode] and the events of the page reach it. This turns off a
// security feature of Chrome. See
// docs/decisions/2026-10-04-keep-site-isolation-on.md.
func WithTargetID(id target.ID) ContextOption {
	return func(c *Context) { c.targetID = id }
}

// CreateBrowserContextOption is an option for the creation of a BrowserContext.
type CreateBrowserContextOption = func(*target.CreateBrowserContextParams)

// WithNewBrowserContext makes a context create a new BrowserContext and a new
// target in it. A child context creates its target in this BrowserContext too,
// unless another option applies. The context disposes the new BrowserContext
// when the context is done.
func WithNewBrowserContext(options ...CreateBrowserContextOption) ContextOption {
	return func(c *Context) {
		if c.first {
			panic("WithNewBrowserContext can not be used before Browser is initialized")
		}

		params := &target.CreateBrowserContextParams{DisposeOnDetach: new(true)}
		for _, o := range options {
			o(params)
		}
		c.createBrowserContextParams = params
	}
}

// WithExistingBrowserContext makes a context create a new target in the given
// BrowserContext.
func WithExistingBrowserContext(id cdp.BrowserContextID) ContextOption {
	return func(c *Context) {
		if c.first {
			panic("WithExistingBrowserContext can not be used before Browser is initialized")
		}
		c.BrowserContextID = id
	}
}

// WithLogf is a shortcut for WithBrowserOption(WithBrowserLogf(f)).
func WithLogf(f func(string, ...any)) ContextOption {
	return WithBrowserOption(WithBrowserLogf(f))
}

// WithErrorf is a shortcut for WithBrowserOption(WithBrowserErrorf(f)).
func WithErrorf(f func(string, ...any)) ContextOption {
	return WithBrowserOption(WithBrowserErrorf(f))
}

// WithDebugf is a shortcut for WithBrowserOption(WithBrowserDebugf(f)).
func WithDebugf(f func(string, ...any)) ContextOption {
	return WithBrowserOption(WithBrowserDebugf(f))
}

// WithBrowserOption passes browser options to the allocator when it allocates
// a new browser. As a result, you can use this option only when the context
// allocates a new browser.
func WithBrowserOption(opts ...BrowserOption) ContextOption {
	return func(c *Context) {
		if c.Browser != nil {
			panic("WithBrowserOption can only be used when allocating a new browser")
		}
		c.browserOpts = append(c.browserOpts, opts...)
	}
}

// RunResponse is an alternative to [Do] for a list of actions that trigger a
// page navigation, such as a click on a link or a button.
//
// RunResponse runs the actions and blocks until a page loads. Then it returns
// the HTTP response of the HTML document. Use it to wait for the page, or to
// catch 404 status codes.
//
// If the actions trigger several navigations, RunResponse uses only the first.
// If the actions trigger no navigation, RunResponse blocks until the context
// is canceled.
func RunResponse(ctx context.Context, steps ...Action[Void]) (*network.Response, error) {
	return Run(ctx, responseAction(steps...))
}

// responseAction makes an action that runs the steps, waits until the page
// loads, and returns the response of the HTML document.
func responseAction(steps ...Action[Void]) Action[*network.Response] {
	return func(ctx context.Context, t *Target) (*network.Response, error) {
		var resp *network.Response
		// loaderID lets us filter the requests of the navigation that
		// loads now.
		var loaderID cdp.LoaderID

		// reqID is the request that we look at now. It can go through
		// several values, for example when the page redirects.
		var reqID network.RequestID

		// frameID corresponds to the target's root frame.
		var frameID cdp.FrameID

		var loadErr error
		hasInit := false
		finished := false

		// First, set up the func that handles events.
		// It listens for lifecycle events. It uses them to make sure that we
		// get the response of a request from the loaderID that we want.
		//
		// The events of several methods must arrive in order, so this uses the
		// internal listener and not one subscription for each method.

		lctx, lcancel := context.WithCancel(ctx)
		defer lcancel()
		handleEvent := func(ev any) {
			switch ev := ev.(type) {
			case *network.EventRequestWillBeSent:
				if ev.LoaderID == loaderID && ev.Type == network.ResourceTypeDocument {
					reqID = ev.RequestID
				}
			case *network.EventLoadingFailed:
				if ev.RequestID == reqID {
					loadErr = &LoadError{ErrorText: ev.ErrorText}
					// If Canceled is true, we will not receive a
					// loadEventFired at all.
					if ev.Canceled {
						finished = true
						lcancel()
					}
				}
			case *network.EventResponseReceived:
				if ev.RequestID == reqID {
					resp = ev.Response
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
			case *page.EventFrameStoppedLoading:
				if hasInit && ev.FrameID == frameID {
					finished = true
					lcancel()
				}
			}
		}
		// earlyEvents buffers the events that arrive before we know which loaderID
		// to look for.
		var earlyEvents []any

		// Obtain frameID from the target.
		t.frameMu.RLock()
		frameID = t.cur
		t.frameMu.RUnlock()

		t.listen(lctx, func(ev any) {
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
				// In some cases, such as ERR_TOO_MANY_REDIRECTS, we never see the
				// "init" lifecycle event that we want. Such "lone" requests also
				// tend to have a loaderID that matches their requestID, for an
				// unknown reason. If we see such a request, use it.
				// TODO: research this more when we have the time.
				if ev.FrameID == frameID && string(ev.LoaderID) == string(ev.RequestID) {
					loaderID = ev.LoaderID
				}
			case *page.EventLifecycleEvent:
				if ev.FrameID == frameID && ev.Name == "init" {
					loaderID = ev.LoaderID
				}
			case *page.EventNavigatedWithinDocument:
				// A fragment navigation does not need extra steps.
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
		if _, err := Steps(steps...)(ctx, t); err != nil {
			return nil, err
		}

		// Third, block until we have finished loading.
		select {
		case <-lctx.Done():
			if loadErr != nil {
				return nil, loadErr
			}

			// If the caller canceled ctx (or a timeout ended it), the select
			// races between lctx.Done and ctx.Done, because lctx is a
			// sub-context of ctx. So we cannot return nil here. Otherwise the
			// race drops 50% of the cancellation errors of the parent context.
			if !finished {
				return nil, ctx.Err()
			}
			return resp, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// waitLoad makes an action that runs the steps and waits until the page loads.
// It does not return the response.
func waitLoad(steps ...Action[Void]) Action[Void] {
	r := responseAction(steps...)
	return func(ctx context.Context, t *Target) (Void, error) {
		_, err := r(ctx, t)
		return Void{}, err
	}
}

// Targets lists the targets of the browser of the context.
func Targets(ctx context.Context) ([]*target.Info, error) {
	c, err := initContextBrowser(ctx)
	if err != nil {
		return nil, err
	}
	// TODO: The initial target (tab) of a new browser can be not ready yet.
	// Do we block until at least one target is available? Right now, the
	// caller has to retry with a timeout.
	res, err := cdp.Call(ctx, c.Browser, target.GetTargets, target.GetTargetsParams{})
	if err != nil {
		return nil, err
	}
	return res.TargetInfos, nil
}

// sleepContext sleeps for the specified duration. It returns ctx.Err() immediately
// if the context is canceled.
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

// retryWithSleep calls f again and again, with a sleep of d between the calls,
// until f returns true (stop) or a non-nil error.
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
	fn  func(ev any)
}

// listen adds a func that the target calls for each event. Canceling ctx
// stops the calls.
//
// The target calls the func synchronously, so the func must not block. Unlike
// the subscriptions of [Events], the func sees the events of all methods in
// the order in which they arrive.
func (t *Target) listen(ctx context.Context, fn func(ev any)) {
	t.listenersMu.Lock()
	t.listeners = append(t.listeners, cancelableListener{ctx, fn})
	t.listenersMu.Unlock()
}

// listen is like [Target.listen] for the events of the browser.
func (b *Browser) listen(ctx context.Context, fn func(ev any)) {
	b.listenersMu.Lock()
	b.listeners = append(b.listeners, cancelableListener{ctx, fn})
	b.listenersMu.Unlock()
}

// WaitNewTarget waits for the current target to open a new target. When fn
// matches a new target that is not attached, WaitNewTarget sends its target ID
// on the returned channel. The channel closes without a value when the context
// has no valid target.
func WaitNewTarget(ctx context.Context, fn func(*target.Info) bool) <-chan target.ID {
	ch := make(chan target.ID, 1)
	c, err := initContextTarget(ctx)
	if err != nil {
		close(ch)
		return ch
	}
	lctx, cancel := context.WithCancel(ctx)
	c.Target.listen(lctx, func(ev any) {
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
			return // already attached, so not a new target
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

// waitTargetGone polls the browser until the target with the given id is no
// longer listed, or until ctx is done.
func waitTargetGone(ctx context.Context, browser cdp.Session, id target.ID) error {
	for {
		res, err := cdp.Call(ctx, browser, target.GetTargets, target.GetTargetsParams{})
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(res.TargetInfos, func(info *target.Info) bool { return info.TargetID == id }) {
			return nil
		}
		if err := sleepContext(ctx, 5*time.Millisecond); err != nil {
			return err
		}
	}
}
