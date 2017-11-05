package chromedp

import (
	"testing"
	"context"
	"github.com/knq/chromedp/cdp"
)

func TestTargetHandler_Listen(t *testing.T) {
	t.Parallel()
	c := testAllocate(t, "input.html")
	defer c.Release()

	// run task list
	err := c.Run(defaultContext, Tasks{
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
		t.Fatal(err)
	}
}
