package chromedp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"iter"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp/internal/testenv"
)

var (
	// these are set up in init
	execPath    string
	testdataDir string
	allocOpts   = DefaultExecAllocatorOptions[:]

	// allocCtx is initialized in TestMain, to cancel before exiting.
	allocCtx context.Context

	// browserCtx is initialized with allocateOnce
	browserCtx context.Context
)

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("could not get working directory: %v", err))
	}
	testdataDir = "file://" + path.Join(wd, "testdata")

	allocTempDir, err = os.MkdirTemp("", "chromedp-test")
	if err != nil {
		panic(fmt.Sprintf("could not create temp directory: %v", err))
	}

	// Disabling the GPU helps portability with some systems like Travis,
	// and can slightly speed up the tests on other systems.
	allocOpts = append(allocOpts, DisableGPU)

	if testenv.VisibleWindow() {
		allocOpts = append(allocOpts, VisibleWindow)
	}

	// Find the exec path once at startup.
	execPath = testenv.ExecPath()
	if execPath == "" {
		execPath = findExecPath()
	}
	allocOpts = append(allocOpts, ExecPath(execPath))

	// Not explicitly needed to be set, as this speeds up the tests
	if testenv.NoSandbox() {
		allocOpts = append(allocOpts, NoSandbox)
	}
}

var browserOpts []ContextOption

// printBrowserVersion starts a browser, prints its version and closes it, so
// that the log of a test run on any system says which browser runs the tests.
func printBrowserVersion() {
	ctx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel = NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()
	// The first Run starts the browser. Call the browser directly.
	if err := Do(ctx); err != nil {
		fmt.Printf("browser: could not start %s: %v\n", execPath, err)
		return
	}
	v, err := cdp.Call(ctx, FromContext(ctx).Browser, browser.GetVersion, cdp.Empty{})
	if err != nil {
		fmt.Printf("browser: could not read the version of %s: %v\n", execPath, err)
		return
	}
	fmt.Printf("browser: %s, revision %s, protocol %s, path %s, user agent %q, %s/%s\n",
		v.Product, v.Revision, v.ProtocolVersion, execPath, v.UserAgent, goruntime.GOOS, goruntime.GOARCH)
}

func TestMain(m *testing.M) {
	var cancel context.CancelFunc
	allocCtx, cancel = NewExecAllocator(context.Background(), allocOpts...)
	printBrowserVersion()

	if testenv.Debug() {
		browserOpts = append(browserOpts, WithDebugf(log.Printf))
	}

	code := m.Run()
	cancel()

	if infos, _ := os.ReadDir(allocTempDir); len(infos) > 0 {
		var leaks []string
		for _, info := range infos {
			leaks = append(leaks, describeLeak(filepath.Join(allocTempDir, info.Name())))
		}
		os.RemoveAll(allocTempDir)
		panic(fmt.Sprintf("leaked %d temporary dirs under %s:\n%s",
			len(infos), allocTempDir, strings.Join(leaks, "\n")))
	} else {
		os.Remove(allocTempDir)
	}

	os.Exit(code)
}

// into makes an action that runs a and stores its value in dst. The tests use
// it to run several actions in one call of Do and still read their values.
func into[T any](dst *T, a Action[T]) Action[Void] {
	return func(ctx context.Context, t *Target) (Void, error) {
		v, err := a(ctx, t)
		if err != nil {
			return Void{}, err
		}
		*dst = v
		return Void{}, nil
	}
}

var allocateOnce sync.Once

