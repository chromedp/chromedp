package chromedp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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

	b := setupExecAllocator(defaultExecAllocatorOptions(false, true)...)
	if !b.keepOpen || b.visibleWindow {
		t.Fatalf("keepOpen %v, visibleWindow %v", b.keepOpen, b.visibleWindow)
	}
	if _, ok := b.initFlags["headless"]; !ok {
		t.Fatal("KeepOpen alone must keep the headless flag")
	}

	ctx, cancel := NewContext(context.Background(), WithKeepOpen())
	defer cancel()
	if !FromContext(ctx).Allocator.(*ExecAllocator).keepOpen {
		t.Fatal("WithKeepOpen did not build an allocator that keeps the browser open")
	}

	allocCtx, cancel := NewExecAllocator(context.Background(), DefaultExecAllocatorOptions[:]...)
	defer cancel()
	ctx, cancel = NewContext(allocCtx, WithKeepOpen())
	defer cancel()
	if FromContext(ctx).Allocator.(*ExecAllocator).keepOpen {
		t.Fatal("WithKeepOpen must not change an allocator that the caller made")
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

// startKept starts a headless browser with KeepOpen and runs one action in it.
// The cleanup kills the browser and removes its profile directory.
func startKept(t *testing.T, extra ...ExecAllocatorOption) (ctx context.Context, cancel context.CancelFunc) {
	t.Helper()
	opts := append(append([]ExecAllocatorOption{}, allocOpts...), KeepOpen)
	opts = append(opts, extra...)
	allocCtx, allocCancel := NewExecAllocator(context.Background(), opts...)
	ctx, cancel = NewContext(allocCtx)
	t.Cleanup(func() {
		cancel()
		allocCancel()
	})
	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	b := FromContext(ctx).Browser
	t.Cleanup(func() {
		// Registered last, so it runs first: kill the browser, then remove
		// the profile that KeepOpen left.
		b.process.Kill()
		<-b.exited
		killProfileProcesses(b.userDataDir)
		removeAllRetry(b.userDataDir)
	})
	return ctx, cancel
}

func TestKeepOpen(t *testing.T) {
	// The default profile directory is under the cache directory. Point it to
	// a temporary directory, so that the test leaves nothing behind.
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	t.Setenv("LocalAppData", cache)

	ctx, cancel := startKept(t)
	wsURL, dir := KeptOpen(ctx)
	if wsURL == "" || dir == "" {
		t.Fatalf("KeptOpen returned %q and %q", wsURL, dir)
	}
	if got, err := os.UserCacheDir(); err == nil && strings.HasPrefix(got, cache) {
		want := filepath.Join(got, "chromedp", keepOpenPrefix+strconv.Itoa(os.Getpid()))
		if dir != want {
			t.Fatalf("want the directory %q, got %q", want, dir)
		}
	}
	b := FromContext(ctx).Browser

	// Cancel the context. The browser must stay.
	cancel()
	select {
	case <-b.exited:
		t.Fatal("the browser exited when the context was canceled")
	case <-time.After(500 * time.Millisecond):
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the user data directory must stay: %v", err)
	}

	// Attach to the browser with a remote allocator, as a later program does.
	remoteCtx, remoteCancel := NewRemoteAllocator(context.Background(), wsURL)
	defer remoteCancel()
	taskCtx, taskCancel := NewContext(remoteCtx)
	defer taskCancel()
	got, err := Run(taskCtx, Evaluate[int](`1 + 2`))
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
}

func TestKeepOpenUserDataDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "profile")
	ctx, _ := startKept(t, UserDataDir(dir))
	if _, got := KeptOpen(ctx); got != dir {
		t.Fatalf("want the directory %q, got %q", dir, got)
	}
	if _, err := os.Stat(filepath.Join(dir, "DevToolsActivePort")); err != nil {
		t.Fatal(err)
	}
}

func TestCancelKeepsKeptBrowser(t *testing.T) {
	t.Parallel()

	ctx, _ := startKept(t, UserDataDir(filepath.Join(t.TempDir(), "profile")))
	b := FromContext(ctx).Browser
	if err := Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.exited:
		t.Fatal("Cancel closed a browser that was kept open")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestExitErrorKeptBrowser(t *testing.T) {
	t.Parallel()

	ctx, _ := startKept(t, UserDataDir(filepath.Join(t.TempDir(), "profile")))
	if err := FromContext(ctx).Browser.process.Kill(); err != nil {
		t.Fatal(err)
	}
	_, err := Run(ctx, Evaluate[int](`1 + 2`))
	var exit *exec.ExitError
	if !errors.As(err, &exit) || !strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("want the exit error of the killed browser, got %v", err)
	}
}

func TestWaitClosed(t *testing.T) {
	t.Parallel()

	for name, opts := range map[string][]ExecAllocatorOption{
		"normal":    nil,
		"kept open": {KeepOpen, UserDataDir(filepath.Join(t.TempDir(), "profile"))},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			allocCtx, allocCancel := NewExecAllocator(context.Background(), append(append([]ExecAllocatorOption{}, allocOpts...), opts...)...)
			defer allocCancel()
			ctx, cancel := NewContext(allocCtx)
			defer cancel()
			if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
				t.Fatal(err)
			}
			b := FromContext(ctx).Browser
			if opts != nil {
				defer func() {
					b.process.Kill()
					<-b.exited
					killProfileProcesses(b.userDataDir)
					removeAllRetry(b.userDataDir)
				}()
			}

			// While the browser runs, WaitClosed ends with the context.
			tctx, tcancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer tcancel()
			if err := WaitClosed(tctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("want the deadline error, got %v", err)
			}

			// When the process exits, it returns nil.
			go func() {
				time.Sleep(100 * time.Millisecond)
				b.process.Kill()
			}()
			done := make(chan error, 1)
			go func() { done <- WaitClosed(ctx) }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("WaitClosed did not return after the browser exited")
			}
		})
	}
}

// TestVisibleWindow starts a browser with a visible window. It needs a
// display, so it skips on Linux without one.
func TestVisibleWindow(t *testing.T) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("a visible window needs DISPLAY or WAYLAND_DISPLAY")
	}
	t.Parallel()

	allocCtx, allocCancel := NewExecAllocator(context.Background(), append(append([]ExecAllocatorOption{}, allocOpts...), VisibleWindow)...)
	defer allocCancel()
	ctx, cancel := NewContext(allocCtx)
	defer cancel() // kills the browser, so that no process stays
	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	if got, err := Run(ctx, Evaluate[int](`1 + 2`)); err != nil || got != 3 {
		t.Fatalf("want 3, got %d and %v", got, err)
	}
}
