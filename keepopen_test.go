package chromedp

import (
	"context"
	"testing"
)

func TestKeepOpenOptions(t *testing.T) {
	t.Parallel()

	a := setupExecAllocator(KeepOpen)
	if !a.keepOpen {
		t.Fatal("KeepOpen did not set keepOpen")
	}
	if a.usesPipe() {
		t.Fatal("a kept browser must use the websocket, because the pipe closes with the program")
	}
	if setupExecAllocator(DefaultExecAllocatorOptions[:]...).keepOpen {
		t.Fatal("the default options must not keep the browser open")
	}

	b := setupExecAllocator(defaultExecAllocatorOptions(false, []ExecAllocatorOption{KeepOpen})...)
	if !b.keepOpen || b.visibleWindow {
		t.Fatalf("keepOpen %v, visibleWindow %v", b.keepOpen, b.visibleWindow)
	}
	if _, ok := b.initFlags["headless"]; !ok {
		t.Fatal("KeepOpen alone must keep the headless flag")
	}

	ctx, cancel := NewContext(context.Background(), WithAllocatorOptions(KeepOpen))
	defer cancel()
	if !FromContext(ctx).Allocator.(*ExecAllocator).keepOpen {
		t.Fatal("WithAllocatorOptions did not build an allocator that keeps the browser open")
	}

	allocCtx, cancel := NewExecAllocator(context.Background(), DefaultExecAllocatorOptions[:]...)
	defer cancel()
	ctx, cancel = NewContext(allocCtx, WithAllocatorOptions(KeepOpen))
	defer cancel()
	if FromContext(ctx).Allocator.(*ExecAllocator).keepOpen {
		t.Fatal("WithAllocatorOptions must not change an allocator that the caller made")
	}
}

func TestKeptOpenWithoutBrowser(t *testing.T) {
	t.Parallel()

	ctx, cancel := NewContext(context.Background())
	defer cancel()
	if u, d := KeptOpen(ctx); u != "" || d != "" {
		t.Fatalf("want empty strings, got %q and %q", u, d)
	}
	if u, d := KeptOpen(context.Background()); u != "" || d != "" {
		t.Fatalf("want empty strings, got %q and %q", u, d)
	}
}
