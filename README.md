# About chromedp

Package `chromedp` is a faster, simpler way to drive browsers supporting the
[Chrome DevTools Protocol][devtools-protocol] in Go without external dependencies.

[![Unit Tests][chromedp-ci-status]][chromedp-ci]
[![Go Reference][goref-chromedp-status]][goref-chromedp]

## Installing

Install in the usual Go way:

```sh
$ go get -u github.com/chromedp/chromedp
```

## Examples

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
it may result in this error. This occurs, for example, if the browser is closed
manually, or if the browser process has been killed or otherwise terminated.

> Chrome exits as soon as my Go program finishes

On Linux, `chromedp` is configured to avoid leaking resources by force-killing
any started Chrome child processes. If you need to launch a long-running Chrome
instance, manually start Chrome and connect using `RemoteAllocator`.

> Executing an action without `Run` results in "invalid context"

By default, a `chromedp` context does not have an executor, however one can be
specified manually if necessary; see [issue #326][github-326]
for an example.

> I can't use an `Action` with `Run` because it returns many values

Wrap it with an `ActionFunc`:

```go
ctx, cancel := chromedp.NewContext()
defer cancel()
chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
	_, err := domain.SomeAction().Do(ctx)
	return err
}))
```

> I want to use chromedp on a headless environment

The simplest way is to run the Go program that uses chromedp inside the
[chromedp/headless-shell][docker-headless-shell] image. That image contains
`headless-shell`, a smaller headless build of Chrome, which `chromedp` is able
to find out of the box.

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
[devtools-protocol]: https://chromedevtools.github.io/devtools-protocol/
[docker-headless-shell]: https://hub.docker.com/r/chromedp/headless-shell/
[github-326]: https://github.com/chromedp/chromedp/issues/326
[gophercon-2017-presentation]: https://www.youtube.com/watch?v=_7pWCg94sKw
[goref-cdproto]: https://pkg.go.dev/github.com/chromedp/cdproto
[goref-chromedp-exec-allocator]: https://pkg.go.dev/github.com/chromedp/chromedp#example-ExecAllocator
[goref-chromedp]: https://pkg.go.dev/github.com/chromedp/chromedp
[goref-chromedp-status]: https://pkg.go.dev/badge/github.com/chromedp/chromedp.svg
