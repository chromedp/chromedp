package chromedp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	"github.com/ledongthuc/pdf"
)

var (
	// these are set up in init
	execPath    string
	testdataDir string
	allocOpts   = DefaultExecAllocatorOptions[:]

	// allocCtx is initialised in TestMain, to cancel before exiting.
	allocCtx context.Context

	// browserCtx is initialised with allocateOnce
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

	if noHeadless := os.Getenv("CHROMEDP_NO_HEADLESS"); noHeadless != "" && noHeadless != "false" {
		allocOpts = append(allocOpts, Flag("headless", false))
	}

	// Find the exec path once at startup.
	execPath = os.Getenv("CHROMEDP_TEST_RUNNER")
	if execPath == "" {
		execPath = findExecPath()
	}
	allocOpts = append(allocOpts, ExecPath(execPath))

	// Not explicitly needed to be set, as this speeds up the tests
	if noSandbox := os.Getenv("CHROMEDP_NO_SANDBOX"); noSandbox != "false" {
		allocOpts = append(allocOpts, NoSandbox)
	}
}

var browserOpts []ContextOption

func TestMain(m *testing.M) {
	var cancel context.CancelFunc
	allocCtx, cancel = NewExecAllocator(context.Background(), allocOpts...)

	if debug := os.Getenv("CHROMEDP_DEBUG"); debug != "" && debug != "false" {
		browserOpts = append(browserOpts, WithDebugf(log.Printf))
	}

	code := m.Run()
	cancel()

	if infos, _ := os.ReadDir(allocTempDir); len(infos) > 0 {
		os.RemoveAll(allocTempDir)
		panic(fmt.Sprintf("leaked %d temporary dirs under %s", len(infos), allocTempDir))
	} else {
		os.Remove(allocTempDir)
	}

	os.Exit(code)
}

var allocateOnce sync.Once

func testAllocate(tb testing.TB, name string) (context.Context, context.CancelFunc) {
	// Start the browser exactly once, as needed.
	allocateOnce.Do(func() { browserCtx, _ = testAllocateSeparate(tb) })

	if browserCtx == nil {
		// allocateOnce.Do failed; continuing would result in panics.
		tb.FailNow()
	}

	// Same browser, new tab; not needing to start new chrome browsers for
	// each test gives a huge speed-up.
	ctx, _ := NewContext(browserCtx)

	// Only navigate if we want an HTML file name, otherwise leave the blank page.
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

func testAllocateSeparate(tb testing.TB) (context.Context, context.CancelFunc) {
	// Entirely new browser, unlike testAllocate.
	ctx, _ := NewContext(allocCtx, browserOpts...)
	if err := Run(ctx); err != nil {
		tb.Fatal(err)
	}
	ListenBrowser(ctx, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventExceptionThrown); ok {
			tb.Errorf("%+v\n", ev.ExceptionDetails)
		}
	})
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
				WaitVisible(`#form`, ByID),
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

	// We should see one attached target, since we closed the second a while
	// ago. If we see two, that means there's a memory leak, as we're
	// holding onto the detached target.
	pages := FromContext(ctx1).Browser.pages
	if len(pages) != 1 {
		t.Fatalf("expected one attached target, got %d", len(pages))
	}
}

func TestCancelError(t *testing.T) {
	t.Parallel()

	ctx1, cancel1 := testAllocate(t, "")
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

	if err := Cancel(allocCtx); err != ErrInvalidContext {
		t.Fatalf("want error %q, got %q", ErrInvalidContext, err)
	}

	/*
		// NOTE: the following test no longer is applicable, as a slight change
		// to chromium's 89 API deprecated the boolean return value

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
	*/
}

func TestPrematureCancel(t *testing.T) {
	t.Parallel()

	// Cancel before the browser is allocated.
	ctx, _ := NewContext(allocCtx, browserOpts...)
	if err := Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx); err != context.Canceled {
		t.Fatalf("wanted canceled context error, got %v", err)
	}
}

