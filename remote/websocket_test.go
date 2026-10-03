package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/internal/chromedptest"
)

// websocketOpts returns the options of the exec allocator in the websocket
// mode, followed by extra.
func websocketOpts(extra ...chromedp.ExecAllocatorOption) []chromedp.ExecAllocatorOption {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedptest.AllocOpts...)
	return append(append(opts, WebSocket), extra...)
}

// captureUserDataDir returns an option that stores the user data directory of
// the browser in *dir, so that a test can check that the allocator removes it.
// The option replaces the default func of ModifyCmdFunc, and it is only for
// tests.
func captureUserDataDir(dir *string) chromedp.ExecAllocatorOption {
	return chromedp.ModifyCmdFunc(func(cmd *exec.Cmd) {
		for _, arg := range cmd.Args {
			if v, ok := strings.CutPrefix(arg, "--user-data-dir="); ok {
				*dir = v
			}
		}
	})
}

func TestDialTimeout(t *testing.T) {
	t.Parallel()

	t.Run("ShortTimeoutError", func(t *testing.T) {
		t.Parallel()
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		url := "ws://" + l.(*net.TCPListener).Addr().String()
		defer l.Close()

		allocCtx, cancel := NewAllocator(t.Context(), url, NoModifyURL, WithDialTimeout(time.Microsecond))
		defer cancel()
		ctx, cancel := chromedp.NewContext(allocCtx)
		defer cancel()
		err = chromedp.Do(ctx)
		got, want := fmt.Sprintf("%v", err), "i/o timeout"
		if !strings.Contains(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("NoTimeoutSuccess", func(t *testing.T) {
		t.Parallel()
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		url := "ws://" + l.(*net.TCPListener).Addr().String()
		defer l.Close()
		go func() {
			conn, err := l.Accept()
			if err == nil {
				conn.Close()
			}
		}()

		allocCtx, cancel := NewAllocator(t.Context(), url, NoModifyURL, WithDialTimeout(0))
		defer cancel()
		ctx, cancel := chromedp.NewContext(allocCtx)
		defer cancel()
		err = chromedp.Do(ctx)
		got := fmt.Sprintf("%v", err)
		if !strings.Contains(got, "EOF") && !strings.Contains(got, "connection reset") {
			t.Fatalf("got %q, want %q or %q", got, "EOF", "connection reset")
		}
	})
}

// TestExecAllocatorWebSocket checks that the websocket mode works, and that the
// flags for a debugging port or address select it. Only the websocket mode
// prints the address line, so the output shows which transport the allocator
// used.
func TestExecAllocatorWebSocket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []chromedp.ExecAllocatorOption
	}{
		{name: "WebSocket option"},
		{name: "remote-debugging-port flag", opts: []chromedp.ExecAllocatorOption{chromedp.Flag("remote-debugging-port", "0")}},
		{name: "remote-debugging-address flag", opts: []chromedp.ExecAllocatorOption{chromedp.Flag("remote-debugging-address", "127.0.0.1")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var dir string
			buf := new(syncBuffer)
			allocCtx, cancel := chromedp.NewExecAllocator(context.Background(),
				websocketOpts(append(test.opts, chromedp.CombinedOutput(buf), captureUserDataDir(&dir))...)...)
			defer cancel()
			ctx, _ := chromedp.NewContext(allocCtx)

			var got string
			if err := chromedp.Do(ctx,
				chromedp.Navigate(chromedptest.TestdataDir+"/form.html"),
				chromedptest.Into(&got, chromedp.Text(chromedp.ID("foo"))),
			); err != nil {
				t.Fatal(err)
			}
			if want := "insert"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
			if err := chromedp.Cancel(ctx); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), "DevTools listening on") {
				t.Fatal("the browser did not use the websocket")
			}
			if _, err := os.Lstat(dir); dir == "" || !os.IsNotExist(err) {
				t.Fatalf("temporary user data dir %q not deleted", dir)
			}
		})
	}
}

// syncBuffer is a bytes.Buffer that is safe to use from more than one
// goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write satisfies the io.Writer interface.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns the text that was written.
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestExitErrorAfterKill kills the browser process and checks that the next
// call returns the exit error of the process. See the issue 408.
func TestExitErrorAfterKill(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), websocketOpts()...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	if err := chromedp.Do(ctx); err != nil {
		t.Fatal(err)
	}
	b := chromedp.FromContext(ctx).Browser
	if err := b.Process().Signal(os.Kill); err != nil {
		t.Fatal(err)
	}

	// The first call can race with the loss of the connection, so ask until
	// the call fails. Every later call must fail in the same way.
	for i := range 3 {
		_, err := chromedp.Run(ctx, chromedp.Evaluate[int](`1 + 2`))
		if err == nil {
			t.Fatalf("call %d: want an error from a dead browser", i)
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("call %d: want an *exec.ExitError in %q", i, err)
		}
		if !strings.Contains(err.Error(), "signal: killed") {
			t.Fatalf("call %d: want the signal in %q", i, err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("call %d: want context.Canceled in %q", i, err)
		}
	}
}

// TestCombinedOutput checks that the websocket mode prints the address line in
// the combined output, and that the output holds all messages of the page.
func TestCombinedOutput(t *testing.T) {
	t.Parallel()

	buf := new(syncBuffer)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(),
		websocketOpts(chromedp.CombinedOutput(buf), chromedp.Flag("enable-logging", "stderr"))...)
	defer cancel()

	taskCtx, _ := chromedp.NewContext(allocCtx)
	if err := chromedp.Do(taskCtx,
		chromedp.Navigate(chromedptest.TestdataDir+"/consolespam.html"),
	); err != nil {
		t.Fatal(err)
	}
	cancel()
	if !strings.Contains(buf.String(), "DevTools listening on") {
		t.Fatal("the output has no websocket string")
	}
	// Recent chrome versions replace many "spam" messages with "spam 1",
	// "spam 2", and so on. Search for the prefix only.
	if want, got := 2000, strings.Count(buf.String(), `"spam`); want != got {
		t.Fatalf("want %d spam console logs, got %d", want, got)
	}
}

