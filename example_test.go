package chromedp_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
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
		log.Fatal(err)
	}
	fmt.Println(title)

	// Output:
	// fancy website title
}

func ExampleRunResponse() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// This server simply shows the URL path as the page title, and contains
	// a link that points to /foo.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<head><title>%s</title></head>
			<body><a id="foo" href="/foo">foo</a></body>
		`, r.URL.Path)
	}))
	defer ts.Close()

	// The Navigate action already waits until a page loads, so Title runs
	// once the page is ready.
	var firstTitle string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Title(&firstTitle),
	); err != nil {
		log.Fatal(err)
	}
	fmt.Println("first title:", firstTitle)

	// However, actions like Click don't always trigger a page navigation,
	// so they don't wait for a page load directly. Wrapping them with
	// RunResponse does that waiting, and also obtains the HTTP response.
	resp, err := chromedp.RunResponse(ctx, chromedp.Click("#foo", chromedp.ByID))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("second status code:", resp.Status)

	// Grabbing the title again should work, as the page has finished
	// loading once more.
	var secondTitle string
	if err := chromedp.Run(ctx, chromedp.Title(&secondTitle)); err != nil {
		log.Fatal(err)
	}
	fmt.Println("second title:", secondTitle)

	// Finally, it's always possible to wrap Navigate with RunResponse, if
	// one wants the response information for that case too.
	resp, err = chromedp.RunResponse(ctx, chromedp.Navigate(ts.URL+"/bar"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("third status code:", resp.Status)

	// Output:
	// first title: /
	// second status code: 200
	// second title: /foo
	// third status code: 200
}

func ExampleExecAllocator() {
	dir, err := os.MkdirTemp("", "chromedp-example")
	if err != nil {
		log.Fatal(err)
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

func ExampleNewContext_reuseBrowser() {
	ts := httptest.NewServer(writeHTML(`
<body>
<script>
	// Show the current cookies.
	var p = document.createElement("p")
	p.innerText = document.cookie
	p.setAttribute("id", "cookies")
	document.body.appendChild(p)

	// Override the cookies.
	document.cookie = "foo=bar"
</script>
</body>
	`))
	defer ts.Close()

	// create a new browser
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// start the browser without a timeout
	if err := chromedp.Run(ctx); err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		func() {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			ctx, cancel = chromedp.NewContext(ctx)
			defer cancel()
			var cookies string
			if err := chromedp.Run(ctx,
				chromedp.Navigate(ts.URL),
				chromedp.Text("#cookies", &cookies),
			); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Cookies at i=%d: %q\n", i, cookies)
		}()
	}

	// Output:
	// Cookies at i=0: ""
	// Cookies at i=1: "foo=bar"
}

func ExampleNewContext_manyTabs() {
	// new browser, first tab
	ctx1, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// ensure the first tab is created
	if err := chromedp.Run(ctx1); err != nil {
		log.Fatal(err)
	}

	// same browser, second tab
	ctx2, _ := chromedp.NewContext(ctx1)

	// ensure the second tab is created
	if err := chromedp.Run(ctx2); err != nil {
		log.Fatal(err)
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
	console.warn("scary warning", 123)
	null.throwsException
</script>
</body>
	`))
	defer ts.Close()

	gotException := make(chan bool, 1)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			fmt.Printf("* console.%s call:\n", ev.Type)
			for _, arg := range ev.Args {
				fmt.Printf("%s - %s\n", arg.Type, arg.Value)
			}
		case *runtime.EventExceptionThrown:
			// Since ts.URL uses a random port, replace it.
			s := ev.ExceptionDetails.Error()
			s = strings.ReplaceAll(s, ts.URL, "<server>")
			// V8 has changed the error messages for property access on null/undefined in version 9.3.310.
			// see: https://chromium.googlesource.com/v8/v8/+/c0fd89c3c089e888c4f4e8582e56db7066fa779b
			//      https://github.com/chromium/chromium/commit/1735cbf94c98c70ff7554a1e9e01bb9a4f91beb6
			// The message is normalized to make it compatible with the versions before this change.
			s = strings.ReplaceAll(s, "Cannot read property 'throwsException' of null", "Cannot read properties of null (reading 'throwsException')")
			fmt.Printf("* %s\n", s)
			gotException <- true
		}
	})

	if err := chromedp.Run(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	<-gotException

	// Output:
	// * console.log call:
	// string - "hello js world"
	// * console.warning call:
	// string - "scary warning"
	// number - 123
	// * exception "Uncaught" (4:6): TypeError: Cannot read properties of null (reading 'throwsException')
	//     at <server>/:5:7
}