func testAllocate(tb testing.TB, name string) (context.Context, context.CancelFunc) {
	// Start the browser exactly once, as needed.
	allocateOnce.Do(func() { browserCtx, _ = testAllocateSeparate(tb) })

	if browserCtx == nil {
		// allocateOnce.Do failed. If we continue, the test panics.
		tb.FailNow()
	}

	// Same browser, new tab. We do not need to start a new chrome browser for
	// each test, and this gives a huge speed-up.
	ctx, _ := NewContext(browserCtx)

	// Navigate only if we want an HTML file name. Otherwise leave the blank page.
	if name != "" {
		if err := Do(ctx, Navigate(testdataDir+"/"+name)); err != nil {
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

func testAllocateSeparate(tb testing.TB) (context.Context, context.CancelFunc) {
	// Entirely new browser, unlike testAllocate.
	ctx, _ := NewContext(allocCtx, browserOpts...)
	if err := Do(ctx); err != nil {
		tb.Fatal(err)
	}
	go func() {
		for ev, err := range Events(ctx, runtime.ExceptionThrown) {
			if err != nil {
				return
			}
			tb.Errorf("%+v\n", ev.ExceptionDetails)
		}
	}()
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
	if err := Do(bctx); err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, _ := NewContext(bctx)
			if err := Do(ctx,
				Navigate(testdataDir+"/form.html"),
				WaitVisible(ID(`form`)),
			); err != nil {
				b.Fatal(err)
			}
			if err := Cancel(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// checkTargets fatals if the browser behind the chromedp context has an
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
	ctx1, cancel1 := testAllocateSeparate(t)
	defer cancel1()

	checkTargets(t, ctx1, 1)

	// Start a second tab on the same browser.
	ctx2, cancel2 := NewContext(ctx1)
	defer cancel2()
	if err := Do(ctx2); err != nil {
		t.Fatal(err)
	}
	checkTargets(t, ctx2, 2)

	// The first context must also see both targets.
	checkTargets(t, ctx1, 2)

	// Canceling the second context must close only the second tab.
	cancel2()
	checkTargets(t, ctx1, 1)

	// We used to have a bug. Run reset the first context as if it was not the
	// first, and this broke its cancellation.
	if err := Do(ctx1); err != nil {
		t.Fatal(err)
	}

	// We must see one attached target, because we closed the second a while
	// ago. If we see two, there is a memory leak, because we hold onto the
	// detached target.
	pages := FromContext(ctx1).Browser.pages
	if len(pages) != 1 {
		t.Fatalf("expected one attached target, got %d", len(pages))
	}
}

func TestCancelError(t *testing.T) {
	t.Parallel()

	ctx1, cancel1 := testAllocate(t, "")
	defer cancel1()
	if err := Do(ctx1); err != nil {
		t.Fatal(err)
	}

	// Open and close a target normally. There is no error.
	ctx2, cancel2 := NewContext(ctx1)
	defer cancel2()
	if err := Do(ctx2); err != nil {
		t.Fatal(err)
	}
	if err := Cancel(ctx2); err != nil {
		t.Fatalf("expected a nil error, got %v", err)
	}

	if err := Cancel(allocCtx); err != ErrInvalidContext {
		t.Fatalf("want error %q, got %q", ErrInvalidContext, err)
	}
}

func TestPrematureCancel(t *testing.T) {
	t.Parallel()

	// Cancel before the browser is allocated.
	ctx, _ := NewContext(allocCtx, browserOpts...)
	if err := Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	if err := Do(ctx); err != context.Canceled {
		t.Fatalf("wanted canceled context error, got %v", err)
	}
}

func TestPrematureCancelTab(t *testing.T) {
	t.Parallel()

	ctx1, cancel := testAllocate(t, "")
	defer cancel()
	if err := Do(ctx1); err != nil {
		t.Fatal(err)
	}

	ctx2, cancel := NewContext(ctx1)
	// Cancel after the browser is allocated, but before we have created a new
	// tab.
	cancel()
	if err := Do(ctx2); err != context.Canceled {
		t.Fatalf("wanted canceled context error, got %v", err)
	}
}

func TestPrematureCancelAllocator(t *testing.T) {
	t.Parallel()

	// To make sure that we do not start any Chrome processes.
	allocCtx, cancel := NewExecAllocator(context.Background(),
		ExecPath("/do-not-run-chrome"))
	// Cancel before the browser is allocated.
	cancel()

	ctx, cancel := NewContext(allocCtx)
	defer cancel()
	if err := Do(ctx); err != context.Canceled {
		t.Fatalf("wanted canceled context error, got %v", err)
	}
}

func TestConcurrentCancel(t *testing.T) {
	t.Parallel()

	// To make sure that we do not start any Chrome processes.
	allocCtx, cancel := NewExecAllocator(context.Background(),
		ExecPath("/do-not-run-chrome"))
	defer cancel()

	var wg sync.WaitGroup
	// 50 is enough for 'go test -race' to easily spot issues.
	for range 50 {
		wg.Add(2)
		ctx, cancel := NewContext(allocCtx)
		go func() {
			cancel()
			wg.Done()
		}()
		go func() {
			_ = Do(ctx)
			wg.Done()
		}()
	}
	wg.Wait()
}

// TestCancelTabDuringAttach cancels a tab context while its first Do attaches
// the target. The cancellation watcher reads Context.Target at that moment,
// and attachTarget writes it. The race detector must find no data race.
func TestCancelTabDuringAttach(t *testing.T) {
	t.Parallel()

	// This test makes many tabs, and a new tab takes the focus of its window.
	// A tab without focus gets no animation frames, which stops the tests that
	// poll. So the test has a browser of its own.
	ctx1, cancel := testAllocateSeparate(t)
	defer cancel()
	b := FromContext(ctx1).Browser

	var wg sync.WaitGroup
	for i := range 150 {
		// Attach to a target that exists, so that the attach is short and
		// the cancellation can fall into it.
		res, err := cdp.Call(ctx1, b, target.CreateTarget, target.CreateTargetParams{URL: "about:blank"})
		if err != nil {
			t.Fatal(err)
		}
		ctx2, cancel := NewContext(ctx1, WithTargetID(res.TargetID))
		wg.Add(1)
		go func() {
			defer wg.Done()
			// The error is not the point of the test. The run can finish
			// before or after the cancellation.
			_ = Do(ctx2)
		}()
		time.Sleep(time.Duration(i) * 50 * time.Microsecond)
		cancel()
	}
	wg.Wait()
}

func TestBrowserEvents(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()
	if err := Do(ctx); err != nil {
		t.Fatal(err)
	}

	// Make sure that many subscriptions work, including a subscription that
	// starts after the browser is allocated. The subscriptions give up
	// when the test takes too long.
	sctx, scancel := context.WithTimeout(ctx, time.Minute)
	defer scancel()
	created := BrowserEvents(sctx, target.TargetCreated)
	attached := BrowserEvents(sctx, target.AttachedToTarget)

	newTabCtx, cancel := NewContext(ctx)
	defer cancel()
	if err := Do(newTabCtx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	cancel()
	id := FromContext(newTabCtx).Target.SessionID

	seenSession := false
	for ev, err := range attached {
		if err != nil {
			t.Fatalf("did not see Target.attachedToTarget for %q: %v", id, err)
		}
		if ev.SessionID == id {
			seenSession = true
			break
		}
	}
	if !seenSession {
		t.Fatalf("did not see Target.attachedToTarget for %q", id)
	}
	totalCount := 0
	for range created {
		totalCount++
		break
	}
	if want, got := 1, totalCount; got < want {
		t.Fatalf("want at least %d browser events; got %d", want, got)
	}
}

func TestEvents(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// Make sure that many subscriptions work, including a subscription that
	// starts after the target is attached. The first one starts
	// before the target exists, and so it opens the target. The second one
	// outlives the context, so that it delivers all events up to the end of
	// the target.
	var navigatedCount, updatedCount int
	var wg sync.WaitGroup
	navigated := Events(ctx, page.FrameNavigated)
	wg.Go(func() {
		for _, err := range navigated {
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					t.Error(err)
				}
				return
			}
			navigatedCount++
		}
	})
	if err := Do(ctx); err != nil {
		t.Fatal(err)
	}
	updated := Events(context.WithoutCancel(ctx), dom.DocumentUpdated)
	wg.Go(func() {
		for _, err := range updated {
			if err != nil {
				t.Error(err)
				return
			}
			updatedCount++
		}
	})

	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	cancel()
	wg.Wait()
	if want := 1; navigatedCount != want {
		t.Fatalf("want %d Page.frameNavigated events; got %d", want, navigatedCount)
	}
	if want := 1; updatedCount < want {
		t.Fatalf("want at least %d DOM.documentUpdated events; got %d", want, updatedCount)
	}
}

func TestLargeEventCount(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// Simulate an environment where Chrome sends 2000 console log events,
	// and we are slow at processing them. In older chromedp versions, this
	// crashed. We filled eventQueue and panicked. 50ms is enough to make the
	// test fail often on old chromedp versions, without making the test too
	// slow.
	events := Events(ctx, runtime.ConsoleAPICalled)
	go func() {
		first := true
		for _, err := range events {
			if err != nil {
				return
			}
			if first {
				time.Sleep(50 * time.Millisecond)
				first = false
			}
		}
	}()

	if err := Do(ctx,
		Navigate(testdataDir+"/consolespam.html"),
		WaitVisible(ID("done")), // wait for the JS to finish
	); err != nil {
		t.Fatal(err)
	}
}

func TestLargeQuery(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<html><body>\n")
		for i := range 2000 {
			fmt.Fprintf(w, `<div>`)
			fmt.Fprintf(w, `<a href="/%d">link %d</a>`, i, i)
			fmt.Fprintf(w, `</div>`)
		}
		fmt.Fprintf(w, "</body></html>\n")
	}))
	defer s.Close()

	// CSSAll queries thousands of events, which triggers thousands of
	// DOM events. The target handler used to deadlock, because the event
	// queues filled up and prevented the wait function from receiving any
	// result.
	var nodes []*Node
	if err := Do(ctx,
		Navigate(s.URL),
		into(&nodes, Nodes(CSSAll("a"))),
	); err != nil {
		t.Fatal(err)
	}
}

