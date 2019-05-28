package chromedp_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func writeHTML(content string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, strings.TrimSpace(content))
	})
}

func ExampleTitle() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`
<head>
	<title>fancy website title</title>
</head>
<body>
	<div id="content"></div>
</body>
	`))
	defer ts.Close()

	var title string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Title(&title),
	); err != nil {
		panic(err)
	}
	fmt.Println(title)

	// Output:
	// fancy website title
}

func ExampleExecAllocator() {
	dir, err := ioutil.TempDir("", "chromedp-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.UserDataDir(dir),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// also set up a custom logger
	taskCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
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

	// Output:
	// DevToolsActivePort has 2 lines
}

func ExampleNewContext_manyTabs() {
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

	// Output:
	// Same browser: true
	// Same tab: false
}

func ExampleListenTarget_consoleLog() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`
<body>
<script>
	console.log("hello js world")
	var p = document.createElement("div");
	p.setAttribute("id", "jsFinished");
	document.body.appendChild(p);
</script>
</body>
	`))
	defer ts.Close()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *cdpruntime.EventConsoleAPICalled:
			fmt.Printf("console.%s call:\n", ev.Type)
			for _, arg := range ev.Args {
				fmt.Printf("%s - %s\n", arg.Type, arg.Value)
			}
		}
	})

	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible("#jsFinished", chromedp.ByID),
	); err != nil {
		panic(err)
	}

	// Output:
	// console.log call:
	// string - "hello js world"
}

func ExampleClickNewTab() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	mux := http.NewServeMux()
	mux.Handle("/first", writeHTML(`
<input id='newtab' type='button' value='open' onclick='window.open("/second", "_blank");'/>
	`))
	mux.Handle("/second", writeHTML(``))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	lctx, cancel := context.WithCancel(ctx)
	ch := make(chan target.ID, 1)
	chromedp.ListenTarget(lctx, func(ev interface{}) {
		if ev, ok := ev.(*target.EventTargetCreated); ok &&
			// if OpenerID == "", this is the first tab.
			ev.TargetInfo.OpenerID != "" {
			ch <- ev.TargetInfo.TargetID
			cancel()
		}
	})
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL+"/first"),
		chromedp.Click("#newtab", chromedp.ByID),
	); err != nil {
		panic(err)
	}
	newCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(<-ch))
	defer cancel()

	var urlstr string
	if err := chromedp.Run(newCtx, chromedp.Location(&urlstr)); err != nil {
		panic(err)
	}
	fmt.Println("new tab's path:", strings.TrimPrefix(urlstr, ts.URL))

	// Output:
	// new tab's path: /second
}

func ExampleAcceptAlert() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	mux := http.NewServeMux()
	mux.Handle("/second", writeHTML(``))
	ts := httptest.NewServer(writeHTML(`
<input id='alert' type='button' value='alert' onclick='alert("alert text");'/>
	`))
	defer ts.Close()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			fmt.Println("closing alert:", ev.Message)
			go func() {
				if err := chromedp.Run(ctx,
					page.HandleJavaScriptDialog(true),
				); err != nil {
					panic(err)
				}
			}()
		}
	})

	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Click("#alert", chromedp.ByID),
	); err != nil {
		panic(err)
	}

	// Output:
	// closing alert: alert text
}

func ExampleRetrieveHTML() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`
<body>
<p id="content" onclick="changeText()">Original content.</p>
<script>
function changeText() {
	document.getElementById("content").textContent = "New content!"
}
</script>
</body>
	`))
	defer ts.Close()

	respBody := make(chan string)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		ev2, ok := ev.(*network.EventResponseReceived)
		if !ok {
			return
		}
		go func() {
			if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				body, err := network.GetResponseBody(ev2.RequestID).Do(ctx)
				respBody <- string(body)
				return err
			})); err != nil {
				panic(err)
			}
		}()
	})

	var outerBefore, outerAfter string
	if err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate(ts.URL),
		chromedp.OuterHTML("#content", &outerBefore),
		chromedp.Click("#content", chromedp.ByID),
		chromedp.OuterHTML("#content", &outerAfter),
	); err != nil {
		panic(err)
	}
	fmt.Println("Original response body:")
	fmt.Println(<-respBody)
	fmt.Println()
	fmt.Println("OuterHTML before clicking:")
	fmt.Println(outerBefore)
	fmt.Println("OuterHTML after clicking:")
	fmt.Println(outerAfter)

	// Output:
	// Original response body:
	// <body>
	// <p id="content" onclick="changeText()">Original content.</p>
	// <script>
	// function changeText() {
	// 	document.getElementById("content").textContent = "New content!"
	// }
	// </script>
	// </body>
	//
	// OuterHTML before clicking:
	// <p id="content" onclick="changeText()">Original content.</p>
	// OuterHTML after clicking:
	// <p id="content" onclick="changeText()">New content!</p>
}
