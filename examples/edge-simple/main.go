// +build windows

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	cdp "github.com/knq/cdp"
	cdptypes "github.com/knq/cdp/cdp"
	"github.com/knq/cdp/client"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create edge instance -- FIXME: not able to launch separate process (yet)
	/*cdp, err := cdp.New(ctxt, cdp.WithRunnerOptions(
		runner.EdgeDiagnosticsAdapter(),
	))*/

	// create edge instance
	watch := client.New().WatchPageTargets(ctxt)
	c, err := cdp.New(ctxt, cdp.WithTargets(watch), cdp.WithLog(log.Printf))
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
		cdp.SendKeys(`#lst-ib`, q+"\n", cdp.ByID),
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