func ExampleWaitNewTarget() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	mux := http.NewServeMux()
	mux.Handle("/first", writeHTML(`
<input id='newtab' type='button' value='open' onclick='window.open("/second", "_blank");'/>
	`))
	mux.Handle("/second", writeHTML(``))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Grab the first spawned tab that isn't blank.
	ch := chromedp.WaitNewTarget(ctx, func(info *target.Info) bool {
		return info.URL != ""
	})
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL+"/first"),
		chromedp.Click("#newtab", chromedp.ByID),
	); err != nil {
		log.Fatal(err)
	}
	newCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(<-ch))
	defer cancel()

	var urlstr string
	if err := chromedp.Run(newCtx, chromedp.Location(&urlstr)); err != nil {
		log.Fatal(err)
	}
	fmt.Println("new tab's path:", strings.TrimPrefix(urlstr, ts.URL))

	// Output:
	// new tab's path: /second
}

func ExampleListenTarget_acceptAlert() {
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
					log.Fatal(err)
				}
			}()
		}
	})

	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Click("#alert", chromedp.ByID),
	); err != nil {
		log.Fatal(err)
	}

	// Output:
	// closing alert: alert text
}

func Example_retrieveHTML() {
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

	var outerBefore, outerAfter string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.OuterHTML("#content", &outerBefore, chromedp.ByQuery),
		chromedp.Click("#content", chromedp.ByQuery),
		chromedp.OuterHTML("#content", &outerAfter, chromedp.ByQuery),
	); err != nil {
		log.Fatal(err)
	}
	fmt.Println("OuterHTML before clicking:")
	fmt.Println(outerBefore)
	fmt.Println("OuterHTML after clicking:")
	fmt.Println(outerAfter)

	// Output:
	// OuterHTML before clicking:
	// <p id="content" onclick="changeText()">Original content.</p>
	// OuterHTML after clicking:
	// <p id="content" onclick="changeText()">New content!</p>
}

func ExampleEmulate() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Emulate(device.IPhone7),
		chromedp.Navigate(`https://duckduckgo.com/`),
		chromedp.SendKeys(`input[name=q]`, "what's my user agent?\n"),
		chromedp.WaitVisible(`#zci-answer`, chromedp.ByID),
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("iphone7-ua.png", buf, 0o644); err != nil {
		log.Fatal(err)
	}

	// Output:
}

func ExamplePrintToPDF() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(`https://pkg.go.dev/github.com/chromedp/chromedp`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = page.PrintToPDF().
				WithDisplayHeaderFooter(false).
				WithLandscape(true).
				Do(ctx)
			return err
		}),
	); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("page.pdf", buf, 0o644); err != nil {
		log.Fatal(err)
	}

	// Output:
}

func ExampleByJSPath() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`
<body>
	<div id="content">cool content</div>
</body>
	`))
	defer ts.Close()

	var ids []cdp.NodeID
	var html string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.NodeIDs(`document`, &ids, chromedp.ByJSPath),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			html, err = dom.GetOuterHTML().WithNodeID(ids[0]).Do(ctx)
			return err
		}),
	); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Outer HTML:")
	fmt.Println(html)

	// Output:
	// Outer HTML:
	// <html><head></head><body>
	// 	<div id="content">cool content</div>
	// </body></html>
}

func ExampleFromNode() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`
<body>
	<p class="content">outer content</p>
	<div id="section"><p class="content">inner content</p></div>
</body>
	`))
	defer ts.Close()

	var nodes []*cdp.Node
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Nodes("#section", &nodes, chromedp.ByQuery),
	); err != nil {
		log.Fatal(err)
	}
	sectionNode := nodes[0]

	var queryRoot, queryFromNode, queryNestedSelector string
	if err := chromedp.Run(ctx,
		// Queries run from the document root by default, so Text will
		// pick the first node it finds.
		chromedp.Text(".content", &queryRoot, chromedp.ByQuery),

		// We can specify a different node to run the query from; in
		// this case, we can tailor the search within #section.
		chromedp.Text(".content", &queryFromNode, chromedp.ByQuery, chromedp.FromNode(sectionNode)),

		// A CSS selector like "#section > .content" achieves the same
		// here, but FromNode allows us to use a node obtained by an
		// entirely separate step, allowing for custom logic.
		chromedp.Text("#section > .content", &queryNestedSelector, chromedp.ByQuery),
	); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Simple query from the document root:", queryRoot)
	fmt.Println("Simple query from the section node:", queryFromNode)
	fmt.Println("Nested query from the document root:", queryNestedSelector)

	// Output:
	// Simple query from the document root: outer content
	// Simple query from the section node: inner content
	// Nested query from the document root: inner content
}

