package chromedp

import (
	"context"
	"errors"
	"testing"
	"time"
)

// testAllocateRuntimeDisabled opens a tab with WithRuntimeDisabled in the
// browser of the tests, and loads the page with the name.
func testAllocateRuntimeDisabled(t *testing.T, name string) context.Context {
	t.Helper()
	// testAllocate starts the browser of the tests once.
	_, cancel := testAllocate(t, "")
	cancel()
	ctx, _ := NewContext(browserCtx, WithRuntimeDisabled())
	t.Cleanup(func() {
		if err := Cancel(ctx); err != nil {
			t.Error(err)
		}
	})
	if err := Do(ctx, Navigate(testdataDir+"/"+name)); err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestRuntimeDisabledHidesTheProtocol(t *testing.T) {
	t.Parallel()

	// With the Runtime domain, the page can tell that the protocol drives it.
	ctx, cancel := testAllocate(t, "runtimeleak.html")
	defer cancel()
	if title, err := Run(ctx, Title()); err != nil || title != "detected" {
		t.Fatalf("by default, the page says %q, %v, want %q", title, err, "detected")
	}

	ctx = testAllocateRuntimeDisabled(t, "runtimeleak.html")
	if title, err := Run(ctx, Title()); err != nil || title != "not detected" {
		t.Fatalf("with the domain disabled, the page says %q, %v, want %q", title, err, "not detected")
	}
}

func TestRuntimeDisabledQueries(t *testing.T) {
	t.Parallel()

	ctx := testAllocateRuntimeDisabled(t, "iframe.html")
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// A query in the top frame runs in an isolated world of that frame.
	ids, err := Run(ctx, Nodes(CSS("#parent")))
	if err != nil || len(ids) != 1 {
		t.Fatalf("query in the top frame: got %d nodes, %v", len(ids), err)
	}

	// A query in the iframe runs in an isolated world of the iframe.
	iframes, err := Run(ctx, Nodes(CSS("iframe")))
	if err != nil {
		t.Fatal(err)
	}
	if err := Do(ctx, WaitVisible(CSS("#form"), FromNode(iframes[0]))); err != nil {
		t.Fatalf("query in the iframe: %v", err)
	}
	forms, err := Run(ctx, Nodes(CSS("#form"), FromNode(iframes[0])))
	if err != nil {
		t.Fatal(err)
	}
	if text, err := Run(ctx, Text(CSS("#foo"), FromNode(forms[0]))); err != nil || text != "insert" {
		t.Fatalf("text in the iframe: got %q, %v", text, err)
	}

	// A navigation drops the worlds, and the next query creates a new one.
	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	if text, err := Run(ctx, Text(CSS("#foo"))); err != nil || text != "insert" {
		t.Fatalf("text after a navigation: got %q, %v", text, err)
	}
}

func TestRuntimeDisabledPoll(t *testing.T) {
	t.Parallel()

	ctx := testAllocateRuntimeDisabled(t, "runtimeleak.html")
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Evaluate runs in the main world, and sees the globals of the page.
	if v, err := Run(ctx, Evaluate[int](`window.pageGlobal`)); err != nil || v != 1 {
		t.Fatalf("Evaluate: got %d, %v, want 1", v, err)
	}
	// Poll runs in the isolated world. It sees the DOM, and not the globals.
	if ok, err := Run(ctx, Poll[bool](`document.body !== null && typeof window.pageGlobal === "undefined"`, WithPollingTimeout(10*time.Second))); err != nil || !ok {
		t.Fatalf("Poll: got %v, %v", ok, err)
	}
	// Poll in a frame finds the world of that frame.
	if err := Do(ctx, Navigate(testdataDir+"/iframe.html")); err != nil {
		t.Fatal(err)
	}
	iframes, err := Run(ctx, Nodes(CSS("iframe")))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := Run(ctx, Poll[bool](`document.querySelector("#form") !== null`, WithPollingInFrame(iframes[0]), WithPollingTimeout(10*time.Second))); err != nil || !ok {
		t.Fatalf("Poll in the iframe: got %v, %v", ok, err)
	}
}

func TestRuntimeDisabledExposeFunc(t *testing.T) {
	t.Parallel()

	ctx := testAllocateRuntimeDisabled(t, "runtimeleak.html")
	err := Do(ctx, ExposeFunc("probe", func(context.Context, string) (string, error) { return "", nil }))
	if !errors.Is(err, ErrRuntimeDisabled) {
		t.Fatalf("got %v, want ErrRuntimeDisabled", err)
	}
}
