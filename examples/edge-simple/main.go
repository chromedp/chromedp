// +build windows

package main

import (
	"context"
	"log"

	"github.com/knq/chromedp"
	"github.com/knq/chromedp/client"
	//"github.com/knq/chromedp/runner"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create edge instance -- FIXME: not able to launch separate process (yet)
	/*cdp, err := chromedp.New(ctxt, chromedp.WithRunnerOptions(
		runner.EdgeDiagnosticsAdapter(),
	))*/

	// create edge instance
	watch := client.New().WatchPageTargets(ctxt)
	if err != nil {
		log.Fatal(err)
	}
	cdp, err := chromedp.New(ctxt, chromedp.WithTargets(watch))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	var res string
	err = googleSearch(cdp, "site:brank.as", &res).Do(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("got attributes: `%s`", res)

	// shutdown chrome
	return
	err = cdp.Shutdown(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	err = cdp.Wait()
	if err != nil {
		log.Fatal(err)
	}
}

func googleSearch(c *chromedp.CDP, q string, res *string) chromedp.Tasks {
	return chromedp.Tasks{
		c.Navigate(`https://www.google.com`),
		c.WaitVisible(`#hplogo`),
		c.AttributeValue(`#hplogo`, "title", res, nil),
	}
}