// cancelAfterFirst ranges over seq in a new goroutine. It calls cancel at the
// first event. The returned channel receives the error that ends the
// iteration, or nil when the iteration ends without an error.
func cancelAfterFirst[E any](seq iter.Seq2[E, error], cancel context.CancelFunc) <-chan error {
	done := make(chan error, 1)
	go func() {
		first := true
		for _, err := range seq {
			if err != nil {
				done <- err
				return
			}
			if first {
				first = false
				cancel()
			}
		}
		done <- nil
	}()
	return done
}

func TestEventsCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocateSeparate(t)
	defer cancel()

	// Make sure that canceling the context of an iterator ends the iteration
	// and removes the subscription.
	browserCtx, browserCancel := context.WithCancel(ctx)
	defer browserCancel()
	targetCtx, targetCancel := context.WithCancel(ctx)
	defer targetCancel()
	browserDone := cancelAfterFirst(BrowserEvents(browserCtx, target.TargetInfoChanged), browserCancel)
	targetDone := cancelAfterFirst(Events(targetCtx, page.FrameNavigated), targetCancel)

	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	for name, done := range map[string]<-chan error{"browser": browserDone, "target": targetDone} {
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("want a canceled %s iteration, got %v", name, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("the %s iteration did not end after its context was cancelled", name)
		}
	}

	b, tg := FromContext(ctx).Browser, FromContext(ctx).Target
	b.events.mu.Lock()
	browserSubs := len(b.events.subs[target.TargetInfoChanged.Method])
	b.events.mu.Unlock()
	tg.events.mu.Lock()
	targetSubs := len(tg.events.subs[page.FrameNavigated.Method])
	tg.events.mu.Unlock()
	if browserSubs != 0 || targetSubs != 0 {
		t.Fatalf("want no subscriptions left, got %d browser and %d target", browserSubs, targetSubs)
	}
}