// TestEvaluateLargeResult makes the browser send one result of 30 MB on the
// websocket. The websocket reads one frame, so a limit makes the test fail. The
// core module has the same test for the pipe. See the issue 401.
func TestEvaluateLargeResult(t *testing.T) {
	t.Parallel()

	const size = 30 << 20
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), websocketOpts()...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	got, err := chromedp.Run(ctx, chromedp.Evaluate[string](fmt.Sprintf(`"x".repeat(%d)`, size)))
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
	if n, err := chromedp.Run(ctx, chromedp.Evaluate[int](`1 + 2`)); err != nil || n != 3 {
		t.Fatalf("want 3, got %d and %v", n, err)
	}
}

func TestLargeOutboundMessages(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), websocketOpts()...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// ~5MiB of JS to test the grow feature of github.com/gobwas/ws.
	expr := fmt.Sprintf("//%s\n", strings.Repeat("x", 5<<20))
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[[]byte](expr)); err != nil {
		t.Fatal(err)
	}
}

// TestExecAllocatorDebugf makes sure that the protocol logger of the browser
// receives the messages of the websocket. The connection gets the logger
// through the method SetDebugf.
func TestExecAllocatorDebugf(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var sent, received int
	debugf := func(format string, _ ...any) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasPrefix(format, "->"):
			sent++
		case strings.HasPrefix(format, "<-"):
			received++
		}
	}
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), websocketOpts()...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithDebugf(debugf))
	defer cancel()
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[int](`1 + 2`)); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if sent == 0 || received == 0 {
		t.Fatalf("the logger got %d sent and %d received messages", sent, received)
	}
}

// startChrome starts a browser with a debugging port and returns its websocket
// address. The cleanup of the test stops the browser.
func startChrome(t *testing.T) string {
	t.Helper()
	procCtx, procCancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, chromedptest.ExecPath,
		"--no-first-run",
		"--no-default-browser-check",
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--user-data-dir="+t.TempDir(),
		"--remote-debugging-port=0",
		"about:blank",
	)
	// Kill is too abrupt: the child processes of Chrome can still write to
	// the temporary directory after the test, and then the cleanup fails.
	// Ask Chrome to exit, and kill it only if it does not.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 10 * time.Second
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		procCancel()
		cmd.Wait()
	})
	wsURL, err := readWebSocketURL(stderr)
	if err != nil {
		t.Fatal(err)
	}
	return wsURL
}

// TestAllocatorBrowserContext checks the browser contexts of a context that the
// remote allocator makes. The first context creates a browser context, and the
// others use it or create their own.
func TestAllocatorBrowserContext(t *testing.T) {
	t.Parallel()

	wsURL := startChrome(t)
	allocCtx, allocCancel := NewAllocator(context.Background(), wsURL)
	defer allocCancel()
	browserCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	if err := chromedp.Do(browserCtx); err != nil {
		t.Fatal(err)
	}

	rootCtx, cancel := chromedp.NewContext(browserCtx, chromedp.WithNewBrowserContext())
	defer cancel()
	if err := chromedp.Do(rootCtx); err != nil {
		t.Fatal(err)
	}
	rootID := chromedp.FromContext(rootCtx).BrowserContextID

	browserContextOf := func(ctx context.Context) cdp.BrowserContextID {
		t.Helper()
		var id cdp.BrowserContextID
		if err := chromedp.Do(ctx,
			chromedp.Func(func(ctx context.Context, _ *chromedp.Target) error {
				res, err := chromedp.Call(ctx, target.GetTargetInfo, target.GetTargetInfoParams{})
				if err != nil {
					return err
				}
				id = res.TargetInfo.BrowserContextID
				return nil
			}),
		); err != nil {
			t.Fatal(err)
		}
		return id
	}
	listed := func(id cdp.BrowserContextID) bool {
		t.Helper()
		var ids []cdp.BrowserContextID
		if err := chromedp.Do(browserCtx,
			chromedp.Func(func(ctx context.Context, _ *chromedp.Target) error {
				res, err := chromedp.CallBrowser(ctx, target.GetBrowserContexts, cdp.Empty{})
				ids = res.BrowserContextIDs
				return err
			}),
		); err != nil {
			t.Fatal(err)
		}
		for _, other := range ids {
			if other == id {
				return true
			}
		}
		return false
	}

	t.Run("WithExistingBrowserContext", func(t *testing.T) {
		ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithExistingBrowserContext(rootID))
		defer cancel()
		if got := browserContextOf(ctx); got != rootID {
			t.Fatalf("want root browser context %q, got %q", rootID, got)
		}
		cancel()
		if !listed(rootID) {
			t.Fatal("the browser context was disposed, want it kept")
		}
	})

	t.Run("WithNewBrowserContext", func(t *testing.T) {
		ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithNewBrowserContext())
		defer cancel()
		id := browserContextOf(ctx)
		if want := chromedp.FromContext(ctx).BrowserContextID; id != want {
			t.Fatalf("want browser context %q, got %q", want, id)
		}
		cancel()
		if listed(id) {
			t.Fatal("the browser context was kept, want it disposed")
		}
	})
}
