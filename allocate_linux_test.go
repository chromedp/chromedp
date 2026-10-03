//go:build linux

package chromedp

import (
	"context"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// TestStartFromLockedThread starts a browser from a goroutine that locks its OS
// thread and then ends. The thread ends with the goroutine. The allocator sets
// Pdeathsig, so the kernel kills a browser that the thread started when the
// thread ends. The browser must survive, because the allocator starts it from
// its own goroutine. See the issue 1566.
func TestStartFromLockedThread(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel := NewContext(allocCtx)
	defer cancel()

	var errc chan error
	for range 1000 {
		errc = make(chan error, 1)
		go func() {
			runtime.LockOSThread()
			if syscall.Gettid() == os.Getpid() {
				// The runtime never ends the main thread. Try again on
				// another thread.
				runtime.UnlockOSThread()
				errc <- nil
				return
			}
			// The goroutine does not unlock the thread, so the thread ends
			// when the goroutine returns.
			errc <- Do(ctx)
		}()
		err := <-errc
		if err != nil {
			t.Fatal(err)
		}
		if FromContext(ctx).Browser != nil {
			break
		}
	}
	if FromContext(ctx).Browser == nil {
		t.Skip("no goroutine ran on a thread other than the main thread")
	}
	// Give the kernel time to deliver the signal of a dead thread.
	time.Sleep(500 * time.Millisecond)
	got, err := Run(ctx, Evaluate[int](`1 + 2`))
	if err != nil {
		t.Fatalf("the browser died after the thread that started it ended: %v", err)
	}
	if got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
}