func TestLogOptions(t *testing.T) {
	t.Parallel()

	var bufMu sync.Mutex
	var buf bytes.Buffer
	fn := func(format string, a ...any) {
		bufMu.Lock()
		fmt.Fprintf(&buf, format, a...)
		fmt.Fprintln(&buf)
		bufMu.Unlock()
	}

	ctx, cancel := NewContext(context.Background(),
		WithErrorf(fn),
		WithLogf(fn),
		WithDebugf(fn),
	)
	defer cancel()
	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	cancel()

	bufMu.Lock()
	got := buf.String()
	bufMu.Unlock()
	for _, want := range []string{
		"Page.navigate",
		"Page.frameNavigated",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain %q", want)
		}
	}
}

func TestBrowserContext(t *testing.T) {
	ctx, cancel := testAllocate(t, "child1.html")
	defer cancel()
	// There is no dedicated protocol command to get the default browser
	// context. Our workaround is to get it from a target that we create
	// without the "browserContextId" parameter.
	defaultBrowserContextID := getBrowserContext(t, ctx)

	// Prepare 2 browser contexts to be used later.
	rootCtx1, cancel := NewContext(browserCtx, WithNewBrowserContext())
	defer cancel()
	if err := Do(rootCtx1); err != nil {
		t.Fatal(err)
	}
	rootBrowserContextID1 := FromContext(rootCtx1).BrowserContextID

	rootCtx2, cancel := NewContext(browserCtx, WithNewBrowserContext())
	defer cancel()
	if err := Do(rootCtx2); err != nil {
		t.Fatal(err)
	}
	rootBrowserContextID2 := FromContext(rootCtx2).BrowserContextID

	tests := []struct {
		name         string
		arrange      func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID)
		wantDisposed bool
		wantPanic    string
	}{
		{
			name: "default",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(browserCtx)
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				return ctx, cancel, defaultBrowserContextID
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "new",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(browserCtx, WithNewBrowserContext())
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				c := FromContext(ctx)
				return ctx, cancel, c.BrowserContextID
			},
			wantDisposed: true,
			wantPanic:    "",
		},
		{
			name: "existing",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(browserCtx, WithExistingBrowserContext(rootBrowserContextID1))
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				return ctx, cancel, rootBrowserContextID1
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "inherited 1",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(rootCtx1)
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				return ctx, cancel, rootBrowserContextID1
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "inherited 2",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx1, _ := NewContext(rootCtx1)
				if err := Do(ctx1); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := NewContext(ctx1)
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				return ctx, cancel, rootBrowserContextID1
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "inherited 3",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx1, _ := NewContext(browserCtx, WithExistingBrowserContext(rootBrowserContextID1))
				if err := Do(ctx1); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := NewContext(ctx1)
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				return ctx, cancel, rootBrowserContextID1
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "break inheritance 1",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(rootCtx1, WithExistingBrowserContext(rootBrowserContextID2))
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				// The target must be added to the second browser context.
				return ctx, cancel, rootBrowserContextID2
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "break inheritance 2",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(rootCtx1, WithNewBrowserContext())
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				c := FromContext(ctx)
				if c.BrowserContextID == rootBrowserContextID1 {
					t.Fatal("a new BrowserContext should be created")
				}
				return ctx, cancel, c.BrowserContextID
			},
			wantDisposed: true,
			wantPanic:    "",
		},
		{
			name: "break inheritance 3",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(rootCtx1, WithTargetID(FromContext(rootCtx2).Target.TargetID))
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}

				c := FromContext(ctx)
				if c.BrowserContextID != "" {
					t.Fatal("when a context is used to attach to a tab, its BrowserContextID should be empty")
				}

				return ctx, cancel, rootBrowserContextID2
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "WithNewBrowserContext when WithTargetID is specified",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, _ := NewContext(rootCtx1)
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := NewContext(browserCtx, WithTargetID(FromContext(ctx).Target.TargetID), WithNewBrowserContext())
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}

				return ctx, cancel, rootBrowserContextID1
			},
			wantDisposed: false,
			wantPanic:    "WithNewBrowserContext can not be used when WithTargetID is specified",
		},
		{
			name: "WithExistingBrowserContext when WithTargetID is specified",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, _ := NewContext(rootCtx1)
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := NewContext(browserCtx, WithTargetID(FromContext(ctx).Target.TargetID), WithExistingBrowserContext(rootBrowserContextID2))
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}

				return ctx, cancel, rootBrowserContextID1
			},
			wantDisposed: false,
			wantPanic:    "WithExistingBrowserContext can not be used when WithTargetID is specified",
		},
		{
			name: "WithNewBrowserContext before Browser is initialized",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(context.Background(), WithNewBrowserContext())
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}

				return ctx, cancel, ""
			},
			wantDisposed: false,
			wantPanic:    "WithNewBrowserContext can not be used before Browser is initialized",
		},
		{
			name: "WithExistingBrowserContext before Browser is initialized",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(context.Background(), WithExistingBrowserContext(rootBrowserContextID1))
				if err := Do(ctx); err != nil {
					t.Fatal(err)
				}

				return ctx, cancel, ""
			},
			wantDisposed: false,
			wantPanic:    "WithExistingBrowserContext can not be used before Browser is initialized",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.wantPanic != "" {
				defer func() {
					if got := fmt.Sprint(recover()); got != test.wantPanic {
						t.Errorf("want panic %q, got %q", test.wantPanic, got)
					}
				}()
			}
			ctx, cancel, want := test.arrange(t)
			defer cancel()

			got := getBrowserContext(t, ctx)

			if got != want {
				switch want {
				case defaultBrowserContextID:
					t.Errorf("want default browser context %q, got %q", want, got)
				case rootBrowserContextID1:
					t.Errorf("want root browser context 1 %q, got %q", want, got)
				case rootBrowserContextID2:
					t.Errorf("want root browser context 2 %q, got %q", want, got)
				default:
					t.Errorf("want browser context %q, got %q", want, got)
				}
			}

			if want == defaultBrowserContextID {
				// There is no way to find out whether the default browser context
				// is disposed, so stop here.
				return
			}

			cancel()

			var ids []cdp.BrowserContextID
			if err := Do(browserCtx,
				Func(func(ctx context.Context, t *Target) error {
					res, err := CallBrowser(ctx, target.GetBrowserContexts, cdp.Empty{})
					ids = res.BrowserContextIDs
					return err
				}),
			); err != nil {
				t.Fatal(err)
			}

			disposed := !slices.Contains(ids, want)

			if disposed != test.wantDisposed {
				t.Errorf("browser context disposed = %v, want %v", disposed, test.wantDisposed)
			}
		})
	}
}