func Example_dump() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`<!doctype html>
<html>
<body>
  <div id="content" style="display:block;">the content</div>
</body>
</html>`))
	defer ts.Close()

	const expr = `(function(d, id, v) {
		var b = d.querySelector('body');
		var el = d.createElement('div');
		el.id = id;
		el.innerText = v;
		b.insertBefore(el, b.childNodes[0]);
	})(document, %q, %q);`

	s := fmt.Sprintf(expr, "thing", "a new thing!")

	var buf bytes.Buffer
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#content`),
		chromedp.Evaluate(s, nil),
		chromedp.WaitVisible(`#thing`),
		chromedp.Dump(`document`, &buf, chromedp.ByJSPath),
	); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Document tree:")
	fmt.Print(buf.String())

	// Output:
	// Document tree:
	// #document <Document>
	//   html <DocumentType>
	//   html
	//     head
	//     body
	//       div#thing
	//         #text "a new thing!"
	//       div#content [style="display:block;"]
	//         #text "the content"
}

func Example_documentDump() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`<!doctype html>
<html>
<body>
  <div id="content" style="display:block;">the content</div>
</body>
</html>`))
	defer ts.Close()

	const expr = `(function(d, id, v) {
		var b = d.querySelector('body');
		var el = d.createElement('div');
		el.id = id;
		el.innerText = v;
		b.insertBefore(el, b.childNodes[0]);
	})(document, %q, %q);`

	s := fmt.Sprintf(expr, "thing", "a new thing!")

	var nodes []*cdp.Node
	if err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Nodes(`document`, &nodes,
			chromedp.ByJSPath, chromedp.Populate(-1, true)),
		chromedp.WaitVisible(`#content`),
		chromedp.Evaluate(s, nil),
		chromedp.WaitVisible(`#thing`),
	); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Document tree:")
	fmt.Print(nodes[0].Dump("  ", "  ", false))

	// Output:
	// Document tree:
	//   #document <Document>
	//     html <DocumentType>
	//     html
	//       head
	//       body
	//         div#thing
	//           #text "a new thing!"
	//         div#content [style="display:block;"]
	//           #text "the content"
}

func ExampleFullScreenshot() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(`https://google.com`),
		chromedp.FullScreenshot(&buf, 90),
	); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("fullScreenshot.jpeg", buf, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote fullScreenshot.jpeg")
	// Output:
	// wrote fullScreenshot.jpeg
}

func ExampleEvaluate() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Ignore the result:
	{
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.scrollTo(0, 100)`, nil),
		); err != nil {
			log.Fatal(err)
		}
	}

	// Receive a primary value:
	{
		var sum int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`1+2`, &sum),
		); err != nil {
			log.Fatal(err)
		}
		fmt.Println(sum)
	}

	// ErrJSUndefined:
	{
		var val int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`undefined`, &val),
		); err != nil {
			fmt.Println(err)
		}
	}

	// ErrJSNull:
	{
		var val int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`null`, &val),
		); err != nil {
			fmt.Println(err)
		}
	}

	// Accept undefined/null result:
	{
		var val *int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`undefined`, &val),
		); err != nil {
			log.Fatal(err)
		}
		fmt.Println(val)
	}

	// Receive an array value:
	{
		var val []int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`[1,2]`, &val),
		); err != nil {
			log.Fatal(err)
		}
		fmt.Println(val)
	}

	// Map and Slice accept undefined/null:
	{
		var val []int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`null`, &val),
		); err != nil {
			log.Fatal(err)
		}
		fmt.Println("slice is nil:", val == nil)
	}

	// Receive the raw bytes:
	{
		var buf []byte
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`alert`, &buf),
		); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", buf)
	}

	// Receive the RemoteObject:
	{
		var res *runtime.RemoteObject
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`alert`, &res),
		); err != nil {
			log.Fatal(err)
		}
		if res.ObjectID != "" {
			fmt.Println("objectId is present")
		}
	}

	// Output:
	// 3
	// encountered an undefined value
	// encountered a null value
	// <nil>
	// [1 2]
	// slice is nil: true
	// {}
	// objectId is present
}
