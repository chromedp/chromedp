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

	allocCtx context.Context

	allocOpts = []ExecAllocatorOption{
		NoFirstRun,
		NoDefaultBrowserCheck,
		Headless,
		DisableGPU,
	}
)

func testAllocate(t *testing.T, path string) (_ context.Context, cancel func()) {
	ctx, cancel := NewContext(allocCtx)

	// Only navigate if we want a path, otherwise leave the blank page.
	if path != "" {
		if err := Run(ctx, Navigate(testdataDir+"/"+path)); err != nil {
			t.Fatal(err)
		}
	}

	//if err := WithLogf(t.Logf)(c.c); err != nil {
	//        t.Fatalf("could not set logf: %v", err)
	//}
	//if err := WithDebugf(t.Logf)(c.c); err != nil {
	//        t.Fatalf("could not set debugf: %v", err)
	//}
	//if err := WithErrorf(t.Errorf)(c.c); err != nil {
	//        t.Fatalf("could not set errorf: %v", err)
	//}

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

	ctx, cancel := NewAllocator(context.Background(), WithExecAllocator(allocOpts...))
	allocCtx = ctx

	code := m.Run()

	cancel()
	FromContext(ctx).Allocator.Wait()
	os.Exit(code)
}
