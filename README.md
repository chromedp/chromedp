# About chromedp

Package chromedp is a faster, simpler way to drive browsers in Go using the
[Chrome Debugging Protocol](https://developer.chrome.com/devtools/docs/debugger-protocol)
(for Chrome, Edge, Safari, etc) without external dependencies (ie, Selenium, PhantomJS, etc).

**NOTE:** chromedp's API is currently unstable, and may change at a moments
notice. There are likely extremely bad bugs lurking in this code. **CAVEAT USER**.

## Installation

Install in the usual way:

```sh
go get -u github.com/knq/chromedp
```

## Usage

Please see the [examples](examples/) directory for examples.

## TODO
* Move timeouts to context (defaults)
* Implement more query selector options (allow over riding context timeouts)
* Contextual actions for "dry run" (or via an accumulator?)
* Network loader / manager
* More examples
* Profiler
* Unit tests / coverage: travis-ci + coveralls integration
