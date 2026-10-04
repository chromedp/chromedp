# The chromedp API

This document describes the generic action API of `chromedp`. It shows the new code next to the old code. The old code is the API of `chromedp` v0.16.0. The new code is the API of v0.17.0. See `docs/decisions/2026-10-03-generic-iterator-api-instead-of-action.md`.

## Principles

1. An action returns its value. The old API filled a pointer that the caller declared first. The new action has the type `Action[T]` and returns a `T`. An action that returns nothing has the type `Action[Void]`.
2. One type describes every step. An action is a plain func, `func(ctx context.Context, t *Target) (T, error)`. A program can write its own action with a func literal. No interface is needed.
3. The target is a session. The `Target` and the `Browser` implement `cdp.Session`. An action sends a protocol command with `cdp.Call(ctx, t, command, params)`. The target is an argument, so it cannot be missing. The old API hid an executor in the context, and a call without it failed at run time.
4. A protocol command is a value. A command has the type `cdp.Command[P, R]`, where `P` is the struct of the parameters and `R` is the struct of the result. A new field in the protocol adds a field to a struct. It does not change a function signature.
5. An event is a value, and an event stream is an iterator. `Events(ctx, page.LoadEventFired)` returns an `iter.Seq2[E, error]` of typed payloads. The subscription starts when `Events` returns. A program can subscribe, trigger the event and then range over the events, so no event is lost.
6. The type of a selector chooses the lookup. A query action takes a `Selectable` value, which is a string or a `[]cdp.NodeID`. A plain string is a `Search`, as before. `CSS`, `CSSAll`, `ID`, `JSPath` and `NodeIDs` select by CSS, by CSS for every match, by id, by JavaScript and by node ids. The `By...` lookup options are gone. An int or a `*Node` does not compile. The options `NodeVisible`, `FromNode`, `ByFunc` and the others keep their names and their use.
7. Old code still runs. `Legacy` wraps an action of the old interface, so a program can move to the new API one step at a time.

Two funcs run actions. `Run` runs one action and returns its value. `Do` runs several actions that return no value, in order, and stops at the first error. Both start the browser and open the tab when the context has none yet.

```go
if err := chromedp.Do(ctx, chromedp.Navigate(url)); err != nil {
	return err
}
title, err := chromedp.Run(ctx, chromedp.Title())
```

## Before and after

Each example has the old code first and the new code second. The new code comes from `example_test.go`. The examples 9, 11 and 14 use a live website and have no `Output` comment, so `go test` builds them and does not run them. Example 15 is new, so it has no Before block, and it has no `Output` comment. Example 7 also uses a live website, but it has an `Output` comment, so `go test` runs it. In every example, `ctx` is a chromedp context made with `chromedp.NewContext`, and `ts` is a test server. The old code of some examples chose a lookup with an option. The Before blocks leave it out, because the default lookup finds the same elements. `docs/MIGRATION.md` lists the old options and the selector types that replace them.

### 1. Navigate and read a value

The old code declared `title` first and passed its address. The new code gets the title from `Run`.

Before:

```go
var title string
if err := chromedp.Run(ctx,
	chromedp.Navigate(ts.URL),
	chromedp.Title(&title),
); err != nil {
	log.Fatal(err)
}
fmt.Println(title)
```

After:

```go
if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
	log.Fatal(err)
}
title, err := chromedp.Run(ctx, chromedp.Title())
if err != nil {
	log.Fatal(err)
}
fmt.Println(title)
```

### 2. Read the HTML of an element, click and read again

The new code has one `Run` for each value. Steps with no value go in `Do`.

Before:

```go
var outerBefore, outerAfter string
if err := chromedp.Run(ctx,
	chromedp.Navigate(ts.URL),
	chromedp.OuterHTML("#content", &outerBefore),
	chromedp.Click("#content"),
	chromedp.OuterHTML("#content", &outerAfter),
); err != nil {
	log.Fatal(err)
}
```

After:

```go
if err := chromedp.Do(ctx, chromedp.Navigate(ts.URL)); err != nil {
	log.Fatal(err)
}
outerBefore, err := chromedp.Run(ctx, chromedp.OuterHTML(chromedp.CSS("#content")))
if err != nil {
	log.Fatal(err)
}
if err := chromedp.Do(ctx, chromedp.Click(chromedp.CSS("#content"))); err != nil {
	log.Fatal(err)
}
outerAfter, err := chromedp.Run(ctx, chromedp.OuterHTML(chromedp.CSS("#content")))
if err != nil {
	log.Fatal(err)
}
```

### 3. Click a link and wait for the new page

A click does not always cause a navigation, so the old `Run` did not wait for a load. `RunResponse` waited and returned the HTTP response. It still does. `WaitEvent` is the new way to wait for any event. It subscribes to the event, runs the click, and returns the first payload that comes after it.

Before:

```go
resp, err := chromedp.RunResponse(ctx, chromedp.Click("#foo"))
if err != nil {
	log.Fatal(err)
}
fmt.Println("status code:", resp.Status)
```

After:

```go
resp, err := chromedp.RunResponse(ctx, chromedp.Click(chromedp.ID("foo")))
if err != nil {
	log.Fatal(err)
}
fmt.Println("status code:", resp.Status)

// Or wait for the load event, with no response.
_, err = chromedp.Run(ctx, chromedp.WaitEvent(page.LoadEventFired, nil,
	chromedp.Click(chromedp.ID("next"))))
```

`NavigateResponse(url)` is new. It is an `Action[*network.Response]`:

```go
resp, err := chromedp.Run(ctx, chromedp.NavigateResponse(ts.URL+"/baz"))
```

### 4. Fill a form

