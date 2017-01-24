// Package chromedp is a high level Chrome Debugging Protocol domain manager
// that simplifies driving web browsers (Chrome, Safari, Edge, Android Web
// Views, and others) for scraping, unit testing, or profiling web pages.
// chromedp requires no third-party dependencies (ie, Selenium), implementing
// the async Chrome Debugging Protocol natively.
package chromedp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	. "github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/client"
	"github.com/knq/chromedp/runner"
)

const (
	// DefaultNewTargetTimeout is the default time to wait for a new target to
	// be started.
	DefaultNewTargetTimeout = 3 * time.Second

	// DefaultCheckDuration is the default time to sleep between a check.
	DefaultCheckDuration = 50 * time.Millisecond
)

// CDP contains information for managing a Chrome process runner, low level
// client and associated target page handlers.
type CDP struct {
	// r is the chrome runner.
	r *runner.Runner

	// opts are command line options to pass to a created runner.
	opts []runner.CommandLineOption

	// watch is the channel for new client targets.
	watch <-chan client.Target

	// cur is the current active target's handler.
	cur FrameHandler

	// handlers is the active handlers.
	handlers []FrameHandler

	// handlerMap is the map of target IDs to its active handler.
	handlerMap map[string]int

	sync.RWMutex
}

// New creates a new Chrome Debugging Protocol client.
func New(ctxt context.Context, opts ...Option) (*CDP, error) {
	var err error

	c := &CDP{
		handlers:   make([]FrameHandler, 0),
		handlerMap: make(map[string]int),
	}

	// apply options
	for _, o := range opts {
		err = o(c)
		if err != nil {
			return nil, err
		}
	}

	// setup context
	if ctxt == nil {
		var cancel func()
		ctxt, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

	// check for supplied runner, if none then create one
	if c.r == nil && c.watch == nil {
		c.r, err = runner.Run(ctxt, c.opts...)
		if err != nil {
			return nil, err
		}
	}

	// watch handlers
	if c.watch == nil {
		c.watch = c.r.WatchPageTargets(ctxt)
	}

	go func() {
		for t := range c.watch {
			go c.AddTarget(ctxt, t)
		}
	}()

	// TODO: fix this
	timeout := time.After(DefaultNewTargetTimeout)

loop:
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

			time.Sleep(DefaultCheckDuration)

		case <-ctxt.Done():
			return nil, ErrContextDone

		case <-timeout:
			break loop
		}
	}

	return nil, errors.New("timeout waiting for initial target")
}

// AddTarget adds a target using the supplied context.
func (c *CDP) AddTarget(ctxt context.Context, t client.Target) {
	c.Lock()
	defer c.Unlock()

	// create target manager
	h, err := NewTargetHandler(t)
	if err != nil {
		log.Printf("error: could not create handler for %s, got: %v", t, err)
		return
	}

	// run
	err = h.Run(ctxt)
	if err != nil {
		log.Printf("error: could not start handler for %s, got: %v", t, err)
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
func (c *CDP) Shutdown(ctxt context.Context, opts ...client.ClientOption) error {
	c.RLock()
	defer c.RUnlock()

	return c.r.Shutdown(ctxt, opts...)
}

// ListTargets returns the target IDs of the managed targets.
func (c *CDP) ListTargets() []string {
	c.RLock()
	defer c.RUnlock()

	targets := make([]string, len(c.handlers))
	i := 0
	for k := range c.handlerMap {
		targets[i] = k
		i++
	}

	return targets
}

// GetHandlerByIndex retrieves the domains manager for the specified index.
func (c *CDP) GetHandlerByIndex(i int) FrameHandler {
	c.RLock()
	defer c.RUnlock()

	if i < 0 || i >= len(c.handlers) {
		return nil
	}

	return c.handlers[i]
}

// GetHandlerByID retrieves the domains manager for the specified target ID.
func (c *CDP) GetHandlerByID(id string) FrameHandler {
	c.RLock()
	defer c.RUnlock()

	if i, ok := c.handlerMap[id]; ok {
		return c.handlers[i]
	}

	return nil
}

// SetHandler sets the active target to the target with the specified index.
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
	}

	return fmt.Errorf("no handler associated with target id %s", id)
}

// newTarget creates a new target using supplied context and options, returning
// the id of the created target only after the target has been started for
// monitoring.
func (c *CDP) newTarget(ctxt context.Context, opts ...client.ClientOption) (string, error) {
	c.RLock()
	cl := c.r.Client(opts...)
	c.RUnlock()

	// new page target
	t, err := cl.NewPageTarget(ctxt)
	if err != nil {
		return "", err
	}

	timeout := time.After(DefaultNewTargetTimeout)

loop:
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
			return "", ErrContextDone

		case <-timeout:
			break loop
		}
	}

	return "", errors.New("timeout waiting for new target to be available")
}

// SetTarget is an action that sets the active Chrome handler to the specified
// index i.
func (c *CDP) SetTarget(i int) Action {
	return ActionFunc(func(context.Context, FrameHandler) error {
		return c.SetHandler(i)
	})
}

// SetTargetByID is an action that sets the active Chrome handler to the handler
// associated with the specified id.
func (c *CDP) SetTargetByID(id string) Action {
	return ActionFunc(func(context.Context, FrameHandler) error {
		return c.SetHandlerByID(id)
	})
}

// NewTarget is an action that creates a new Chrome target, and sets it as the
// active target.
func (c *CDP) NewTarget(id *string, opts ...client.ClientOption) Action {
	return ActionFunc(func(ctxt context.Context, h FrameHandler) error {
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

// NewTargetWithURL creates a new Chrome target, sets it as the active target,
// and then navigates to the specified url.
func (c *CDP) NewTargetWithURL(urlstr string, id *string, opts ...client.ClientOption) Action {
	return ActionFunc(func(ctxt context.Context, h FrameHandler) error {
		n, err := c.newTarget(ctxt, opts...)
		if err != nil {
			return err
		}

		l := c.GetHandlerByID(n)
		if l == nil {
			return errors.New("could not retrieve newly created target")
		}

		/*err = Navigate(l, urlstr).Do(ctxt)
		if err != nil {
			return err
		}

		if id != nil {
			*id = n
		}*/

		return nil
	})
}

// CloseByIndex closes the Chrome target with specified index i.
func (c *CDP) CloseByIndex(i int) Action {
	return ActionFunc(func(ctxt context.Context, h FrameHandler) error {
		return nil
	})
}

// CloseByID closes the Chrome target with the specified id.
func (c *CDP) CloseByID(id string) Action {
	return ActionFunc(func(ctxt context.Context, h FrameHandler) error {
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

// Option is a Chrome Debugging Protocol option.
type Option func(*CDP) error

// WithRunner is a option to specify the underlying Chrome runner to monitor
// for page handlers.
func WithRunner(r *runner.Runner) Option {
	return func(c *CDP) error {
		c.r = r
		return nil
	}
}

// WithTargets is an option to specify the incoming targets to monitor for page
// handlers.
func WithTargets(watch <-chan client.Target) Option {
	return func(c *CDP) error {
		c.watch = watch
		return nil
	}
}

// WithRunnerOptions is a option to specify the options to pass to a newly
// created Chrome process runner.
func WithRunnerOptions(opts ...runner.CommandLineOption) Option {
	return func(c *CDP) error {
		c.opts = opts
		return nil
	}
}