func TestPrematureCancelTab(t *testing.T) {
	t.Parallel()

	ctx1, cancel := testAllocate(t, "")
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

	var wg sync.WaitGroup
	// 50 is enough for 'go test -race' to easily spot issues.
	for i := 0; i < 50; i++ {
		wg.Add(2)
		ctx, cancel := NewContext(allocCtx)
		go func() {
			cancel()
			wg.Done()
		}()
		go func() {
			_ = Run(ctx)
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestListenBrowser(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// Check that many ListenBrowser callbacks work, including adding
	// callbacks after the browser has been allocated.
	var totalCount int32
	ListenBrowser(ctx, func(ev interface{}) {
		// using sync/atomic, as the browser is shared.
		atomic.AddInt32(&totalCount, 1)
	})
	if err := Run(ctx); err != nil {
		t.Fatal(err)
	}
	seenSessions := make(map[target.SessionID]bool)
	ListenBrowser(ctx, func(ev interface{}) {
		if ev, ok := ev.(*target.EventAttachedToTarget); ok {
			seenSessions[ev.SessionID] = true
		}
	})

	newTabCtx, cancel := NewContext(ctx)
	defer cancel()
	if err := Run(newTabCtx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	cancel()
	if id := FromContext(newTabCtx).Target.SessionID; !seenSessions[id] {
		t.Fatalf("did not see Target.attachedToTarget for %q", id)
	}
	if want, got := int32(1), atomic.LoadInt32(&totalCount); got < want {
		t.Fatalf("want at least %d browser events; got %d", want, got)
	}
}

func TestListenTarget(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// Check that many listen callbacks work, including adding callbacks
	// after the target has been attached to.
	var navigatedCount, updatedCount int
	ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*page.EventFrameNavigated); ok {
			navigatedCount++
		}
	})
	if err := Run(ctx); err != nil {
		t.Fatal(err)
	}
	ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*dom.EventDocumentUpdated); ok {
			updatedCount++
		}
	})

	if err := Run(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	cancel()
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
	// would crash as we would fill eventQueue and panic. 50ms is enough to
	// make the test fail somewhat reliably on old chromedp versions,
	// without making the test too slow.
	first := true
	ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*runtime.EventConsoleAPICalled); ok && first {
			time.Sleep(50 * time.Millisecond)
			first = false
		}
	})

	if err := Run(ctx,
		Navigate(testdataDir+"/consolespam.html"),
		WaitVisible("#done", ByID), // wait for the JS to finish
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
		for i := 0; i < 2000; i++ {
			fmt.Fprintf(w, `<div>`)
			fmt.Fprintf(w, `<a href="/%d">link %d</a>`, i, i)
			fmt.Fprintf(w, `</div>`)
		}
		fmt.Fprintf(w, "</body></html>\n")
	}))
	defer s.Close()

	// ByQueryAll queries thousands of events, which triggers thousands of
	// DOM events. The target handler used to get into a deadlock, as the
	// event queues would fill up and prevent the wait function from
	// receiving any result.
	var nodes []*cdp.Node
	if err := Run(ctx,
		Navigate(s.URL),
		Nodes("a", &nodes, ByQueryAll),
	); err != nil {
		t.Fatal(err)
	}
}

