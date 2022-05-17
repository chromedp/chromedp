package chromedp

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
)

// PollAction are actions that will wait for a general Javascript predicate.
//
// See Poll for details on building poll tasks.
type PollAction Action

// pollTask holds information pertaining to an poll task.
//
// See Poll for details on building poll tasks.
type pollTask struct {
	frame     *cdp.Node // the frame to evaluate the predicate, defaults to the root page
	predicate string
	polling   string        // the polling mode, defaults to "raf" (triggered by requestAnimationFrame)
	interval  time.Duration // the interval when the poll is triggered by a timer
	timeout   time.Duration // the poll timeout, defaults to 30 seconds
	args      []interface{}
	res       interface{}
}

// Do executes the poll task in the browser,
// until the predicate either returns truthy value or the timeout happens.
func (p *pollTask) Do(ctx context.Context) error {
	t := cdp.ExecutorFromContext(ctx).(*Target)
	if t == nil {
		return ErrInvalidTarget
	}
	var (
		execCtx runtime.ExecutionContextID
		ok      bool
	)

	for {
		_, _, execCtx, ok = t.ensureFrame()
		if ok {
			break
		}
		if err := sleepContext(ctx, 5*time.Millisecond); err != nil {
			return err
		}
	}

	if p.frame != nil {
		t.frameMu.RLock()
		frameID := t.enclosingFrame(p.frame)
		execCtx = t.execContexts[frameID]
		t.frameMu.RUnlock()
	}

	args := make([]interface{}, 0, len(p.args)+3)
	args = append(args, p.predicate)
	if p.interval > 0 {
		args = append(args, p.interval.Milliseconds())
	} else {
		args = append(args, p.polling)
	}
	args = append(args, p.timeout.Milliseconds())
	for _, arg := range p.args {
		args = append(args, arg)
	}

	err := CallFunctionOn(waitForPredicatePageFunction, p.res,
		func(p *runtime.CallFunctionOnParams) *runtime.CallFunctionOnParams {
			return p.WithExecutionContextID(execCtx).
				WithAwaitPromise(true).
				WithUserGesture(true)
		},
		args...,
	).Do(ctx)

	// FIXME: sentinel error?
	if err != nil && err.Error() == "encountered an undefined value" {
		return ErrPollingTimeout
	}

	return err
}

// Poll is a poll action that will wait for a general Javascript predicate.
// It builds the predicate from a Javascript expression.
//
// This is a copy of puppeteer's page.waitForFunction.
// see https://github.com/puppeteer/puppeteer/blob/v8.0.0/docs/api.md#pagewaitforfunctionpagefunction-options-args.
// It's named Poll intentionally to avoid messing up with the Wait* query actions.
// The behavior is not guaranteed to be compatible.
// For example, our implementation makes the poll task not survive from a navigation,
// and an error is raised in this case (see unit test TestPoll/NotSurviveNavigation).
//
// Polling Options
//
// The default polling mode is "raf", to constantly execute pageFunction in requestAnimationFrame callback.
// This is the tightest polling mode which is suitable to observe styling changes.
// The WithPollingInterval option makes it to poll the predicate with a specified interval.
// The WithPollingMutation option makes it to poll the predicate on every DOM mutation.
//
// The WithPollingTimeout option specifies the maximum time to wait for the predicate returns truthy value.
// It defaults to 30 seconds. Pass 0 to disable timeout.
//
// The WithPollingInFrame option specifies the frame in which to evaluate the predicate.
// If not specified, it will be evaluated in the root page of the current tab.
//
// The WithPollingArgs option provides extra arguments to pass to the predicate.
// Only apply this option when the predicate is built from a function.
// See PollFunction.
func Poll(expression string, res interface{}, opts ...PollOption) PollAction {
	predicate := fmt.Sprintf(`return (%s);`, expression)
	return poll(predicate, res, opts...)
}

// PollFunction is a poll action that will wait for a general Javascript predicate.
// It builds the predicate from a Javascript function.
//
// See Poll for details on building poll tasks.
func PollFunction(pageFunction string, res interface{}, opts ...PollOption) PollAction {
	predicate := fmt.Sprintf(`return (%s)(...args);`, pageFunction)

	return poll(predicate, res, opts...)
}

func poll(predicate string, res interface{}, opts ...PollOption) PollAction {
	p := &pollTask{
		predicate: predicate,
		polling:   "raf",
		timeout:   30 * time.Second,
		res:       res,
	}

	// apply options
	for _, o := range opts {
		o(p)
	}
	return p
}

// PollOption is an poll task option.
type PollOption = func(task *pollTask)

// WithPollingInterval makes it to poll the predicate with the specified interval.
func WithPollingInterval(interval time.Duration) PollOption {
	return func(w *pollTask) {
		w.polling = ""
		w.interval = interval
	}
}

// WithPollingMutation makes it to poll the predicate on every DOM mutation.
func WithPollingMutation() PollOption {
	return func(w *pollTask) {
		w.polling = "mutation"
		w.interval = 0
	}
}

// WithPollingTimeout specifies the maximum time to wait for the predicate returns truthy value.
// It defaults to 30 seconds. Pass 0 to disable timeout.
func WithPollingTimeout(timeout time.Duration) PollOption {
	return func(w *pollTask) {
		w.timeout = timeout
	}
}

// WithPollingInFrame specifies the frame in which to evaluate the predicate.
// If not specified, it will be evaluated in the root page of the current tab.
func WithPollingInFrame(frame *cdp.Node) PollOption {
	return func(w *pollTask) {
		w.frame = frame
	}
}

// WithPollingArgs provides extra arguments to pass to the predicate.
func WithPollingArgs(args ...interface{}) PollOption {
	return func(w *pollTask) {
		w.args = args
	}
}
