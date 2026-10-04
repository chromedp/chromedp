package chromedp

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/chromedp/cdproto/runtime"
)

func TestEvaluateNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		want       int
		wantErr    string
	}{
		{
			name:       "normal",
			expression: "123",
			want:       123,
		},
		{
			name:       "undefined",
			expression: "",
			wantErr:    "encountered an undefined value",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			res, err := Run(ctx, Evaluate[int](test.expression))
			if test.wantErr == "" && err != nil {
				t.Fatalf("got error: %v", err)
			}
			if test.wantErr != "" && (err == nil || test.wantErr != err.Error()) {
				t.Fatalf("wanted error: %q, got: %q", test.wantErr, err)
			} else if res != test.want {
				t.Fatalf("want: %v, got: %v", test.want, res)
			}
		})
	}
}

func TestEvaluateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		want       string
		wantErr    string
	}{
		{
			name:       "normal",
			expression: "'str'",
			want:       "str",
		},
		{
			name:       "undefined",
			expression: "",
			wantErr:    "encountered an undefined value",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			res, err := Run(ctx, Evaluate[string](test.expression))
			if test.wantErr == "" && err != nil {
				t.Fatalf("got error: %v", err)
			}
			if test.wantErr != "" && (err == nil || test.wantErr != err.Error()) {
				t.Fatalf("wanted error: %q, got: %q", test.wantErr, err)
			} else if res != test.want {
				t.Fatalf("want: %v, got: %v", test.want, res)
			}
		})
	}
}

func TestEvaluateBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		want       []byte
	}{
		{
			name:       "normal",
			expression: "'bytes'",
			want:       []byte(`"bytes"`),
		},
		{
			name:       "undefined",
			expression: "",
			want:       []byte(nil),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			res, err := Run(ctx, Evaluate[[]byte](test.expression))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if !reflect.DeepEqual(res, test.want) {
				t.Fatalf("want: %v, got: %v", test.want, res)
			}
		})
	}
}

func TestEvaluateRemoteObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		wantType   string
	}{
		{
			name:       "object",
			expression: "window",
			wantType:   "object",
		},
		{
			name:       "function",
			expression: "window.alert",
			wantType:   "function",
		},
		{
			name:       "undefined",
			expression: "",
			wantType:   "undefined",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			res, err := Run(ctx, Evaluate[*runtime.RemoteObject](test.expression))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if string(res.Type) != test.wantType {
				t.Fatalf("want type: %v, got type: %v", test.wantType, res.Type)
			}
		})
	}
}

// TestEvaluateLargeResult makes the browser send one result of 30 MB on the
// pipe. The pipe reads up to a zero byte, so a limit makes the test fail. The
// module remote has the same test for the websocket. See the issue 401.
func TestEvaluateLargeResult(t *testing.T) {
	t.Parallel()

	const size = 30 << 20
	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel := NewContext(allocCtx)
	defer cancel()

	got, err := Run(ctx, Evaluate[string](fmt.Sprintf(`"x".repeat(%d)`, size)))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != size {
		t.Fatalf("want %d bytes, got %d", size, len(got))
	}
	if got[0] != 'x' || got[size-1] != 'x' {
		t.Fatal("the result has the wrong content")
	}

	// The same connection must still work after a large message.
	if n, err := Run(ctx, Evaluate[int](`1 + 2`)); err != nil || n != 3 {
		t.Fatalf("want 3, got %d and %v", n, err)
	}
}

// TestEvaluateNullAndUndefined makes sure that the expression null gives
// ErrJSNull and the expression undefined gives ErrJSUndefined, for a type that
// cannot be nil, and that a type that can be nil gets nil.
func TestEvaluateNullAndUndefined(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	if _, err := Run(ctx, Evaluate[int]("null")); !errors.Is(err, ErrJSNull) {
		t.Errorf("Evaluate[int](null): got error %v, want %v", err, ErrJSNull)
	}
	if _, err := Run(ctx, Evaluate[string]("null")); !errors.Is(err, ErrJSNull) {
		t.Errorf("Evaluate[string](null): got error %v, want %v", err, ErrJSNull)
	}
	if _, err := Run(ctx, Evaluate[bool]("undefined")); !errors.Is(err, ErrJSUndefined) {
		t.Errorf("Evaluate[bool](undefined): got error %v, want %v", err, ErrJSUndefined)
	}
	if p, err := Run(ctx, Evaluate[*int]("null")); err != nil || p != nil {
		t.Errorf("Evaluate[*int](null): got %v, %v, want nil, nil", p, err)
	}
	if m, err := Run(ctx, Evaluate[map[string]int]("null")); err != nil || m != nil {
		t.Errorf("Evaluate[map](null): got %v, %v, want nil, nil", m, err)
	}
	// The string "null" is a value, and not the JavaScript null.
	if s, err := Run(ctx, Evaluate[string](`"null"`)); err != nil || s != "null" {
		t.Errorf(`Evaluate[string]("null"): got %q, %v`, s, err)
	}
	if n, err := Run(ctx, Evaluate[int]("0")); err != nil || n != 0 {
		t.Errorf("Evaluate[int](0): got %d, %v", n, err)
	}
}

// TestEvaluateAwaitPromise makes sure that EvalAwaitPromise waits for a promise
// and that a rejected promise is an error.
func TestEvaluateAwaitPromise(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	n, err := Run(ctx, Evaluate[int](`new Promise(resolve => setTimeout(() => resolve(42), 20))`, EvalAwaitPromise))
	if err != nil || n != 42 {
		t.Errorf("got %d, %v, want 42", n, err)
	}
	var exc *ExceptionError
	_, err = Run(ctx, Evaluate[int](`Promise.reject(new Error("no"))`, EvalAwaitPromise))
	if !errors.As(err, &exc) {
		t.Errorf("got error %v, want an *ExceptionError", err)
	}
}

func TestEvaluateNil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
	}{
		{
			name:       "number",
			expression: "123",
		},
		{
			name:       "string",
			expression: "'str'",
		},
		{
			name:       "undefined",
			expression: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			_, err := Run(ctx, Evaluate[Void](test.expression))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
		})
	}
}
