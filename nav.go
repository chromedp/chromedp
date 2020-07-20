package chromedp

import (
	"context"
	"errors"
	"fmt"

	"github.com/chromedp/cdproto/page"
)

// NavigateAction are actions which always trigger a page navigation, waiting
// for the page to load.
//
// Note that these actions don't collect HTTP response information; for that,
// see RunResponse.
type NavigateAction Action

// Navigate is an action that navigates the current frame.
func Navigate(urlstr string) NavigateAction {
	return responseAction(nil, ActionFunc(func(ctx context.Context) error {
		_, _, errorText, err := page.Navigate(urlstr).Do(ctx)
		if err != nil {
			return err
		}
		if errorText != "" {
			return fmt.Errorf("page load error %s", errorText)
		}
		return nil
	}))
}

// NavigationEntries is an action that retrieves the page's navigation history
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
func NavigateToHistoryEntry(entryID int64) NavigateAction {
	return responseAction(nil, page.NavigateToHistoryEntry(entryID))
}

// NavigateBack is an action that navigates the current frame backwards in its
// history.
func NavigateBack() NavigateAction {
	return responseAction(nil, ActionFunc(func(ctx context.Context) error {
		cur, entries, err := page.GetNavigationHistory().Do(ctx)
		if err != nil {
			return err
		}

		if cur <= 0 || cur > int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}

		entryID := entries[cur-1].ID
		return page.NavigateToHistoryEntry(entryID).Do(ctx)
	}))
}

// NavigateForward is an action that navigates the current frame forwards in
// its history.
func NavigateForward() NavigateAction {
	return responseAction(nil, ActionFunc(func(ctx context.Context) error {
		cur, entries, err := page.GetNavigationHistory().Do(ctx)
		if err != nil {
			return err
		}

		if cur < 0 || cur >= int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}

		entryID := entries[cur+1].ID
		return page.NavigateToHistoryEntry(entryID).Do(ctx)
	}))
}

// Reload is an action that reloads the current page.
func Reload() NavigateAction {
	return responseAction(nil, page.Reload())
}

// Stop is an action that stops all navigation and pending resource retrieval.
func Stop() Action {
	return page.StopLoading()
}

// CaptureScreenshot is an action that captures/takes a screenshot of the
// current browser viewport.
//
// See the Screenshot action to take a screenshot of a specific element.
//
// See the 'screenshot' example in the https://github.com/chromedp/examples
// project for an example of taking a screenshot of the entire page.
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

// Location is an action that retrieves the document location.
func Location(urlstr *string) Action {
	if urlstr == nil {
		panic("urlstr cannot be nil")
	}
	return EvaluateAsDevTools(`document.location.toString()`, urlstr)
}

// Title is an action that retrieves the document title.
func Title(title *string) Action {
	if title == nil {
		panic("title cannot be nil")
	}
	return EvaluateAsDevTools(`document.title`, title)
}
