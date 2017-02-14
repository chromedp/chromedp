package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	cdp "github.com/knq/chromedp"
	cdptypes "github.com/knq/chromedp/cdp"
	rundom "github.com/knq/chromedp/cdp/runtime"
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
	err = c.Run(ctxt, visible())
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

func visible() cdp.Tasks {
	var res *rundom.RemoteObject
	return cdp.Tasks{
		cdp.Navigate("file:" + os.Getenv("GOPATH") + "/src/github.com/knq/chromedp/testdata/visible.html"),
		cdp.Evaluate(makeVisibleScript, &res),
		cdp.ActionFunc(func(context.Context, cdptypes.Handler) error {
			log.Printf(">>> res: %+v", res)
			return nil
		}),
		cdp.WaitVisible(`#box1`),
		cdp.ActionFunc(func(context.Context, cdptypes.Handler) error {
			log.Printf(">>>>>>>>>>>>>>>>>>>> BOX1 IS VISIBLE")
			return nil
		}),
		cdp.WaitVisible(`#box2`),
		cdp.ActionFunc(func(context.Context, cdptypes.Handler) error {
			log.Printf(">>>>>>>>>>>>>>>>>>>> BOX2 IS VISIBLE")
			return nil
		}),
		cdp.ActionFunc(func(context.Context, cdptypes.Handler) error {
			log.Printf(">>>>>>>>>>>>>>>>>>>> WAITING TO EXIT")
			time.Sleep(150 * time.Second)
			return errors.New("exiting")
		}),
	}
}

const (
	makeVisibleScript = `setTimeout(function() {
	document.querySelector('#box1').style.display = '';
}, 30000);`
)
