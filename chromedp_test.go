package chromedp

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
)

var (
	// these are set up in init
	execPath    string
	testdataDir string
	allocOpts   []ExecAllocatorOption

	// browserCtx is initialised with allocateOnce
	browserCtx context.Context
)

func init() {
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
}

var allocateOnce sync.Once

func testAllocate(tb testing.TB, name string) (context.Context, context.CancelFunc) {
	// Start the browser exactly once, as needed.
	allocateOnce.Do(func() {
		allocCtx, _ := NewExecAllocator(context.Background(), allocOpts...)

		var browserOpts []ContextOption
		if debug := os.Getenv("CHROMEDP_DEBUG"); debug != "" && debug != "false" {
			browserOpts = append(browserOpts, WithDebugf(log.Printf))
		}

		// start the browser
		browserCtx, _ = NewContext(allocCtx, browserOpts...)
		if err := Run(browserCtx); err != nil {
			panic(err)
		}
	})

	// Same browser, new tab; not needing to start new chrome browsers for
	// each test gives a huge speed-up.
	ctx, _ := NewContext(browserCtx)

	// Only navigate if we want an html file name, otherwise leave the blank page.
	if name != "" {
		if err := Run(ctx, Navigate(testdataDir+"/"+name)); err != nil {
			tb.Fatal(err)
		}
	}

	cancel := func() {
		if err := Cancel(ctx); err != nil {
			tb.Error(err)
		}
	}
	return ctx, cancel
}

func BenchmarkTabNavigate(b *testing.B) {
	b.ReportAllocs()

	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()

	// start the browser
	bctx, _ := NewContext(allocCtx)
	if err := Run(bctx); err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, _ := NewContext(bctx)
			if err := Run(ctx,
				Navigate(testdataDir+"/form.html"),
				WaitVisible(`#form`, ByID), // for form.html
			); err != nil {
				b.Fatal(err)
			}
			if err := Cancel(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
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

	ctx2, cancel := NewContext(ctx1)
	// Cancel after the browser is allocated, but before we've created a new
	// tab.
	cancel()
	if err := Run(ctx2); err != context.Canceled {
		t.Fatalf("wanted canceled context error, got %v", err)
	}
}

func TestPrematureCancelAllocator(t *testing.T) {
	t.Parallel()

	// To ensure we don't actually fire any Chrome processes.
	allocCtx, cancel := NewExecAllocator(context.Background(),
		ExecPath("/do-not-run-chrome"))
	// Cancel before the browser is allocated.
	cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()
	if err := Run(ctx); err != context.Canceled {
		t.Fatalf("wanted canceled context error, got %v", err)
	}
}

func TestConcurrentCancel(t *testing.T) {
	t.Parallel()

	// To ensure we don't actually fire any Chrome processes.
	allocCtx, cancel := NewExecAllocator(context.Background(),
		ExecPath("/do-not-run-chrome"))
	defer cancel()

	// 50 is enough for 'go test -race' to easily spot issues.
	for i := 0; i < 50; i++ {
		ctx, cancel := NewContext(allocCtx)
		go cancel()
		go Run(ctx)
	}
}

func TestListenBrowser(t *testing.T) {
	t.Parallel()

	ctx, cancel := NewContext(context.Background())
	defer cancel()

	// Check that many ListenBrowser callbacks work.
	var attachedCount, totalCount int
	ListenBrowser(ctx, func(ev interface{}) {
		if _, ok := ev.(*target.EventAttachedToTarget); ok {
			attachedCount++
		}
	})
	ListenBrowser(ctx, func(ev interface{}) {
		totalCount++
	})

	if err := Run(ctx,
		Navigate(testdataDir+"/form.html"),
		WaitVisible(`#form`, ByID), // for form.html
	); err != nil {
		t.Fatal(err)
	}
	if want := 1; attachedCount != want {
		t.Fatalf("want %d Page.frameNavigated events; got %d", want, attachedCount)
	}
	if want := 1; totalCount < want {
		t.Fatalf("want at least %d DOM.documentUpdated events; got %d", want, totalCount)
	}
}

func TestListenTarget(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// Check that many ListenTarget callbacks work.
	var navigatedCount, updatedCount int
	ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*page.EventFrameNavigated); ok {
			navigatedCount++
		}
	})
	ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*dom.EventDocumentUpdated); ok {
			updatedCount++
		}
	})

	if err := Run(ctx,
		Navigate(testdataDir+"/form.html"),
		WaitVisible(`#form`, ByID), // for form.html
	); err != nil {
		t.Fatal(err)
	}
	if want := 1; navigatedCount != want {
		t.Fatalf("want %d Page.frameNavigated events; got %d", want, navigatedCount)
	}
	if want := 1; updatedCount < want {
		t.Fatalf("want at least %d DOM.documentUpdated events; got %d", want, updatedCount)
	}
}
