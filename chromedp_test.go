package chromedp

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
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

	allocTempDir, err = ioutil.TempDir("", "chromedp-test")
	if err != nil {
		panic(fmt.Sprintf("could not create temp directory: %v", err))
	}

	// Disabling the GPU helps portability with some systems like Travis,
	// and can slightly speed up the tests on other systems.
	allocOpts = append(allocOpts, DisableGPU)

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

	if infos, _ := ioutil.ReadDir(allocTempDir); len(infos) > 0 {
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

func testAllocateSeparate(tb testing.TB) (context.Context, context.CancelFunc) {
	// Entirely new browser, unlike testAllocate.
	ctx, _ := NewContext(allocCtx, browserOpts...)
	if err := Run(ctx); err != nil {
		tb.Fatal(err)
	}
	ListenBrowser(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *cdpruntime.EventExceptionThrown:
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

	// 50 is enough for 'go test -race' to easily spot issues.
	for i := 0; i < 50; i++ {
		ctx, cancel := NewContext(allocCtx)
		go cancel()
		go Run(ctx)
	}
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
		if _, ok := ev.(*cdpruntime.EventConsoleAPICalled); ok && first {
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

func TestLargeOutboundMessages(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// ~50KiB of JS should fit just fine in our current buffer of 1MiB.
	expr := fmt.Sprintf("//%s\n", strings.Repeat("x", 50<<10))
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
			_, err := target.CloseTarget(c.Target.TargetID).Do(ctx)
			if err != nil {
				return err
			}
			return nil
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

	dir, err := ioutil.TempDir("", "chromedp-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

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

	if err := Run(ctx,
		Navigate(s.URL),
		page.SetDownloadBehavior(page.SetDownloadBehaviorBehaviorAllow).WithDownloadPath(dir),
		Click("#download", ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	// TODO: wait for the download to finish, and check that the file is in
	// the directory.
}

func TestGracefulBrowserShutdown(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "chromedp-test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

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