func getBrowserContext(tb testing.TB, ctx context.Context) cdp.BrowserContextID {
	var id cdp.BrowserContextID
	if err := Do(ctx,
		Func(func(ctx context.Context, t *Target) error {
			res, err := Call(ctx, target.GetTargetInfo, target.GetTargetInfoParams{})
			if err != nil {
				return err
			}
			id = res.TargetInfo.BrowserContextID
			return nil
		}),
	); err != nil {
		tb.Fatal(err)
	}
	return id
}

func TestLargeOutboundMessages(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// ~5MiB of JS to test a large outbound message. The module remote tests the
	// grow feature of its websocket.
	expr := fmt.Sprintf("//%s\n", strings.Repeat("x", 5<<20))
	if _, err := Run(ctx, Evaluate[[]byte](expr)); err != nil {
		t.Fatal(err)
	}
}

func TestDirectCloseTarget(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	c := FromContext(ctx)
	want := "to close the target, cancel its context"

	// Make sure that nothing is closed by running the action twice.
	for range 2 {
		err := Do(ctx, Func(func(ctx context.Context, t *Target) error {
			_, err := Call(ctx, target.CloseTarget, target.CloseTargetParams{TargetID: c.Target.TargetID})
			return err
		}))
		got := fmt.Sprint(err)
		if !strings.Contains(got, want) {
			t.Fatalf("want %q, got %q", want, got)
		}
	}
}

func TestDirectCloseBrowser(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocateSeparate(t)
	defer cancel()

	want := "use chromedp.Cancel"

	// Make sure that nothing is closed by running the action twice.
	for range 2 {
		_, err := CallBrowser(ctx, browser.Close, cdp.Empty{})
		got := fmt.Sprint(err)
		if !strings.Contains(got, want) {
			t.Fatalf("want %q, got %q", want, got)
		}
	}
}

