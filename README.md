# About chromedp

<p align="center">
  <img src="https://raw.githubusercontent.com/chromedp/logo/main/chromedp.svg" alt="chromedp logo" width="160">
</p>

Package `chromedp` drives browsers that speak the
[Chrome DevTools Protocol][devtools-protocol] from Go. It needs no external
driver.

[![Unit Tests][chromedp-ci-status]][chromedp-ci]
[![Go Reference][goref-chromedp-status]][goref-chromedp]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

## Installing

Install the package with `go get`. The module needs Go 1.27 or newer. Version
v0.18.0 uses the typed `cdproto` v0.157.4 and has the generic and iterator API.
The earlier versions of `cdproto`, v0.157.0, v0.157.1 and v0.157.2, have the old API.

```sh
go get -u github.com/chromedp/chromedp
```

The core module uses only the Go standard library and `cdproto`. It starts a
browser and talks to it through a pipe. Code that needs a websocket is in a
second module, `github.com/chromedp/chromedp/remote`. Install it only when you
connect to a browser that runs already, when you want the exec allocator to use a
websocket, or when you keep the browser open after the program ends:

```sh
go get -u github.com/chromedp/chromedp/remote
```

## Usage

An action is a func that runs against a browser tab and returns a value.
`chromedp.Do` runs actions that return nothing, and `chromedp.Run` runs one
action and returns its value:

