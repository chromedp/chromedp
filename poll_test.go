package chromedp

import (
	"testing"
	"time"

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
		// errAlt is the text of err in newer Chrome versions.
		errAlt string
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
			errAlt: "Inspected target navigated or closed (-32000)",
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
			var action Action[Void]
			var res bool
			if test.isFunc {
				action = into(&res, PollFunction[bool](test.js, test.opts...))
			} else {
				action = into(&res, Poll[bool](test.js, test.opts...))
			}
			startTime := time.Now()
			err := Do(tabCtx,
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
				} else if test.err != err.Error() && (test.errAlt == "" || test.errAlt != err.Error()) {
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

	frames, err := Run(ctx, Nodes(`frame[src="child1.html"]`, ByQuery))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	res, err := Run(ctx, Poll[string](`document.querySelector("#child1>p").textContent`, WithPollingInFrame(frames[0])))
	if err != nil {
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
	if err := Do(ctx, into(&res, Poll[*runtime.RemoteObject](expression, WithPollingTimeout(10*time.Millisecond)))); err != nil {
		t.Fatalf("got error: %v", err)
	}

	wantClassName := "Window"
	if res.ClassName != wantClassName {
		t.Fatalf("want class name of remote object to be %q, got %q", wantClassName, res.ClassName)
	}
}

func TestPollTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "poll.html")
	defer cancel()

	if err := Do(ctx, Poll[Void]("false", WithPollingTimeout(10*time.Millisecond))); err != ErrPollingTimeout {
		t.Errorf("got error: %v, want error: %v", err, ErrPollingTimeout)
	}

	var res1 *runtime.RemoteObject
	if err := Do(ctx, into(&res1, Poll[*runtime.RemoteObject]("false", WithPollingTimeout(10*time.Millisecond)))); err != ErrPollingTimeout {
		t.Errorf("got error: %v, want error: %v", err, ErrPollingTimeout)
	}

	var res2 []byte
	if err := Do(ctx, into(&res2, Poll[[]byte]("false", WithPollingTimeout(10*time.Millisecond)))); err != ErrPollingTimeout {
		t.Errorf("got error: %v, want error: %v", err, ErrPollingTimeout)
	}
}
