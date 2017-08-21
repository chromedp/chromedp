package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	cdp "github.com/knq/chromedp"
	cdptypes "github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/network"
)

var (
	flagPort = flag.Int("port", 8544, "port")
)

func main() {
	var err error

	flag.Parse()

	// setup http server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		buf, err := json.MarshalIndent(req.Cookies(), "", "  ")
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(res, indexHTML, string(buf))
	})
	go http.ListenAndServe(fmt.Sprintf(":%d", *flagPort), mux)

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
	err = c.Run(ctxt, setcookies(fmt.Sprintf("http://localhost:%d", *flagPort), &res))
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

	log.Printf("passed cookies: %s", res)
}

func setcookies(host string, res *string) cdp.Tasks {
	return cdp.Tasks{
		cdp.ActionFunc(func(ctxt context.Context, h cdptypes.Handler) error {
			expr := cdptypes.TimeSinceEpoch(time.Now().Add(180 * 24 * time.Hour))
			success, err := network.SetCookie("cookiename", "cookievalue").
				WithExpires(&expr).
				WithDomain("localhost").
				WithHTTPOnly(true).
				Do(ctxt, h)
			if err != nil {
				return err
			}
			if !success {
				return errors.New("could not set cookie")
			}
			return nil
		}),
		cdp.Navigate(host),
		cdp.Text(`#result`, res, cdp.ByID, cdp.NodeVisible),
		cdp.ActionFunc(func(ctxt context.Context, h cdptypes.Handler) error {
			cookies, err := network.GetAllCookies().Do(ctxt, h)
			if err != nil {
				return err
			}

			for i, cookie := range cookies {
				log.Printf("cookie %d: %+v", i, cookie)
			}

			return nil
		}),
	}
}

const (
	indexHTML = `<!doctype html>
<html>
<body>
  <div id="result">%s</div>
</body>
</html>`
)
