package chromedp

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
)

// windowID returns the id of the window of the tab of ctx.
func windowID(t *testing.T, ctx context.Context) browser.WindowID {
	t.Helper()
	c := FromContext(ctx)
	res, err := cdp.Call(ctx, c.Browser, browser.GetWindowForTarget, browser.GetWindowForTargetParams{TargetID: c.Target.TargetID})
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	return res.WindowID
}

func TestWithNewWindow(t *testing.T) {
	t.Parallel()

	// Use a browser of its own, because the tabs of this test must not change
	// the focus of the shared browser of the other tests.
	root, cancel := testAllocateSeparate(t)
	defer cancel()
	if _, err := Run(root, Title()); err != nil {
		t.Fatalf("got error: %v", err)
	}
	first := windowID(t, root)

	tests := []struct {
		name string
		opts []ContextOption
		same bool
	}{
		{"Default", nil, false},
		{"NewWindow", []ContextOption{WithNewWindow(true)}, false},
		{"SharedWindow", []ContextOption{WithNewWindow(false)}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tab, cancel := NewContext(root, test.opts...)
			defer cancel()
			if _, err := Run(tab, Title()); err != nil {
				t.Fatalf("got error: %v", err)
			}
			if got := windowID(t, tab) == first; got != test.same {
				t.Errorf("expected the same window as the first tab to be %t, got %t", test.same, got)
			}
		})
	}

	t.Run("Inherited", func(t *testing.T) {
		parent, cancel := NewContext(root, WithNewWindow(false))
		defer cancel()
		if _, err := Run(parent, Title()); err != nil {
			t.Fatalf("got error: %v", err)
		}
		child, cancel := NewContext(parent)
		defer cancel()
		if _, err := Run(child, Title()); err != nil {
			t.Fatalf("got error: %v", err)
		}
		if windowID(t, child) != first {
			t.Error("expected a child context to inherit WithNewWindow(false)")
		}
	})
}