`SendKeys` and `SetValue` return no value. `Value` returns the value of the field.

Before:

```go
var name, color string
if err := chromedp.Run(ctx,
	chromedp.Navigate(ts.URL),
	chromedp.SendKeys("#name", "Ada"),
	chromedp.SetValue("#color", "green"),
	chromedp.Value("#name", &name),
	chromedp.Value("#color", &color),
); err != nil {
	log.Fatal(err)
}
```

After:

```go
if err := chromedp.Do(ctx,
	chromedp.Navigate(ts.URL),
	chromedp.SendKeys(chromedp.ID("name"), "Ada"),
	chromedp.SetValue(chromedp.ID("color"), "green"),
); err != nil {
	log.Fatal(err)
}
name, err := chromedp.Run(ctx, chromedp.Value(chromedp.ID("name")))
if err != nil {
	log.Fatal(err)
}
color, err := chromedp.Run(ctx, chromedp.Value(chromedp.ID("color")))
if err != nil {
	log.Fatal(err)
}
```

### 5. Wait for an event with no race

The old `ListenTarget` called a func inside the loop that handles the browser events. The func switched on the type of the event and had to return fast. A program that waited for an event needed a channel, and it had to register the func before the trigger. The new code subscribes first, triggers the event and then reads it.

In both blocks, `exceptionText` is a small helper that formats the details of the exception as text.

Before:

```go
gotException := make(chan bool, 1)
chromedp.ListenTarget(ctx, func(ev any) {
	switch ev := ev.(type) {
	case *runtime.EventConsoleAPICalled:
		fmt.Printf("* console.%s call:\n", ev.Type)
		for _, arg := range ev.Args {
			fmt.Printf("%s - %s\n", arg.Type, arg.Value)
		}
	case *runtime.EventExceptionThrown:
		fmt.Printf("* %s\n", exceptionText(ev))
		gotException <- true
	}
})
if err := chromedp.Run(ctx, chromedp.Navigate(ts.URL)); err != nil {
	log.Fatal(err)
}
<-gotException
```

After:

```go
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
	fmt.Printf("* %s\n", exceptionText(ev))
	break
}
```

The payload `ev` has its own type in each loop. Nothing is converted from `any`. The loop ends when the context ends, and the last value is the error of the context.

### 6. Close a dialog while an action waits

An alert blocks the page, and so it blocks the click that opened it. A second goroutine must close the dialog. The new code uses a typed event and a typed command.

Before:

```go
chromedp.ListenTarget(ctx, func(ev any) {
	if ev, ok := ev.(*page.EventJavascriptDialogOpening); ok {
		fmt.Println("closing alert:", ev.Message)
		go func() {
			if err := chromedp.Run(ctx, page.HandleJavaScriptDialog(true)); err != nil {
				log.Fatal(err)
			}
		}()
	}
})
```

After:

```go
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
```

### 7. Screenshots

A screenshot action returns the bytes of the image. The old code passed the address of a byte slice.

Before:

```go
var buf []byte
if err := chromedp.Run(ctx,
	chromedp.Navigate(`https://google.com`),
	chromedp.FullScreenshot(&buf, 90),
); err != nil {
	log.Fatal(err)
}
```

After:

```go
if err := chromedp.Do(ctx, chromedp.Navigate(`https://google.com`)); err != nil {
	log.Fatal(err)
}
buf, err := chromedp.Run(ctx, chromedp.FullScreenshot(90))
if err != nil {
	log.Fatal(err)
}
```

The same change applies to `Screenshot(sel, opts...)`, `ScreenshotScale`, `ScreenshotNodes` and `CaptureScreenshot()`. Each returns an `Action[[]byte]`.

### 8. Evaluate with a typed result

`Evaluate[T]` decodes the result into the type `T`. The type replaces the pointer, and the type decides how the result is read. `Void` ignores the result. `[]byte` returns the raw JSON. `*runtime.RemoteObject` returns the object without a conversion.

Before:

```go
var sum int
if err := chromedp.Run(ctx, chromedp.Evaluate(`1 + 2`, &sum)); err != nil {
	log.Fatal(err)
}

var raw []byte
err := chromedp.Run(ctx, chromedp.Evaluate(`({ a: 1 })`, &raw))

var obj *runtime.RemoteObject
err = chromedp.Run(ctx, chromedp.Evaluate(`({ a: 1 })`, &obj))

// Ignore the result.
err = chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, 100)`, nil))
```

After:

```go
sum, err := chromedp.Run(ctx, chromedp.Evaluate[int](`1 + 2`))
if err != nil {
	log.Fatal(err)
}

raw, err := chromedp.Run(ctx, chromedp.Evaluate[[]byte](`({ a: 1 })`))

obj, err := chromedp.Run(ctx, chromedp.Evaluate[*runtime.RemoteObject](`({ a: 1 })`))

// Ignore the result.
err = chromedp.Do(ctx, chromedp.Evaluate[chromedp.Void](`window.scrollTo(0, 100)`))
```

`CallFunctionOn[T]`, `JavascriptAttribute[T]`, `Poll[T]` and `PollFunction[T]` work in the same way.

### 9. Emulate a device

`Emulate` returns no value, so it goes in `Do` with the navigation. The screenshot comes from `Run`.

Before:

