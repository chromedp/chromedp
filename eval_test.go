package chromedp

import (
	"reflect"
	"testing"

	"github.com/chromedp/cdproto/runtime"
)

func TestEvaluateNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		res        int
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			err := Run(ctx,
				Evaluate(tt.expression, &tt.res),
			)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("got error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || tt.wantErr != err.Error()) {
				t.Fatalf("wanted error: %q, got: %q", tt.wantErr, err)
			} else if tt.res != tt.want {
				t.Fatalf("want: %v, got: %v", tt.want, tt.res)
			}
		})
	}
}

func TestEvaluateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		res        string
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			err := Run(ctx,
				Evaluate(tt.expression, &tt.res),
			)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("got error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || tt.wantErr != err.Error()) {
				t.Fatalf("wanted error: %q, got: %q", tt.wantErr, err)
			} else if tt.res != tt.want {
				t.Fatalf("want: %v, got: %v", tt.want, tt.res)
			}
		})
	}
}

func TestEvaluateBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		res        []byte
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			err := Run(ctx,
				Evaluate(tt.expression, &tt.res),
			)
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if !reflect.DeepEqual(tt.res, tt.want) {
				t.Fatalf("want: %v, got: %v", tt.want, tt.res)
			}
		})
	}
}

func TestEvaluateRemoteObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expression string
		res        *runtime.RemoteObject
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			err := Run(ctx,
				Evaluate(tt.expression, &tt.res),
			)
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if string(tt.res.Type) != tt.wantType {
				t.Fatalf("want type: %v, got type: %v", tt.wantType, tt.res.Type)
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			err := Run(ctx,
				Evaluate(tt.expression, nil),
			)
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
		})
	}
}
