package chromedp

import (
	"context"
	"errors"

	"github.com/chromedp/cdproto/page"
)

// Navigate navigates the current frame.
func Navigate(urlstr string) Action {
	return ActionFunc(func(ctx context.Context) error {
		_, _, _, err := page.Navigate(urlstr).Do(ctx)
		if err != nil {
			return err
		}
		return waitLoaded(ctx)
	})
}

// waitLoaded blocks until a target receives a Page.loadEventFired.
func waitLoaded(ctx context.Context) error {
	// TODO: this function is inherently racy, as we don't run ListenTarget
	// until after the navigate action is fired. For example, adding
	// time.Sleep(time.Second) at the top of this body makes most tests hang
	// forever, as they miss the load event.
	//
	// However, setting up the listener before firing the navigate action is
	// also racy, as we might get a load event from a previous navigate.
	//
	// For now, the second race seems much more common in real scenarios, so
	// keep the first approach. Is there a better way to deal with this?
	ch := make(chan bool)
	lctx, cancel := context.WithCancel(ctx)
	ListenTarget(lctx, func(ev interface{}) {
		if _, ok := ev.(*page.EventLoadEventFired); ok {
			cancel()
			close(ch)
		}
	})
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		return waitLoaded(ctx)
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

		entryID := entries[cur-1].ID
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		return waitLoaded(ctx)
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

		entryID := entries[cur+1].ID
		if err := page.NavigateToHistoryEntry(entryID).Do(ctx); err != nil {
			return err
		}
		return waitLoaded(ctx)
	})
}

// Reload reloads the current page.
func Reload() Action {
	return ActionFunc(func(ctx context.Context) error {
		if err := page.Reload().Do(ctx); err != nil {
			return err
		}
		return waitLoaded(ctx)
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