```go
var buf []byte
if err := chromedp.Run(ctx,
	chromedp.Emulate(device.IPhone7),
	chromedp.Navigate(`https://duckduckgo.com/`),
	chromedp.SendKeys(`textarea[name=q]`, "what's my user agent?\n"),
	chromedp.WaitVisible(`#zci-answer`),
	chromedp.CaptureScreenshot(&buf),
); err != nil {
	log.Fatal(err)
}
```

After:

```go
if err := chromedp.Do(ctx,
	chromedp.Emulate(device.IPhone7),
	chromedp.Navigate(`https://duckduckgo.com/`),
	chromedp.SendKeys(`textarea[name=q]`, "what's my user agent?\n"),
	chromedp.WaitVisible(chromedp.ID(`zci-answer`)),
); err != nil {
	log.Fatal(err)
}
buf, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
if err != nil {
	log.Fatal(err)
}
```

### 10. Listen to network events with an iterator

The old code needed a listener, and the program had to keep its own state. The new code ranges over the events and stops with `break`.

Before:

```go
chromedp.ListenTarget(ctx, func(ev any) {
	if ev, ok := ev.(*network.EventResponseReceived); ok {
		fmt.Println(ev.Response.URL, ev.Response.Status)
	}
})
if err := chromedp.Run(ctx, chromedp.Navigate(ts.URL)); err != nil {
	log.Fatal(err)
}
```

After:

```go
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
```

`BrowserEvents` is the same for the events of the browser, such as `target.TargetCreated`.

### 11. Send a custom command with cdp.Call

A protocol command is not an action any more. An action sends it with `cdp.Call` on its target. The old code used a `Do` method on the generated parameters. The new parameter types have no methods.

Before:

```go
var buf []byte
if err := chromedp.Run(ctx,
	chromedp.Navigate(`https://pkg.go.dev/github.com/chromedp/chromedp`),
	chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		buf, _, err = page.PrintToPDF().WithDisplayHeaderFooter(false).WithLandscape(true).Do(ctx)
		return err
	}),
); err != nil {
	log.Fatal(err)
}
```

After:

```go
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
```

Outside an action, `chromedp.Call(ctx, command, params)` runs the command on the target of the context. `chromedp.CallBrowser` runs it on the browser.

```go
res, err := chromedp.Call(ctx, page.GetFrameTree, cdp.Empty{})
```

### 12. Run an old action with Legacy

An action of the old interface has a `Do(context.Context) error` method. `Legacy` makes it an `Action[Void]`. The old action receives the same context, so `chromedp.Call` works inside it.

Before:

```go
var title string
if err := chromedp.Run(ctx,
	chromedp.Navigate(ts.URL),
	oldTitle{&title},
); err != nil {
	log.Fatal(err)
}
```

After:

```go
var title string
if err := chromedp.Do(ctx,
	chromedp.Navigate(ts.URL),
	chromedp.Legacy(oldTitle{&title}),
); err != nil {
	log.Fatal(err)
}
```

The type `oldTitle` is the same in both. It has the method `Do(ctx context.Context) error`, and it sends its commands with `chromedp.Call(ctx, ...)`.

### 13. Run a query from another node

`FromNode` keeps its name. It works with a `CSS` or a `CSSAll` selector. A `Search` or a `JSPath` selector ignores it. The old code chose the CSS lookup with an option, and `docs/MIGRATION.md` lists it. Each `Text` returns its value.

```go
queryRoot, err := chromedp.Run(ctx, chromedp.Text(chromedp.CSS(".content")))
if err != nil {
	log.Fatal(err)
}
queryFromNode, err := chromedp.Run(ctx,
	chromedp.Text(chromedp.CSS(".content"), chromedp.FromNode(sectionNode)))
if err != nil {
	log.Fatal(err)
}
```

### 14. Show a visible window, or leave the browser open

The default allocator runs Chrome in headless mode. `WithVisibleWindow` builds it with a visible, maximized window. The variable `CHROMEDP_VISIBLEWINDOW` does the same with no change in the code. `WaitClosed` keeps the program alive until the user closes the window. On Linux the first `Run` returns `ErrNoDisplay` when `DISPLAY` and `WAYLAND_DISPLAY` are both empty.

```go
ctx, cancel := chromedp.NewContext(context.Background(), chromedp.WithVisibleWindow())
defer cancel()
if err := chromedp.Do(ctx, chromedp.Navigate("https://example.com")); err != nil {
	log.Fatal(err)
}
chromedp.WaitClosed(ctx)
```

`WithKeepOpen` of the module `github.com/chromedp/chromedp/remote` leaves the browser open when the program ends. It uses the websocket and starts Chrome detached. `KeptOpen` returns the address and the profile directory, and `remote.NewAllocator` attaches to the address later. The program must not delete the profile directory. The core module has no websocket code, so a program that uses `WithKeepOpen` runs `go get github.com/chromedp/chromedp/remote` first.

### 15. Connect to a remote browser that needs headers

A hosted browser service can need a header, such as `Authorization`, on the websocket request. `remote.WithDialHTTPHeader` sets the headers of a remote allocator, and `remote.WithConnHTTPHeader` is the same option for `remote.DialContext`. The headers go with the websocket request only. The request to `/json/version` has none, so give the allocator the full websocket address and use `remote.NoModifyURL`. The remote allocator is in the module `github.com/chromedp/chromedp/remote`.

```go
header := http.Header{"Authorization": {"Bearer " + token}}
allocCtx, cancel := remote.NewAllocator(context.Background(), wsURL,
	remote.NoModifyURL,
	remote.WithDialHTTPHeader(header),
)
defer cancel()
ctx, cancel := chromedp.NewContext(allocCtx)
defer cancel()
```

```go
ctx, cancel := chromedp.NewContext(context.Background(),
	chromedp.WithVisibleWindow(), remote.WithKeepOpen())
