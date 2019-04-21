package chromedp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/target"
)

var (
	execPath    string
	testdataDir string

	browserCtx context.Context

	// allocOpts is filled in TestMain
	allocOpts []ExecAllocatorOption
)

func testAllocate(tb testing.TB, path string) (_ context.Context, cancel func()) {
	// Same browser, new tab; not needing to start new chrome browsers for
	// each test gives a huge speed-up.
	ctx, _ := NewContext(browserCtx)

	// Only navigate if we want a path, otherwise leave the blank page.
	if path != "" {
		if err := Run(ctx, Navigate(testdataDir+"/"+path)); err != nil {
			tb.Fatal(err)
		}
	}

	cancelErr := func() {
		if err := Cancel(ctx); err != nil {
			tb.Error(err)
		}
	}
	return ctx, cancelErr
}

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("could not get working directory: %v", err))
	}
	testdataDir = "file://" + path.Join(wd, "testdata")

	// build on top of the default options
	allocOpts = append(allocOpts, DefaultExecAllocatorOptions...)

	// disabling the GPU helps portability with some systems like Travis,
	// and can slightly speed up the tests on other systems
	allocOpts = append(allocOpts, DisableGPU)

	// find the exec path once at startup
	// it's worth noting that newer versions of chrome (64+) run much faster
	// than older ones -- same for headless_shell ...
	execPath = os.Getenv("CHROMEDP_TEST_RUNNER")
	if execPath == "" {
		execPath = findExecPath()
	}
	allocOpts = append(allocOpts, ExecPath(execPath))

	// not explicitly needed to be set, as this vastly speeds up unit tests
	if noSandbox := os.Getenv("CHROMEDP_NO_SANDBOX"); noSandbox != "false" {
		allocOpts = append(allocOpts, NoSandbox)
	}

	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)

	var browserOpts []ContextOption
	if debug := os.Getenv("CHROMEDP_DEBUG"); debug != "" && debug != "false" {
		browserOpts = append(browserOpts, WithDebugf(log.Printf))
	}

	// start the browser
	browserCtx, _ = NewContext(allocCtx, browserOpts...)
	if err := Run(browserCtx); err != nil {
		panic(err)
	}

	code := m.Run()

	cancel()
	os.Exit(code)
}

// checkPages fatals if the browser behind the chromedp context has an
// unexpected number of pages (tabs).
func checkTargets(tb testing.TB, ctx context.Context, want int) {
	tb.Helper()
	infos, err := Targets(ctx)
	if err != nil {
		tb.Fatal(err)
	}
	var pages []*target.Info
	for _, info := range infos {
		if info.Type == "page" {
			pages = append(pages, info)
		}
	}
	if got := len(pages); want != got {
		var summaries []string
		for _, info := range pages {
			summaries = append(summaries, fmt.Sprintf("%v", info))
		}
		tb.Fatalf("want %d targets, got %d:\n%s",
			want, got, strings.Join(summaries, "\n"))
	}
}

func TestTargets(t *testing.T) {
	t.Parallel()

	// Start one browser with one tab.
	ctx1, cancel1 := NewContext(context.Background())
	defer cancel1()
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}

	checkTargets(t, ctx1, 1)

	// Start a second tab on the same browser.
	ctx2, cancel2 := NewContext(ctx1)
	defer cancel2()
	if err := Run(ctx2); err != nil {
		t.Fatal(err)
	}
	checkTargets(t, ctx2, 2)

	// The first context should also see both targets.
	checkTargets(t, ctx1, 2)

	// Cancelling the second context should close the second tab alone.
	cancel2()
	checkTargets(t, ctx1, 1)

	// We used to have a bug where Run would reset the first context as if
	// it weren't the first, breaking its cancellation.
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserQuit(t *testing.T) {
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

func TestCancelError(t *testing.T) {
	t.Parallel()

	ctx1, cancel1 := NewContext(context.Background())
	defer cancel1()
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}

	// Open and close a target normally; no error.
	ctx2, cancel2 := NewContext(ctx1)
	defer cancel2()
	if err := Run(ctx2); err != nil {
		t.Fatal(err)
	}
	if err := Cancel(ctx2); err != nil {
		t.Fatalf("expected a nil error, got %v", err)
	}

	// Make "cancel" close the wrong target; error.
	ctx3, cancel3 := NewContext(ctx1)
	defer cancel3()
	if err := Run(ctx3); err != nil {
		t.Fatal(err)
	}
	FromContext(ctx3).Target.TargetID = "wrong"
	if err := Cancel(ctx3); err == nil {
		t.Fatalf("expected a non-nil error, got %v", err)
	}
}

func TestPrematureCancel(t *testing.T) {
	t.Parallel()

	// Cancel before the browser is allocated.
	ctx, cancel := NewContext(context.Background())
	cancel()
	if err := Run(ctx); err != context.Canceled {
		t.Fatalf("wanted canceled context error, got %v", err)
	}
}

func TestPrematureCancelTab(t *testing.T) {
	t.Parallel()

	ctx1, cancel := NewContext(context.Background())
	defer cancel()
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}

	// Cancel after the browser is allocated, but before we've created a new
	// tab.
	ctx2, cancel := NewContext(ctx1)
	cancel()
	Run(ctx2)
}
