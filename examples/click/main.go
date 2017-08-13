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
	c, err := cdp.New(ctxt, cdp.WithLog(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	err = c.Run(ctxt, click())
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

func click() cdp.Tasks {
	return cdp.Tasks{
		cdp.Navigate(`https://golang.org/pkg/time/`),
		cdp.WaitVisible(`#footer`),
		cdp.Click(`#pkg-overview`, cdp.NodeVisible),
		cdp.Sleep(150 * time.Second),
	}
}