defer cancel()
if err := chromedp.Do(ctx, chromedp.Navigate("https://example.com")); err != nil {
	log.Fatal(err)
}
wsURL, dir := chromedp.KeptOpen(ctx)
fmt.Println("attach to", wsURL, "profile", dir)
```

Both options only apply when `NewContext` builds the default allocator. For an allocator that you make, add the allocator options `VisibleWindow` and `KeepOpen` to `NewExecAllocator`. `KeepOpen` also needs the allocator option `remote.WebSocket`, or `NewExecAllocator` returns `ErrNoDialer` at the first `Run`. `docs/decisions/2026-10-03-a-visible-window-is-an-opt-in.md` explains the choices.

### 16. Tap on a touch screen

`Click` sends mouse events. A page that listens for `touchstart` and `touchend` needs a touch tap. `Tap` scrolls the first element that matches the selector into view, finds its center as `Click` does, and sends a `touchStart` event and a `touchEnd` event. `TapXY` does the same for a point. The browser sends the touch events and the click event that follows them only when touch emulation is on, so run `EmulateViewport` with `EmulateTouch` first. This example has no Before block, because the old API had no touch action.

```go
if err := chromedp.Do(ctx,
	chromedp.EmulateViewport(375, 812, chromedp.EmulateMobile, chromedp.EmulateTouch),
	chromedp.Navigate(url),
	chromedp.Tap(chromedp.CSS("#menu")),
); err != nil {
	log.Fatal(err)
}
```

### 17. Leave the tab open when the context ends

The cancellation of a context closes its tab. A remote browser service can keep its tabs for a client that comes back. `WithDetachOnCancel` makes the cancellation only detach from the tab. A later context attaches to the tab with `WithTargetID`. A context that `WithNewBrowserContext` made keeps its browser context too, and the program must dispose of it with `target.DisposeBrowserContext`. This example has no Before block, because the old API had no such option.

```go
ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithDetachOnCancel())
if err := chromedp.Do(ctx, chromedp.Navigate(url)); err != nil {
	log.Fatal(err)
}
id := chromedp.FromContext(ctx).Target.TargetID
cancel() // the tab stays open

ctx, cancel = chromedp.NewContext(allocCtx, chromedp.WithTargetID(id))
defer cancel()
```

### 18. Run actions in parallel

Contexts that share one browser run in separate tabs, and they are safe to use in parallel. Call `Run` once on a parent context so that it has a browser. Then make a child context with `NewContext` for each goroutine. A child of a context that has no browser yet starts a browser of its own. Do not share one context between goroutines, because the actions of one context share one tab and can race. The first `Run` on a context starts the browser, and it must not run at the same time as another `Run` on that context. The old API had the same rules, so this example has no Before block.

```go
ctx, cancel := chromedp.NewContext(context.Background())
defer cancel()
if err := chromedp.Do(ctx); err != nil { // starts the browser
	log.Fatal(err)
}
var wg sync.WaitGroup
for _, url := range urls {
	wg.Go(func() {
		tabCtx, cancel := chromedp.NewContext(ctx) // a new tab
		defer cancel()
		if err := chromedp.Do(tabCtx, chromedp.Navigate(url)); err != nil {
			log.Println(url, err)
		}
	})
}
wg.Wait()
```

### 19. Work in an iframe from another site

Chrome runs an iframe from another site (a cross-site iframe) in its own process, as a separate target of the type `iframe`. The node of the `iframe` element has no `ContentDocument`, so `FromNode` finds nothing in it. `chromedp` does not attach to the iframe target by itself, because it ignores `Target.attachedToTarget` for it. The network events of the iframe do not reach the context of the page. An iframe from the same site is not affected. This example has no Before block, because the old API had the same limit.

To work in the iframe, attach to its target with `WithTargetID`. The new context runs actions in the iframe and gets its events. The parent context must have run once, so that it has a browser.

```go
infos, err := chromedp.Targets(ctx)
if err != nil {
	log.Fatal(err)
}
for _, info := range infos {
	if info.Type == "iframe" && strings.HasPrefix(info.URL, "https://other.example/") {
		frameCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(info.TargetID))
		defer cancel()
		text, err := chromedp.Run(frameCtx, chromedp.Text(chromedp.CSS("#inner")))
		// ...
	}
}
```

The other way is to turn off site isolation, so that the iframe stays in the process of the page. Then `FromNode` and the events of the page reach it.

```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
	chromedp.Flag("disable-features", "SitePerProcess,IsolateOrigins"),
	chromedp.Flag("disable-site-isolation-trials", true),
)
```

This turns off a security feature of Chrome, so use it only for pages that you trust. The default options keep site isolation on. See `docs/decisions/2026-10-04-keep-site-isolation-on.md`.

### 20. Send a new line to an editor

`SendKeys` sends the character `\n` as the Enter key, in the form `\r`, as `kb/kb.go` says. That is a keyDown event, a char event with the text `\r`, and a keyUp event. The key `kb.Enter` is the same as `\r`, so it sends the same events. An editor that handles Enter on keydown and cancels the default, such as Lexical, can insert two line breaks, because the char event inserts one more. To send the Enter key with no char event, send a rawKeyDown event and a keyUp event with `input.DispatchKeyEvent`. The page of a test with such an editor got one line break with this action, and two with `SendKeys` and `\n`. The old API had the same behavior, so this example has no Before block.

```go
enter := chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
	for _, typ := range []input.DispatchKeyEventType{
		input.DispatchKeyEventTypeRawKeyDown, input.DispatchKeyEventTypeKeyUp,
	} {
		_, err := cdp.Call(ctx, t, input.DispatchKeyEvent, input.DispatchKeyEventParams{
			Type: typ, Key: "Enter", Code: "Enter",
			WindowsVirtualKeyCode: 13, NativeVirtualKeyCode: 13,
		})
		if err != nil {
			return err
		}
	}
	return nil
})
err := chromedp.Do(ctx,
	chromedp.SendKeys(sel, "Hello"),
	enter,
	chromedp.SendKeys(sel, "World"),
)
```

### 21. Set a timeout for one action

The first `Run` on a context starts the browser, and it binds the life of the browser to the context that you pass to that call. A context from `context.WithTimeout` that you use for the first `Run` closes the browser when the timeout ends. To limit one action, start the browser first with a context that has no timeout. Then run the action with a context that you derive from it. When the timeout ends, `Run` returns an error that wraps `context.DeadlineExceeded`, and the tab and the browser stay open. `Run` has no option for the timeout of one action. The old API had the same behavior, so this example has no Before block.

```go
ctx, cancel := chromedp.NewContext(context.Background())
defer cancel()
if err := chromedp.Do(ctx); err != nil { // starts the browser, with no timeout
	log.Fatal(err)
}

