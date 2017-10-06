// examples/simple/main.go
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

func googleSearch(q, text string, site, res *string) cdp.Tasks {
	var buf []byte
	sel := fmt.Sprintf(`//a[text()[contains(., '%s')]]`, text)
	return cdp.Tasks{
		cdp.Navigate(`https://www.google.com`),
		cdp.WaitVisible(`#hplogo`, cdp.ByID),
		cdp.SendKeys(`#lst-ib`, q+"\n", cdp.ByID),
		cdp.WaitVisible(`#res`, cdp.ByID),
		cdp.Text(sel, res),
		cdp.Click(sel),
		cdp.WaitVisible(`a[href="/brankas-for-business"]`, cdp.ByQuery),
		cdp.WaitNotVisible(`.preloader-content`, cdp.ByQuery),
		cdp.Location(site),
		cdp.ScrollIntoView(`.banner-section.third-section`, cdp.ByQuery),
		cdp.Sleep(2 * time.Second), // wait for animation to finish
		cdp.Screenshot(`.banner-section.third-section`, &buf, cdp.ByQuery),
		cdp.ActionFunc(func(context.Context, cdptypes.Handler) error {
			return ioutil.WriteFile("screenshot.png", buf, 0644)
		}),
	}
}
