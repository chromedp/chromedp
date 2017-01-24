package main

import (
	"context"
	"log"
	"time"

	cdp "github.com/knq/chromedp"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := cdp.New(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	err = c.Run(ctxt, submit(`https://brank.as/`, `#outro-invite-form input[name="email"]`))
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
}

func submit(urlstr, sel string) cdp.Tasks {
	return cdp.Tasks{
		cdp.Navigate(urlstr),
		cdp.Sleep(2 * time.Second),
		cdp.WaitVisible(sel, cdp.ByQuery),
		cdp.WaitNotVisible(`div.v-middle > div.la-ball-clip-rotate`, cdp.ByQuery),
		cdp.Submit(sel, cdp.ElementVisible, cdp.ByQuery),
		cdp.Sleep(120 * time.Second),
	}
}
