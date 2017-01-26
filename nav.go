package chromedp

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/page"
	rundom "github.com/knq/chromedp/cdp/runtime"
)

// Navigate navigates the current frame.
func Navigate(urlstr string) Action {
	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		frameID, err := page.Navigate(urlstr).Do(ctxt, h)
		if err != nil {
			return err
		}

		return h.SetActive(ctxt, frameID)
	})
}

// NavigationEntries is an action to retrieve the page's navigation history
// entries.
func NavigationEntries(currentIndex *int64, entries *[]*page.NavigationEntry) Action {
	if currentIndex == nil || entries == nil {
		panic("currentIndex and entries cannot be nil")
	}

	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		var err error
		*currentIndex, *entries, err = page.GetNavigationHistory().Do(ctxt, h)
		return err
	})
}

// NavigateToHistoryEntry is an action to navigate to the specified navigation
// entry.
func NavigateToHistoryEntry(entryID int64) Action {
	return page.NavigateToHistoryEntry(entryID)
}

// NavigateBack navigates the current frame backwards in its history.
func NavigateBack(ctxt context.Context, h cdp.FrameHandler) error {
	cur, entries, err := page.GetNavigationHistory().Do(ctxt, h)
	if err != nil {
		return err
	}

	i := 0
	for ; i < len(entries); i++ {
		if entries[i].ID == cur {
			break
		}
	}

	if i == 0 {
		return errors.New("already on oldest navigation entry")
	}

	return page.NavigateToHistoryEntry(entries[i-1].ID).Do(ctxt, h)
}

// NavigateForward navigates the current frame forwards in its history.
func NavigateForward(ctxt context.Context, h cdp.FrameHandler) error {
	cur, entries, err := page.GetNavigationHistory().Do(ctxt, h)
	if err != nil {
		return err
	}

	i := len(entries) - 1
	for ; i > 0; i-- {
		if entries[i].ID == cur {
			break
		}
	}

	if i == len(entries)-1 {
		return errors.New("already on newest navigation entry")
	}

	return page.NavigateToHistoryEntry(entries[i+1].ID).Do(ctxt, h)
}

// CaptureScreenshot captures takes a full page screenshot.
func CaptureScreenshot(res *[]byte) Action {
	if res == nil {
		panic("res cannot be nil")
	}

	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		var err error
		*res, err = page.CaptureScreenshot().Do(ctxt, h)
		return err
	})
}

// AddOnLoadScript adds a script to evaluate on page load.
func AddOnLoadScript(source string, id *page.ScriptIdentifier) Action {
	if id == nil {
		panic("id cannot be nil")
	}

	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		var err error
		*id, err = page.AddScriptToEvaluateOnLoad(source).Do(ctxt, h)
		return err
	})
}

// RemoveOnLoadScript removes a script to evaluate on page load.
func RemoveOnLoadScript(id page.ScriptIdentifier) Action {
	return page.RemoveScriptToEvaluateOnLoad(id)
}

// Stop stops all navigation and pending resource retrieval.
func Stop() Action {
	return page.StopLoading()
}

// Evaluate evaluates a script.
func Evaluate(expression string, res **rundom.RemoteObject) Action {
	if res == nil {
		panic("res cannot be nil")
	}

	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		v, exp, err := rundom.Evaluate(expression).Do(ctxt, h)
		if err != nil {
			return err
		}
		if exp != nil {
			log.Printf(">>> GOT EXECPTION: %v", exp)
		}
		*res = v
		return nil
	})
}

// Location retrieves the URL location.
func Location(urlstr *string) Action {
	if urlstr == nil {
		panic("urlstr cannot be nil")
	}
	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		res, exp, err := rundom.Evaluate(`location.toString()`).Do(ctxt, h)
		if err != nil {
			return err
		}
		if exp != nil {
			return fmt.Errorf("got exception evaluating script: %#v", exp)
		}
		if res.Type != rundom.TypeString || len(res.Value) < 2 {
			return fmt.Errorf("expected string of at least length 2, got %s length %d", res.Subtype, len(res.Value))
		}

		*urlstr = string(res.Value[1 : len(res.Value)-1])

		return nil
	})
}