tctx, tcancel := context.WithTimeout(ctx, 5*time.Second)
defer tcancel()
err := chromedp.Do(tctx, chromedp.Navigate(url)) // only this call has the timeout
```

### 22. Turn off disable-dev-shm-usage

`DefaultExecAllocatorOptions` sets `disable-dev-shm-usage` to true, so that Chrome keeps the files of its shared memory in the temporary directory and not in `/dev/shm`. The flag is on because `/dev/shm` is small in many containers, and Chrome crashes when it fills the space. The cost is file-backed memory in the temporary directory, which can be slower, and which a program that runs for a long time can see as growing memory. If `/dev/shm` is large enough, set the flag to false after the default options. The old API had the same default, so this example has no Before block.

```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
	chromedp.Flag("disable-dev-shm-usage", false),
)
allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
defer cancel()
```

### 23. Look up elements in a custom way

The set of selector types is closed. `Selectable` accepts a string type and a `[]cdp.NodeID` type, and a type that you define cannot add a lookup. `ByFunc` is the extension point. It replaces the lookup of the selector with a func. The func receives the `*Target` of the tab, which is a session for `cdp.Call`, and the `*Node` where the query starts. That node is the root of the document, or the node of `FromNode`. The func returns the node IDs of the elements. The selector is then only a label in error messages, so pass `""`. The rest of the query works as usual, so the wait options and the actions apply to the nodes that the func returns. `docs/MIGRATION.md` lists the `ByFunc` change.

```go
func byTestID(id string) chromedp.QueryOption {
	return chromedp.ByFunc(func(ctx context.Context, t *chromedp.Target, n *chromedp.Node) ([]cdp.NodeID, error) {
		res, err := cdp.Call(ctx, t, dom.QuerySelectorAll, dom.QuerySelectorAllParams{
			NodeID:   n.NodeID,
			Selector: fmt.Sprintf("[data-testid=%q]", id),
		})
		return res.NodeIDs, err
	})
}

