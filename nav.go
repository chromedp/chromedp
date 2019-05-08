package chromedp

import (
	"context"
	"errors"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
)

// Navigate navigates the current frame.
func Navigate(urlstr string) Action {
	return ActionFunc(func(ctx context.Context) error {
		ch := listenLoaded(ctx)
		frameID, _, _, err := page.Navigate(urlstr).Do(ctx)
		if err != nil {
			return err
		}
		ch <- frameID
		<-ch
		return nil
	})
}

// listenLoaded sets up a listener before running an action that will load a
// frame, so that later we can block until said frame has finished loading. A
// channel is used to receive the frame ID to wait for and to block, since
// page.Navigate returns the ID, but the listener must be set up before.
func listenLoaded(ctx context.Context) chan cdp.FrameID {
	ch := make(chan cdp.FrameID)
	ctx, cancel := context.WithCancel(ctx)
	var frameID cdp.FrameID
	ListenTarget(ctx, func(ev interface{}) {
		evs, ok := ev.(*page.EventFrameStoppedLoading)
		if !ok {
			return
		}
		if frameID == "" {
			frameID = <-ch
		}
		if evs.FrameID == frameID {
			cancel()
			close(ch)
		}
	})
	return ch
}

// NavigationEntries is an action to retrieve the page's navigation history
// entries.
func NavigationEntries(currentIndex *int64, entries *[]*page.NavigationEntry) Action {
	if currentIndex == nil || entries == nil {
		panic("currentIndex and entries cannot be nil")
	}

	return ActionFunc(func(ctx context.Context) error {
		var err error
		*currentIndex, *entries, err = page.GetNavigationHistory().Do(ctx)
		return err
	})
}

// NavigateToHistoryEntry is an action to navigate to the specified navigation
// entry.
func NavigateToHistoryEntry(entryID int64) Action {
	return ActionFunc(func(ctx context.Context) error {
		ch := listenLoaded(ctx)
		frameID := FromContext(ctx).Target.cur.ID
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		ch <- frameID
		<-ch
		return nil
	})
}

// NavigateBack navigates the current frame backwards in its history.
func NavigateBack() Action {
	return ActionFunc(func(ctx context.Context) error {
		cur, entries, err := page.GetNavigationHistory().Do(ctx)
		if err != nil {
			return err
		}

		if cur <= 0 || cur > int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}

		ch := listenLoaded(ctx)
		frameID := FromContext(ctx).Target.cur.ID
		entryID := entries[cur-1].ID
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		ch <- frameID
		<-ch
		return nil
	})
}

// NavigateForward navigates the current frame forwards in its history.
func NavigateForward() Action {
	return ActionFunc(func(ctx context.Context) error {
		cur, entries, err := page.GetNavigationHistory().Do(ctx)
		if err != nil {
			return err
		}

		if cur < 0 || cur >= int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}

		ch := listenLoaded(ctx)
		frameID := FromContext(ctx).Target.cur.ID
		entryID := entries[cur+1].ID
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		ch <- frameID
		<-ch
		return nil
	})
}

// Reload reloads the current page.
func Reload() Action {
	return ActionFunc(func(ctx context.Context) error {
		ch := listenLoaded(ctx)
		frameID := FromContext(ctx).Target.cur.ID
		if err := page.Reload().Do(ctx); err != nil {
			return err
		}
		ch <- frameID
		<-ch
		return nil
	})
}

// Stop stops all navigation and pending resource retrieval.
func Stop() Action {
	return page.StopLoading()
}

// CaptureScreenshot captures takes a screenshot of the current viewport.
//
// Note: this an alias for page.CaptureScreenshot.
func CaptureScreenshot(res *[]byte) Action {
	if res == nil {
		panic("res cannot be nil")
	}

	return ActionFunc(func(ctx context.Context) error {
		var err error
		*res, err = page.CaptureScreenshot().Do(ctx)
		return err
	})
}

// AddOnLoadScript adds a script to evaluate on page load.
/*func AddOnLoadScript(source string, id *page.ScriptIdentifier) Action {
	if id == nil {
		panic("id cannot be nil")
	}

	return ActionFunc(func(ctx context.Context) error {
		var err error
		*id, err = page.AddScriptToEvaluateOnLoad(source).Do(ctx)
		return err
	})
}

// RemoveOnLoadScript removes a script to evaluate on page load.
func RemoveOnLoadScript(id page.ScriptIdentifier) Action {
	return page.RemoveScriptToEvaluateOnLoad(id)
}*/

// Location retrieves the document location.
func Location(urlstr *string) Action {
	if urlstr == nil {
		panic("urlstr cannot be nil")
	}

	return EvaluateAsDevTools(`document.location.toString()`, urlstr)
}

// Title retrieves the document title.
func Title(title *string) Action {
	if title == nil {
		panic("title cannot be nil")
	}

	return EvaluateAsDevTools(`document.title`, title)
}
