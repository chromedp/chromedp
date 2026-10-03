package chromedp

import (
	"context"
	"testing"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
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

	// A new browser context has no window, so its first tab opens in a new
	// window, even when the tabs of the browser share a window.
	t.Run("NewBrowserContext", func(t *testing.T) {
		tab, cancel := NewContext(root, WithNewWindow(false), WithNewBrowserContext())
		defer cancel()
		if _, err := Run(tab, Title()); err != nil {
			t.Fatalf("got error: %v", err)
		}
		if windowID(t, tab) == first {
			t.Error("expected the tab of a new browser context to have its own window")
		}
	})

	// The browser context that the test creates has no window either, and
	// WithExistingBrowserContext names it.
	t.Run("ExistingBrowserContext", func(t *testing.T) {
		res, err := cdp.Call(root, FromContext(root).Browser, target.CreateBrowserContext, target.CreateBrowserContextParams{})
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		tab, cancel := NewContext(root, WithNewWindow(false), WithExistingBrowserContext(res.BrowserContextID))
		defer cancel()
		if _, err := Run(tab, Title()); err != nil {
			t.Fatalf("got error: %v", err)
		}
		if windowID(t, tab) == first {
			t.Error("expected the tab of another browser context to have its own window")
		}
	})
}