err := chromedp.Do(ctx, chromedp.Click("", byTestID("submit")))
```

## What the new API removes

The new API removes the interface `Action`, the type `ActionFunc`, the type `Tasks`, and the funcs `ListenTarget` and `ListenBrowser`. It also removes the types `QueryAction`, `NavigateAction`, `EvaluateAction`, `CallAction`, `PollAction`, `MouseAction`, `KeyAction` and `EmulateAction`. Each type is now `Action[T]`. It also removes the six lookup options that start with `By`, except `ByFunc`. A selector type replaces each of them. `docs/MIGRATION.md` lists every change, with the old name and the new name.

## The exported funcs and types

This part lists the signatures that `go doc -all` prints for the packages of the three modules. Run `go doc` on a name for its documentation. The first part is the core package `github.com/chromedp/chromedp`. The part "The module remote" follows it. The module `test` has no exported names.

## Functions

These are the functions of the package that no type owns.

```go
func BrowserEvents[E any](ctx context.Context, ev cdp.Event[E]) iter.Seq2[E, error]
func ButtonLeft(p *input.DispatchMouseEventParams)
func ButtonMiddle(p *input.DispatchMouseEventParams)
func ButtonNone(p *input.DispatchMouseEventParams)
func ButtonRight(p *input.DispatchMouseEventParams)
func Call[P, R any](ctx context.Context, cmd cdp.Command[P, R], params P) (R, error)
func CallBrowser[P, R any](ctx context.Context, cmd cdp.Command[P, R], params P) (R, error)
func Cancel(ctx context.Context) error
func DisableGPU(a *ExecAllocator)
func Do(ctx context.Context, steps ...Action[Void]) error
func EmulateLandscape(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams)
func EmulateMobile(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams)
func EmulatePortrait(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams)
func EmulateTouch(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams)
func EvalAsValue(p *runtime.EvaluateParams)
func EvalIgnoreExceptions(p *runtime.EvaluateParams)
func EvalWithCommandLineAPI(p *runtime.EvaluateParams)
func Events[E any](ctx context.Context, ev cdp.Event[E]) iter.Seq2[E, error]
func Headless(a *ExecAllocator)
func IgnoreCertErrors(a *ExecAllocator)
func KeepOpen(a *ExecAllocator)
func KeptOpen(ctx context.Context) (wsURL, userDataDir string)
func NewAllocatorContext(parent context.Context, a Allocator) (context.Context, context.CancelFunc)
func NewContext(parent context.Context, opts ...ContextOption) (context.Context, context.CancelFunc)
func NewExecAllocator(parent context.Context, opts ...ExecAllocatorOption) (context.Context, context.CancelFunc)
func NoDefaultBrowserCheck(a *ExecAllocator)
func NoFirstRun(a *ExecAllocator)
func NoInheritEnv(a *ExecAllocator)
func NoSandbox(a *ExecAllocator)
func NodeEnabled(s *Selector)
func NodeNotPresent(s *Selector)
func NodeNotVisible(s *Selector)
func NodeReady(s *Selector)
func NodeSelected(s *Selector)
func NodeVisible(s *Selector)
func Run[T any](ctx context.Context, a Action[T]) (T, error)
func RunResponse(ctx context.Context, steps ...Action[Void]) (*network.Response, error)
func Targets(ctx context.Context) ([]*target.Info, error)
func VisibleWindow(a *ExecAllocator)
func WaitClosed(ctx context.Context) error
func WaitNewTarget(ctx context.Context, fn func(*target.Info) bool) <-chan target.ID
```

## Types, with their constructors and methods

```go
type Action[T any] func(ctx context.Context, t *Target) (T, error)
func AttributeValue[S Selectable](sel S, name string, opts ...QueryOption) Action[AttributeResult]
func Attributes[S Selectable](sel S, opts ...QueryOption) Action[map[string]string]
func AttributesAll[S Selectable](sel S, opts ...QueryOption) Action[[]map[string]string]
func Blur[S Selectable](sel S, opts ...QueryOption) Action[Void]
func CallFunctionOn[T any](functionDeclaration string, opt CallOption, args ...any) Action[T]
func CaptureScreenshot() Action[[]byte]
func Clear[S Selectable](sel S, opts ...QueryOption) Action[Void]
func Click[S Selectable](sel S, opts ...QueryOption) Action[Void]
func ComputedStyle[S Selectable](sel S, opts ...QueryOption) Action[[]*css.ComputedStyleProperty]
func Dimensions[S Selectable](sel S, opts ...QueryOption) Action[*dom.BoxModel]
func DoubleClick[S Selectable](sel S, opts ...QueryOption) Action[Void]
func Dump[S Selectable](sel S, w io.Writer, opts ...QueryOption) Action[Void]
func DumpTo[S Selectable](sel S, w io.Writer, prefix, indent string, nodeIDs bool, depth int64, pierce bool, wait time.Duration, opts ...QueryOption) Action[Void]
func Emulate(device Device) Action[Void]
func EmulateReset() Action[Void]
func EmulateViewport(width, height int64, opts ...EmulateViewportOption) Action[Void]
func Evaluate[T any](expression string, opts ...EvaluateOption) Action[T]
func EvaluateAsDevTools[T any](expression string, opts ...EvaluateOption) Action[T]
func Focus[S Selectable](sel S, opts ...QueryOption) Action[Void]
func FullScreenshot(quality int) Action[[]byte]
func Func(f func(ctx context.Context, t *Target) error) Action[Void]
func InnerHTML[S Selectable](sel S, opts ...QueryOption) Action[string]
func JavascriptAttribute[T any, S Selectable](sel S, name string, opts ...QueryOption) Action[T]
func KeyEvent(keys string, opts ...KeyOption) Action[Void]
func KeyEventNode(n *Node, keys string, opts ...KeyOption) Action[Void]
func Legacy(a OldAction) Action[Void]
func Location() Action[string]
func MatchedStyle[S Selectable](sel S, opts ...QueryOption) Action[*css.GetMatchedStylesForNodeResult]
func MouseClickNode(n *Node, opts ...MouseOption) Action[Void]
func MouseClickXY(x, y float64, opts ...MouseOption) Action[Void]
func MouseEvent(typ input.DispatchMouseEventType, x, y float64, opts ...MouseOption) Action[Void]
func Navigate(urlstr string) Action[Void]
func NavigateBack() Action[Void]
func NavigateForward() Action[Void]
func NavigateResponse(urlstr string) Action[*network.Response]
func NavigateToHistoryEntry(entryID int64) Action[Void]
func NavigationEntries() Action[page.GetNavigationHistoryResult]
func Nodes[S Selectable](sel S, opts ...QueryOption) Action[[]*Node]
func OuterHTML[S Selectable](sel S, opts ...QueryOption) Action[string]
func Poll[T any](expression string, opts ...PollOption) Action[T]
func PollFunction[T any](pageFunction string, opts ...PollOption) Action[T]
func Query[S Selectable](sel S, opts ...QueryOption) Action[Void]
func QueryAfter[T any, S Selectable](sel S, f func(ctx context.Context, t *Target, nodes []*Node) (T, error), opts ...QueryOption) Action[T]
func QueryNodeIDs[S Selectable](sel S, opts ...QueryOption) Action[[]cdp.NodeID]
func Reload() Action[Void]
func RemoveAttribute[S Selectable](sel S, name string, opts ...QueryOption) Action[Void]
func Reset[S Selectable](sel S, opts ...QueryOption) Action[Void]
func ResetViewport() Action[Void]
func Screenshot[S Selectable](sel S, opts ...QueryOption) Action[[]byte]
func ScreenshotNodes(nodes []*Node, scale float64) Action[[]byte]
func ScreenshotScale[S Selectable](sel S, scale float64, opts ...QueryOption) Action[[]byte]
func ScrollIntoView[S Selectable](sel S, opts ...QueryOption) Action[Void]
func SendKeys[S Selectable](sel S, v string, opts ...QueryOption) Action[Void]
func SetAttributeValue[S Selectable](sel S, name, value string, opts ...QueryOption) Action[Void]
func SetAttributes[S Selectable](sel S, attributes map[string]string, opts ...QueryOption) Action[Void]
func SetJavascriptAttribute[S Selectable](sel S, name, value string, opts ...QueryOption) Action[Void]
func SetUploadFiles[S Selectable](sel S, files []string, opts ...QueryOption) Action[Void]
func SetValue[S Selectable](sel S, value string, opts ...QueryOption) Action[Void]
func Sleep(d time.Duration) Action[Void]
func Steps(steps ...Action[Void]) Action[Void]
func Stop() Action[Void]
func Submit[S Selectable](sel S, opts ...QueryOption) Action[Void]
func Tap[S Selectable](sel S, opts ...QueryOption) Action[Void]
func TapXY(x, y float64) Action[Void]
func Text[S Selectable](sel S, opts ...QueryOption) Action[string]
func TextContent[S Selectable](sel S, opts ...QueryOption) Action[string]
func Title() Action[string]
func Value[S Selectable](sel S, opts ...QueryOption) Action[string]
func WaitEnabled[S Selectable](sel S, opts ...QueryOption) Action[Void]
func WaitEvent[E, T any](ev cdp.Event[E], match func(E) bool, trigger Action[T]) Action[E]
func WaitNotPresent[S Selectable](sel S, opts ...QueryOption) Action[Void]
func WaitNotVisible[S Selectable](sel S, opts ...QueryOption) Action[Void]
func WaitReady[S Selectable](sel S, opts ...QueryOption) Action[Void]
func WaitSelected[S Selectable](sel S, opts ...QueryOption) Action[Void]
func WaitVisible[S Selectable](sel S, opts ...QueryOption) Action[Void]

