# About chromedp

<p align="center">
  <img src="https://raw.githubusercontent.com/chromedp/logo/main/chromedp.svg" alt="chromedp logo" width="160">
</p>

Package `chromedp` is a faster, simpler way to drive browsers supporting the
[Chrome DevTools Protocol][devtools-protocol] in Go without external dependencies.

[![Unit Tests][chromedp-ci-status]][chromedp-ci]
[![Go Reference][goref-chromedp-status]][goref-chromedp]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

## Installing

Install in the usual Go way:

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

[`docs/API.md`](docs/API.md) describes the API with more than ten examples that
show the old code and the new code side by side. [`docs/MIGRATION.md`](docs/MIGRATION.md)
lists every renamed and removed name.

Refer to the [Go reference][goref-chromedp] for the documentation and examples.
Additionally, the [examples][chromedp-examples] repository contains more
examples on complex actions, and other common high-level tasks such as taking
full page screenshots.

## Frequently Asked Questions

> I can't see any Chrome browser window

By default, Chrome is run in headless mode. See `DefaultExecAllocatorOptions`, and
[an example][goref-chromedp-exec-allocator] to override the default options.

> I'm seeing "context canceled" errors

When the connection to the browser is lost, `chromedp` cancels the context, and
it can result in this error. This occurs, for example, if the browser is closed
manually, or if the browser process was killed or otherwise terminated.

> How does chromedp talk to the browser it starts?

By default, through a pipe. `chromedp` starts Chrome with `--remote-debugging-pipe`
and uses two extra file descriptors, so Chrome opens no debugging port. To use a
websocket and a debugging port instead, add the `chromedp.WebSocket` option to the
exec allocator. A `remote-debugging-port` or `remote-debugging-address` flag also
selects the websocket. On Windows `chromedp` always uses the websocket.

> Chrome exits as soon as my Go program finishes

On Linux, `chromedp` is configured to avoid leaking resources by force-killing
any started Chrome child processes. If you need to launch a long-running Chrome
instance, manually start Chrome and connect using `RemoteAllocator`.

> Calling an action or a command results in "invalid context"

`chromedp.Do`, `chromedp.Run` and `chromedp.Call` need a context that came from
`chromedp.NewContext`, because the context holds the browser and the tab.
A context that did not come from `chromedp.NewContext` gives `ErrInvalidContext`.

> How do I send a protocol command that has no action?

Call it with `chromedp.Call`, which runs the command on the tab of the context:

```go
ctx, cancel := chromedp.NewContext(context.Background())
defer cancel()

res, err := chromedp.Call(ctx, page.GetFrameTree, cdp.Empty{})
```

Inside an action, call `cdp.Call(ctx, t, command, params)` with the tab `t` that
the action receives. The commands and their parameter structs are in
`github.com/chromedp/cdproto`.

> I have an action of the old kind

Wrap it with `chromedp.Legacy`. The old kind is a value with a `Do(context.Context) error`
method.

> I want to use chromedp on a headless environment

The simplest way is to run the Go program that uses chromedp inside the
[chromedp/headless-shell][docker-headless-shell] image. That image contains
`headless-shell`, a smaller headless build of Chrome, which `chromedp` finds
out of the box.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before you send a change. The rules
for people and coding agents are in [AGENTS.md](AGENTS.md). The plan, the
backlog and every recorded decision are in [docs/](docs/PLAN.md). Running the
tests needs Chrome or the `headless-shell` image.

## Questions and ideas

Ask a question, or suggest a feature, in [Discussions][discussions]. The issue
tracker is for bugs. You can also chat on [Discord][discord].

## Resources

* [`headless-shell`][docker-headless-shell] - A build of `headless-shell` that is used for testing `chromedp`
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
