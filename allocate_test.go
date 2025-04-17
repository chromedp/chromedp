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
	if err := Run(taskCtx,
		Navigate(testdataDir+"/form.html"),
		Text("#foo", &got, ByID),
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
	if err := Run(taskCtx); err != nil {
		t.Fatal(err)
	}

	// Canceling the pool context should stop all browsers too.
	allocCancel()

	tempDir := FromContext(taskCtx).Browser.userDataDir
	if _, err := os.Lstat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("temporary user data dir %q not deleted", tempDir)
	}
}

func TestExecAllocatorKillBrowser(t *testing.T) {
	t.Parallel()

	// Simulate a scenario where we navigate to a page that never responds,
	// and the browser is killed while it's loading.
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

	// Run should error with something other than "deadline exceeded" in
	// much less than 3s.
	switch err := Run(ctx, Navigate(s.URL)); err {
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
	// should be an immediate error.
	err := Run(ctx, Navigate(testdataDir+"/form.html"))

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
				h, err := os.Hostname()
				if err != nil {
					t.Fatal(err)
				}
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

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	wsURL, err := readOutput(stderr, nil, nil)
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
		if len(infos) > 1 {
			t.Fatalf("expected Targets on a new RemoteAllocator context to return at most one, got: %d", len(infos))
		}
	}

	defer taskCancel()
	want := "insert"
	var got string
	if err := Run(taskCtx,
		Navigate(testdataDir+"/form.html"),
		Text("#foo", &got, ByID),
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

	// Check that cancel closed the tabs. Don't just count the
	// number of targets, as perhaps the initial blank tab hasn't
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
	// Run should error way before the 5s timeout.
	// TODO: a "defer cancel()" here adds a 1s timeout, since we try to
	// close the target twice. Fix that.
	ctx, _ := NewContext(allocCtx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Connect to the browser, then kill it.
	if err := Run(ctx); err != nil {
		t.Fatal(err)
	}
	procCancel()
	switch err := Run(ctx, Navigate(testdataDir+"/form.html")); err {
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
		// Use a bad listen address, so Chrome exits straight away.
		append([]ExecAllocatorOption{Flag("remote-debugging-address", "_")},
			allocOpts...)...)
	defer cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()

	// set the "s" flag to let "." match "\n"
	// in GitHub Actions, the error text could be:
	// "chrome failed to start:\n/bin/bash: /etc/profile.d/env_vars.sh: Permission denied\nmkdir: cannot create directory ‘/run/user/1001’: Permission denied\n[0321/081807.491906:ERROR:headless_shell.cc(720)] Invalid devtools server address\n"
	want := `failed to start`
	got := fmt.Sprintf("%v", Run(ctx))
	if !strings.Contains(got, want) {
		t.Fatalf("want error to match %q, got %q", want, got)
	}
}

func TestCombinedOutput(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	allocCtx, cancel := NewExecAllocator(context.Background(),
		append([]ExecAllocatorOption{
			CombinedOutput(buf),
			Flag("enable-logging", true),
		}, allocOpts...)...)
	defer cancel()

	taskCtx, _ := NewContext(allocCtx)
	if err := Run(taskCtx,
		Navigate(testdataDir+"/consolespam.html"),
	); err != nil {
		t.Fatal(err)
	}
	cancel()
	if !strings.Contains(buf.String(), "DevTools listening on") {
		t.Fatalf("failed to find websocket string in browser output test")
	}
	// Recent chrome versions have started replacing many "spam" messages
	// with "spam 1", "spam 2", and so on. Search for the prefix only.
	if want, got := 2000, strings.Count(buf.String(), `"spam`); want != got {
		t.Fatalf("want %d spam console logs, got %d", want, got)
	}
}

func TestCombinedOutputError(t *testing.T) {
	t.Parallel()

	// CombinedOutput used to hang the allocator if Chrome errored straight
	// away, as there was no output to copy and the CombinedOutput would
	// never signal it's done.
	buf := new(bytes.Buffer)
	allocCtx, cancel := NewExecAllocator(context.Background(),
		// Use a bad listen address, so Chrome exits straight away.
		append([]ExecAllocatorOption{
			Flag("remote-debugging-address", "_"),
			CombinedOutput(buf),
		}, allocOpts...)...)
	defer cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()
	got := fmt.Sprint(Run(ctx))
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
	if err := Run(ctx,
		Evaluate(`Intl.DateTimeFormat().resolvedOptions().timeZone`, &ret),
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
	// This needs to panic, as we try to set up a browser logf function
	// after the browser has already been set up earlier.
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
	if err := Run(ctx,
		Evaluate(`Intl.DateTimeFormat().resolvedOptions().timeZone`, &ret),
	); err != nil {
		t.Fatal(err)
	}

	if ret != tz {
		t.Fatalf("got %s, want %s", ret, tz)
	}
}

// TestStartsWithNonBlankTab is a regression test to make sure chromedp won't
// hang when the browser is started with a non-blank tab.
//
// In the following cases, the browser will start with a non-blank tab:
// 1. with the "--app" option (should disable headless mode);
// 2. URL other than "about:blank" is placed in the command line arguments.
//
// It's hard to disable headless mode on test servers, so we will go with
// case 2 here.
func TestStartsWithNonBlankTab(t *testing.T) {
	t.Parallel()

	allocCtx, cancel := NewExecAllocator(context.Background(),
		append(allocOpts,
			ModifyCmdFunc(func(cmd *exec.Cmd) {
				// it assumes that the last argument is "about:blank" and
				// replace it with other URL.
				cmd.Args[len(cmd.Args)-1] = testdataDir + "/form.html"
			}),
		)...)
	defer cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := Run(ctx,
		Navigate(testdataDir+"/form.html"),
	); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			t.Error("chromedp hangs when the browser starts with a non-blank tab.")
		} else {
			t.Errorf("got error %s, want nil", err)
		}
	}
}
