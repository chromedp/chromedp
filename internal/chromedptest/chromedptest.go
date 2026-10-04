// Package chromedptest holds the helpers that the tests of the chromedp
// modules share. The tests of the modules remote and test import it. It uses
// the exported API of chromedp only. The tests of the core package cannot
// import it, because it imports the core package. They import
// [github.com/chromedp/chromedp/internal/testenv] instead.
//
// A test binary calls [Main] from its TestMain. Then [Allocate] and
// [AllocateSeparate] make the contexts of the tests.
package chromedptest

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/internal/testenv"
)

var (
	// TestdataDir is the file URL of the directory testdata in the working
	// directory of the test. Main sets it.
	TestdataDir string

	// ExecPath is the browser binary of the tests. When the tests found no
	// browser, it is "google-chrome", and the allocator searches for one itself.
	// Main sets it.
	ExecPath string

	// AllocOpts are the options of the exec allocator of the tests. Main sets
	// them.
	AllocOpts []chromedp.ExecAllocatorOption

	// AllocCtx is the context of the exec allocator that Main makes.
	AllocCtx context.Context

	// BrowserOpts are the context options that every browser of the tests
	// gets. Main sets them from the environment.
	BrowserOpts []chromedp.ContextOption

	// browserCtx is the shared browser. Allocate makes it on first use.
	browserCtx context.Context

	allocateOnce sync.Once
)

// tempPrefix starts the name of the temporary user data directory of an exec
// allocator.
const tempPrefix = "chromedp-runner"

// Main runs the tests of a test binary. Call it from TestMain. It reads the
// environment variables of the tests, makes the exec allocator, runs the tests
// and exits. The exec allocator puts its temporary user data directories in a
// directory of its own, and Main panics when a directory stays there after the
// tests.
func Main(m *testing.M) {
	os.Exit(run(m))
}

// printBrowserVersion starts a browser, prints its version and closes it, so
// that the log of a test run on any system says which browser runs the tests.
func printBrowserVersion() {
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), AllocOpts...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()
	// The first Do starts the browser. Call the browser directly.
	if err := chromedp.Do(ctx); err != nil {
		fmt.Printf("browser: could not start the browser: %v\n", err)
		return
	}
	v, err := cdp.Call(ctx, chromedp.FromContext(ctx).Browser, browser.GetVersion, cdp.Empty{})
	if err != nil {
		fmt.Printf("browser: could not read the version: %v\n", err)
		return
	}
	fmt.Printf("browser: %s, revision %s, protocol %s, user agent %q, %s/%s\n",
		v.Product, v.Revision, v.ProtocolVersion, v.UserAgent, goruntime.GOOS, goruntime.GOARCH)
}

func run(m *testing.M) int {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("could not get working directory: %v", err))
	}
	TestdataDir = "file://" + path.Join(filepath.ToSlash(wd), "testdata")

	// The allocator makes its directories with os.MkdirTemp, and that uses the
	// temporary directory of the system. Point it to a directory that only
	// this test binary uses, so that the check below finds a leak.
	tempDir, err := os.MkdirTemp("", "chromedp-test")
	if err != nil {
		panic(fmt.Sprintf("could not create temp directory: %v", err))
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		os.Setenv(name, tempDir)
	}

	// Disabling the GPU helps portability with some systems like Travis,
	// and can slightly speed up the tests on other systems.
	AllocOpts = append(chromedp.DefaultExecAllocatorOptions[:len(chromedp.DefaultExecAllocatorOptions):len(chromedp.DefaultExecAllocatorOptions)], chromedp.DisableGPU)
	if testenv.VisibleWindow() {
		AllocOpts = append(AllocOpts, chromedp.VisibleWindow)
	}
	ExecPath = testenv.ExecPath()
	if ExecPath != "" {
		AllocOpts = append(AllocOpts, chromedp.ExecPath(ExecPath))
	} else {
		// The allocator searches for a browser itself. A test that starts the
		// browser without the allocator needs a name.
		ExecPath = "google-chrome"
	}
	// Not explicitly needed to be set, as this speeds up the tests.
	if testenv.NoSandbox() {
		AllocOpts = append(AllocOpts, chromedp.NoSandbox)
	}
	if testenv.Debug() {
		BrowserOpts = append(BrowserOpts, chromedp.WithDebugf(log.Printf))
	}

	var cancel context.CancelFunc
	AllocCtx, cancel = chromedp.NewExecAllocator(context.Background(), AllocOpts...)

	printBrowserVersion()

	code := m.Run()
	cancel()

	// The allocator removes the user data directory of a browser in a goroutine,
	// after the browser process exits. Give the goroutines of a slow machine
	// time to finish before the check.
	waitNoRunnerDirs(tempDir, 15*time.Second)

	var leaks []string
	if entries, _ := os.ReadDir(tempDir); len(entries) > 0 {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), tempPrefix) {
				leaks = append(leaks, describeLeak(filepath.Join(tempDir, entry.Name())))
			}
		}
	}
	os.RemoveAll(tempDir)
	if len(leaks) > 0 {
		panic(fmt.Sprintf("leaked %d temporary dirs under %s:\n%s",
			len(leaks), tempDir, strings.Join(leaks, "\n")))
	}
	return code
}

