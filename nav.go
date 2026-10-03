package chromedp

import (
	"context"
	"errors"
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
)

// NavigateAction are actions which always trigger a page navigation, waiting
// for the page to load.
//
// Note that these actions don't collect HTTP response information; for that,
// see [RunResponse].
type NavigateAction Action

// Navigate is an action that navigates the current frame.
func Navigate(urlstr string) NavigateAction {
	return responseAction(nil, ActionFunc(func(ctx context.Context) error {
		switch res, err := Call(ctx, page.Navigate, page.NavigateParams{URL: urlstr}); {
		case err != nil:
			return err
		case res.ErrorText != "":
			return fmt.Errorf("page load error %s", res.ErrorText)
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
		res, err := Call(ctx, page.GetNavigationHistory, cdp.Empty{})
		if err != nil {
			return err
		}
		*currentIndex, *entries = res.CurrentIndex, res.Entries
		return nil
	})
}

// NavigateToHistoryEntry is an action to navigate to the specified navigation
// entry.
func NavigateToHistoryEntry(entryID int64) NavigateAction {
	return responseAction(nil, navigateToHistoryEntry(entryID))
}

// NavigateBack is an action that navigates the current frame backwards in its
// history.
func NavigateBack() NavigateAction {
	return responseAction(nil, ActionFunc(func(ctx context.Context) error {
		res, err := Call(ctx, page.GetNavigationHistory, cdp.Empty{})
		if err != nil {
			return err
		}

		cur, entries := res.CurrentIndex, res.Entries
		if cur <= 0 || cur > int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}

		return navigateToHistoryEntry(entries[cur-1].ID).Do(ctx)
	}))
}

// NavigateForward is an action that navigates the current frame forwards in
// its history.
func NavigateForward() NavigateAction {
	return responseAction(nil, ActionFunc(func(ctx context.Context) error {
		res, err := Call(ctx, page.GetNavigationHistory, cdp.Empty{})
		if err != nil {
			return err
		}

		cur, entries := res.CurrentIndex, res.Entries
		if cur < 0 || cur >= int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}

		return navigateToHistoryEntry(entries[cur+1].ID).Do(ctx)
	}))
}

// Reload is an action that reloads the current page.
func Reload() NavigateAction {
	return responseAction(nil, ActionFunc(func(ctx context.Context) error {
		_, err := Call(ctx, page.Reload, page.ReloadParams{})
		return err
	}))
}

// Stop is an action that stops all navigation and pending resource retrieval.
func Stop() Action {
	return ActionFunc(func(ctx context.Context) error {
		_, err := Call(ctx, page.StopLoading, cdp.Empty{})
		return err
	})
}

// navigateToHistoryEntry is an action that sends Page.navigateToHistoryEntry.
func navigateToHistoryEntry(entryID int64) Action {
	return ActionFunc(func(ctx context.Context) error {
		_, err := Call(ctx, page.NavigateToHistoryEntry, page.NavigateToHistoryEntryParams{EntryID: entryID})
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
