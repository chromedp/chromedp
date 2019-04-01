package chromedp_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/chromedp/chromedp"
)

func ExampleTitle() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var title string
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://github.com/chromedp/chromedp/issues"),
		chromedp.WaitVisible("#start-of-content", chromedp.ByID),
		chromedp.Title(&title),
	); err != nil {
		panic(err)
	}

	fmt.Println(title)

	// wait for the resources to be cleaned up
	cancel()
	chromedp.FromContext(ctx).Allocator.Wait()

	// no expected output, to not run this test as part of 'go test'; it's
	// too slow, requiring internet access.
}

func ExampleExecAllocatorOption() {
	dir, err := ioutil.TempDir("", "chromedp-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.UserDataDir(dir),
	}

	allocCtx, cancel := chromedp.NewAllocator(context.Background(),
		chromedp.WithExecAllocator(opts...))
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// ensure that the browser process is started
	if err := chromedp.Run(taskCtx); err != nil {
		panic(err)
	}

	path := filepath.Join(dir, "DevToolsActivePort")
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	lines := bytes.Split(bs, []byte("\n"))
	fmt.Printf("DevToolsActivePort has %d lines\n", len(lines))

	// wait for the resources to be cleaned up
	cancel()
	chromedp.FromContext(allocCtx).Allocator.Wait()

	// Output:
	// DevToolsActivePort has 2 lines
}

func ExampleManyTabs() {
	// new browser, first tab
	ctx1, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// ensure the first tab is created
	if err := chromedp.Run(ctx1); err != nil {
		panic(err)
	}

	// same browser, second tab
	ctx2, _ := chromedp.NewContext(ctx1)

	// ensure the second tab is created
	if err := chromedp.Run(ctx2); err != nil {
		panic(err)
	}

	c1 := chromedp.FromContext(ctx1)
	c2 := chromedp.FromContext(ctx2)

	fmt.Printf("Same browser: %t\n", c1.Browser == c2.Browser)
	fmt.Printf("Same tab: %t\n", c1.Target == c2.Target)

	// wait for the resources to be cleaned up
	cancel()
	c1.Allocator.Wait()

	// Output:
	// Same browser: true
	// Same tab: false
}
