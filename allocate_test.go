package chromedp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

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
			WebSocket, // the timeout only applies to the websocket mode
			CombinedOutput(buf),
			Flag("enable-logging", "stderr"),
			WSURLReadTimeout(1), // trigger err
		}, allocOpts...)...)
	defer cancel()

	ctx, _ := NewContext(allocCtx, browserOpts...)

	if _, err := FromContext(ctx).Allocator.Allocate(ctx, WithDialTimeout(1)); err != nil &&
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
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	kill := make(chan struct{}, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kill <- struct{}{}
		<-ctx.Done() // block until the end of the test
	}))
	defer s.Close()
	go func() {
		<-kill
		b := FromContext(ctx).Browser
		if err := b.process.Signal(os.Kill); err != nil {
			t.Error(err)
		}
	}()

	// Run must return an error other than "deadline exceeded" in much less
	// than 3s.
	switch err := Do(ctx, Navigate(s.URL)); err {
	case nil:
		// TODO: figure out why this happens sometimes on Travis
		// t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v", err)
	}
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

func TestRemoteAllocator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		modifyURL func(wsURL string) string
		opts      []RemoteAllocatorOption
		wantErr   string
	}{
		{
			name:      "original wsURL",
			modifyURL: func(wsURL string) string { return wsURL },
		},
		{
			name: "detect from ws",
			modifyURL: func(wsURL string) string {
				return wsURL[0:strings.Index(wsURL, "devtools")]
			},
		},
		{
			name: "detect from http",
			modifyURL: func(wsURL string) string {
				return "http" + wsURL[2:strings.Index(wsURL, "devtools")]
			},
		},
		{
			name: "hostname",
			modifyURL: func(wsURL string) string {
				// Chrome ignores "remote-debugging-address" and
				// listens on the loopback interface only, so the
				// machine hostname is not reachable.
				h := "localhost"
				u, err := url.Parse(wsURL)
				if err != nil {
					t.Fatal(err)
				}
				_, port, err := net.SplitHostPort(u.Host)
				if err != nil {
					t.Fatal(err)
				}
				u.Host = net.JoinHostPort(h, port)
				u.Path = "/"
				return u.String()
			},
		},
		{
			name: "NoModifyURL",
			modifyURL: func(wsURL string) string {
				return wsURL[0:strings.Index(wsURL, "devtools")]
			},
			opts:    []RemoteAllocatorOption{NoModifyURL},
			wantErr: "could not dial",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testRemoteAllocator(t, test.modifyURL, test.wantErr, test.opts)
		})
	}
}

func testRemoteAllocator(t *testing.T, modifyURL func(wsURL string) string, wantErr string, opts []RemoteAllocatorOption) {
	tempDir := t.TempDir()

	procCtx, procCancel := context.WithCancel(context.Background())
	defer procCancel()
	cmd := exec.CommandContext(procCtx, execPath,
		// TODO: deduplicate these with allocOpts in chromedp_test.go
		"--no-first-run",
		"--no-default-browser-check",
		"--headless",
		"--disable-gpu",
		"--no-sandbox",

		// TODO: perhaps deduplicate this code with ExecAllocator
		"--user-data-dir="+tempDir,
		"--remote-debugging-address=0.0.0.0",
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
	defer stderr.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	wsURL, _, err := readOutput(stderr, nil)
	if err != nil {
		t.Fatal(err)
	}
	allocCtx, allocCancel := NewRemoteAllocator(context.Background(), modifyURL(wsURL), opts...)
	defer allocCancel()

	taskCtx, taskCancel := NewContext(allocCtx,
		// This used to crash when used with RemoteAllocator.
		WithLogf(func(format string, args ...any) {}),
	)

	{
		infos, err := Targets(taskCtx)
		if len(wantErr) > 0 {
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("\ngot error:\n\t%v\nwant error contains:\n\t%s", err, wantErr)
			}

			procCancel()
			cmd.Wait()
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		// Current Chrome also lists targets that are not pages, such as
		// "browser_ui", "service_worker" and "background_page".
		var pages int
		for _, info := range infos {
			if info.Type == "page" {
				pages++
			}
		}
		if pages > 1 {
			t.Fatalf("expected Targets on a new RemoteAllocator context to return at most one page, got: %d", pages)
		}
	}

	defer taskCancel()
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
	targetID := FromContext(taskCtx).Target.TargetID
	if err := Cancel(taskCtx); err != nil {
		t.Fatal(err)
	}

	// Make sure that cancel closed the tabs. Do not just count the
	// number of targets, as perhaps the initial blank tab has not
	// come up yet.
	targetsCtx, targetsCancel := NewContext(allocCtx)
	defer targetsCancel()
	infos, err := Targets(targetsCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		if info.TargetID == targetID {
			t.Fatalf("target from previous iteration wasn't closed: %v", targetID)
		}
	}
	targetsCancel()

	// Finally, if we kill the browser and the websocket connection drops,
	// Run must return an error well before the 5s timeout.
	// TODO: a "defer cancel()" here adds a 1s timeout, because we try to
	// close the target twice. Fix that.
	ctx, _ := NewContext(allocCtx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Connect to the browser, then kill it.
	if err := Do(ctx); err != nil {
		t.Fatal(err)
	}
	procCancel()
	switch err := Do(ctx, Navigate(testdataDir+"/form.html")); err {
	case nil:
		// TODO: figure out why this happens sometimes on Travis
		// t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v", err)
	}
	cmd.Wait()
}

func TestExecAllocatorMissingWebsocketAddr(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := NewExecAllocator(context.Background(),
		// Ask for a debugging pipe that is not open, so Chrome exits
		// straight away. Chrome ignores a bad "remote-debugging-address".
		// This needs the websocket mode, as the pipe mode opens the pipe.
		append([]ExecAllocatorOption{WebSocket, Flag("remote-debugging-pipe", true)},
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

	for _, test := range []struct {
		name string
		opts []ExecAllocatorOption
		// wantListening is true when the output must have the websocket
		// address line, which only the websocket mode prints.
		wantListening bool
	}{
		{name: "Pipe"},
		{name: "WebSocket", opts: []ExecAllocatorOption{WebSocket}, wantListening: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			buf := new(syncBuffer)
			allocCtx, cancel := NewExecAllocator(context.Background(),
				append(append([]ExecAllocatorOption{
					CombinedOutput(buf),
					Flag("enable-logging", "stderr"),
				}, test.opts...), allocOpts...)...)
			defer cancel()

			taskCtx, _ := NewContext(allocCtx)
			if err := Do(taskCtx,
				Navigate(testdataDir+"/consolespam.html"),
			); err != nil {
				t.Fatal(err)
			}
			cancel()
			if got := strings.Contains(buf.String(), "DevTools listening on"); got != test.wantListening {
				t.Fatalf("output has the websocket string: %v, want %v", got, test.wantListening)
			}
			// Recent chrome versions replace many "spam" messages with "spam 1",
			// "spam 2", and so on. Search for the prefix only.
			if want, got := 2000, strings.Count(buf.String(), `"spam`); want != got {
				t.Fatalf("want %d spam console logs, got %d", want, got)
			}
		})
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
			WebSocket,
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

func TestEnv(t *testing.T) {
	t.Parallel()

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

	ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
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