func TestDialTimeout(t *testing.T) {
	t.Parallel()

	t.Run("ShortTimeoutError", func(t *testing.T) {
		t.Parallel()
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		url := "ws://" + l.(*net.TCPListener).Addr().String()
		defer l.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err = NewBrowser(ctx, url, WithDialTimeout(time.Microsecond))
		got, want := fmt.Sprintf("%v", err), "i/o timeout"
		if !strings.Contains(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("NoTimeoutSuccess", func(t *testing.T) {
		t.Parallel()
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		url := "ws://" + l.(*net.TCPListener).Addr().String()
		defer l.Close()
		go func() {
			conn, err := l.Accept()
			if err == nil {
				conn.Close()
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err = NewBrowser(ctx, url, WithDialTimeout(0))
		got := fmt.Sprintf("%v", err)
		if !strings.Contains(got, "EOF") && !strings.Contains(got, "connection reset") {
			t.Fatalf("got %q, want %q or %q", got, "EOF", "connection reset")
		}
	})
}

func TestListenCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocateSeparate(t)
	defer cancel()

	// Check that cancelling a listen context stops the listener.
	var browserCount, targetCount int

	ctx1, cancel1 := context.WithCancel(ctx)
	ListenBrowser(ctx1, func(ev interface{}) {
		browserCount++
		cancel1()
	})

	ctx2, cancel2 := context.WithCancel(ctx)
	ListenTarget(ctx2, func(ev interface{}) {
		targetCount++
		cancel2()
	})

	if err := Run(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	if want := 1; browserCount != 1 {
		t.Fatalf("want %d browser events; got %d", want, browserCount)
	}
	if want := 1; targetCount != 1 {
		t.Fatalf("want %d target events; got %d", want, targetCount)
	}
}

func TestLogOptions(t *testing.T) {
	t.Parallel()

	var bufMu sync.Mutex
	var buf bytes.Buffer
	fn := func(format string, a ...interface{}) {
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
	if err := Run(ctx, Navigate(testdataDir+"/form.html")); err != nil {
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
	// There is not a dedicated cdp command to get the default browser context.
	// Our workaround is to get it from a target which is created without the
	// "browserContextId" parameter.
	defaultBrowserContextID := getBrowserContext(t, ctx)

	// Prepare 2 browser contexts to be used later.
	rootCtx1, cancel := NewContext(browserCtx, WithNewBrowserContext())
	defer cancel()
	if err := Run(rootCtx1); err != nil {
		t.Fatal(err)
	}
	rootBrowserContextID1 := FromContext(rootCtx1).BrowserContextID

	rootCtx2, cancel := NewContext(browserCtx, WithNewBrowserContext())
	defer cancel()
	if err := Run(rootCtx2); err != nil {
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
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx1); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := NewContext(ctx1)
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx1); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := NewContext(ctx1)
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
					t.Fatal(err)
				}
				// The target should be added to the second browser context.
				return ctx, cancel, rootBrowserContextID2
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "break inheritance 2",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				ctx, cancel := NewContext(rootCtx1, WithNewBrowserContext())
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := NewContext(browserCtx, WithTargetID(FromContext(ctx).Target.TargetID), WithNewBrowserContext())
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := NewContext(browserCtx, WithTargetID(FromContext(ctx).Target.TargetID), WithExistingBrowserContext(rootBrowserContextID2))
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
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
				if err := Run(ctx); err != nil {
					t.Fatal(err)
				}

				return ctx, cancel, ""
			},
			wantDisposed: false,
			wantPanic:    "WithExistingBrowserContext can not be used before Browser is initialized",
		},
		{
			name: "remote allocator WithExistingBrowserContext ",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				c := FromContext(browserCtx)
				var conn *net.TCPConn
				if chromedpConn, ok := c.Browser.conn.(*Conn); ok {
					conn, _ = chromedpConn.conn.(*net.TCPConn)
				}
				if conn == nil {
					t.Skip("skip when the remote debugging address is not available")
				}
				actx, _ := NewRemoteAllocator(context.Background(), "ws://"+conn.RemoteAddr().String())
				ctx, cancel := NewContext(actx, WithExistingBrowserContext(rootBrowserContextID1))
				if err := Run(ctx); err != nil {
					t.Fatal(err)
				}

				return ctx, cancel, rootBrowserContextID1
			},
			wantDisposed: false,
			wantPanic:    "",
		},
		{
			name: "remote allocator WithNewBrowserContext",
			arrange: func(t *testing.T) (context.Context, context.CancelFunc, cdp.BrowserContextID) {
				c := FromContext(browserCtx)
				var conn *net.TCPConn
				if chromedpConn, ok := c.Browser.conn.(*Conn); ok {
					conn, _ = chromedpConn.conn.(*net.TCPConn)
				}
				if conn == nil {
					t.Skip("skip when the remote debugging address is not available")
				}
				actx, _ := NewRemoteAllocator(context.Background(), "ws://"+conn.RemoteAddr().String())
				ctx, cancel := NewContext(actx, WithNewBrowserContext())
				if err := Run(ctx); err != nil {
					t.Fatal(err)
				}

				return ctx, cancel, FromContext(ctx).BrowserContextID
			},
			wantDisposed: true,
			wantPanic:    "",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic != "" {
				defer func() {
					if got := fmt.Sprint(recover()); got != tt.wantPanic {
						t.Errorf("want panic %q, got %q", tt.wantPanic, got)
					}
				}()
			}
			ctx, cancel, want := tt.arrange(t)
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
				// There is not way to check whether the default browser context
				// is disposed, so stop here.
				return
			}

			cancel()

			var ids []cdp.BrowserContextID
			if err := Run(browserCtx,
				ActionFunc(func(ctx context.Context) error {
					c := FromContext(ctx)
					var err error
					ids, err = target.GetBrowserContexts().Do(cdp.WithExecutor(ctx, c.Browser))
					return err
				}),
			); err != nil {
				t.Fatal(err)
			}

			disposed := !contains(ids, want)

			if disposed != tt.wantDisposed {
				t.Errorf("browser context disposed = %v, want %v", disposed, tt.wantDisposed)
			}
		})
	}
}

