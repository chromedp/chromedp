package chromedp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// errStubDialer is the error of stubDialer.
var errStubDialer = errors.New("i/o timeout: the test has no websocket")

// stubDialer is a Dialer for the tests of the websocket mode that never need a
// connection. The core module has no websocket code, so the tests that need a
// real connection are in the module remote. The dialer fails, so that a test
// that reaches it fails with an error that names it.
func stubDialer(context.Context, string) (Transport, error) {
	return nil, errStubDialer
}

func TestExecAllocator(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()

	// TODO: test that multiple child contexts are run in different
	// processes and browsers.

	taskCtx, cancel := NewContext(allocCtx)
	defer cancel()

	want := "insert"
	var got string
	if err := Do(taskCtx,
		Navigate(testdataDir+"/form.html"),
		into(&got, Text(ID("foo"))),
	); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	cancel()

	tempDir := FromContext(taskCtx).Browser.userDataDir
	if _, err := os.Lstat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("temporary user data dir %q not deleted", tempDir)
	}
}

func TestExecAllocatorCancelParent(t *testing.T) {
	t.Parallel()

	allocCtx, allocCancel := NewExecAllocator(context.Background(), allocOpts...)
	defer allocCancel()

	// TODO: test that multiple child contexts are run in different
	// processes and browsers.

	taskCtx, _ := NewContext(allocCtx)
	if err := Do(taskCtx); err != nil {
		t.Fatal(err)
	}

	// Canceling the pool context must stop all browsers too.
	allocCancel()

	tempDir := FromContext(taskCtx).Browser.userDataDir
	if _, err := os.Lstat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("temporary user data dir %q not deleted", tempDir)
	}
}

func TestExecAllocatorCombinedOutputPanic(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	allocCtx, cancel := NewExecAllocator(context.Background(),
		append([]ExecAllocatorOption{
			WithDialer(stubDialer), // the timeout only applies to the websocket mode
			CombinedOutput(buf),
			Flag("enable-logging", "stderr"),
			WSURLReadTimeout(1), // trigger err
		}, allocOpts...)...)
	defer cancel()

	ctx, _ := NewContext(allocCtx, browserOpts...)

	if _, err := FromContext(ctx).Allocator.Allocate(ctx); err != nil &&
		!strings.HasPrefix(err.Error(), "websocket url timeout reached") &&
		!strings.Contains(err.Error(), "i/o timeout") {
		t.Fatal(err)
	}

	// give time for the `readOutput` goroutine to finish
	// this can vary depending on the system, so we give it a bit more time
	time.Sleep(5 * time.Second)

	cancel()
	// dir cleanup occurs after 10 milliseconds so this gives a bit more time
	// for it. Otherwise the test can fail with a panic about the directory
	// that is not removed
	time.Sleep(20 * time.Millisecond)
}

func TestExecAllocatorKillBrowser(t *testing.T) {
	t.Parallel()

	// Simulate a scenario where we navigate to a page that never responds,
	// and the browser is killed while it is loading.
	ctx, _ := testAllocateSeparate(t)
	// A busy CI runner can need several seconds to load the page. The limit
	// does not slow the test, because the browser dies at once when the
	// request arrives.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var killed atomic.Bool
	kill := make(chan struct{}, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kill <- struct{}{}
		<-ctx.Done() // block until the end of the test
	}))
	defer s.Close()
	go func() {
		<-kill
		b := FromContext(ctx).Browser
		killed.Store(true)
		if err := b.process.Signal(os.Kill); err != nil {
			t.Error(err)
		}
	}()

	// Run must return an error other than "deadline exceeded" in much less
	// than the limit.
	switch err := Do(ctx, Navigate(s.URL)); err {
	case nil:
		// TODO: figure out why this happens sometimes on Travis
		// t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v (the browser was killed: %t)", err, killed.Load())
	}
}

// TestDefaultFeatureNames makes sure that no feature in the list of the
// default options has the form of a command line switch. Chrome names a
// feature in CamelCase, and it ignores a name that it does not know. The old
// value "site-per-process" is a switch name, so it did nothing. See the issue
// 1605.
func TestDefaultFeatureNames(t *testing.T) {
	t.Parallel()

	a := setupExecAllocator(DefaultExecAllocatorOptions[:]...)
	for _, flag := range []string{"enable-features", "disable-features"} {
		value, ok := a.initFlags[flag].(string)
		if !ok {
			continue
		}
		for name := range strings.SplitSeq(value, ",") {
			if name == "" || strings.Contains(name, "-") {
				t.Errorf("%s has the name %q, which is not a feature name", flag, name)
			}
		}
	}
}

