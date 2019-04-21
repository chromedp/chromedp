package chromedp

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
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

	// We should be able to do the following steps repeatedly; do it twice
	// to check for idempotency.
	// 1) connect and create a target (tab)
	// 2) run some actions
	// 3) close the target and connection
	for i := 0; i < 3; i++ {
		allocCtx, allocCancel := NewRemoteAllocator(context.Background(), wsURL)
		defer allocCancel()

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
		allocCancel()
	}
}
