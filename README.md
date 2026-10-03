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
v0.17.0 uses the typed `cdproto` v0.157.3 and has the generic and iterator API.
The earlier versions of `cdproto`, v0.157.0, v0.157.1 and v0.157.2, have the old API.

```sh
go get -u github.com/chromedp/chromedp
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

See the [Go reference][goref-chromedp] for the documentation and examples. The
[examples][chromedp-examples] repository has more examples of complex actions
and of other common tasks, such as full page screenshots.

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

To leave the browser open after the program ends, add `WithKeepOpen`. The
program can print the address, and another program can attach to it with
`NewRemoteAllocator`. The profile directory stays on disk, and you must delete
it yourself.

```go
ctx, _ := chromedp.NewContext(context.Background(), chromedp.WithVisibleWindow(), chromedp.WithKeepOpen())
chromedp.Do(ctx, chromedp.Navigate("https://example.com"))
wsURL, profile := chromedp.KeptOpen(ctx)
fmt.Println("attach to", wsURL, "profile", profile)
// The program ends here. The browser stays open.
```

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
and uses two extra file descriptors, so Chrome opens no debugging port. To use a
websocket and a debugging port instead, add the `chromedp.WebSocket` option to the
exec allocator. A `remote-debugging-port` or `remote-debugging-address` flag also
selects the websocket, and so does `WithKeepOpen`. On Windows `chromedp` always
uses the websocket.

> Chrome exits as soon as my Go program finishes

On Linux, `chromedp` kills the Chrome child processes that it started, so that no
resources leak. To leave Chrome open, add `chromedp.WithKeepOpen`. See
[Visible browser](#visible-browser). You can also start Chrome yourself and
connect with `RemoteAllocator`.

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

> I want to use chromedp on a headless environment

Run the Go program that uses `chromedp` inside the
[chromedp/headless-shell][docker-headless-shell] image. The image has
`headless-shell`, a smaller headless build of Chrome. `chromedp` finds it by
default.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before you send a change.
[AGENTS.md](AGENTS.md) holds the rules for people and coding agents. The tests
need Chrome or the `headless-shell` image.

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
