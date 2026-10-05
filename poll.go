package chromedp

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/runtime"
)

// pollTask holds the data of a poll task.
//
// See Poll for how to build poll tasks.
type pollTask struct {
	frame     *Node // the frame to evaluate the predicate, defaults to the root page
	predicate string
	polling   string        // the polling mode, defaults to "raf" (triggered by requestAnimationFrame)
	interval  time.Duration // the interval when the poll is triggered by a timer
	timeout   time.Duration // the poll timeout, defaults to 30 seconds
	args      []any
}

// run executes the poll task in the browser until the predicate returns a
// truthy value or the timeout ends.
func runPoll[T any](ctx context.Context, t *Target, p *pollTask) (T, error) {
	var (
		zero    T
		execCtx runtime.ExecutionContextID
		ok      bool
	)

	for {
		_, _, execCtx, ok = t.ensureFrame()
		if ok {
			break
		}
		if err := sleepContext(ctx, 5*time.Millisecond); err != nil {
			return zero, err
		}
	}

	if p.frame != nil {
		t.frameMu.RLock()
		frameID := t.enclosingFrame(p.frame)
		execCtx = t.execContexts[frameID]
		t.frameMu.RUnlock()
	}

	args := make([]any, 0, len(p.args)+3)
	args = append(args, p.predicate)
	if p.interval > 0 {
		args = append(args, p.interval.Milliseconds())
	} else {
		args = append(args, p.polling)
	}
	args = append(args, p.timeout.Milliseconds())
	args = append(args, p.args...)

	res, r, err := callFunctionOn[T](ctx, t, waitForPredicatePageFunction,
		func(p *runtime.CallFunctionOnParams) {
			p.ExecutionContextID = execCtx
			p.AwaitPromise = ptr(true)
			p.UserGesture = ptr(true)
		},
		args...,
	)

	if r != nil && r.Type == "undefined" {
		return zero, ErrPollingTimeout
	}

	return res, err
}

// Poll is a poll action that waits for a general JavaScript predicate.
// It builds the predicate from a JavaScript expression.
//
// This is a copy of [page.waitForFunction] of puppeteer.
// It is named Poll on purpose, so that it does not mix with the Wait* query actions.
// The behavior is not guaranteed to be compatible.
// For example, in our implementation the poll task does not survive a navigation.
// In this case the action returns an error (see unit test TestPoll/NotSurviveNavigation).
//
// # Polling Options
//
// The default polling mode is "raf". It runs pageFunction in a requestAnimationFrame callback all the time.
// This is the tightest polling mode, and it is suitable to observe styling changes.
// The WithPollingInterval option polls the predicate at a given interval.
// The WithPollingMutation option polls the predicate on every DOM mutation.
//
// The WithPollingTimeout option sets the maximum time to wait until the predicate returns a truthy value.
// It defaults to 30 seconds. Pass 0 to disable the timeout.
//
// The WithPollingInFrame option sets the frame in which to evaluate the predicate.
// Without it, the action evaluates the predicate in the root page of the current tab.
//
// The WithPollingArgs option gives extra arguments to the predicate.
// Use this option only when the predicate is built from a function.
// See [PollFunction].
//
// The action returns the truthy value of the predicate, decoded into the type
// T. T is handled as in [Evaluate]. Use [Void] when the value does not matter.
//
// [page.waitForFunction]: https://github.com/puppeteer/puppeteer/blob/v8.0.0/docs/api.md#pagewaitforfunctionpagefunction-options-args
func Poll[T any](expression string, opts ...PollOption) Action[T] {
	predicate := fmt.Sprintf(`return (%s);`, expression)
	return poll[T](predicate, opts...)
}

// PollFunction is a poll action that waits for a general JavaScript predicate.
// It builds the predicate from a JavaScript function.
//
// See [Poll] for how to build poll tasks.
func PollFunction[T any](pageFunction string, opts ...PollOption) Action[T] {
	predicate := fmt.Sprintf(`return (%s)(...args);`, pageFunction)

	return poll[T](predicate, opts...)
}

func poll[T any](predicate string, opts ...PollOption) Action[T] {
	p := &pollTask{
		predicate: predicate,
		polling:   "raf",
		timeout:   30 * time.Second,
	}

	// apply options
	for _, o := range opts {
		o(p)
	}
	return func(ctx context.Context, t *Target) (T, error) {
		return runPoll[T](ctx, t, p)
	}
}

// PollOption is a poll task option.
type PollOption = func(task *pollTask)

// WithPollingInterval polls the predicate at the given interval.
func WithPollingInterval(interval time.Duration) PollOption {
	return func(w *pollTask) {
		w.polling = ""
		w.interval = interval
	}
}

// WithPollingMutation polls the predicate on every DOM mutation.
func WithPollingMutation() PollOption {
	return func(w *pollTask) {
		w.polling = "mutation"
		w.interval = 0
	}
}

// WithPollingTimeout sets the maximum time to wait until the predicate returns a truthy value.
// It defaults to 30 seconds. Pass 0 to disable the timeout.
func WithPollingTimeout(timeout time.Duration) PollOption {
	return func(w *pollTask) {
		w.timeout = timeout
	}
}

// WithPollingInFrame sets the frame in which to evaluate the predicate.
// Without it, the action evaluates the predicate in the root page of the current tab.
func WithPollingInFrame(frame *Node) PollOption {
	return func(w *pollTask) {
		w.frame = frame
	}
}

// WithPollingArgs provides extra arguments to pass to the predicate.
func WithPollingArgs(args ...any) PollOption {
	return func(w *pollTask) {
		w.args = args
	}
}