func TestDownloadIntoDir(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	dir := t.TempDir()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data.bin":
			w.Header().Set("Content-Type", "application/octet-stream")
			fmt.Fprintf(w, "some binary data")
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `go <a id="download" href="/data.bin">download</a> stuff/`)
		}
	}))
	defer s.Close()

	progress := Events(ctx, browser.DownloadProgress)

	if err := Do(ctx,
		Navigate(s.URL),
		Func(func(ctx context.Context, t *Target) error {
			_, err := Call(ctx, browser.SetDownloadBehavior, browser.SetDownloadBehaviorParams{
				Behavior:      browser.SetDownloadBehaviorBehaviorAllowAndName,
				DownloadPath:  dir,
				EventsEnabled: new(true),
			})
			return err
		}),
		Click(CSS("#download")),
	); err != nil {
		t.Fatal(err)
	}

	for ev, err := range progress {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev.State != browser.DownloadProgressStateCompleted {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, ev.GUID)); err != nil {
			t.Fatalf("want error nil, got: %v", err)
		}
		break
	}
}

func TestGracefulBrowserShutdown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// TODO(mvdan): this does not work with DefaultExecAllocatorOptions+UserDataDir
	opts := []ExecAllocatorOption{
		NoFirstRun,
		NoDefaultBrowserCheck,
		Headless,
		UserDataDir(dir),
	}
	actx, cancel := NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/set" {
			http.SetCookie(w, &http.Cookie{
				Name:    "cookie1",
				Value:   "value1",
				Expires: time.Now().AddDate(0, 0, 1), // one day later
			})
		}
	}))
	defer ts.Close()

	{
		ctx, _ := NewContext(actx)
		if err := Do(ctx, Navigate(ts.URL+"/set")); err != nil {
			t.Fatal(err)
		}

		// Close the browser gracefully.
		if err := Cancel(ctx); err != nil {
			t.Fatal(err)
		}
	}
	{
		ctx, _ := NewContext(actx)
		// Close Chrome gracefully. If it is killed, its child processes
		// can write to dir after the test, and the cleanup of dir fails.
		defer func() {
			if err := Cancel(ctx); err != nil {
				t.Error(err)
			}
		}()
		var got string
		if err := Do(ctx,
			Navigate(ts.URL),
			into(&got, EvaluateAsDevTools[string]("document.cookie")),
		); err != nil {
			t.Fatal(err)
		}
		if want := "cookie1=value1"; got != want {
			t.Fatalf("want cookies %q; got %q", want, got)
		}
	}
}

func TestAttachingToWorkers(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc, pageJS, wantSelf string
	}{
		{"DedicatedWorker", "new Worker('/worker.js')", "DedicatedWorkerGlobalScope"},
		{"ServiceWorker", "navigator.serviceWorker.register('/worker.js')", "ServiceWorkerGlobalScope"},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `
					<html>
						<body>
							<script>
								%s
							</script>
						</body>
					</html>`, tc.pageJS)
			})
			mux.HandleFunc("/worker.js", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/javascript")
				io.WriteString(w, "console.log('I am worker code.');")
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()

			ctx, cancel := NewContext(context.Background())
			defer cancel()

			sctx, scancel := context.WithTimeout(ctx, time.Minute)
			defer scancel()
			attached := Events(sctx, target.AttachedToTarget)

			if err := Do(ctx, Navigate(ts.URL)); err != nil {
				t.Fatalf("Failed to navigate to the test page: %q", err)
			}

			var targetID target.ID
			for ev, err := range attached {
				if err != nil {
					t.Fatalf("Failed to wait for the worker target: %q", err)
				}
				if strings.Contains(ev.TargetInfo.Type, "worker") {
					targetID = ev.TargetInfo.TargetID
					break
				}
			}
			ctx, cancel = NewContext(ctx, WithTargetID(targetID))
			defer cancel()

			if err := Do(ctx, Func(func(ctx context.Context, t *Target) error {
				if r, err := Call(ctx, runtime.Evaluate, runtime.EvaluateParams{Expression: "self"}); err != nil {
					return err
				} else if r.Result.ClassName != tc.wantSelf {
					return fmt.Errorf("Global scope type mismatch: got %q want: %q", r.Result.ClassName, tc.wantSelf)
				}
				return nil
			})); err != nil {
				t.Fatalf("Failed to check evaluating JavaScript in a worker target: %q", err)
			}
		})
	}
}

