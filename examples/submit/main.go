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
	err = c.Run(ctxt, submit(`https://github.com/search`, `//input[@name="q"]`, `chromedp`, &res))
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

	log.Printf("got: `%s`", res)
}

func submit(urlstr, sel, q string, res *string) cdp.Tasks {
	return cdp.Tasks{
		cdp.Navigate(urlstr),
		cdp.WaitVisible(sel),
		cdp.SendKeys(sel, q),
		cdp.Submit(sel),
		cdp.WaitNotPresent(`//*[@id="code_search"]/h2/svg`),
		cdp.Text(`//*[@id="js-pjax-container"]/div[2]/div/div[2]/ul/li/p`, res),
	}
}
