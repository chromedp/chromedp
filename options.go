package chromedp

import (
	"github.com/knq/chromedp/client"
	"github.com/knq/chromedp/runner"
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
