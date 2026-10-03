package remote_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// This example makes the exec allocator connect with a websocket. The browser
// then opens a debugging port and writes the file DevToolsActivePort in its
// user data directory.
func ExampleWebSocket() {
	dir, err := os.MkdirTemp("", "chromedp-example")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		// The child processes of Chrome can still write in the directory for
		// a short time after the browser stops. Then RemoveAll fails with
		// "directory not empty", so try again.
		for range 50 {
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.UserDataDir(dir),
		// Chrome writes the DevToolsActivePort file only when it opens
		// a debugging port, so this example uses the websocket mode.
		remote.WebSocket,
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// also set up a custom logger
	taskCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	// make sure that the browser process is started
	if err := chromedp.Do(taskCtx); err != nil {
		log.Fatal(err)
	}

	path := filepath.Join(dir, "DevToolsActivePort")
	bs, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	lines := bytes.Split(bs, []byte("\n"))
	fmt.Printf("DevToolsActivePort has %d lines\n", len(lines))

	// Output:
	// DevToolsActivePort has 2 lines
}

// This example leaves the browser open when the program ends. The program prints
// the address, so that another program can attach with NewAllocator. The
// profile directory stays on disk, and the user must delete it.
func ExampleWithKeepOpen() {
	ctx, cancel := chromedp.NewContext(context.Background(),
		chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	defer cancel()

	if err := chromedp.Do(ctx, chromedp.Navigate("https://example.com")); err != nil {
		log.Fatal(err)
	}
	wsURL, profile := chromedp.KeptOpen(ctx)
	fmt.Println("attach to", wsURL)
	fmt.Println("profile directory", profile)
}

// This example attaches to a browser that an earlier program left open.
func ExampleNewAllocator() {
	wsURL := os.Args[1] // the address that the earlier program printed

	allocCtx, cancel := remote.NewAllocator(context.Background(), wsURL)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	title, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(title)
}

// This example connects to a hosted browser service that needs an
// Authorization header on the websocket request.
func ExampleWithDialHTTPHeader() {
	const wsURL = "wss://browser.example.com/devtools/browser/id"
	header := http.Header{"Authorization": {"Bearer " + os.Getenv("BROWSER_TOKEN")}}

	allocCtx, cancel := remote.NewAllocator(context.Background(), wsURL,
		remote.NoModifyURL,
		remote.WithDialHTTPHeader(header),
	)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	title, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(title)
}