func TestRunResponse(t *testing.T) {
	t.Parallel()

	// This test includes many edge cases for RunResponse: navigations that
	// fail to start, responses that return errors, responses that redirect,
	// and so on.
	// We also test each of those with different actions, such as a straight
	// navigation and a click.
	// The important part is an iframe that keeps reloading every 100ms in the
	// main page. If RunResponse does not filter the events for the top level
	// frame properly, the tests fail often.

	indexTmpl := template.Must(template.New("").Parse(`
		<html>
			<body>
				<a id="url_index" href="/index">index</a>
				<a id="url_200" href="/200">200</a>
				<a id="url_404" href="/404">404</a>
				<a id="url_500" href="/500">500</a>
				<a id="url_badtls" href="https://{{.Host}}/index">badtls</a>
				<a id="url_badprotocol" href="bad://{{.Host}}/index">badprotocol</a>
				<a id="url_unimplementedprotocol" href="ftp://{{.Host}}/index">unimplementedprotocol</a>
				<a id="url_plain" href="/plain">plain</a>
				<a id="url_two" href="/two">two</a>
				<a id="url_one" href="/one">one</a>
				<a id="url_infinite" href="/infinite">infinite</a>
				<a id="url_badiframe" href="/badiframe">badiframe</a>

				<script>
					setInterval(function(){
						document.getElementById("reloadingframe").src += "";
					}, 100);
				</script>
				<iframe id="reloadingframe" src="/reloadingframe"></iframe>
			</body>
		</html>`))
	mux := http.NewServeMux()
	mux.HandleFunc("/index", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		indexTmpl.Execute(w, r)
	})
	mux.HandleFunc("/200", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "OK")
	})
	mux.HandleFunc("/500", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "500", 500)
	})
	mux.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "OK")
	})
	mux.HandleFunc("/two", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/one", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/one", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/zero", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/zero", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})
	mux.HandleFunc("/infinite", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/infinite", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/badiframe", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body><iframe src="badurl://localhost/"></iframe></body></html>`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	tests := []struct {
		name string
		url  string

		wantErr       string
		wantSuffixURL string
		wantStatus    int64
	}{
		{
			name:       "200",
			url:        "200",
			wantStatus: 200,
		},
		{
			name:       "404",
			url:        "404",
			wantStatus: 404,
		},
		{
			name:       "500",
			url:        "500",
			wantStatus: 500,
		},

		// Use the local http server as https. This is a TLS error and the
		// load fails. If we do not capture the "loading failed" error, we
		// block until the timeout and give a generic "deadline exceeded"
		// error.
		{
			name:    "BadTLS",
			url:     strings.ReplaceAll(ts.URL, "http://", "https://") + "/index",
			wantErr: "ERR_SSL_PROTOCOL_ERROR",
		},

		// In this case, the "loading failed" event is received, but the
		// load itself is canceled immediately, so we never receive a
		// load event of any sort.
		{
			name:    "BadProtocol",
			url:     strings.ReplaceAll(ts.URL, "http://", "bad://") + "/index",
			wantErr: "ERR_ABORTED",
		},

		// Make sure that loading a non-HTML document still works normally.
		{
			name:          "NonHTML",
			url:           "plain",
			wantSuffixURL: "/plain",
		},

		{
			name:          "BadIframe",
			url:           "badiframe",
			wantSuffixURL: "/badiframe",
		},

		{
			name:          "OneRedirect",
			url:           "one",
			wantSuffixURL: "/zero",
		},
		{
			name:          "TwoRedirects",
			url:           "two",
			wantSuffixURL: "/zero",
		},
		{
			name:    "InfiniteRedirects",
			url:     "infinite",
			wantErr: "ERR_TOO_MANY_REDIRECTS",
		},
	}

	for _, test := range tests {
		allocate := func(t *testing.T) context.Context {
			ctx, cancel := testAllocate(t, "")
			t.Cleanup(cancel)
			ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
			t.Cleanup(cancel)

			if err := Do(ctx, Navigate(ts.URL+"/index")); err != nil {
				t.Fatalf("Failed to navigate to the test page: %q", err)
			}
			return ctx
		}
		checkResults := func(t *testing.T, resp *network.Response, err error) {
			if test.wantErr == "" && err != nil {
				t.Fatalf("wanted nil error, got %v", err)
			}
			if got := fmt.Sprint(err); !strings.Contains(got, test.wantErr) {
				t.Fatalf("wanted error to contain %q, got %q", test.wantErr, got)
			}
			if test.wantErr == "" && resp == nil {
				t.Fatalf("expected response to be non-nil")
			} else if test.wantErr != "" && resp != nil {
				t.Fatalf("expected response to be nil")
			}

			url := ""
			status := int64(0)
			if resp != nil {
				url = resp.URL
				status = resp.Status
			}
			if !strings.HasSuffix(url, test.wantSuffixURL) {
				t.Fatalf("wanted response URL to end with %q, got %q", test.wantSuffixURL, url)
			}
			if want := test.wantStatus; want != 0 && status != want {
				t.Fatalf("wanted status code %d, got %d", want, status)
			}

			if resp != nil {
				// The protocol type says seconds, but the browser sends
				// milliseconds. See https://github.com/chromedp/pdlgen/issues/22.
				responseTime := time.UnixMilli(int64(resp.ResponseTime))
				latency := time.Since(responseTime)
				if latency > time.Hour || latency < -time.Hour {
					t.Errorf("responseTime does not hold a reasonable value %s. "+
						"Maybe it's in seconds now and we should remove the workaround. "+
						"See https://github.com/chromedp/pdlgen/issues/22.",
						responseTime)
				}
			}
		}
		t.Run("Navigate"+test.name, func(t *testing.T) {
			t.Parallel()
			ctx := allocate(t)

			url := test.url
			if !strings.Contains(url, "/") {
				url = ts.URL + "/" + url
			}
			resp, err := RunResponse(ctx, Navigate(url))
			checkResults(t, resp, err)
		})
		t.Run("Click"+test.name, func(t *testing.T) {
			t.Parallel()
			ctx := allocate(t)

			query := "#url_" + strings.ToLower(test.name)
			if !strings.Contains(test.url, "/") {
				query = "#url_" + test.url
			}
			resp, err := RunResponse(ctx, Click(CSS(query)))
			checkResults(t, resp, err)
		})
	}
}

func TestRunResponse_noResponse(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/200", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body>
		<a id="same" href="/200">same</a>
		<a id="fragment" href="/200#fragment">fragment</a>
		</body></html>`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	steps := []struct {
		name     string
		action   Action[Void]
		wantResp bool
	}{
		{"FirstNavigation", Navigate(ts.URL + "/200"), true},
		{"RepeatedNavigation", Navigate(ts.URL + "/200"), true},
		{"FragmentNavigation", Navigate(ts.URL + "/200#foo"), false},

		{"FirstClick", Click(CSS("#same")), true},
		{"RepeatedClick", Click(CSS("#same")), true},
		{"FragmentClick", Click(CSS("#fragment")), false},

		{"Blank", Navigate("about:blank"), false},
	}
	// Do not use sub-tests, as these are all sequential steps that cannot
	// happen independently of each other.
	for _, step := range steps {
		resp, err := RunResponse(ctx, step.action)
		if err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if resp == nil && step.wantResp {
			t.Fatalf("%s: wanted a response, got nil", step.name)
		} else if resp != nil && !step.wantResp {
			t.Fatalf("%s: did not want a response, got: %#v", step.name, resp)
		}
	}
}

