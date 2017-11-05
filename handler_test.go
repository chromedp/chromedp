package chromedp

import (
	"testing"
	"context"
	"log"
	"github.com/knq/chromedp/cdp"
)

func TestTargetHandler_Listen(t *testing.T) {
	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := New(ctxt, WithLog(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	err = c.Run(ctxt, Tasks{
		ActionFunc(func(ctxt context.Context, h cdp.Handler) error {
			echan := h.Listen(cdp.EventNetworkRequestWillBeSent)
			th := h.(*TargetHandler)
			if chs, ok := th.lsnr[cdp.EventNetworkRequestWillBeSent]; ok {
				if len(chs) != 1 {
					t.Fatal("len(chs) != 1")
				}
				if chs[0] != echan {
					t.Fatal("chs[0] != echan ")
				}
			} else {
				t.Fatal("th.lsnr[cdp.EventNetworkRequestWillBeSent] !ok")
			}
			if len(th.lsnrchs) != 1 {
				t.Fatal("len(th.lsnrchs) != 1")
			}
			if len(th.lsnrchs[echan]) != 1 {
				t.Fatal("len(th.lsnrchs[echan]) != 1")
			}
			if !th.lsnrchs[echan][cdp.EventNetworkRequestWillBeSent] {
				t.Fatal("!th.lsnrchs[echan][cdp.EventNetworkRequestWillBeSent]")
			}

			h.Release(echan)
			if _, ok := <-echan; ok {
				t.Fatal("<-echan; ok")
			}
			if chs, ok := th.lsnr[cdp.EventNetworkRequestWillBeSent]; ok && len(chs) > 0 {
				t.Fatal("th.lsnr[cdp.EventNetworkRequestWillBeSent]; ok && len(chs) > 0")
			}
			if len(th.lsnrchs) != 0 {
				t.Fatal("len(th.lsnrchs) != 0")
			}
			return nil
		}),
	})

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
