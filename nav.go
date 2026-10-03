package chromedp

import (
	"context"
	"errors"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
)

// Navigate is an action that navigates the current frame and waits for the
// page to load.
//
// This action does not collect HTTP response information. To get it, use
// [NavigateResponse] or [RunResponse].
func Navigate(urlstr string) Action[Void] {
	return waitLoad(navigate(urlstr))
}

// NavigateResponse is like [Navigate], and returns the HTTP response of the
// HTML document.
func NavigateResponse(urlstr string) Action[*network.Response] {
	return responseAction(navigate(urlstr))
}

// navigate is the action that sends Page.navigate, without waiting for the
// page to load.
func navigate(urlstr string) Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		switch res, err := cdp.Call(ctx, t, page.Navigate, page.NavigateParams{URL: urlstr}); {
		case err != nil:
			return err
		case res.ErrorText != "":
			return &LoadError{ErrorText: res.ErrorText}
		}
		return nil
	})
}

// NavigationEntries is an action that retrieves the page's navigation history
// entries. The result holds the index of the current entry and the entries.
func NavigationEntries() Action[page.GetNavigationHistoryResult] {
	return func(ctx context.Context, t *Target) (page.GetNavigationHistoryResult, error) {
		return cdp.Call(ctx, t, page.GetNavigationHistory, cdp.Empty{})
	}
}

// NavigateToHistoryEntry is an action to navigate to the specified navigation
// entry, and wait for the page to load.
func NavigateToHistoryEntry(entryID int64) Action[Void] {
	return waitLoad(navigateToHistoryEntry(entryID))
}

// NavigateBack is an action that navigates the current frame backwards in its
// history, and waits for the page to load.
func NavigateBack() Action[Void] {
	return waitLoad(Func(func(ctx context.Context, t *Target) error {
		res, err := cdp.Call(ctx, t, page.GetNavigationHistory, cdp.Empty{})
		if err != nil {
			return err
		}

		cur, entries := res.CurrentIndex, res.Entries
		if cur <= 0 || cur > int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}

		_, err = navigateToHistoryEntry(entries[cur-1].ID)(ctx, t)
		return err
	}))
}

// NavigateForward is an action that navigates the current frame forwards in
// its history, and waits for the page to load.
func NavigateForward() Action[Void] {
	return waitLoad(Func(func(ctx context.Context, t *Target) error {
		res, err := cdp.Call(ctx, t, page.GetNavigationHistory, cdp.Empty{})
		if err != nil {
			return err
		}

		cur, entries := res.CurrentIndex, res.Entries
		if cur < 0 || cur >= int64(len(entries)-1) {
			return errors.New("invalid navigation entry")
		}

		_, err = navigateToHistoryEntry(entries[cur+1].ID)(ctx, t)
		return err
	}))
}

// Reload is an action that reloads the current page, and waits for the page to
// load.
func Reload() Action[Void] {
	return waitLoad(Func(func(ctx context.Context, t *Target) error {
		_, err := cdp.Call(ctx, t, page.Reload, page.ReloadParams{})
		return err
	}))
}

// Stop is an action that stops all navigation and pending resource retrieval.
func Stop() Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		_, err := cdp.Call(ctx, t, page.StopLoading, cdp.Empty{})
		return err
	})
}

// navigateToHistoryEntry is an action that sends Page.navigateToHistoryEntry.
func navigateToHistoryEntry(entryID int64) Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		_, err := cdp.Call(ctx, t, page.NavigateToHistoryEntry, page.NavigateToHistoryEntryParams{EntryID: entryID})
		return err
	})
}

// Location is an action that retrieves the document location.
func Location() Action[string] {
	return EvaluateAsDevTools[string](`document.location.toString()`)
}

// Title is an action that retrieves the document title.
func Title() Action[string] {
	return EvaluateAsDevTools[string](`document.title`)
}
