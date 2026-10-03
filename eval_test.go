package chromedp

import (
	"context"
	"fmt"
	"reflect"
	"slices"
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

// TestEvaluateLargeResult makes the browser send one result of 30 MB on each
// transport. The pipe reads up to a zero byte, and the websocket reads one
// frame, so a limit in either one makes the test fail. See the issue 401.
func TestEvaluateLargeResult(t *testing.T) {
	t.Parallel()

	const size = 30 << 20
	for _, name := range []string{"pipe", "websocket"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := allocOpts
			if name == "websocket" {
				opts = append(slices.Clone(allocOpts), WebSocket)
			}
			allocCtx, cancel := NewExecAllocator(context.Background(), opts...)
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
		})
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
