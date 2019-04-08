package chromedp

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
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