// TestWebGL tests that WebGL is correctly configured in headless-shell.
//
// This is a regression test for https://github.com/chromedp/chromedp/issues/1073.
func TestWebGL(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "webgl.html")
	defer cancel()

	var buf []byte
	if err := Do(ctx,
		Poll[Void]("rendered", WithPollingTimeout(2*time.Second)),
		into(&buf, Screenshot(CSS(`#c`))),
	); err != nil {
		if errors.Is(err, ErrPollingTimeout) {
			t.Fatal("The cube is not rendered in 2s.")
		} else {
			t.Fatal(err)
		}
	}

	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 200 || bounds.Dy() != 200 {
		t.Fatalf("Unexpected screenshot size. got: %d x %d, want 200 x 200.", bounds.Dx(), bounds.Dy())
	}

	isWhite := func(c color.Color) bool {
		r, g, b, _ := c.RGBA()
		return r == 0xffff && g == 0xffff && b == 0xffff
	}
	if isWhite(img.At(100, 100)) {
		t.Fatal("When the cube is rendered correctly, the color at the middle of the canvas should not be white.")
	}
}

// TestPDFTemplate tests that the resource pack is loaded in headless-shell.
//
// regression test for https://github.com/chromedp/chromedp/issues/1551
func TestPDFBackground(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	var buf []byte
	if err := Do(ctx,
		Navigate("about:blank"),
		Func(func(ctx context.Context, t *Target) error {
			frameTree, err := Call(ctx, page.GetFrameTree, cdp.Empty{})
			if err != nil {
				return err
			}
			_, err = Call(ctx, page.SetDocumentContent, page.SetDocumentContentParams{
				FrameID: frameTree.FrameTree.Frame.ID,
				HTML: `
				<html lang="en">
					<head></head>
					<body style="background-color:green">
						<p>Lorem ipsum</p>
					</body>
				</html>
			`,
			})
			return err
		}),
		Func(func(ctx context.Context, t *Target) error {
			res, err := Call(ctx, page.PrintToPDF, page.PrintToPDFParams{})
			buf = res.Data
			return err
		}),
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("background.pdf", buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// describeLeak lists the files under dir, to help find who left it.
func describeLeak(dir string) string {
	var files []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			files = append(files, fmt.Sprintf("%s (error: %v)", p, err))
			return nil
		}
		if p != dir {
			files = append(files, strings.TrimPrefix(p, dir))
		}
		return nil
	})
	return fmt.Sprintf("%s: %d entries: %s", dir, len(files), strings.Join(files, " "))
}