// waitNoRunnerDirs waits until dir has no entry that the allocator made, or
// until the timeout.
func waitNoRunnerDirs(dir string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		found := false
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), tempPrefix) {
				found = true
			}
		}
		if !found {
			return
		}
		time.Sleep(50 * time.Millisecond)
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

// Into makes an action that runs a and stores its value in dst. The tests use
// it to run several actions in one call of chromedp.Do and still read their
// values.
func Into[T any](dst *T, a chromedp.Action[T]) chromedp.Action[chromedp.Void] {
	return func(ctx context.Context, t *chromedp.Target) (chromedp.Void, error) {
		v, err := a(ctx, t)
		if err != nil {
			return chromedp.Void{}, err
		}
		*dst = v
		return chromedp.Void{}, nil
	}
}

// Allocate returns the context of a new tab in the browser that all tests
// share, and a func that closes the tab. The first call starts the browser. If
// name is not empty, the tab opens the file name of the directory testdata.
func Allocate(tb testing.TB, name string) (context.Context, context.CancelFunc) {
	tb.Helper()
	// Start the browser exactly once, as needed.
	allocateOnce.Do(func() { browserCtx, _ = AllocateSeparate(tb) })

	if browserCtx == nil {
		// allocateOnce.Do failed. If we continue, the test panics.
		tb.FailNow()
	}

	// Same browser, new tab. We do not need to start a new chrome browser for
	// each test, and this gives a huge speed-up.
	ctx, _ := chromedp.NewContext(browserCtx)

	// Navigate only if we want an HTML file name. Otherwise leave the blank page.
	if name != "" {
		if err := chromedp.Do(ctx, chromedp.Navigate(TestdataDir+"/"+name)); err != nil {
			tb.Fatal(err)
		}
	}

	cancel := func() {
		if err := chromedp.Cancel(ctx); err != nil {
			tb.Error(err)
		}
	}
	return ctx, cancel
}

// BrowserContext returns the context of the shared browser that Allocate
// starts. It is nil before the first call of Allocate.
func BrowserContext() context.Context {
	return browserCtx
}

// AllocateSeparate returns the context of an entirely new browser, and a func
// that closes it. Unlike Allocate, it shares nothing with the other tests.
func AllocateSeparate(tb testing.TB) (context.Context, context.CancelFunc) {
	tb.Helper()
	ctx, _ := chromedp.NewContext(AllocCtx, BrowserOpts...)
	if err := chromedp.Do(ctx); err != nil {
		tb.Fatal(err)
	}
	go func() {
		for ev, err := range chromedp.Events(ctx, runtime.ExceptionThrown) {
			if err != nil {
				return
			}
			tb.Errorf("%+v\n", ev.ExceptionDetails)
		}
	}()
	cancel := func() {
		if err := chromedp.Cancel(ctx); err != nil {
			tb.Error(err)
		}
	}
	return ctx, cancel
}
