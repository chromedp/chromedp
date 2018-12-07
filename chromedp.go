// Package chromedp is a high level Chrome DevTools Protocol client that
// simplifies driving browsers for scraping, unit testing, or profiling web
// pages using the CDP.
//
// chromedp requires no third-party dependencies, implementing the async Chrome
// DevTools Protocol entirely in Go.
package chromedp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"

	"github.com/chromedp/chromedp/client"
	"github.com/chromedp/chromedp/runner"
)

const (
	// DefaultNewTargetTimeout is the default time to wait for a new target to
	// be started.
	DefaultNewTargetTimeout = 3 * time.Second

	// DefaultCheckDuration is the default time to sleep between a check.
	DefaultCheckDuration = 50 * time.Millisecond

	// DefaultPoolStartPort is the default start port number.
	DefaultPoolStartPort = 9000

	// DefaultPoolEndPort is the default end port number.
	DefaultPoolEndPort = 10000
)

// CDP is the high-level Chrome DevTools Protocol browser manager, handling the
// browser process runner, WebSocket clients, associated targets, and network,
// page, and DOM events.
type CDP struct {
	// r is the chrome runner.
	r *runner.Runner

	// opts are command line options to pass to a created runner.
	opts []runner.CommandLineOption

	// watch is the channel for new client targets.
	watch <-chan client.Target

	// cur is the current active target's handler.
	cur cdp.Executor

	// handlers is the active handlers.
	handlers []*TargetHandler

	// handlerMap is the map of target IDs to its active handler.
	handlerMap map[string]int

	// logging funcs
	logf, debugf, errf func(string, ...interface{})

	sync.RWMutex
}

// New creates and starts a new CDP instance.
func New(ctxt context.Context, opts ...Option) (*CDP, error) {
	c := &CDP{
		handlers:   make([]*TargetHandler, 0),
		handlerMap: make(map[string]int),
		logf:       log.Printf,
		debugf:     func(string, ...interface{}) {},
		errf:       func(s string, v ...interface{}) { log.Printf("error: "+s, v...) },
	}

	// apply options
	for _, o := range opts {
		if err := o(c); err != nil {
			return nil, err
		}
	}

	// check for supplied runner, if none then create one
	if c.r == nil && c.watch == nil {
		var err error
		c.r, err = runner.Run(ctxt, c.opts...)
		if err != nil {
			return nil, err
		}
	}

	// watch handlers
	if c.watch == nil {
		c.watch = c.r.Client().WatchPageTargets(ctxt)
	}

	go func() {
		for t := range c.watch {
			if t == nil {
				return
			}
			go c.AddTarget(ctxt, t)
		}
	}()

	// TODO: fix this
	timeout := time.After(defaultNewTargetTimeout)

	// wait until at least one target active
	for {
		select {
		default:
			c.RLock()
			exists := c.cur != nil
			c.RUnlock()
			if exists {
				return c, nil
			}

			// TODO: fix this
			time.Sleep(DefaultCheckDuration)

		case <-ctxt.Done():
			return nil, ctxt.Err()

		case <-timeout:
			return nil, errors.New("timeout waiting for initial target")
		}
	}
}

// AddTarget adds a target using the supplied context.
func (c *CDP) AddTarget(ctxt context.Context, t client.Target) {
	c.Lock()
	defer c.Unlock()

	// create target manager
	h, err := NewTargetHandler(t, c.logf, c.debugf, c.errf)
	if err != nil {
		c.errf("could not create handler for %s: %v", t, err)
		return
	}

	// run
	if err := h.Run(ctxt); err != nil {
		c.errf("could not start handler for %s: %v", t, err)
		return
	}

	// add to active handlers
	c.handlers = append(c.handlers, h)
	c.handlerMap[t.GetID()] = len(c.handlers) - 1
	if c.cur == nil {
		c.cur = h
	}
}

// Wait waits for the Chrome runner to terminate.
func (c *CDP) Wait() error {
	c.RLock()
	r := c.r
	c.RUnlock()

	if r != nil {
		return r.Wait()
	}

	return nil
}

// Shutdown closes all Chrome page handlers.
func (c *CDP) Shutdown(ctxt context.Context, opts ...client.Option) error {
	c.RLock()
	defer c.RUnlock()

	if c.r != nil {
		return c.r.Shutdown(ctxt, opts...)
	}

	return nil
}

// ListTargets returns the target IDs of the managed targets.
func (c *CDP) ListTargets() []string {
	c.RLock()
	defer c.RUnlock()

	i, targets := 0, make([]string, len(c.handlers))
	for k := range c.handlerMap {
		targets[i] = k
		i++
	}

	return targets
}

// GetHandlerByIndex retrieves the domains manager for the specified index.
func (c *CDP) GetHandlerByIndex(i int) cdp.Executor {
	c.RLock()
	defer c.RUnlock()

	if i < 0 || i >= len(c.handlers) {
		return nil
	}

	return c.handlers[i]
}

// GetHandlerByID retrieves the domains manager for the specified target ID.
func (c *CDP) GetHandlerByID(id string) cdp.Executor {
	c.RLock()
	defer c.RUnlock()

	if i, ok := c.handlerMap[id]; ok {
		return c.handlers[i]
	}

	return nil
}