func getBrowserContext(tb testing.TB, ctx context.Context) cdp.BrowserContextID {
	var id cdp.BrowserContextID
	if err := Run(ctx,
		ActionFunc(func(ctx context.Context) error {
			info, err := target.GetTargetInfo().Do(ctx)
			id = info.BrowserContextID
			return err
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

	// ~5MiB of JS to test the grow feature of github.com/gobwas/ws.
	expr := fmt.Sprintf("//%s\n", strings.Repeat("x", 5<<20))
	res := new([]byte)
	if err := Run(ctx, Evaluate(expr, res)); err != nil {
		t.Fatal(err)
	}
}

func TestDirectCloseTarget(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	c := FromContext(ctx)
	want := "to close the target, cancel its context"

	// Check that nothing is closed by running the action twice.
	for i := 0; i < 2; i++ {
		err := Run(ctx, ActionFunc(func(ctx context.Context) error {
			return target.CloseTarget(c.Target.TargetID).Do(ctx)
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

	c := FromContext(ctx)
	want := "use chromedp.Cancel"

	// Check that nothing is closed by running the action twice.
	for i := 0; i < 2; i++ {
		err := browser.Close().Do(cdp.WithExecutor(ctx, c.Browser))
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

	done := make(chan string, 1)
	ListenTarget(ctx, func(v interface{}) {
		if ev, ok := v.(*browser.EventDownloadProgress); ok {
			if ev.State == browser.DownloadProgressStateCompleted {
				done <- ev.GUID
				close(done)
			}
		}
	})

	if err := Run(ctx,
		Navigate(s.URL),
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).WithDownloadPath(dir).WithEventsEnabled(true),
		Click("#download", ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ctx.Done():
		t.Fatalf("unexpected error: %v", ctx.Err())
	case guid := <-done:
		if _, err := os.Stat(filepath.Join(dir, guid)); err != nil {
			t.Fatalf("want error nil, got: %v", err)
		}
	}
}

func TestGracefulBrowserShutdown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// TODO(mvdan): this doesn't work with DefaultExecAllocatorOptions+UserDataDir
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
		if err := Run(ctx, Navigate(ts.URL+"/set")); err != nil {
			t.Fatal(err)
		}

		// Close the browser gracefully.
		if err := Cancel(ctx); err != nil {
			t.Fatal(err)
		}
	}
	{
		ctx, _ := NewContext(actx)
		var got string
		if err := Run(ctx,
			Navigate(ts.URL),
			EvaluateAsDevTools("document.cookie", &got),
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

			ch := make(chan target.ID, 1)

			ListenTarget(ctx, func(ev interface{}) {
				if ev, ok := ev.(*target.EventAttachedToTarget); ok {
					if !strings.Contains(ev.TargetInfo.Type, "worker") {
						return
					}
					ch <- ev.TargetInfo.TargetID
				}
			})

			if err := Run(ctx, Navigate(ts.URL)); err != nil {
				t.Fatalf("Failed to navigate to the test page: %q", err)
			}

			targetID := <-ch
			ctx, cancel = NewContext(ctx, WithTargetID(targetID))
			defer cancel()

			if err := Run(ctx, ActionFunc(func(ctx context.Context) error {
				if r, _, err := runtime.Evaluate("self").Do(ctx); err != nil {
					return err
				} else if r.ClassName != tc.wantSelf {
					return fmt.Errorf("Global scope type mismatch: got %q want: %q", r.ClassName, tc.wantSelf)
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

	// This test includes many edge cases for RunResponse; navigations that
	// fail to start, responses that return errors, responses that redirect,
	// and so on.
	// We also test each of those with different actions, such as a straight
	// navigation, as well as a click.
	// What's important here is that we have an iframe that keeps reloading
	// every 100ms in the main page. If RunResponse doesn't properly filter
	// events for the top level frame, the tests should fail pretty often.

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

		// Use the local http server as https, which should be a TLS
		// error and fail to load. If we don't capture the "loading
		// failed" error, we will block until the timeout is hit and
		// give a generic "deadline exceeded" error.
		{
			name:    "BadTLS",
			url:     strings.ReplaceAll(ts.URL, "http://", "https://") + "/index",
			wantErr: "ERR_SSL_PROTOCOL_ERROR",
		},

		// In this case, the "loading failed" event is received, but the
		// load itself is cancelled immediately, so we never receive a
		// load event of any sort.
		{
			name:    "BadProtocol",
			url:     strings.ReplaceAll(ts.URL, "http://", "bad://") + "/index",
			wantErr: "ERR_ABORTED",
		},

		// Check that loading a non-HTML document still works normally.
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
		test := test
		allocate := func(t *testing.T) context.Context {
			ctx, cancel := testAllocate(t, "")
			t.Cleanup(cancel)
			ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
			t.Cleanup(cancel)

			if err := Run(ctx, Navigate(ts.URL+"/index")); err != nil {
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
				latency := time.Since(resp.ResponseTime.Time())
				if latency > time.Hour || latency < -time.Hour {
					t.Errorf("responseTime does not hold a reasonable value %s. "+
						"Maybe it's in seconds now and we should remove the workaround. "+
						"See https://github.com/chromedp/pdlgen/issues/22.",
						resp.ResponseTime.Time())
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
			resp, err := RunResponse(ctx, Click(query, ByQuery))
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

	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	steps := []struct {
		name     string
		action   Action
		wantResp bool
	}{
		{"FirstNavigation", Navigate(ts.URL + "/200"), true},
		{"RepeatedNavigation", Navigate(ts.URL + "/200"), true},
		{"FragmentNavigation", Navigate(ts.URL + "/200#foo"), false},

		{"FirstClick", Click("#same", ByQuery), true},
		{"RepeatedClick", Click("#same", ByQuery), true},
		{"FragmentClick", Click("#fragment", ByQuery), false},

		{"Blank", Navigate("about:blank"), false},
	}
	// Don't use sub-tests, as these are all sequential steps that can't
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
// This is a regress test for https://github.com/chromedp/chromedp/issues/1073.
func TestWebGL(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "webgl.html")
	defer cancel()

	var buf []byte
	if err := Run(ctx,
		Poll("rendered", nil, WithPollingTimeout(2*time.Second)),
		Screenshot(`#c`, &buf, ByQuery),
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
// When it's correctly loaded, the header/footer templates that use the
// following values should work as expected:
//   - title
//   - url
//   - pageNumber
//   - totalPages
//
// This is a regress test for https://github.com/chromedp/chromedp/issues/922.
func TestPDFTemplate(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	var buf []byte
	if err := Run(ctx,
		Navigate("about:blank"),
		ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}

			return page.SetDocumentContent(frameTree.Frame.ID, `
				<html>
					<head>
						<title>PDF Template</title>
					</head>
					<body>
						Hello World!
					</body>
				</html>
			`).Do(ctx)
		}),
		ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = page.PrintToPDF().
				WithMarginTop(0.5).
				WithMarginBottom(0.5).
				WithDisplayHeaderFooter(true).
				WithHeaderTemplate(`<div style="font-size:8px;width:100%;text-align:center;"><span class="title"></span> -- <span class="url"></span></div>`).
				WithFooterTemplate(`<div style="font-size:8px;width:100%;text-align:center;">(<span class="pageNumber"></span> / <span class="totalPages"></span>)</div>`).
				Do(ctx)

			return err
		}),
	); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.GetPlainText()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte("PDF Template -- about:blank" + "(1 / 1)" + "Hello World!")
	l := len(want)
	// try to reuse buf
	if len(buf) >= l {
		buf = buf[0:l]
	} else {
		buf = make([]byte, l)
	}
	n, err := io.ReadFull(b, buf)
	if err != nil && !(errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) {
		t.Fatal(err)
	}
	buf = buf[:n]

	if !bytes.Equal(buf, want) {
		t.Errorf("page.PrintToPDF produces unexpected content. got: %q, want: %q", buf, want)
	}
}

func contains(v []cdp.BrowserContextID, id cdp.BrowserContextID) bool {
	for _, i := range v {
		if i == id {
			return true
		}
	}
	return false
}
