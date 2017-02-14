package main

import (
	"context"
	"io/ioutil"
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
	var buf []byte
	err = c.Run(ctxt, screenshot(`https://brank.as/`, `#contact-form`, &buf))
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

	err = ioutil.WriteFile("contact-form.png", buf, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func screenshot(urlstr, sel string, res *[]byte) cdp.Tasks {
	return cdp.Tasks{
		cdp.Navigate(urlstr),
		cdp.Sleep(2 * time.Second),
		cdp.WaitVisible(sel, cdp.ByID),
		cdp.WaitNotVisible(`div.v-middle > div.la-ball-clip-rotate`, cdp.ByQuery),
		cdp.Screenshot(sel, res, cdp.NodeVisible, cdp.ByID),
	}
}
