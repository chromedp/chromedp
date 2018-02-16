# About chromedp [![Build Status][1]][2] [![Coverage Status][3]][4]

Package chromedp is a faster, simpler way to drive browsers in Go using the
[Chrome Debugging Protocol][5] (for Chrome, Edge, Safari, etc) without external
dependencies (ie, Selenium, PhantomJS, etc).

**NOTE:** chromedp's API is currently unstable, and may change at a moments
notice. There are likely extremely bad bugs lurking in this code. **CAVEAT USER**.

## Installing

Install in the usual way:

```sh
go get -u github.com/chromedp/chromedp
```

## Using

Below is a simple Google search performed using chromedp (taken from
[examples/simple][6]):

This example shows logic for a simple search for a known website, clicking on
the right link, and then taking a screenshot of a specific element on the
loaded page and saving that to a local file on disk.

```go
// Command simple is a chromedp example demonstrating how to do a simple google
// search.
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := chromedp.New(ctxt, chromedp.WithLog(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	var site, res string
	err = c.Run(ctxt, googleSearch("site:brank.as", "Home", &site, &res))
	if err != nil {
		log.Fatal(err)
	}

	// shutdown chrome
	err = c.Shutdown(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	// wait for chrome to finish
	err = c.Wait()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("saved screenshot from search result listing `%s` (%s)", res, site)
}

func googleSearch(q, text string, site, res *string) chromedp.Tasks {
	var buf []byte
	sel := fmt.Sprintf(`//a[text()[contains(., '%s')]]`, text)
	return chromedp.Tasks{
		chromedp.Navigate(`https://www.google.com`),
		chromedp.WaitVisible(`#hplogo`, chromedp.ByID),
		chromedp.SendKeys(`#lst-ib`, q+"\n", chromedp.ByID),
		chromedp.WaitVisible(`#res`, chromedp.ByID),
		chromedp.Text(sel, res),
		chromedp.Click(sel),
		chromedp.WaitNotVisible(`.preloader-content`, chromedp.ByQuery),
		chromedp.WaitVisible(`a[href*="twitter"]`, chromedp.ByQuery),
		chromedp.Location(site),
		chromedp.ScrollIntoView(`.banner-section.third-section`, chromedp.ByQuery),
		chromedp.Sleep(2 * time.Second), // wait for animation to finish
		chromedp.Screenshot(`.banner-section.third-section`, &buf, chromedp.ByQuery),
		chromedp.ActionFunc(func(context.Context, cdp.Executor) error {
			return ioutil.WriteFile("screenshot.png", buf, 0644)
		}),
	}
}
```

Please see the [examples][6] project for more examples. Please refer to the
[GoDoc API listing][7] for a summary of the API and Actions.

## Resources

* [chromedp: A New Way to Drive the Web][8] - GopherCon SG 2017 talk
* [Chrome DevTools Protocol][5] - Chrome Debugging Protocol Domain documentation
* [chromedp examples][6] - various `chromedp` examples
* [`github.com/chromedp/cdproto`][9] - GoDoc listing for the CDP domains used by `chromedp`
* [`github.com/chromedp/cdproto-gen`][10] - tool used to generate `cdproto`
* [`github.com/chromedp/chromedp-proxy`][11] - a simple CDP proxy for logging/debugging CDP clients and browser instances

## TODO

* Move timeouts to context (defaults)
* Implement more query selector options (allow over riding context timeouts)
* Contextual actions for "dry run" (or via an accumulator?)
* Network loader / manager
* Profiler

[1]: https://travis-ci.org/chromedp/chromedp.svg
[2]: https://travis-ci.org/chromedp/chromedp
[3]: https://coveralls.io/repos/chromedp/chromedp/badge.svg?branch=master&service=github
[4]: https://coveralls.io/github/chromedp/chromedp?branch=master
[5]: https://chromedevtools.github.io/devtools-protocol/
[6]: https://github.com/chromedp/examples
[7]: https://godoc.org/github.com/chromedp/chromedp
[8]: https://www.youtube.com/watch?v=_7pWCg94sKw
[9]: https://godoc.org/github.com/chromedp/cdproto
[10]: https://github.com/chromedp/cdproto-gen
[11]: https://github.com/chromedp/chromedp-proxy
