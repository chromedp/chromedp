package chromedp

import (
	"github.com/igsky/chromedp/client"
	"github.com/igsky/chromedp/runner"
	"github.com/igsky/chromedp/cdp/network"
	"github.com/igsky/chromedp/cdp/inspector"
	"github.com/igsky/chromedp/cdp/page"
	"github.com/igsky/chromedp/cdp/dom"
	"github.com/igsky/chromedp/cdp/css"
	logdom "github.com/igsky/chromedp/cdp/log"
	rundom "github.com/igsky/chromedp/cdp/runtime"
)

// Option is a Chrome Debugging Protocol option.
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

// WithRunnerOptions is a CDP option to specify the options to pass to a newly
// created Chrome process runner.
func WithRunnerOptions(opts ...runner.CommandLineOption) Option {
	return func(c *CDP) error {
		c.opts = opts
		return nil
	}
}

// LogFunc is the common logging func type.
type LogFunc func(string, ...interface{})

// WithLogf is a CDP option to specify a func to receive general logging.
func WithLogf(f LogFunc) Option {
	return func(c *CDP) error {
		c.logf = f
		return nil
	}
}

// WithDebugf is a CDP option to specify a func to receive debug logging (ie,
// protocol information).
func WithDebugf(f LogFunc) Option {
	return func(c *CDP) error {
		c.debugf = f
		return nil
	}
}

// WithErrorf is a CDP option to specify a func to receive error logging.
func WithErrorf(f LogFunc) Option {
	return func(c *CDP) error {
		c.errorf = f
		return nil
	}
}

// WithLog is a CDP option that sets the logging, debugging, and error funcs to
// f.
func WithLog(f LogFunc) Option {
	return func(c *CDP) error {
		c.logf = f
		c.debugf = f
		c.errorf = f
		return nil
	}
}

// WithConsolef is a CDP option to specify a func to receive chrome log events.
//
// Note: NOT YET IMPLEMENTED.
func WithConsolef(f LogFunc) Option {
	return func(c *CDP) error {
		return nil
	}
}

// HookChain is a slice of MsgHooks
type HookChain []MsgHook

// Add is a convenient way to add new MsgHook
func (c *HookChain) Add(h MsgHook) {
	*c = append(*c, h)
}

// Process passes msg through all hooks
func (c *HookChain) Process(msg interface{}) {
	for _, hook := range *c {
		hook(msg)
	}
}

// MsgHook is a CDP message handler
type MsgHook func(interface{})

// WithCustomHook adds provided hook to CDP
func WithCustomHook(f MsgHook) Option {
	return func(c *CDP) error {
		c.hookChain.Add(f)
		return nil
	}
}

type ChromeDomains []Action

func WithDefaultDomains() Option {
	return func(c *CDP) error {
		c.Config.domains = []Action{
			logdom.Enable(),
			rundom.Enable(),
			inspector.Enable(),
			page.Enable(),
			dom.Enable(),
			css.Enable(),
		}
		return nil
	}
}

func WithCustomDomain(f func() Action) Option {
	return func(c *CDP) error {
		c.Config.domains = append(c.Config.domains, f())
		return nil
	}
}

// Config is a struct of all optional features
type Config struct {
	// logging funcs
	logf, debugf, errorf LogFunc

	// optional message hooks
	hookChain HookChain

	// optional custom domains
	// note: overrides default behaviour
	domains ChromeDomains
}


