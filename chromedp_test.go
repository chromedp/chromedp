package chromedp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"runtime"
	"testing"
	"time"
)

var (
	testdataDir string

	browserCtx context.Context

	allocOpts = []ExecAllocatorOption{
		NoFirstRun,
		NoDefaultBrowserCheck,
		Headless,
		DisableGPU,
	}
)

func testAllocate(t *testing.T, path string) (_ context.Context, cancel func()) {
	// Same browser, new tab; not needing to start new chrome browsers for
	// each test gives a huge speed-up.
	ctx, cancel := NewContext(browserCtx)

	// Only navigate if we want a path, otherwise leave the blank page.
	if path != "" {
		if err := Run(ctx, Navigate(testdataDir+"/"+path)); err != nil {
			t.Fatal(err)
		}
	}

	return ctx, cancel
}

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("could not get working directory: %v", err))
	}
	testdataDir = "file://" + path.Join(wd, "testdata")

	// it's worth noting that newer versions of chrome (64+) run much faster
	// than older ones -- same for headless_shell ...
	if execPath := os.Getenv("CHROMEDP_TEST_RUNNER"); execPath != "" {
		allocOpts = append(allocOpts, ExecPath(execPath))
	}
	// not explicitly needed to be set, as this vastly speeds up unit tests
	if noSandbox := os.Getenv("CHROMEDP_NO_SANDBOX"); noSandbox != "false" {
		allocOpts = append(allocOpts, NoSandbox)
	}

	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)

	// start the browser
	browserCtx, _ = NewContext(allocCtx)
	if err := Run(browserCtx); err != nil {
		panic(err)
	}

	code := m.Run()

	cancel()
	os.Exit(code)
}

func TestTargets(t *testing.T) {
	t.Parallel()

	// Start one browser with one tab.
	ctx1, cancel1 := NewContext(context.Background())
	defer cancel1()
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}

	wantTargets := func(ctx context.Context, want int) {
		t.Helper()
		infos, err := Targets(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(infos); want != got {
			t.Fatalf("want %d targets, got %d", want, got)
		}
	}
	wantTargets(ctx1, 1)

	// Start a second tab on the same browser.
	ctx2, cancel2 := NewContext(ctx1)
	defer cancel2()
	if err := Run(ctx2); err != nil {
		t.Fatal(err)
	}
	wantTargets(ctx2, 2)

	// The first context should also see both targets.
	wantTargets(ctx1, 2)

	// Cancelling the second context should close the second tab alone.
	cancel2()
	wantTargets(ctx1, 1)

	// We used to have a bug where Run would reset the first context as if
	// it weren't the first, breaking its cancellation.
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserQuit(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt isn't supported on Windows")
	}

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
		if err := b.process.Signal(os.Interrupt); err != nil {
			t.Error(err)
		}
		serve <- true
	}()

	// Run should error with something other than "deadline exceeded" in
	// much less than 5s.
	ctx2, _ := context.WithTimeout(ctx, 5*time.Second)
	switch err := Run(ctx2, Navigate(s.URL)); err {
	case nil:
		t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v", err)
	}
}