// TestExitErrorAfterKill kills the browser process and checks that the next
// call returns the exit error of the process. See the issue 408.
func TestExitErrorAfterKill(t *testing.T) {
	t.Parallel()

	// The module remote tests the websocket.
	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel := NewContext(allocCtx)
	defer cancel()
	if err := Do(ctx); err != nil {
		t.Fatal(err)
	}
	b := FromContext(ctx).Browser
	if err := b.process.Signal(os.Kill); err != nil {
		t.Fatal(err)
	}

	// The first call can race with the loss of the connection, so
	// ask until the call fails. Every later call must fail in the
	// same way.
	for i := range 3 {
		_, err := Run(ctx, Evaluate[int](`1 + 2`))
		if err == nil {
			t.Fatalf("call %d: want an error from a dead browser", i)
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("call %d: want an *exec.ExitError in %q", i, err)
		}
		if !strings.Contains(err.Error(), killedText) {
			t.Fatalf("call %d: want %q in %q", i, killedText, err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("call %d: want context.Canceled in %q", i, err)
		}
	}
}

// TestNoExitErrorAfterCancel makes sure that a program that stops the browser
// itself gets no exit error. The process ends with a signal then, too.
func TestNoExitErrorAfterCancel(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel := NewContext(allocCtx)
	if err := Do(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := Run(ctx, Evaluate[int](`1 + 2`)); err != context.Canceled {
		t.Fatalf("want exactly context.Canceled, got %v", err)
	}
}

// TestRetryRemove checks that the removal of a user data directory tries again
// when it fails. Child processes of Chrome can write in the directory after the
// browser exits, and then os.RemoveAll fails with "directory not empty". The
// retry must have a bound. See the issue 1544.
func TestRetryRemove(t *testing.T) {
	t.Parallel()

	notEmpty := &os.PathError{Op: "unlinkat", Path: "dir", Err: syscall.ENOTEMPTY}

	t.Run("succeeds after failures", func(t *testing.T) {
		t.Parallel()

		calls := 0
		remove := func(string) error {
			calls++
			if calls < 4 {
				return notEmpty
			}
			return nil
		}
		if err := retryRemove("dir", remove, 5*time.Second, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if calls != 4 {
			t.Fatalf("want 4 calls, got %d", calls)
		}
	})

	t.Run("gives up", func(t *testing.T) {
		t.Parallel()

		calls := 0
		remove := func(string) error {
			calls++
			return notEmpty
		}
		start := time.Now()
		err := retryRemove("dir", remove, 50*time.Millisecond, 5*time.Millisecond)
		if !errors.Is(err, syscall.ENOTEMPTY) {
			t.Fatalf("want the last error, got %v", err)
		}
		if calls < 2 {
			t.Fatalf("want a retry, got %d calls", calls)
		}
		if time.Since(start) > 2*time.Second {
			t.Fatalf("the retry took %v, which is far above the limit", time.Since(start))
		}
	})

	t.Run("real directory", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(t.TempDir(), "profile")
		if err := os.MkdirAll(filepath.Join(dir, "Default"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := removeAllRetry(dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("the directory is still there: %v", err)
		}
	})
}

func TestSkipNewContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()

	// Using the allocator context directly (without calling NewContext)
	// must return an error at once.
	err := Do(ctx, Navigate(testdataDir+"/form.html"))

	want := ErrInvalidContext
	if err != want {
		t.Fatalf("want error to be %q, got %q", want, err)
	}
}

func TestExecAllocatorMissingWebsocketAddr(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := NewExecAllocator(context.Background(),
		// Ask for a debugging pipe that is not open, so Chrome exits
		// straight away. Chrome ignores a bad "remote-debugging-address".
		// This needs the websocket mode, as the pipe mode opens the pipe.
		append([]ExecAllocatorOption{WithDialer(stubDialer), Flag("remote-debugging-pipe", true)},
			allocOpts...)...)
	defer cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()

	// set the "s" flag to let "." match "\n"
	// in GitHub Actions, the error text can be:
	// "chrome failed to start:\n/bin/bash: /etc/profile.d/env_vars.sh: Permission denied\nmkdir: cannot create directory ‘/run/user/1001’: Permission denied\n[0321/081807.491906:ERROR:chrome_main_delegate.cc(1164)] Remote debugging pipe file descriptors are not open.\n"
	want := `failed to start`
	got := fmt.Sprintf("%v", Do(ctx))
	if !strings.Contains(got, want) {
		t.Fatalf("want error to match %q, got %q", want, got)
	}
}

func TestCombinedOutput(t *testing.T) {
	t.Parallel()

	// The pipe mode prints no websocket address. The module remote tests the
	// websocket mode.
	buf := new(syncBuffer)
	allocCtx, cancel := NewExecAllocator(context.Background(),
		append([]ExecAllocatorOption{
			CombinedOutput(buf),
			Flag("enable-logging", "stderr"),
		}, allocOpts...)...)
	defer cancel()

	taskCtx, _ := NewContext(allocCtx)
	if err := Do(taskCtx,
		Navigate(testdataDir+"/consolespam.html"),
	); err != nil {
		t.Fatal(err)
	}
	cancel()
	if strings.Contains(buf.String(), "DevTools listening on") {
		t.Fatal("the output has the websocket string")
	}
	// Recent chrome versions replace many "spam" messages with "spam 1",
	// "spam 2", and so on. Search for the prefix only.
	if want, got := 2000, strings.Count(buf.String(), `"spam`); want != got {
		t.Fatalf("want %d spam console logs, got %d", want, got)
	}
}

func TestCombinedOutputError(t *testing.T) {
	t.Parallel()

	// CombinedOutput used to hang the allocator if Chrome errored straight
	// away, because there was no output to copy and CombinedOutput never
	// signaled that it was done.
	buf := new(bytes.Buffer)
	allocCtx, cancel := NewExecAllocator(context.Background(),
		// Ask for a debugging pipe that is not open, so Chrome exits
		// straight away. Chrome ignores a bad "remote-debugging-address".
		append([]ExecAllocatorOption{
			WithDialer(stubDialer),
			Flag("remote-debugging-pipe", true),
			CombinedOutput(buf),
		}, allocOpts...)...)
	defer cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()
	got := fmt.Sprint(Do(ctx))
	want := "failed to start"
	if !strings.Contains(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// skipTimeZoneEnvOnWindows skips a test that sets the time zone of the browser
// with the variable TZ. The browser reads the time zone from the system on
// Windows and ignores the variable.
func skipTimeZoneEnvOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the browser ignores the TZ variable on Windows")
	}
}

func TestEnv(t *testing.T) {
	t.Parallel()
	skipTimeZoneEnvOnWindows(t)

	tz := "Australia/Melbourne"
	allocCtx, cancel := NewExecAllocator(context.Background(),
		append([]ExecAllocatorOption{
			Env("TZ=" + tz),
		}, allocOpts...)...)
	defer cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()

	var ret string
	if err := Do(ctx,
		into(&ret, Evaluate[string](`Intl.DateTimeFormat().resolvedOptions().timeZone`)),
	); err != nil {
		t.Fatal(err)
	}

	if ret != tz {
		t.Fatalf("got %s, want %s", ret, tz)
	}
}

func TestWithBrowserOptionAlreadyAllocated(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocateSeparate(t)
	defer cancel()

	defer func() {
		want := "when allocating a new browser"
		if got := fmt.Sprint(recover()); !strings.Contains(got, want) {
			t.Errorf("expected a panic containing %q, got %q", want, got)
		}
	}()
	// This must panic, because we try to set up a browser logf func after the
	// browser was set up earlier.
	_, _ = NewContext(ctx,
		WithLogf(func(format string, args ...any) {}),
	)
}

func TestModifyCmdFunc(t *testing.T) {
	t.Parallel()
	skipTimeZoneEnvOnWindows(t)

	tz := "Atlantic/Reykjavik"
	allocCtx, cancel := NewExecAllocator(context.Background(),
		append([]ExecAllocatorOption{
			ModifyCmdFunc(func(cmd *exec.Cmd) {
				cmd.Env = append(cmd.Env, "TZ="+tz)
			}),
		}, allocOpts...)...)
	defer cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()

	var ret string
	if err := Do(ctx,
		into(&ret, Evaluate[string](`Intl.DateTimeFormat().resolvedOptions().timeZone`)),
	); err != nil {
		t.Fatal(err)
	}

	if ret != tz {
		t.Fatalf("got %s, want %s", ret, tz)
	}
}

// TestStartsWithNonBlankTab is a regression test. It makes sure that chromedp
// does not hang when the browser starts with a non-blank tab.
//
// The browser starts with a non-blank tab in these cases:
// 1. The "--app" option is used (this disables headless mode).
// 2. The command line arguments hold a URL other than "about:blank".
//
// It is hard to disable headless mode on test servers, so this test uses
// case 2.
func TestStartsWithNonBlankTab(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := NewExecAllocator(context.Background(),
		append(allocOpts,
			ModifyCmdFunc(func(cmd *exec.Cmd) {
				// it assumes that the last argument is "about:blank" and
				// replaces it with another URL.
				cmd.Args[len(cmd.Args)-1] = testdataDir + "/form.html"
			}),
		)...)
	defer cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()

	// The limit includes the start of the browser, which takes several seconds
	// on a busy CI runner. The browser that hangs never ends, so any limit
	// finds it.
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := Do(ctx,
		Navigate(testdataDir+"/form.html"),
	); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			t.Error("chromedp hangs when the browser starts with a non-blank tab.")
		} else {
			t.Errorf("got error %s, want nil", err)
		}
	}
}

// killedText is the text that the error of a killed process has. Windows has no
// signals, and it ends the process with the exit status 1.
var killedText = func() string {
	if runtime.GOOS == "windows" {
		return "exit status 1"
	}
	return "signal: killed"
}()

// commandArgs starts a browser with a program that does not exist and returns the
// arguments that ModifyCmdFunc saw. Allocate fails after it, which the test
// ignores.
func commandArgs(t *testing.T, opts ...ExecAllocatorOption) []string {
	t.Helper()

	var args []string
	opts = append(opts,
		ExecPath("/do-not-run-chrome"),
		ModifyCmdFunc(func(cmd *exec.Cmd) { args = slices.Clone(cmd.Args[1:]) }),
	)
	allocCtx, cancel := NewExecAllocator(context.Background(), opts...)
	defer cancel()
	ctx, cancel := NewContext(allocCtx)
	defer cancel()
	if err := Do(ctx); err == nil {
		t.Fatal("expected an error for a program that does not exist")
	}
	if args == nil {
		t.Fatal("ModifyCmdFunc did not run")
	}
	return args
}

// TestFlagOrder checks that the arguments of the command keep the order of the
// flags. Chrome needs --flag-switches-begin and --flag-switches-end around the
// switches of chrome://flags. See the issue 1483.
func TestFlagOrder(t *testing.T) {
	t.Parallel()

	opts := append(slices.Clone(DefaultExecAllocatorOptions[:]),
		Flag("flag-switches-begin", true),
		Flag("disable-features", "IPH_DemoMode,UserEducationExperienceVersion2"),
		Flag("flag-switches-end", true),
	)
	args := commandArgs(t, opts...)

	// The default flags come first in their listed order. Setting
	// disable-features again keeps its first place with the new value.
	want := []string{
		"--no-first-run",
		"--no-default-browser-check",
		"--headless",
		"--hide-scrollbars",
		"--mute-audio",
		"--disable-background-networking",
		"--enable-features=NetworkService,NetworkServiceInProcess",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-breakpad",
		"--disable-client-side-phishing-detection",
		"--disable-default-apps",
		"--disable-dev-shm-usage",
		"--disable-extensions",
		"--disable-features=IPH_DemoMode,UserEducationExperienceVersion2",
		"--disable-hang-monitor",
		"--disable-ipc-flooding-protection",
		"--disable-popup-blocking",
		"--disable-prompt-on-repost",
		"--disable-renderer-backgrounding",
		"--disable-sync",
		"--force-color-profile=srgb",
		"--metrics-recording-only",
		"--safebrowsing-disable-auto-update",
		"--enable-automation",
		"--password-store=basic",
		"--use-mock-keychain",
		"--flag-switches-begin",
		"--flag-switches-end",
	}
	if len(args) < len(want) || !slices.Equal(args[:len(want)], want) {
		t.Fatalf("the flags are not in order:\nwant %v\ngot  %v", want, args)
	}
}

func TestFlagOrderNewFlags(t *testing.T) {
	t.Parallel()

	args := commandArgs(t,
		Flag("a", true),
		Flag("flag-switches-begin", true),
		Flag("b", "1"),
		Flag("c", true),
		Flag("flag-switches-end", true),
		Flag("d", true),
		Flag("b", "2"),
		Flag("c", false),
	)
	// A flag that is set again keeps its place. A false boolean flag leaves
	// the command line.
	want := []string{"--a", "--flag-switches-begin", "--b=2", "--flag-switches-end", "--d"}
	if len(args) < len(want) || !slices.Equal(args[:len(want)], want) {
		t.Fatalf("want the flags %v first, got %v", want, args)
	}
}

func TestFlagOrderAfterVisibleWindow(t *testing.T) {
	t.Parallel()

	a := setupExecAllocator(Headless, Flag("x", true), VisibleWindow, Flag("y", true))
	if want := []string{"x", "start-maximized", "y"}; !slices.Equal(a.flagOrder, want) {
		t.Fatalf("want the order %v, got %v", want, a.flagOrder)
	}
	if len(a.flagOrder) != len(a.initFlags) {
		t.Fatalf("the order has %d names and the map has %d flags", len(a.flagOrder), len(a.initFlags))
	}
}
