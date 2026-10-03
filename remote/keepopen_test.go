package remote

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/internal/chromedptest"
)

// keepOpenOpts returns the options of the exec allocator with KeepOpen and the
// websocket, followed by extra.
func keepOpenOpts(extra ...chromedp.ExecAllocatorOption) []chromedp.ExecAllocatorOption {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedptest.AllocOpts...)
	return append(append(opts, WebSocket, chromedp.KeepOpen), extra...)
}

// removeAllRetry removes dir. Chrome child processes can still write in dir for
// a short time after the main process exits, so it tries again.
func removeAllRetry(dir string) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.RemoveAll(dir)
		if err == nil || !time.Now().Before(deadline) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// browserAlive reports whether a browser listens on the websocket address.
func browserAlive(wsURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := DialContext(ctx, wsURL)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// killKept kills the kept browser of ctx, waits until it exits and removes its
// profile directory.
func killKept(ctx context.Context) {
	wsURL, dir := chromedp.KeptOpen(ctx)
	if p := chromedp.FromContext(ctx).Browser.Process(); p != nil {
		p.Kill()
	}
	for range 500 {
		if !browserAlive(wsURL) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	killProfileProcesses(dir)
	removeAllRetry(dir)
}

func TestWithKeepOpen(t *testing.T) {
	// The default profile directory is under the cache directory. Point it to
	// a temporary directory, so that the test leaves nothing behind.
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	t.Setenv("LocalAppData", cache)

	ctx, cancel := chromedp.NewContext(context.Background(), WithKeepOpen())
	defer cancel()
	if err := chromedp.Do(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killKept(ctx) })
	if wsURL, dir := chromedp.KeptOpen(ctx); wsURL == "" || dir == "" {
		t.Fatalf("WithKeepOpen did not build an allocator that keeps the browser open: %q and %q", wsURL, dir)
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), chromedptest.AllocOpts...)
	defer allocCancel()
	other, cancel := chromedp.NewContext(allocCtx, WithKeepOpen())
	defer cancel()
	if err := chromedp.Do(other); err != nil {
		t.Fatal(err)
	}
	if wsURL, _ := chromedp.KeptOpen(other); wsURL != "" {
		t.Fatal("WithKeepOpen must not change an allocator that the caller made")
	}
}

// startKept starts a headless browser with KeepOpen and runs one action in it.
// The cleanup kills the browser and removes its profile directory.
func startKept(t *testing.T, extra ...chromedp.ExecAllocatorOption) (ctx context.Context, cancel context.CancelFunc) {
	t.Helper()
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), keepOpenOpts(extra...)...)
	ctx, cancel = chromedp.NewContext(allocCtx)
	t.Cleanup(func() {
		cancel()
		allocCancel()
	})
	if err := chromedp.Do(ctx, chromedp.Navigate(chromedptest.TestdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	// Registered last, so it runs first: kill the browser, then remove the
	// profile that KeepOpen left.
	t.Cleanup(func() { killKept(ctx) })
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
	wsURL, dir := chromedp.KeptOpen(ctx)
	if wsURL == "" || dir == "" {
		t.Fatalf("KeptOpen returned %q and %q", wsURL, dir)
	}
	if got, err := os.UserCacheDir(); err == nil && strings.HasPrefix(got, cache) {
		want := filepath.Join(got, "chromedp", "keepopen-"+strconv.Itoa(os.Getpid()))
		if dir != want {
			t.Fatalf("want the directory %q, got %q", want, dir)
		}
	}

	// Cancel the context. The browser must stay.
	cancel()
	time.Sleep(500 * time.Millisecond)
	if !browserAlive(wsURL) {
		t.Fatal("the browser exited when the context was canceled")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the user data directory must stay: %v", err)
	}

	// Attach to the browser with a remote allocator, as a later program does.
	remoteCtx, remoteCancel := NewAllocator(context.Background(), wsURL)
	defer remoteCancel()
	taskCtx, taskCancel := chromedp.NewContext(remoteCtx)
	defer taskCancel()
	got, err := chromedp.Run(taskCtx, chromedp.Evaluate[int](`1 + 2`))
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
	ctx, _ := startKept(t, chromedp.UserDataDir(dir))
	if _, got := chromedp.KeptOpen(ctx); got != dir {
		t.Fatalf("want the directory %q, got %q", dir, got)
	}
	if _, err := os.Stat(filepath.Join(dir, "DevToolsActivePort")); err != nil {
		t.Fatal(err)
	}
}

func TestCancelKeepsKeptBrowser(t *testing.T) {
	t.Parallel()

	ctx, _ := startKept(t, chromedp.UserDataDir(filepath.Join(t.TempDir(), "profile")))
	wsURL, _ := chromedp.KeptOpen(ctx)
	if err := chromedp.Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if !browserAlive(wsURL) {
		t.Fatal("Cancel closed a browser that was kept open")
	}
}

func TestExitErrorKeptBrowser(t *testing.T) {
	t.Parallel()

	ctx, _ := startKept(t, chromedp.UserDataDir(filepath.Join(t.TempDir(), "profile")))
	if err := chromedp.FromContext(ctx).Browser.Process().Kill(); err != nil {
		t.Fatal(err)
	}
	_, err := chromedp.Run(ctx, chromedp.Evaluate[int](`1 + 2`))
	var exit *exec.ExitError
	if !errors.As(err, &exit) || !strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("want the exit error of the killed browser, got %v", err)
	}
}

func TestWaitClosed(t *testing.T) {
	t.Parallel()

	for name, opts := range map[string][]chromedp.ExecAllocatorOption{
		"normal":    {WebSocket},
		"kept open": {WebSocket, chromedp.KeepOpen, chromedp.UserDataDir(filepath.Join(t.TempDir(), "profile"))},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), append(append([]chromedp.ExecAllocatorOption{}, chromedptest.AllocOpts...), opts...)...)
			defer allocCancel()
			ctx, cancel := chromedp.NewContext(allocCtx)
			defer cancel()
			if err := chromedp.Do(ctx, chromedp.Navigate(chromedptest.TestdataDir+"/form.html")); err != nil {
				t.Fatal(err)
			}
			b := chromedp.FromContext(ctx).Browser
			if len(opts) > 1 {
				defer killKept(ctx)
			}

			// While the browser runs, WaitClosed ends with the context.
			tctx, tcancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer tcancel()
			if err := chromedp.WaitClosed(tctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("want the deadline error, got %v", err)
			}

			// When the process exits, it returns nil.
			go func() {
				time.Sleep(100 * time.Millisecond)
				b.Process().Kill()
			}()
			done := make(chan error, 1)
			go func() { done <- chromedp.WaitClosed(ctx) }()
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
