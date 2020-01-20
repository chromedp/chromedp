package chromedp

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
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
	wsURL, err := readOutput(stderr, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	allocCtx, allocCancel := NewRemoteAllocator(context.Background(), wsURL)
	defer allocCancel()

	taskCtx, taskCancel := NewContext(allocCtx)
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
		// TODO: figure out why this happens sometimes on Travis
		// t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v", err)
	}
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

	want := regexp.MustCompile(`failed to start:\n.*Invalid devtools`)
	got := fmt.Sprintf("%v", Run(ctx))
	if !want.MatchString(got) {
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
