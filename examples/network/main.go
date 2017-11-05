// examples/simple/main.go
package main

import (
	"context"
	"log"
	cdp "github.com/knq/chromedp"
	cdptypes "github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/network"
	"strings"
	"io/ioutil"
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
	var data []byte
	err = c.Run(ctxt, googleSearch(&data))
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

	err = ioutil.WriteFile("first_image_loaded.png", data, 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("first png image saved to first_image_loaded.png")
}

func googleSearch(data *[]byte) cdp.Tasks {
	var (
		fchan <-chan interface{}
		reqId network.RequestID
		e     error
	)
	return cdp.Tasks{
		cdp.ActionFunc(func(ctxt context.Context, h cdptypes.Handler) error {
			fchan = h.Listen(cdptypes.EventNetworkLoadingFinished)
			go func() {
				echan := h.Listen(cdptypes.EventNetworkRequestWillBeSent)
				defer h.Release(echan)
			LOOP:
				for d := range echan {
					switch d.(type) {
					case *network.EventRequestWillBeSent:
						req := d.(*network.EventRequestWillBeSent)
						if strings.HasSuffix(req.Request.URL, ".png") {
							reqId = req.RequestID
							break LOOP
						}
					}
				}
			}()
			return nil
		}),
		cdp.Navigate(`https://www.google.com`),
		cdp.ActionFunc(func(ctxt context.Context, h cdptypes.Handler) error {
			for d := range fchan {
				switch d.(type) {
				case *network.EventLoadingFinished:
					res := d.(*network.EventLoadingFinished)
					if reqId == res.RequestID {
						*data, e = network.GetResponseBody(reqId).Do(ctxt, h)
						if e != nil{
							log.Fatal(e)
						}
						return nil
					}
				}
			}
			return nil
		}),
	}
}
