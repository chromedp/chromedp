package chromedp

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
)

func TestPoll(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	tests := []struct {
		name         string
		js           string
		isFunction   bool
		options      []PollOption
		hash         string
		wantErr      error
		wantMinDelay time.Duration
	}{
		{
			name:       "ExpressionPredicate",
			js:         "globalThis.__FOO === 1",
			isFunction: false,
		},
		{
			name:       "LambdaCallPredicate",
			js:         "(() => globalThis.__FOO === 1)()",
			isFunction: false,
		},
		{
			name:       "MultilinePredicate",
			js:         "\n(() => globalThis.__FOO === 1)()\n",
			isFunction: false,
		},
		{
			name:       "FunctionPredicate",
			js:         "function foo(){return globalThis.__FOO === 1;}",
			isFunction: true,
		},
		{
			name:       "LambdaAsFunction",
			js:         "() => globalThis.__FOO === 1",
			isFunction: true,
		},
		{
			name:       "Timeout",
			js:         "false",
			isFunction: false,
			options:    []PollOption{WithPollingTimeout(10 * time.Millisecond)},
			wantErr:    ErrPollingTimeout,
		},
		{
			name: "ResolvedRightBeforeExecutionContextDisposal",
			js: `()=>{
				if (window.location.hash === '#reload'){
					window.location.replace(window.location.href.substring(0, 0 - '#reload'.length));
				}
				return true;
			}`,
			isFunction: true,
			hash:       "#reload",
		},
		{
			name: "NotSurviveNavigation",
			js: `()=>{
				if (window.location.hash === '#navigate'){
					window.location.replace(window.location.href.substring(0, 0 - '#navigate'.length));
				} else {
					return globalThis.__FOO === 1;;
				}
			}`,
			isFunction: true,
			hash:       "#navigate",
			wantErr:    &cdproto.Error{Code: -32000, Message: "Execution context was destroyed."},
		},
		{
			name:         "PollingInterval",
			js:           "globalThis.__FOO === 1",
			isFunction:   false,
			options:      []PollOption{WithPollingInterval(100 * time.Millisecond)},
			hash:         "#100",
			wantMinDelay: 50 * time.Millisecond,
		},
		{
			name:         "PollingMutation",
			js:           "globalThis.__FOO === 1",
			isFunction:   false,
			options:      []PollOption{WithPollingMutation(), WithPollingTimeout(200 * time.Millisecond)},
			hash:         "#mutation",
			wantMinDelay: 50 * time.Millisecond,
		},
		{
			name:       "TimeoutWithoutMutation ",
			js:         "globalThis.__FOO === 1",
			isFunction: false,
			options:    []PollOption{WithPollingMutation(), WithPollingTimeout(100 * time.Millisecond)},
			wantErr:    ErrPollingTimeout,
		},
		{
			name:       "TimeoutBeforeMutation ",
			js:         "globalThis.__FOO === 1",
			isFunction: false,
			options:    []PollOption{WithPollingMutation(), WithPollingTimeout(50 * time.Millisecond)},
			hash:       "#mutation",
			wantErr:    ErrPollingTimeout,
		},
		{
			name:       "ExtraArgs",
			js:         "(a1, a2) => a1 === 1 && a2 === 'str'",
			isFunction: true,
			options:    []PollOption{WithPollingArgs(1, "str")},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tabCtx, tabCancel := NewContext(ctx)
			defer tabCancel()
			var action PollAction
			var res bool
			if test.isFunction {
				action = PollFunction(test.js, &res, test.options...)
			} else {
				action = Poll(test.js, &res, test.options...)
			}

			startTime := time.Now()
			err := Run(tabCtx,
				Navigate(testdataDir+"/poll.html"+test.hash),
				action,
			)
			if !reflect.DeepEqual(test.wantErr, err) {
				t.Fatalf("want error to be %q, got %q", test.wantErr, err)
			}
			if test.wantErr == nil && !res {
				t.Fatal("it should resolve with truthy result")
			}
			if test.wantMinDelay != 0 {
				delay := time.Since(startTime)
				if delay < test.wantMinDelay {
					t.Fatalf("want min delay to be %v, got %v", test.wantMinDelay, delay)
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
