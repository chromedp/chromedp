# About chromedp [![GoDoc][1]][2] [![Build Status][3]][4]

Package chromedp is a faster, simpler way to drive browsers supporting the
[Chrome DevTools Protocol][5] in Go without external dependencies (like
Selenium or PhantomJS).

## Installing

Install in the usual Go way:

	go get -u github.com/chromedp/chromedp

## Examples

Refer to the [GoDoc page][7] for the documentation and examples. Additionally,
the [examples][6] repository contains more complex examples.

## Frequently Asked Questions

> I can't see any Chrome browser window

By default, Chrome is run in headless mode. See `DefaultExecAllocatorOptions`, and
[an example](https://godoc.org/github.com/chromedp/chromedp#example-ExecAllocator)
to override the default options.

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
specified manually if necessary; see [issue #326](https://github.com/chromedp/chromedp/issues/326)
for an example.

> I can't use an `Action` with `Run` because it returns many values

Wrap it with an `ActionFunc`:

```go
chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
	_, err := domain.SomeAction().Do(ctx)
	return err
}))
```

> I want to use chromedp on a headless environment

The simplest way is to run the Go program that uses chromedp inside the
[chromedp/headless-shell][8] image. That image contains `headless-shell`, a
smaller headless build of Chrome, which `chromedp` is able to find out of the
box.

## Resources

* [chromedp: A New Way to Drive the Web][9] - GopherCon SG 2017 talk
* [Chrome DevTools Protocol][5] - Chrome DevTools Protocol Domain documentation
* [chromedp examples][6] - various `chromedp` examples
* [`github.com/chromedp/cdproto`][10] - GoDoc listing for the CDP domains used by `chromedp`
* [`github.com/chromedp/cdproto-gen`][11] - tool used to generate `cdproto`
* [`github.com/chromedp/chromedp-proxy`][12] - a simple CDP proxy for logging CDP clients and browsers

[1]: https://godoc.org/github.com/chromedp/chromedp?status.svg
[2]: https://godoc.org/github.com/chromedp/chromedp
[3]: https://travis-ci.org/chromedp/chromedp.svg
[4]: https://travis-ci.org/chromedp/chromedp
[5]: https://chromedevtools.github.io/devtools-protocol/
[6]: https://github.com/chromedp/examples
[7]: https://godoc.org/github.com/chromedp/chromedp
[8]: https://hub.docker.com/r/chromedp/headless-shell/
[9]: https://www.youtube.com/watch?v=_7pWCg94sKw
[10]: https://godoc.org/github.com/chromedp/cdproto
[11]: https://github.com/chromedp/cdproto-gen
[12]: https://github.com/chromedp/chromedp-proxy
