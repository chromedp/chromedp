package chromedp_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/chromedp/chromedp"
)

func ExampleTitle() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var title string
	if err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate("https://github.com/chromedp/chromedp/issues"),
		chromedp.WaitVisible("#start-of-content", chromedp.ByID),
		chromedp.Title(&title),
	}); err != nil {
		panic(err)
	}

	fmt.Println(title)

	// wait for the resources to be cleaned up
	cancel()
	chromedp.FromContext(ctx).Wait()

	// Output:
	// Issues · chromedp/chromedp · GitHub
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
	if err := chromedp.Run(taskCtx, chromedp.Tasks{}); err != nil {
		panic(err)
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	fmt.Println("Files in UserDataDir:")
	for _, file := range files {
		fmt.Println(file.Name())
	}

	// wait for the resources to be cleaned up
	cancel()
	chromedp.FromContext(allocCtx).Wait()

	// Output:
	// Files in UserDataDir:
	// Default
	// DevToolsActivePort
}
