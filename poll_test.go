package chromedp

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
)

func TestPoll(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	tests := []struct {
		name   string
		js     string
		isFunc bool
		opts   []PollOption
		hash   string
		err    string
		delay  time.Duration
	}{
		{
			name:   "ExpressionPredicate",
			js:     "globalThis.__FOO === 1",
			isFunc: false,
		},
		{
			name:   "LambdaCallPredicate",
			js:     "(() => globalThis.__FOO === 1)()",
			isFunc: false,
		},
		{
			name:   "MultilinePredicate",
			js:     "\n(() => globalThis.__FOO === 1)()\n",
			isFunc: false,
		},
		{
			name:   "FunctionPredicate",
			js:     "function foo() { return globalThis.__FOO === 1; }",
			isFunc: true,
		},
		{
			name:   "LambdaAsFunction",
			js:     "() => globalThis.__FOO === 1",
			isFunc: true,
		},
		{
			name:   "Timeout",
			js:     "false",
			isFunc: false,
			opts:   []PollOption{WithPollingTimeout(10 * time.Millisecond)},
			err:    ErrPollingTimeout.Error(),
		},
		{
			name: "ResolvedRightBeforeExecutionContextDisposal",
			js: `() => {
				if (window.location.hash === '#reload'){
					window.location.replace(window.location.href.substring(0, 0 - '#reload'.length));
				}
				return true;
			}`,
			isFunc: true,
			hash:   "#reload",
		},
		{
			name: "NotSurviveNavigation",
			js: `() => {
				if (window.location.hash === '#navigate'){
					window.location.replace(window.location.href.substring(0, 0 - '#navigate'.length));
				} else {
					return globalThis.__FOO === 1;
				}
			}`,
			isFunc: true,
			hash:   "#navigate",
			err:    "Execution context was destroyed. (-32000)",
		},
		{
			name:   "PollingInterval",
			js:     "globalThis.__FOO === 1",
			isFunc: false,
			opts:   []PollOption{WithPollingInterval(100 * time.Millisecond)},
			hash:   "#100",
			delay:  50 * time.Millisecond,
		},
		{
			name: "PollingMutation",
			js: `() => {
				if (globalThis.__Mutation === 1){
					return true;
				} else {
					globalThis.__Mutation = 1;
					setTimeout(() => {
						document.body.appendChild(document.createElement('div'))
					}, 100);
				}
			}`,
			isFunc: true,
			opts:   []PollOption{WithPollingMutation(), WithPollingTimeout(200 * time.Millisecond)},
			delay:  50 * time.Millisecond,
		},
		{
			name:   "TimeoutWithoutMutation",
			js:     "globalThis.__Mutation === 1",
			isFunc: false,
			opts:   []PollOption{WithPollingMutation(), WithPollingTimeout(100 * time.Millisecond)},
			err:    ErrPollingTimeout.Error(),
		},
		{
			name: "TimeoutBeforeMutation",
			js: `() => {
				if (globalThis.__Mutation === 1){
					return true;
				} else {
					globalThis.__Mutation = 1;
					setTimeout(() => {
						document.body.appendChild(document.createElement('div'))
					}, 100);
				}
			}`,
			isFunc: true,
			opts:   []PollOption{WithPollingMutation(), WithPollingTimeout(50 * time.Millisecond)},
			err:    ErrPollingTimeout.Error(),
		},
		{
			name:   "ExtraArgs",
			js:     "(a1, a2) => a1 === 1 && a2 === 'str'",
			isFunc: true,
			opts:   []PollOption{WithPollingArgs(1, "str")},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tabCtx, tabCancel := NewContext(ctx)
			defer tabCancel()
			var action PollAction
			var res bool
			if test.isFunc {
				action = PollFunction(test.js, &res, test.opts...)
			} else {
				action = Poll(test.js, &res, test.opts...)
			}
			startTime := time.Now()
			err := Run(tabCtx,
				Navigate(testdataDir+"/poll.html"+test.hash),
				action,
			)
			if test.err == "" {
				if err != nil {
					t.Fatalf("got error: %v", err)
				} else if !res {
					t.Fatalf("got no error, but res is not true")
				}

			} else {
				if err == nil {
					t.Fatalf("expected err to be %q, got: %v", test.err, err)
				} else if test.err != err.Error() {
					t.Fatalf("want error to be %v, got: %v", test.err, err)
				}
			}
			if test.delay != 0 {
				delay := time.Since(startTime)
				if delay < test.delay {
					t.Fatalf("expected delay to be greater than %v, got: %v", test.delay, delay)
				}
			}
		})
	}
}

func TestPollFrame(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "frameset.html")
	defer cancel()

	var res string
	var frames []*cdp.Node
	if err := Run(ctx,
		Nodes(`frame[src="child1.html"]`, &frames, ByQuery),
		ActionFunc(func(ctx context.Context) error {
			return Poll(`document.querySelector("#child1>p").textContent`, &res, WithPollingInFrame(frames[0])).Do(ctx)
		}),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}

	want := "child one"
	if res != want {
		t.Fatalf("want result to be %q, got %q", want, res)
	}
}

func TestPollRemoteObject(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "poll.html")
	defer cancel()

	expression := `window`

	var res *runtime.RemoteObject
	if err := Run(ctx, Poll(expression, &res, WithPollingTimeout(10*time.Millisecond))); err != nil {
		t.Fatalf("got error: %v", err)
	}

	wantClassName := "Window"
	if res.ClassName != wantClassName {
		t.Fatalf("want class name of remote object to be %q, got %q", wantClassName, res.ClassName)
	}
}
