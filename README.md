# About chromedp [![Build Status](https://travis-ci.org/knq/chromedp.svg)](https://travis-ci.org/knq/chromedp) [![Coverage Status](https://coveralls.io/repos/knq/chromedp/badge.svg?branch=master&service=github)](https://coveralls.io/github/knq/chromedp?branch=master) #

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

Below is a simple Google search performed using chromedp (taken from
[examples/simple](examples/simple/main.go)):

This example shows logic for a simple search for a known website, clicking on
the right link, and then taking a screenshot of a specific element on the
loaded page and saving that to a local file on disk.

```go
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	cdp "github.com/knq/chromedp"
	cdptypes "github.com/knq/chromedp/cdp"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := cdp.New(ctxt, cdp.WithLog(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	var site, res string
	err = c.Run(ctxt, googleSearch("site:brank.as", "Easy Money Management", &site, &res))
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

	log.Printf("saved screenshot of #testimonials from search result listing `%s` (%s)", res, site)
}

func googleSearch(q, text string, site, res *string) cdp.Tasks {
	var buf []byte
	sel := fmt.Sprintf(`//a[text()[contains(., '%s')]]`, text)
	return cdp.Tasks{
		cdp.Navigate(`https://www.google.com`),
		cdp.Sleep(2 * time.Second),
		cdp.WaitVisible(`#hplogo`, cdp.ByID),
		cdp.SendKeys(`#lst-ib`, text +"\n", cdp.ByID),
		cdp.WaitVisible(`#res`, cdp.ByID),
		cdp.Text(sel, res),
		cdp.Click(sel),
		cdp.Sleep(2 * time.Second),
		cdp.WaitVisible(`#footer`, cdp.ByQuery),
		cdp.WaitNotVisible(`div.v-middle > div.la-ball-clip-rotate`, cdp.ByQuery),
		cdp.Location(site),
		cdp.Screenshot(`#testimonials`, &buf, cdp.ByID),
		cdp.ActionFunc(func(context.Context, cdptypes.Handler) error {
			return ioutil.WriteFile("testimonials.png", buf, 0644)
		}),
	}
}
```

Please see the [examples](examples/) directory for some more examples, and
please refer to the [GoDoc API listing](https://godoc.org/github.com/knq/chromedp)
for a summary of the API and Actions.

## Links + Resources
* [chromedp: A New Way to Drive the Web (GopherCon SG 2017)](https://www.youtube.com/watch?v=_7pWCg94sKw)
* [Chrome DevTools Protocol Viewer](https://chromedevtools.github.io/devtools-protocol/)

## TODO
* Move timeouts to context (defaults)
* Implement more query selector options (allow over riding context timeouts)
* Contextual actions for "dry run" (or via an accumulator?)
* Network loader / manager
* Profiler