// SetHandler sets the active handler to the target with the specified index.
func (c *CDP) SetHandler(i int) error {
	c.Lock()
	defer c.Unlock()

	if i < 0 || i >= len(c.handlers) {
		return fmt.Errorf("no handler associated with target index %d", i)
	}

	c.cur = c.handlers[i]

	return nil
}

// SetHandlerByID sets the active target to the target with the specified id.
func (c *CDP) SetHandlerByID(id string) error {
	c.Lock()
	defer c.Unlock()

	if i, ok := c.handlerMap[id]; ok {
		c.cur = c.handlers[i]
		return nil
	}

	return fmt.Errorf("no handler associated with target id %s", id)
}

// newTarget creates a new target using supplied context and options, returning
// the id of the created target only after the target has been started for
// monitoring.
func (c *CDP) newTarget(ctxt context.Context, opts ...client.Option) (string, error) {
	c.RLock()
	cl := c.r.Client(opts...)
	c.RUnlock()

	// new page target
	t, err := cl.NewPageTarget(ctxt)
	if err != nil {
		return "", err
	}

	timeout := time.After(DefaultNewTargetTimeout)

	for {
		select {
		default:
			var ok bool
			id := t.GetID()
			c.RLock()
			_, ok = c.handlerMap[id]
			c.RUnlock()
			if ok {
				return id, nil
			}

			time.Sleep(DefaultCheckDuration)

		case <-ctxt.Done():
			return "", ctxt.Err()

		case <-timeout:
			return "", errors.New("timeout waiting for new target to be available")
		}
	}
}

// SetTarget is an action that sets the active Chrome handler to the specified
// index i.
func (c *CDP) SetTarget(i int) Action {
	return ActionFunc(func(context.Context, cdp.Executor) error {
		return c.SetHandler(i)
	})
}

// SetTargetByID is an action that sets the active Chrome handler to the handler
// associated with the specified id.
func (c *CDP) SetTargetByID(id string) Action {
	return ActionFunc(func(context.Context, cdp.Executor) error {
		return c.SetHandlerByID(id)
	})
}

// NewTarget is an action that creates a new Chrome target, and sets it as the
// active target.
func (c *CDP) NewTarget(id *string, opts ...client.Option) Action {
	return ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
		n, err := c.newTarget(ctxt, opts...)
		if err != nil {
			return err
		}

		if id != nil {
			*id = n
		}

		return nil
	})
}

// CloseByIndex closes the Chrome target with specified index i.
func (c *CDP) CloseByIndex(i int) Action {
	return ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
		return nil
	})
}

// CloseByID closes the Chrome target with the specified id.
func (c *CDP) CloseByID(id string) Action {
	return ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
		return nil
	})
}

// Run executes the action against the current target using the supplied
// context.
func (c *CDP) Run(ctxt context.Context, a Action) error {
	c.RLock()
	cur := c.cur
	c.RUnlock()

	return a.Do(ctxt, cur)
}

// Option is a Chrome DevTools Protocol option.
type Option func(*CDP) error

// WithRunner is a CDP option to specify the underlying Chrome runner to
// monitor for page handlers.
func WithRunner(r *runner.Runner) Option {
	return func(c *CDP) error {
		c.r = r
		return nil
	}
}

// WithTargets is a CDP option to specify the incoming targets to monitor for
// page handlers.
func WithTargets(watch <-chan client.Target) Option {
	return func(c *CDP) error {
		c.watch = watch
		return nil
	}
}

// WithClient is a CDP option to use the incoming targets from a client.
func WithClient(ctxt context.Context, cl *client.Client) Option {
	return func(c *CDP) error {
		return WithTargets(cl.WatchPageTargets(ctxt))(c)
	}
}

// WithURL is a CDP option to use a client with the specified URL.
func WithURL(ctxt context.Context, urlstr string) Option {
	return func(c *CDP) error {
		return WithClient(ctxt, client.New(client.URL(urlstr)))(c)
	}
}

// WithRunnerOptions is a CDP option to specify the options to pass to a newly
// created Chrome process runner.
func WithRunnerOptions(opts ...runner.CommandLineOption) Option {
	return func(c *CDP) error {
		c.opts = opts
		return nil
	}
}

// WithLogf is a CDP option to specify a func to receive general logging.
func WithLogf(f func(string, ...interface{})) Option {
	return func(c *CDP) error {
		c.logf = f
		return nil
	}
}

// WithDebugf is a CDP option to specify a func to receive debug logging (ie,
// protocol information).
func WithDebugf(f func(string, ...interface{})) Option {
	return func(c *CDP) error {
		c.debugf = f
		return nil
	}
}

// WithErrorf is a CDP option to specify a func to receive error logging.
func WithErrorf(f func(string, ...interface{})) Option {
	return func(c *CDP) error {
		c.errf = f
		return nil
	}
}

// WithLog is a CDP option that sets the logging, debugging, and error funcs to
// f.
func WithLog(f func(string, ...interface{})) Option {
	return func(c *CDP) error {
		c.logf, c.debugf, c.errf = f, f, f
		return nil
	}
}

// WithConsolef is a CDP option to specify a func to receive chrome log events.
//
// Note: NOT YET IMPLEMENTED.
func WithConsolef(f func(string, ...interface{})) Option {
	return func(c *CDP) error {
		return nil
	}
}

var (
	// defaultNewTargetTimeout is the default target timeout -- used by
	// testing.
	defaultNewTargetTimeout = DefaultNewTargetTimeout
)