type Allocator interface { ... }

type Attacher interface { ... }

type AttributeResult struct { ... }

type Browser struct { ... }
func NewBrowserTransport(ctx context.Context, tr Transport, opts ...BrowserOption) (*Browser, error)
func (b *Browser) Call(ctx context.Context, method string, params, res any) error
func (b *Browser) Process() *os.Process
func (b *Browser) Subscribe(method string) (<-chan jsontext.Value, func())

type BrowserOption = func(*Browser)
func WithBrowserDebugf(f func(string, ...any)) BrowserOption
func WithBrowserErrorf(f func(string, ...any)) BrowserOption
func WithBrowserLogf(f func(string, ...any)) BrowserOption
func WithConsolef(f func(string, ...any)) BrowserOption

type CSS string

type CSSAll string

type CallOption = func(params *runtime.CallFunctionOnParams)

type Context struct { ... }
func FromContext(ctx context.Context) *Context

type ContextOption = func(*Context)
func WithBrowserOption(opts ...BrowserOption) ContextOption
func WithAllocatorOptions(opts ...ExecAllocatorOption) ContextOption
func WithDebugf(f func(string, ...any)) ContextOption
func WithDetachOnCancel() ContextOption
func WithErrorf(f func(string, ...any)) ContextOption
func WithExistingBrowserContext(id cdp.BrowserContextID) ContextOption
func WithLogf(f func(string, ...any)) ContextOption
func WithNewBrowserContext(options ...CreateBrowserContextOption) ContextOption
func WithNewWindow(newWindow bool) ContextOption
func WithTargetID(id target.ID) ContextOption
func WithVisibleWindow() ContextOption

type CreateBrowserContextOption = func(*target.CreateBrowserContextParams)

type Device interface { ... }

type Dialer = func(ctx context.Context, wsURL string) (Transport, error)

type EmulateViewportOption = func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams)
func EmulateOrientation(orientation emulation.ScreenOrientationType, angle int64) EmulateViewportOption
func EmulateScale(scale float64) EmulateViewportOption

type Error string
func (err Error) Error() string

type EvaluateOption = func(*runtime.EvaluateParams)
func EvalObjectGroup(objectGroup string) EvaluateOption

type ExceptionError struct { ... }
func (e *ExceptionError) Error() string

type ExecAllocator struct { ... }
func (a *ExecAllocator) Allocate(ctx context.Context, opts ...BrowserOption) (*Browser, error)
func (a *ExecAllocator) Wait()

type ExecAllocatorOption = func(*ExecAllocator)
func CombinedOutput(w io.Writer) ExecAllocatorOption
func Env(vars ...string) ExecAllocatorOption
func ExecPath(path string) ExecAllocatorOption
func Flag(name string, value any) ExecAllocatorOption
func ModifyCmdFunc(f func(cmd *exec.Cmd)) ExecAllocatorOption
func ProxyServer(proxy string) ExecAllocatorOption
func UserAgent(userAgent string) ExecAllocatorOption
func UserDataDir(dir string) ExecAllocatorOption
func WSURLReadTimeout(t time.Duration) ExecAllocatorOption
func WithDialer(d Dialer) ExecAllocatorOption
func WindowSize(width, height int) ExecAllocatorOption

type Frame struct { ... }

type FrameState uint16
func (fs FrameState) String() string

type ID string

type JSPath string

type KeyOption = func(*input.DispatchKeyEventParams)
func KeyModifiers(modifiers ...Modifier) KeyOption

type LoadError struct { ... }
func (e *LoadError) Error() string
func (e *LoadError) Unwrap() error

type Modifier = kb.Modifier

type MouseOption = func(*input.DispatchMouseEventParams)
func Button(btn string) MouseOption
func ButtonModifiers(modifiers ...Modifier) MouseOption
func ButtonType(button input.MouseButton) MouseOption
func ClickCount(n int) MouseOption

type Node struct { ... }
func (n *Node) Attribute(name string) (string, bool)
func (n *Node) AttributeValue(name string) string
func (n *Node) Dump(prefix, indent string, nodeIDs bool) string
func (n *Node) FullXPath() string
func (n *Node) FullXPathByID() string
func (n *Node) PartialXPath() string
func (n *Node) PartialXPathByID() string
func (n *Node) WriteTo(w io.Writer, prefix, indent string, nodeIDs bool) (int, error)