```go
ctx, cancel := chromedp.NewContext(context.Background())
defer cancel()

if err := chromedp.Do(ctx, chromedp.Navigate(`https://pkg.go.dev/`)); err != nil {
	log.Fatal(err)
}
title, err := chromedp.Run(ctx, chromedp.Title())
if err != nil {
	log.Fatal(err)
}
fmt.Println(title)
```

A page event is an iterator. `chromedp.Events` subscribes when it returns, so
a program can subscribe, trigger the event, and then read it:

```go
loaded := chromedp.Events(ctx, page.LoadEventFired)
if err := chromedp.Do(ctx, chromedp.Navigate(`https://pkg.go.dev/`)); err != nil {
	log.Fatal(err)
}
for ev, err := range loaded {
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ev.Timestamp)
	break
}
```

[`docs/API.md`](docs/API.md) describes the API. It has 14 examples that show the
old code and the new code side by side. [`docs/MIGRATION.md`](docs/MIGRATION.md)
lists every renamed and removed name.

See the [Go reference][goref-chromedp] for the documentation and examples.

## More examples

The [examples][chromedp-examples] repository has 43 programs for larger tasks.
Each program is one `main.go` file. Run one with `go run`, for example
`go run github.com/chromedp/examples/tabs@latest`. Every program takes the flag `-v`
to print the protocol messages. Every program except `remote` takes the flag
`-visible` to show the browser window. Most programs take the flag
`-visible-on-terminal` to draw the page in the terminal while they run, with
[`termcast`](#show-the-browser-in-the-terminal). Every program except `fast`
reads a local test site and needs no internet. Use the flag `-url` to read
another site.

Read pages and fill forms:

- [click](https://github.com/chromedp/examples/tree/main/click) clicks an element that a selector finds.
- [text](https://github.com/chromedp/examples/tree/main/text) reads the text of an element.
- [eval](https://github.com/chromedp/examples/tree/main/eval) runs JavaScript in the page and decodes the result.
- [structeval](https://github.com/chromedp/examples/tree/main/structeval) decodes JavaScript results into Go structs, slices and maps, and shows the errors.
- [logic](https://github.com/chromedp/examples/tree/main/logic) mixes actions and Go code to read a list.
- [subtree](https://github.com/chromedp/examples/tree/main/subtree) walks a subtree of the DOM.
- [selectors](https://github.com/chromedp/examples/tree/main/selectors) shows the typed selectors side by side.
- [frames](https://github.com/chromedp/examples/tree/main/frames) reaches elements inside an iframe and a shadow root.
- [submit](https://github.com/chromedp/examples/tree/main/submit) fills out and submits a form.
- [keys](https://github.com/chromedp/examples/tree/main/keys) sends key events to an element.
- [upload](https://github.com/chromedp/examples/tree/main/upload) uploads a file on a form.
- [visible](https://github.com/chromedp/examples/tree/main/visible) waits until an element is visible.
- [dragdrop](https://github.com/chromedp/examples/tree/main/dragdrop) drags and drops with the mouse and with HTML5 drag and drop.

Network, sessions and files:

- [cookie](https://github.com/chromedp/examples/tree/main/cookie) sets cookies on requests.
- [headers](https://github.com/chromedp/examples/tree/main/headers) adds extra HTTP headers.
- [proxy](https://github.com/chromedp/examples/tree/main/proxy) signs in to a proxy server that needs a password.
- [intercept](https://github.com/chromedp/examples/tree/main/intercept) blocks, mocks and changes requests with the Fetch domain.
- [session](https://github.com/chromedp/examples/tree/main/session) saves a login session and restores it in another browser.
- [har](https://github.com/chromedp/examples/tree/main/har) writes a HAR file from the network events of a page.
- [download_file](https://github.com/chromedp/examples/tree/main/download_file) downloads a file with a headless browser.
- [download_image](https://github.com/chromedp/examples/tree/main/download_image) downloads an image from the network response.

Events and the protocol:

- [eventsiter](https://github.com/chromedp/examples/tree/main/eventsiter) reads the events of a page with iterators, and waits for the network to be idle.
- [console](https://github.com/chromedp/examples/tree/main/console) reads the console and the uncaught exceptions of a page.
- [dialogs](https://github.com/chromedp/examples/tree/main/dialogs) answers alert, confirm, prompt and beforeunload dialogs.
- [popups](https://github.com/chromedp/examples/tree/main/popups) works with popups and several targets, and with browser contexts.
- [exposefunc](https://github.com/chromedp/examples/tree/main/exposefunc) calls Go functions from the page.
- [extension](https://github.com/chromedp/examples/tree/main/extension) loads a browser extension, uBlock Origin Lite, from a folder on disk and shows that it blocks the ads of a page.
- [rawcall](https://github.com/chromedp/examples/tree/main/rawcall) sends protocol commands that have no action, such as timezone, locale, geolocation and throttling.

Screens and devices:

- [screenshot](https://github.com/chromedp/examples/tree/main/screenshot) takes a screenshot of an element and of the whole page.
- [pdf](https://github.com/chromedp/examples/tree/main/pdf) prints a page to a PDF file.
- [pdfoptions](https://github.com/chromedp/examples/tree/main/pdfoptions) prints a page to PDF files with different options.
- [pdfstream](https://github.com/chromedp/examples/tree/main/pdfstream) reads a printed PDF as a stream.
- [emulate](https://github.com/chromedp/examples/tree/main/emulate) emulates a device, such as an iPhone.
- [screencast](https://github.com/chromedp/examples/tree/main/screencast) saves the frames of a page as JPEG files.
- [termcast](https://github.com/chromedp/examples/tree/main/termcast) plays an animated SVG and streams the screen of the browser to the terminal with `termcast`.

Several tabs and browsers:

- [tabs](https://github.com/chromedp/examples/tree/main/tabs) uses several tabs of one browser, with or without a window for each tab, and switches between them.
- [workers](https://github.com/chromedp/examples/tree/main/workers) runs many jobs at the same time in one browser with a pool of goroutines.
- [multi](https://github.com/chromedp/examples/tree/main/multi) uses the headless-shell image in a container.
- [remote](https://github.com/chromedp/examples/tree/main/remote) connects to a browser that is already running, with the module `remote`.

The program [fast](https://github.com/chromedp/examples/tree/main/fast) reads a live site, `fast.com`, and draws an image in the terminal. It can fail when the site changes. The programs [forecast](https://github.com/chromedp/examples/tree/main/forecast) and [geoip](https://github.com/chromedp/examples/tree/main/geoip) also draw an image in the terminal. Run these programs in a terminal that can show images.
The tags of the examples repository follow the tags of `chromedp`.

## Show the browser in the terminal

[`termcast`][termcast] streams the screen of a `chromedp` page to a terminal
that can show images (Kitty, iTerm2 or Sixel). It uses the screencast of the
Chrome DevTools Protocol, so it works with a headless browser, also over `ssh`.
Start the browser first, and then start the stream with the context.

```go
ctx, cancel := chromedp.NewContext(context.Background())
defer cancel()
chromedp.Do(ctx, chromedp.Navigate("https://example.com")) // starts the browser
s, err := termcast.Start(ctx)                               // draws 4 frames each second
if err != nil {
	log.Fatal(err) // termcast.ErrNoGraphics when the terminal has no graphics
}
defer s.Stop()
```

The stream clears the terminal at each redraw. It holds the log lines while it
runs, and prints them after the final frame. The type `termcast.Flags` adds the
flags `-visible-on-terminal` and `-terminal-fps` to a program, as the programs
in the [examples][chromedp-examples] repository have them.

## Visible browser

By default, `chromedp` runs Chrome in headless mode, so no window opens. To see
the browser, add `WithVisibleWindow`. Set the variable `CHROMEDP_VISIBLEWINDOW=1`
to get the same result with no change in the code. On Linux, a visible window
needs `DISPLAY` or `WAYLAND_DISPLAY`. Without them, the first `Run` returns
`chromedp.ErrNoDisplay`.

```go
ctx, cancel := chromedp.NewContext(context.Background(), chromedp.WithVisibleWindow())
defer cancel()
chromedp.Do(ctx, chromedp.Navigate("https://example.com"))
chromedp.WaitClosed(ctx) // block until the user closes the window
```

To leave the browser open after the program ends, add `remote.WithKeepOpen` from
the module `github.com/chromedp/chromedp/remote`. The program can print the
address, and another program can attach to it with `remote.NewAllocator`. The
profile directory stays on disk, and you must delete it yourself.

```go
ctx, _ := chromedp.NewContext(context.Background(), chromedp.WithVisibleWindow(), remote.WithKeepOpen())
chromedp.Do(ctx, chromedp.Navigate("https://example.com"))
wsURL, profile := chromedp.KeptOpen(ctx)
fmt.Println("attach to", wsURL, "profile", profile)
// The program ends here. The browser stays open.
```

## Remote browser

To connect to a browser that runs already, such as a container or a hosted
service, use `remote.NewAllocator` with the websocket address of the browser:

```go
allocCtx, cancel := remote.NewAllocator(context.Background(), "ws://127.0.0.1:9222/")
defer cancel()
ctx, cancel := chromedp.NewContext(allocCtx)
defer cancel()
title, err := chromedp.Run(ctx, chromedp.Title())
```

A hosted service can need a header on the websocket request. Give the full
address of the browser and the options `remote.NoModifyURL` and
`remote.WithDialHTTPHeader`.

## Frequently Asked Questions

> I cannot see any Chrome browser window

By default, `chromedp` runs Chrome in headless mode. See
[Visible browser](#visible-browser). See also
`DefaultExecAllocatorOptions`, and see [an example][goref-chromedp-exec-allocator]
that overrides the default options.

> I see "context canceled" errors

When the connection to the browser is lost, `chromedp` cancels the context. This
can cause the error. It happens, for example, when someone closes the browser by
hand, or when something kills the browser process. When the browser process
dies on its own, the error also holds the exit error of the process. Use
`errors.As` with an `*exec.ExitError` to read the signal or the exit status.

> How does chromedp talk to the browser it starts?

By default, through a pipe. `chromedp` starts Chrome with `--remote-debugging-pipe`
and two pipes, so Chrome opens no debugging port. On Windows the pipes are
handles that the switch `--remote-debugging-io-pipes` names. To use a
websocket and a debugging port instead, add the `remote.WebSocket` option of the
module `github.com/chromedp/chromedp/remote` to the exec allocator. A
`remote-debugging-port` or `remote-debugging-address` flag also selects the
websocket, and so does `KeepOpen`, and they all need `remote.WebSocket`. A
program for Windows needs no `remote` module for the default pipe.

> Chrome exits as soon as my Go program finishes

On Linux, `chromedp` kills the Chrome child processes that it started, so that no
resources leak. To leave Chrome open, add `remote.WithKeepOpen`. See
[Visible browser](#visible-browser). You can also start Chrome yourself and
connect with `remote.NewAllocator`.

> Calling an action or a command results in "invalid context"

`chromedp.Do`, `chromedp.Run` and `chromedp.Call` need a context that came from
`chromedp.NewContext`, because the context holds the browser and the tab. Any
other context gives `ErrInvalidContext`.

> How do I send a protocol command that has no action?

Call it with `chromedp.Call`, which runs the command on the tab of the context:

```go
ctx, cancel := chromedp.NewContext(context.Background())
defer cancel()

