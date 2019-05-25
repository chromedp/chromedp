# About chromedp [![GoDoc][1]][2] [![Build Status][3]][4]

Package chromedp is a faster, simpler way to drive browsers supporting the
[Chrome DevTools Protocol][5] in Go using the without external dependencies
(like Selenium or PhantomJS).

## Installing

Install in the usual Go way:

	go get -u github.com/chromedp/chromedp

## Examples

Refer to the [GoDoc page][7] for the documentation and examples. The
[examples][6] repository contains more complex scenarios.

## Frequently Asked Questions

* I can't see any Chrome browser window

By default, it's run in headless mode. See `DefaultExecAllocatorOptions`, and
[an example](https://godoc.org/github.com/chromedp/chromedp#example-ExecAllocator)
to override said options.

* I'm seeing "context canceled" errors

If the connection to the browser is dropped, the context will be cancelled,
which can be an unexpected reason for this error. For example, if the browser is
closed manually.

* Chrome exits as soon as my Go program finishes

This is set up on Linux to avoid leaking resources. If you want Chrome to be a
long-running process, start it separately and connect to it via `RemoteAllocator`.

* Execute an action results in "invalid context"

By default, a chromedp context doesn't have an executor set up. You can specify
one; see #326.

* I can't use an `Action` with `Run` because it returns many values

Wrap it with an `ActionFunc`:

```
chromedp.Do(ctx, ActionFunc(func(ctx context.Context) error {
	_, err := domain.SomeAction().Do(ctx)
	return err
}))
```

## Resources

* [chromedp: A New Way to Drive the Web][8] - GopherCon SG 2017 talk
* [Chrome DevTools Protocol][5] - Chrome DevTools Protocol Domain documentation
* [chromedp examples][6] - various `chromedp` examples
* [`github.com/chromedp/cdproto`][9] - GoDoc listing for the CDP domains used by `chromedp`
* [`github.com/chromedp/cdproto-gen`][10] - tool used to generate `cdproto`
* [`github.com/chromedp/chromedp-proxy`][11] - a simple CDP proxy for logging CDP clients and browsers

[1]: https://godoc.org/github.com/chromedp/chromedp?status.svg
[2]: https://godoc.org/github.com/chromedp/chromedp
[3]: https://travis-ci.org/chromedp/chromedp.svg
[4]: https://travis-ci.org/chromedp/chromedp
[5]: https://chromedevtools.github.io/devtools-protocol/
[6]: https://github.com/chromedp/examples
[7]: https://godoc.org/github.com/chromedp/chromedp
[8]: https://www.youtube.com/watch?v=_7pWCg94sKw
[9]: https://godoc.org/github.com/chromedp/cdproto
[10]: https://github.com/chromedp/cdproto-gen
[11]: https://github.com/chromedp/chromedp-proxy
