package main

import (
	"context"
	"log"

	cdp "github.com/igsky/chromedp"
	"github.com/igsky/chromedp/cdp/network"
	"github.com/igsky/chromedp/client"
	"time"
)

func main() {
	var statusCode float64
	var html string
	url := "https://golang.org/"

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome
	c, err := cdp.New(ctxt,
		cdp.WithTargets(client.New().WatchPageTargets(ctxt)),
		cdp.WithDefaultDomains(),
		// enable network domain, to get status code
		cdp.WithCustomDomain(network.Enable()),
		// set handler to capture status code
		cdp.WithCustomHook(
			func(msg interface{}) {
				// look for responses received
				if v, ok := msg.(*network.EventResponseReceived); ok {
					// check for right resource url
					if v.Response.URL == url {
						statusCode = v.Response.Status
					}
				}
			},
		))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	err = c.Run(ctxt, scrape(&html, url))
	if err != nil {
		log.Fatal(err)
	}

	// show results
	log.Printf("Status code: %v\n", statusCode)
	log.Printf("Html: %s", html)
}

func scrape(res *string, url string) cdp.Tasks {
	return cdp.Tasks{
		cdp.Navigate(url),
		cdp.Sleep(1 * time.Second),
		cdp.InnerHTML(`html`, res),
	}
}
