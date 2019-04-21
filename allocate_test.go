package chromedp

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

	// Simulate a scenario where we navigate to a page that's slow to
	// respond, and the browser is closed before we can finish the
	// navigation.
	serve := make(chan bool, 1)
	close := make(chan bool, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close <- true
		<-serve
		fmt.Fprintf(w, "response")
	}))
	defer s.Close()

	ctx, cancel := NewContext(context.Background())
	defer cancel()
	if err := Run(ctx); err != nil {
		t.Fatal(err)
	}

	go func() {
		<-close
		b := FromContext(ctx).Browser
		if err := b.process.Signal(os.Kill); err != nil {
			t.Error(err)
		}
		serve <- true
	}()

	// Run should error with something other than "deadline exceeded" in
	// much less than 5s.
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch err := Run(ctx2, Navigate(s.URL)); err {
	case nil:
		t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v", err)
	}
}

func TestSkipNewContext(t *testing.T) {
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

	tempDir, err := ioutil.TempDir("", "chromedp-runner")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	procCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(procCtx, execPath,
		// TODO: deduplicate these with allocOpts in chromedp_test.go
		"--no-first-run",
		"--no-default-browser-check",
		"--headless",
		"--disable-gpu",
		"--no-sandbox",

		// TODO: perhaps deduplicate this code with ExecAllocator
		"--user-data-dir="+tempDir,
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
	wsURL, err := portFromStderr(stderr)
	if err != nil {
		t.Fatal(err)
	}
	allocCtx, allocCancel := NewRemoteAllocator(context.Background(), wsURL)
	defer allocCancel()

	// We should be able to do the following steps repeatedly; do it twice
	// to check for idempotency.
	// 1) connect and create a target (tab)
	// 2) run some actions
	// 3) close the target and connection
	for i := 0; i < 3; i++ {
		taskCtx, taskCancel := NewContext(allocCtx)
		defer taskCancel()

		// check that previous runs closed their tabs
		checkTargets(t, taskCtx, 1)

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
		if err := Cancel(taskCtx); err != nil {
			t.Fatal(err)
		}
	}

	// Finally, if we kill the browser and the websocket connection drops,
	// Run should error way before the 5s timeout.
	ctx, cancel := NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Connect to the browser, then kill it.
	if err := Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Signal(os.Kill); err != nil {
		t.Error(err)
	}
	switch err := Run(ctx, Navigate(testdataDir+"/form.html")); err {
	case nil:
		t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v", err)
	}
}
