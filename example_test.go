package chromedp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
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

	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	title, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
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
	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	firstTitle, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
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
	secondTitle, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("second title:", secondTitle)

	// It's always possible to wrap Navigate with RunResponse, if one wants
	// the response information for that case too.
	resp, err = chromedp.RunResponse(ctx, chromedp.Navigate(ts.URL+"/bar"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("third status code:", resp.Status)

	// NavigateResponse is a shorter way to do the same.
	resp, err = chromedp.Run(ctx, chromedp.NavigateResponse(ts.URL+"/baz"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("fourth status code:", resp.Status)

	// Output:
	// first title: /
	// second status code: 200
	// second title: /foo
	// third status code: 200
	// fourth status code: 200
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
		// Chrome writes the DevToolsActivePort file only when it opens
		// a debugging port, so this example uses the websocket mode.
		chromedp.WebSocket,
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// also set up a custom logger
	taskCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	// ensure that the browser process is started
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
	if err := chromedp.Do(ctx); err != nil {
		log.Fatal(err)
	}

	for i := range 2 {
		func() {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			ctx, cancel = chromedp.NewContext(ctx)
			defer cancel()
			if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
				log.Fatal(err)
			}
			cookies, err := chromedp.Run(ctx, chromedp.Text("#cookies"))
			if err != nil {
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
	if err := chromedp.Do(ctx1); err != nil {
		log.Fatal(err)
	}

	// same browser, second tab
	ctx2, _ := chromedp.NewContext(ctx1)

	// ensure the second tab is created
	if err := chromedp.Do(ctx2); err != nil {
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

func ExampleEvents_consoleLog() {
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

	// Subscribe first. The subscriptions buffer the events, so the loops
	// below see the events of the navigation although they start later.
	console := chromedp.Events(ctx, runtime.ConsoleAPICalled)
	exceptions := chromedp.Events(ctx, runtime.ExceptionThrown)

	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}

	calls := 0
	for ev, err := range console {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("* console.%s call:\n", ev.Type)
		for _, arg := range ev.Args {
			fmt.Printf("%s - %s\n", arg.Type, arg.Value)
		}
		if calls++; calls == 2 {
			break
		}
	}
	for ev, err := range exceptions {
		if err != nil {
			log.Fatal(err)
		}
		// Since ts.URL uses a random port, replace it.
		s := (&chromedp.ExceptionError{ExceptionDetails: ev.ExceptionDetails}).Error()
		s = strings.ReplaceAll(s, ts.URL, "<server>")
		// V8 has changed the error messages for property access on null/undefined in version 9.3.310.
		// see: https://chromium.googlesource.com/v8/v8/+/c0fd89c3c089e888c4f4e8582e56db7066fa779b
		//      https://github.com/chromium/chromium/commit/1735cbf94c98c70ff7554a1e9e01bb9a4f91beb6
		// The message is normalized to make it compatible with the versions before this change.
		s = strings.ReplaceAll(s, "Cannot read property 'throwsException' of null", "Cannot read properties of null (reading 'throwsException')")
		fmt.Printf("* %s\n", s)
		break
	}

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
	if err := chromedp.Do(ctx,
		chromedp.Navigate(ts.URL+"/first"),
		chromedp.Click("#newtab", chromedp.ByID),
	); err != nil {
		log.Fatal(err)
	}
	newCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(<-ch))
	defer cancel()

	urlstr, err := chromedp.Run(newCtx, chromedp.Location())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("new tab's path:", strings.TrimPrefix(urlstr, ts.URL))

	// Output:
	// new tab's path: /second
}

func ExampleEvents_acceptAlert() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`
<input id='alert' type='button' value='alert' onclick='alert("alert text");'/>
	`))
	defer ts.Close()

	// The page waits for the dialog, and so does the Click below. A second
	// goroutine closes the dialog.
	dialogs := chromedp.Events(ctx, page.JavascriptDialogOpening)
	go func() {
		for ev, err := range dialogs {
			if err != nil {
				return
			}
			fmt.Println("closing alert:", ev.Message)
			_, err := chromedp.Call(ctx, page.HandleJavaScriptDialog, page.HandleJavaScriptDialogParams{Accept: true})
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	if err := chromedp.Do(ctx,
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

	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	outerBefore, err := chromedp.Run(ctx, chromedp.OuterHTML("#content", chromedp.ByQuery))
	if err != nil {
		log.Fatal(err)
	}
	if err := chromedp.Do(ctx, chromedp.Click("#content", chromedp.ByQuery)); err != nil {
		log.Fatal(err)
	}
	outerAfter, err := chromedp.Run(ctx, chromedp.OuterHTML("#content", chromedp.ByQuery))
	if err != nil {
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

	if err := chromedp.Do(ctx,
		chromedp.Emulate(device.IPhone7),
		chromedp.Navigate(`https://duckduckgo.com/`),
		chromedp.SendKeys(`textarea[name=q]`, "what's my user agent?\n"),
		chromedp.WaitVisible(`#zci-answer`, chromedp.ByID),
	); err != nil {
		log.Fatal(err)
	}
	buf, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("iphone7-ua.png", buf, 0o644); err != nil {
		log.Fatal(err)
	}

	// This example uses a live website and writes a file, so it has no
	// Output comment: go test builds it and does not run it. A live website
	// can change or be slow, and the example made the tests fail on CI.
}

func ExamplePrintToPDF() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// A custom action sends the protocol command with cdp.Call. It returns
	// the bytes of the PDF file.
	printToPDF := func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		res, err := cdp.Call(ctx, t, page.PrintToPDF, page.PrintToPDFParams{
			DisplayHeaderFooter: new(false),
			Landscape:           new(true),
		})
		return res.Data, err
	}

	if err := chromedp.Do(ctx, chromedp.Navigate(`https://pkg.go.dev/github.com/chromedp/chromedp`)); err != nil {
		log.Fatal(err)
	}
	buf, err := chromedp.Run(ctx, printToPDF)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("page.pdf", buf, 0o644); err != nil {
		log.Fatal(err)
	}

	// This example uses a live website and writes a file, so it has no
	// Output comment: go test builds it and does not run it. A live website
	// can change or be slow, and the example made the tests fail on CI.
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

	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	ids, err := chromedp.Run(ctx, chromedp.NodeIDs(`document`, chromedp.ByJSPath))
	if err != nil {
		log.Fatal(err)
	}
	res, err := chromedp.Call(ctx, dom.GetOuterHTML, dom.GetOuterHTMLParams{NodeID: ids[0]})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Outer HTML:")
	fmt.Println(res.OuterHTML)

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

	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	nodes, err := chromedp.Run(ctx, chromedp.Nodes("#section", chromedp.ByQuery))
	if err != nil {
		log.Fatal(err)
	}
	sectionNode := nodes[0]

	// Queries run from the document root by default, so Text will pick the
	// first node it finds.
	queryRoot, err := chromedp.Run(ctx, chromedp.Text(".content", chromedp.ByQuery))
	if err != nil {
		log.Fatal(err)
	}

	// We can specify a different node to run the query from; in this case,
	// we can tailor the search within #section.
	queryFromNode, err := chromedp.Run(ctx, chromedp.Text(".content", chromedp.ByQuery, chromedp.FromNode(sectionNode)))
	if err != nil {
		log.Fatal(err)
	}

	// A CSS selector like "#section > .content" achieves the same here, but
	// FromNode allows us to use a node obtained by an entirely separate
	// step, allowing for custom logic.
	queryNestedSelector, err := chromedp.Run(ctx, chromedp.Text("#section > .content", chromedp.ByQuery))
	if err != nil {
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
	if err := chromedp.Do(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#content`),
		chromedp.Evaluate[chromedp.Void](s),
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

	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	nodes, err := chromedp.Run(ctx, chromedp.Nodes(`document`,
		chromedp.ByJSPath, chromedp.Populate(-1, true)))
	if err != nil {
		log.Fatal(err)
	}
	if err := chromedp.Do(ctx,
		chromedp.WaitVisible(`#content`),
		chromedp.Evaluate[chromedp.Void](s),
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

	if err := chromedp.Do(ctx, chromedp.Navigate(`https://google.com`)); err != nil {
		log.Fatal(err)
	}
	buf, err := chromedp.Run(ctx, chromedp.FullScreenshot(90))
	if err != nil {
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
		if err := chromedp.Do(ctx, chromedp.Evaluate[chromedp.Void](`window.scrollTo(0, 100)`)); err != nil {
			log.Fatal(err)
		}
	}

	// Receive a primary value:
	{
		sum, err := chromedp.Run(ctx, chromedp.Evaluate[int](`1+2`))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(sum)
	}

	// ErrJSUndefined:
	{
		if _, err := chromedp.Run(ctx, chromedp.Evaluate[int](`undefined`)); err != nil {
			fmt.Println(err)
		}
	}

	// ErrJSNull:
	{
		if _, err := chromedp.Run(ctx, chromedp.Evaluate[int](`null`)); err != nil {
			fmt.Println(err)
		}
	}

	// Accept undefined/null result:
	{
		val, err := chromedp.Run(ctx, chromedp.Evaluate[*int](`undefined`))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(val)
	}

	// Receive an array value:
	{
		val, err := chromedp.Run(ctx, chromedp.Evaluate[[]int](`[1,2]`))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(val)
	}

	// Map and Slice accept undefined/null:
	{
		val, err := chromedp.Run(ctx, chromedp.Evaluate[[]int](`null`))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("slice is nil:", val == nil)
	}

	// Receive the raw bytes:
	{
		buf, err := chromedp.Run(ctx, chromedp.Evaluate[[]byte](`alert`))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", buf)
	}

	// Receive the RemoteObject:
	{
		res, err := chromedp.Run(ctx, chromedp.Evaluate[*runtime.RemoteObject](`alert`))
		if err != nil {
			log.Fatal(err)
		}
		if res.ObjectID != "" {
			fmt.Println("objectId is present")
		}
	}

	// Output:
	// 3
	// encountered an undefined value
	// <nil>
	// [1 2]
	// slice is nil: true
	// {}
	// objectId is present
}

func ExampleClick() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<head><title>%s</title></head><body><a id="next" href="/next">next</a></body>`, r.URL.Path)
	}))
	defer ts.Close()

	// Click does not wait for a page load, so WaitEvent subscribes to the
	// load event first, then runs the click, and returns the first event
	// that comes after it. No event is lost between the click and the wait.
	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	_, err := chromedp.Run(ctx, chromedp.WaitEvent(page.LoadEventFired, nil,
		chromedp.Click("#next", chromedp.ByID)))
	if err != nil {
		log.Fatal(err)
	}
	title, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("title after the click:", title)

	// Output:
	// title after the click: /next
}

func ExampleSendKeys() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`
<body>
<form id="form"><input id="name" name="name"/><input id="color" name="color"/></form>
</body>
	`))
	defer ts.Close()

	if err := chromedp.Do(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.SendKeys("#name", "Ada", chromedp.ByID),
		chromedp.SetValue("#color", "green", chromedp.ByID),
	); err != nil {
		log.Fatal(err)
	}
	name, err := chromedp.Run(ctx, chromedp.Value("#name", chromedp.ByID))
	if err != nil {
		log.Fatal(err)
	}
	color, err := chromedp.Run(ctx, chromedp.Value("#color", chromedp.ByID))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(name, color)

	// Output:
	// Ada green
}

func ExampleEvents_network() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// The page has an image that does not exist.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeHTML(`<body><img src="/missing.png"/></body>`).ServeHTTP(w, r)
	}))
	defer ts.Close()

	// Subscribe before the navigation. The loop below can start after it.
	responses := chromedp.Events(ctx, network.ResponseReceived)

	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	for ev, err := range responses {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(strings.TrimPrefix(ev.Response.URL, ts.URL), ev.Response.Status)
		if ev.Type == network.ResourceTypeImage {
			break
		}
	}

	// Output:
	// / 200
	// /missing.png 404
}

func ExampleCall() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`<title>called</title>`))
	defer ts.Close()

	// Any protocol command runs on the target of an action. The target is
	// a cdp.Session, so cdp.Call takes it as is.
	frameTree := func(ctx context.Context, t *chromedp.Target) (*page.FrameTree, error) {
		res, err := cdp.Call(ctx, t, page.GetFrameTree, cdp.Empty{})
		return res.FrameTree, err
	}

	if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
		log.Fatal(err)
	}
	tree, err := chromedp.Run(ctx, frameTree)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(strings.TrimPrefix(tree.Frame.URL, ts.URL) == "/")

	// Outside an action, use chromedp.Call with the context.
	res, err := chromedp.Call(ctx, page.GetFrameTree, cdp.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.FrameTree.Frame.ID == tree.Frame.ID)

	// Output:
	// true
	// true
}

// oldTitle is an action of the previous version of chromedp. It has a Do
// method.
type oldTitle struct{ res *string }

func (a oldTitle) Do(ctx context.Context) error {
	res, err := chromedp.Call(ctx, runtime.Evaluate, runtime.EvaluateParams{
		Expression:    "document.title",
		ReturnByValue: new(true),
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(res.Result.Value, a.res)
}

func ExampleLegacy() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ts := httptest.NewServer(writeHTML(`<title>old action</title>`))
	defer ts.Close()

	var title string
	if err := chromedp.Do(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Legacy(oldTitle{&title}),
	); err != nil {
		log.Fatal(err)
	}
	fmt.Println(title)

	// Output:
	// old action
}