type NodeIDs []cdp.NodeID

type NodeState uint8
func (ns NodeState) String() string

type NodeType int64
func (t NodeType) String() string

type OldAction interface { ... }

type PipeConn struct { ... }
func NewPipeConn(r io.ReadCloser, w io.WriteCloser, opts ...PipeOption) *PipeConn
func (c *PipeConn) Close() error
func (c *PipeConn) Read(_ context.Context, msg *cdproto.Message) error
func (c *PipeConn) SetDebugf(f func(string, ...any))
func (c *PipeConn) Write(_ context.Context, msg *cdproto.Message) error

type PipeOption = func(*PipeConn)
func WithPipeDebugf(f func(string, ...any)) PipeOption

type PollOption = func(task *pollTask)
func WithPollingArgs(args ...any) PollOption
func WithPollingInFrame(frame *Node) PollOption
func WithPollingInterval(interval time.Duration) PollOption
func WithPollingMutation() PollOption
func WithPollingTimeout(timeout time.Duration) PollOption

type PopulateOption = func(*time.Duration)
func PopulateWait(wait time.Duration) PopulateOption

type QueryOption = func(*Selector)
func After(f func(ctx context.Context, t *Target, nodes []*Node) error) QueryOption
func AtLeast(n int) QueryOption
func ByFunc(f func(context.Context, *Target, *Node) ([]cdp.NodeID, error)) QueryOption
func FromNode(node *Node) QueryOption
func Populate(depth int64, pierce bool, opts ...PopulateOption) QueryOption
func RetryInterval(interval time.Duration) QueryOption
func WaitFunc(wait func(context.Context, *Target, *Frame, runtime.ExecutionContextID, ...cdp.NodeID) ([]*Node, error)) QueryOption

type Search string

type Selectable interface { ... }

type Selector struct { ... }

type Target struct { ... }
func (t *Target) Call(ctx context.Context, method string, params, res any) error
func (t *Target) Subscribe(method string) (<-chan jsontext.Value, func())

type Transport interface { ... }

type Void = struct{}
```

## Constants and variables

The package has these constants:

```go
MousePressed
MouseReleased
MouseMoved
MouseWheel
ModifierNone
ModifierAlt
ModifierCtrl
ModifierMeta
ModifierShift
ModifierCommand
NodeTypeElement
NodeTypeAttribute
NodeTypeText
NodeTypeCDATA
NodeTypeEntityReference
NodeTypeEntity
NodeTypeProcessingInstruction
NodeTypeComment
NodeTypeDocument
NodeTypeDocumentType
NodeTypeDocumentFragment
NodeTypeNotation
EmptyFrameID
EmptyNodeID
ErrInvalidDimensions
ErrNoDialer
ErrNoResults
ErrHasResults
ErrNotVisible
ErrVisible
ErrDisabled
ErrNotSelected
ErrInvalidBoxModel
ErrChannelClosed
ErrInvalidTarget
ErrPageLoad
ErrNoDisplay
ErrInvalidContext
ErrPollingTimeout
ErrJSUndefined
ErrJSNull
FrameDOMContentEventFired
FrameLoadEventFired
FrameAttached
FrameNavigated
FrameLoading
FrameScheduledNavigation
NodeStateReady
NodeStateVisible
NodeStateHighlighted
```

It has these variables:

```go
DefaultUnmarshalOptions
DefaultMarshalOptions
DefaultExecAllocatorOptions
```

## The module remote

The package `github.com/chromedp/chromedp/remote` holds the code that needs a websocket. The core module does not import it. Run `go get github.com/chromedp/chromedp/remote` to use it. `docs/decisions/2026-10-04-the-core-uses-only-the-standard-library.md` explains the split, and `docs/MIGRATION.md` lists the old and the new names.

```go
var ErrInvalidMessage = errors.New("invalid websocket message")
func NewAllocator(parent context.Context, url string, opts ...Option) (context.Context, context.CancelFunc)
func NoModifyURL(a *Allocator)
func WebSocket(a *chromedp.ExecAllocator)
func WithKeepOpen() chromedp.ContextOption

type Allocator struct { ... }
func (a *Allocator) Allocate(ctx context.Context, opts ...chromedp.BrowserOption) (*chromedp.Browser, error)
func (a *Allocator) Attaches() bool
func (a *Allocator) Wait()

type Conn struct { ... }
func DialContext(ctx context.Context, urlstr string, opts ...DialOption) (*Conn, error)
func (c *Conn) Close() error
func (c *Conn) Read(_ context.Context, msg *cdproto.Message) error
func (c *Conn) SetDebugf(f func(string, ...any))
func (c *Conn) Write(_ context.Context, msg *cdproto.Message) error

type DialOption = func(*Conn)
func WithConnDebugf(f func(string, ...any)) DialOption
func WithConnHTTPHeader(h http.Header) DialOption

type Option = func(*Allocator)
func WithDialHTTPHeader(h http.Header) Option
func WithDialTimeout(d time.Duration) Option
```

`NewAllocator` attaches to a browser that runs already. `WebSocket` makes the exec allocator of the core use a websocket, and `WithKeepOpen` builds the default allocator of `NewContext` so that it keeps the browser open. The core hooks that `remote` uses are `Dialer`, `WithDialer`, `WithAllocatorOptions`, `Attacher` and `NewAllocatorContext`.

### Use a websocket with the exec allocator

The pipe is the default. A program that needs a debugging port adds `remote.WebSocket` to the options of the allocator.

```go
opts := append(chromedp.DefaultExecAllocatorOptions[:], remote.WebSocket)
allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
defer cancel()
ctx, cancel := chromedp.NewContext(allocCtx)
defer cancel()
```
