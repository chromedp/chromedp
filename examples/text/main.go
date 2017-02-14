package main

import (
	"context"
	"log"

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
	var res string
	err = c.Run(ctxt, text(&res))
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

	log.Printf("overview: %s", res)
}

func text(res *string) cdp.Tasks {
	return cdp.Tasks{
		cdp.Navigate(`https://golang.org/pkg/time/`),
		cdp.Text(`#pkg-overview`, res, cdp.NodeVisible, cdp.ByID),
	}
}
