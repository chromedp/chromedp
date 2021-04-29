package chromedp

import (
	"context"
	"encoding/json"
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
		root    *cdp.Node
		execCtx runtime.ExecutionContextID
		ok      bool
	)
	for !ok {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
		_, root, execCtx, ok = t.ensureFrame()
	}

	fromNode := p.frame
	if fromNode == nil {
		fromNode = root
	} else {
		t.frameMu.RLock()
		frameID := t.enclosingFrame(fromNode)
		execCtx = t.execContexts[frameID]
		t.frameMu.RUnlock()
	}

	ea := &errAppender{args: make([]*runtime.CallArgument, 0, len(p.args)+3)}
	ea.append(p.predicate)
	if p.interval > 0 {
		ea.append(p.interval.Milliseconds())
	} else {
		ea.append(p.polling)
	}
	ea.append(p.timeout.Milliseconds())
	for _, arg := range p.args {
		ea.append(arg)
	}
	if ea.err != nil {
		return ea.err
	}

	v, exp, err := runtime.CallFunctionOn(waitForPredicatePageFunction).
		WithExecutionContextID(execCtx).
		WithReturnByValue(false).
		WithAwaitPromise(true).
		WithUserGesture(true).
		WithArguments(ea.args).
		Do(ctx)
	if err != nil {
		return err
	}
	if exp != nil {
		return exp
	}

	if v.Type == "undefined" {
		return ErrPollingTimeout
	}

	// it's okay to discard the result.
	if p.res == nil {
		return nil
	}

	switch x := p.res.(type) {
	case **runtime.RemoteObject:
		*x = v
		return nil

	case *[]byte:
		*x = v.Value
		return nil
	default:
		return json.Unmarshal(v.Value, p.res)
	}
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

// errAppender is to help accumulating the arguments and simplifying error checks.
//
// see https://blog.golang.org/errors-are-values
type errAppender struct {
	args []*runtime.CallArgument
	err  error
}

// append method calls the json.Marshal method to marshal the value and appends it to the slice.
// It records the first error for future reference.
// As soon as an error occurs, the append method becomes a no-op but the error value is saved.
func (ea *errAppender) append(v interface{}) {
	if ea.err != nil {
		return
	}
	var b []byte
	b, ea.err = json.Marshal(v)
	ea.args = append(ea.args, &runtime.CallArgument{Value: b})
}
