# About chromedp [![Build Status][1]][2]

Package chromedp is a faster, simpler way to drive browsers supporting the
[Chrome DevTools Protocol][3] in Go using the without external dependencies
(like Selenium or PhantomJS).

## Installing

Install in the usual Go way:

	go get -u github.com/chromedp/chromedp

## Examples

Refer to the [GoDoc page][5] for the documentation and examples. The
[examples][4] repository contains more complex scenarios.

## Resources

* [chromedp: A New Way to Drive the Web][6] - GopherCon SG 2017 talk
* [Chrome DevTools Protocol][3] - Chrome DevTools Protocol Domain documentation
* [chromedp examples][4] - various `chromedp` examples
* [`github.com/chromedp/cdproto`][7] - GoDoc listing for the CDP domains used by `chromedp`
* [`github.com/chromedp/cdproto-gen`][8] - tool used to generate `cdproto`
* [`github.com/chromedp/chromedp-proxy`][9] - a simple CDP proxy for logging CDP clients and browsers

[1]: https://travis-ci.org/chromedp/chromedp.svg
[2]: https://travis-ci.org/chromedp/chromedp
[3]: https://chromedevtools.github.io/devtools-protocol/
[4]: https://github.com/chromedp/examples
[5]: https://godoc.org/github.com/chromedp/chromedp
[6]: https://www.youtube.com/watch?v=_7pWCg94sKw
[7]: https://godoc.org/github.com/chromedp/cdproto
[8]: https://github.com/chromedp/cdproto-gen
[9]: https://github.com/chromedp/chromedp-proxy