res, err := chromedp.Call(ctx, page.GetFrameTree, cdp.Empty{})
```

Inside an action, call `cdp.Call(ctx, t, command, params)` with the target `t`
that the action receives. The target is the tab of the context. The commands and
their parameter structs are in `github.com/chromedp/cdproto`.

> I have an action of the old kind

Wrap it with `chromedp.Legacy`. The old kind is a value with a `Do(context.Context) error`
method.

> Is it safe to use one context from several goroutines?

Contexts that share one browser run in separate tabs, and they are safe to use
in parallel. Call `chromedp.Run` once on a parent context so that it has a
browser, and then make a child context for each goroutine with
`chromedp.NewContext`. A child of a context that has no browser yet starts a
browser of its own.

Do not share one context between goroutines. The actions of one context run in
one tab, so they can race. For example, two `Navigate` actions on the same tab
can fail. The first `Run` on a context starts the browser, and it must
not run at the same time as another `Run` on that context.

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

> I cannot reach an element or the network events of an iframe

Chrome runs an iframe from another site (a cross-site iframe) in its own
process, as a separate target of the type `iframe`. The DOM tree of the page does
not hold its content. The node of the `iframe` element has no `ContentDocument`,
so a query with `FromNode` finds nothing and waits until the context ends.
`chromedp` does not attach to the iframe target by itself, so the network events
of the iframe do not reach the context of the page. An iframe from the same site
is not affected.

To work in the iframe, find its target with `chromedp.Targets` and attach to it
with `chromedp.WithTargetID`. The new context runs actions in the iframe and
gets its events. The parent context must have run once, so that it has a
browser.

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

The other way is to turn off site isolation, so that the iframe stays in the
process of the page. Add `Flag("disable-features", "SitePerProcess,IsolateOrigins")`
and `Flag("disable-site-isolation-trials", true)` to the options of the exec
allocator. Then `FromNode` and the events of the page reach the iframe. This
turns off a security feature of Chrome. Use it only for pages that you trust. See
[`docs/decisions/2026-10-04-keep-site-isolation-on.md`](docs/decisions/2026-10-04-keep-site-isolation-on.md).

> SendKeys with a new line inserts two line breaks

`SendKeys` sends the character `\n` as the Enter key, in the form `\r`. That is a
keyDown event, a char event with the text `\r`, and a keyUp event. The key
`kb.Enter` is the same as `\r`, so it sends the same events and does not help. An
editor that handles Enter on keydown and cancels the default, such as Lexical,
can insert two line breaks, because the char event inserts one more. To send the
Enter key with no char event, send a rawKeyDown event and a keyUp event with
`input.DispatchKeyEvent`, and send the text before and after it with `SendKeys`:

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

> The browser closes when the timeout of my context ends

The first `Run` on a context starts the browser, and it binds the life of the
browser to the context that you pass to that call. When that context ends, the
browser stops. So a context from `context.WithTimeout` that you use for the
first `Run` closes the browser when the timeout ends, and every later call on the
parent context fails with `context canceled`.

To limit one action, start the browser first with a context that has no timeout.
Then run the action with a context that you derive from it. When the timeout
ends, `Run` returns an error that wraps `context.DeadlineExceeded`. The tab and
the browser stay open, and `ctx` still works. `Run` has no option for the timeout
of one action.

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

> Why does chromedp pass disable-dev-shm-usage, and can I turn it off?

`DefaultExecAllocatorOptions` sets `disable-dev-shm-usage` to true. Chrome then
keeps the files of its shared memory in the temporary directory, such as `/tmp`,
and not in `/dev/shm`. The flag is on because `/dev/shm` is small in many
containers (64 MB in a default Docker container), and Chrome crashes when it
fills the space.

The cost is that the shared memory files are file-backed memory in the temporary
directory. They can be slower, and a program that runs for a long time and opens
many pages can see growing memory. If `/dev/shm` is large enough, for example a
memory volume that you mount in a container, turn the flag off after the default
options:

```go
opts := append(chromedp.DefaultExecAllocatorOptions[:],
	chromedp.Flag("disable-dev-shm-usage", false),
)
allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
defer cancel()
```

> I want to use chromedp on a headless environment

Run the Go program that uses `chromedp` inside the
[chromedp/headless-shell][docker-headless-shell] image. The image has
`headless-shell`, a smaller headless build of Chrome. `chromedp` finds it by
default.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before you send a change.
[AGENTS.md](AGENTS.md) holds the rules for people and coding agents. The tests
need Chrome or the `headless-shell` image. The repository holds three modules, and
[AGENTS.md](AGENTS.md) tells how to test each one.

These documents are in `docs/`:

| Document | Holds |
| --- | --- |
| [`docs/PLAN.md`](docs/PLAN.md) | the purpose, the architecture and the open questions |
| [`docs/PROGRESS.md`](docs/PROGRESS.md) | where the work stands |
| [`docs/BACKLOG.md`](docs/BACKLOG.md) | known work that is not done |
| [`docs/API.md`](docs/API.md) | the new API, with old and new code side by side |
| [`docs/MIGRATION.md`](docs/MIGRATION.md) | every renamed and removed name |
| [`docs/decisions/README.md`](docs/decisions/README.md) | the index of every recorded decision |

## Questions and ideas

Ask a question, or suggest a feature, in [Discussions][discussions]. The issue
tracker is for bugs. You can also chat on [Discord][discord].

## Resources

* [`headless-shell`][docker-headless-shell] - A build of `headless-shell` that the tests use
* [chromedp: A New Way to Drive the Web][gophercon-2017-presentation] - GopherCon SG 2017 talk
* [Chrome DevTools Protocol][devtools-protocol] - Chrome DevTools Protocol reference
* [chromedp examples][chromedp-examples] - More complicated examples for `chromedp`
* [`github.com/chromedp/cdproto`][goref-cdproto] - Go reference for the generated Chrome DevTools Protocol API
* [`github.com/chromedp/pdlgen`][chromedp-pdlgen] - tool used to generate `cdproto`
* [`github.com/chromedp/chromedp-proxy`][chromedp-proxy] - a simple CDP proxy for logging CDP clients and browsers

[chromedp-ci]: https://github.com/chromedp/chromedp/actions/workflows/test.yml (Test CI)
[chromedp-ci-status]: https://github.com/chromedp/chromedp/actions/workflows/test.yml/badge.svg (Test CI)
[chromedp-examples]: https://github.com/chromedp/examples
[termcast]: https://github.com/chromedp/termcast
[chromedp-pdlgen]: https://github.com/chromedp/pdlgen
[chromedp-proxy]: https://github.com/chromedp/chromedp-proxy
[discussions]: https://github.com/chromedp/chromedp/discussions
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"
[devtools-protocol]: https://chromedevtools.github.io/devtools-protocol/
[docker-headless-shell]: https://hub.docker.com/r/chromedp/headless-shell/
[gophercon-2017-presentation]: https://www.youtube.com/watch?v=_7pWCg94sKw
[goref-cdproto]: https://pkg.go.dev/github.com/chromedp/cdproto
[goref-chromedp-exec-allocator]: https://pkg.go.dev/github.com/chromedp/chromedp#example-ExecAllocator
[goref-chromedp]: https://pkg.go.dev/github.com/chromedp/chromedp
[goref-chromedp-status]: https://pkg.go.dev/badge/github.com/chromedp/chromedp.svg
[release-status]: https://img.shields.io/github/v/release/chromedp/chromedp?display_name=tag&sort=semver (Latest Release)
[releases]: https://github.com/chromedp/chromedp/releases (Releases)
