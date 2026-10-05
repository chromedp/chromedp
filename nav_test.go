package chromedp

import (
	"context"
	"errors"
	"fmt"
	_ "image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
)

func TestNavigate(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	var urlstr, title string
	if err := Do(ctx,
		into(&urlstr, Location()),
		into(&title, Title()),
	); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(urlstr, "image.html") {
		t.Errorf("want to be on image.html, at %q", urlstr)
	}
	exptitle := "this is title"
	if title != exptitle {
		t.Errorf("want title to be %q, got %q", title, exptitle)
	}
}

func TestNavigationEntries(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	tests := []struct {
		file, waitID string
	}{
		{"form.html", "#form"},
		{"image.html", "#icon-brankas"},
	}

	history, err := Run(ctx, NavigationEntries())
	if err != nil {
		t.Fatal(err)
	}

	if len(history.Entries) != 1 {
		t.Errorf("expected to have 1 navigation entry: got %d", len(history.Entries))
	}
	if history.CurrentIndex != 0 {
		t.Errorf("expected navigation index is 0, got: %d", history.CurrentIndex)
	}

	expIdx, expEntries := 1, 2
	for i, test := range tests {
		if err := Do(ctx, Navigate(testdataDir+"/"+test.file)); err != nil {
			t.Fatal(err)
		}
		history, err := Run(ctx, NavigationEntries())
		if err != nil {
			t.Fatal(err)
		}
		if len(history.Entries) != expEntries {
			t.Errorf("test %d expected to have %d navigation entry: got %d", i, expEntries, len(history.Entries))
		}
		if want := int64(i + 1); history.CurrentIndex != want {
			t.Errorf("test %d expected navigation index is %d, got: %d", i, want, history.CurrentIndex)
		}

		expIdx++
		expEntries++
	}
}

// TestNavigateLoadError checks the error of a page that does not load. A
// program must find it with errors.Is and errors.As. The text of the error is
// the same as in earlier versions. See the issue 793.
func TestNavigateLoadError(t *testing.T) {
	t.Parallel()

	// Nothing listens on port 1, so the browser gets a refused connection.
	const urlstr = "http://127.0.0.1:1/"
	tests := map[string]func(ctx context.Context) error{
		"Navigate": func(ctx context.Context) error {
			return Do(ctx, Navigate(urlstr))
		},
		"NavigateResponse": func(ctx context.Context) error {
			_, err := Run(ctx, NavigateResponse(urlstr))
			return err
		},
		"RunResponse": func(ctx context.Context) error {
			_, err := RunResponse(ctx, Navigate(urlstr))
			return err
		},
	}
	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "")
			defer cancel()
			err := run(ctx)
			if err == nil {
				t.Fatal("want a load error")
			}
			if !errors.Is(err, ErrPageLoad) {
				t.Errorf("errors.Is(%q, ErrPageLoad) is false", err)
			}
			var loadErr *LoadError
			ok := errors.As(err, &loadErr)
			if !ok {
				t.Fatalf("want a *LoadError in %q", err)
			}
			if !strings.HasPrefix(loadErr.ErrorText, "net::ERR_") {
				t.Errorf("want a text with the prefix net::ERR_, got %q", loadErr.ErrorText)
			}
			if want := "page load error " + loadErr.ErrorText; err.Error() != want {
				t.Errorf("want the text %q, got %q", want, err.Error())
			}
		})
	}
}

func TestNavigateToHistoryEntry(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	history, err := Run(ctx, NavigationEntries())
	if err != nil {
		t.Fatal(err)
	}
	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}

	entry := history.Entries[history.CurrentIndex]
	var title string
	if err := Do(ctx,
		NavigateToHistoryEntry(entry.ID),
		into(&title, Title()),
	); err != nil {
		t.Fatal(err)
	}
	if title != entry.Title {
		t.Errorf("expected title to be %q, instead title is %q", entry.Title, title)
	}
}

func TestNavigateBack(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	var title, exptitle string
	if err := Do(ctx,
		into(&exptitle, Title()),

		Navigate(testdataDir+"/image.html"),

		NavigateBack(),
		into(&title, Title()),
	); err != nil {
		t.Fatal(err)
	}

	if title != exptitle {
		t.Errorf("expected title to be %q, instead title is %q", exptitle, title)
	}
}

func TestNavigateForward(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	var title, exptitle string
	if err := Do(ctx,
		Navigate(testdataDir+"/image.html"),
		into(&exptitle, Title()),

		NavigateBack(),
		NavigateForward(),

		into(&title, Title()),
	); err != nil {
		t.Fatal(err)
	}

	if title != exptitle {
		t.Errorf("expected title to be %q, instead title is %q", exptitle, title)
	}
}

func TestStop(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()
	if err := Do(ctx, Stop()); err != nil {
		t.Fatal(err)
	}
}

func TestReload(t *testing.T) {
	t.Parallel()

	count := 0
	// create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		// Current Chrome also requests "/favicon.ico". Do not count it.
		if req.URL.Path != "/" {
			http.NotFound(res, req)
			return
		}
		fmt.Fprintf(res, `<html>
<head>
	<title>Title %d</title>
</head>
</html>`, count)
		count++
	})
	s := httptest.NewServer(mux)
	defer s.Close()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	var firstTitle, secondTitle string
	if err := Do(ctx,
		Navigate(s.URL),
		into(&firstTitle, Title()),
		Reload(),
		into(&secondTitle, Title()),
	); err != nil {
		t.Fatal(err)
	}
	if want := "Title 0"; firstTitle != want {
		t.Errorf("expected first title to be %q, instead title is %q", want, firstTitle)
	}
	if want := "Title 1"; secondTitle != want {
		t.Errorf("expected second title to be %q, instead title is %q", want, secondTitle)
	}
}

func TestLocation(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	var urlstr string
	if err := Do(ctx, into(&urlstr, Location())); err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(urlstr, "form.html") {
		t.Fatalf("expected to be on form.html, got %q", urlstr)
	}
}

func TestTitle(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	var title string
	if err := Do(ctx, into(&title, Title())); err != nil {
		t.Fatal(err)
	}

	exptitle := "this is title"
	if title != exptitle {
		t.Fatalf("expected title to be %q, got %q", exptitle, title)
	}
}

func TestQueryIframe(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "iframe.html")
	defer cancel()

	var iframes, forms []*Node
	if err := Do(ctx, into(&iframes, Nodes(CSS(`iframe`)))); err != nil {
		t.Fatal(err)
	}
	iframe := iframes[0]
	if err := Do(ctx, into(&forms, Nodes(CSS(`#form`), FromNode(iframe)))); err != nil {
		t.Fatal(err)
	}
	form := forms[0]

	var gotFoo string
	if err := Do(ctx,
		WaitVisible(CSS(`#form`), FromNode(iframe)),
		into(&gotFoo, Text(CSS("#foo"), FromNode(form))),

		Click(CSS("#btn1"), FromNode(iframe)),
		Click(CSS("#btn2"), FromNode(form)),
	); err != nil {
		t.Fatal(err)
	}
	if want := "insert"; gotFoo != want {
		t.Fatalf("wanted %q, got %q", want, gotFoo)
	}
}

func TestNavigateContextTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// Serve the page, but cancel the context almost immediately after.
	// Navigate must not block while it waits for the load to finish, because
	// the load can never come when the target is canceled.
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.AfterFunc(time.Millisecond, cancel)
	}))
	defer s.Close()

	if err := Do(ctx, Navigate(s.URL)); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func writeHTML(content string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, strings.TrimSpace(content))
	})
}

func TestNavigateWhileLoading(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	mux := http.NewServeMux()
	mux.Handle("/", writeHTML(`
<img src="/img.jpg"></img>
	`))
	ch := make(chan struct{})
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, r *http.Request) {
		<-ch
	})
	s := httptest.NewServer(mux)
	defer s.Close()

	// First, navigate to a page that starts loading, but does not finish.
	// Then, tell the server to finish loading the page.
	// Immediately after, navigate to another page.
	// Finally, get the page title, which must match the last page.
	//
	// This caused problems in the past. The first page can fire its load event
	// just as we start the second navigate. Then the second navigate got
	// confused. It blocked forever, or it did not wait for the right load
	// event (the second).
	var title string
	if err := Do(ctx,
		Func(func(ctx context.Context, t *Target) error {
			lctx, cancel := context.WithCancel(ctx)
			defer cancel()
			events := Events(lctx, page.LifecycleEvent)

			_, err := cdp.Call(ctx, t, page.Navigate, page.NavigateParams{URL: s.URL})
			if err != nil {
				return err
			}

			// Make sure that the Page.lifecycleEvent with the name "init" is
			// emitted before the second navigate starts.
			//
			// Otherwise, this event can be emitted after the second navigate
			// starts, and the second navigate handles the wrong events. See
			// https://github.com/chromedp/chromedp/issues/1080.
			//
			// The implementation of responseAction() is buggy in this case.
			// It is hard to fix, because there is no way to tell whether the
			// events are from the first navigate.
			//
			// ZekeLu deflakes this test by making sure that the second
			// navigate does not see this event from the first navigate.
			//
			// You can reproduce the issue if you comment out the next line.
			for ev, err := range events {
				if err != nil {
					return err
				}
				if ev.Name == "init" {
					break
				}
			}
			ch <- struct{}{}
			return nil
		}),
		Navigate(testdataDir+"/image.html"),
		into(&title, Title()),
	); err != nil {
		t.Fatal(err)
	}
	exptitle := "this is title"
	if title != exptitle {
		t.Errorf("want title to be %q, got %q", exptitle, title)
	}
}

func TestNavigateWithoutWaitingForLoad(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// If we run a query without waiting for the page to load, chromedp used
	// to panic.
	if err := Do(ctx,
		Func(func(ctx context.Context, t *Target) error {
			_, err := cdp.Call(ctx, t, page.Navigate, page.NavigateParams{URL: testdataDir + "/form.html"})
			return err
		}),
		WaitVisible(ID(`form`)), // for form.html
	); err != nil {
		t.Fatal(err)
	}
}

func TestNavigateCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	loadStarted := make(chan struct{})
	mux := http.NewServeMux()
	mux.Handle("/", writeHTML(`<img src="/img.jpg"></img>`))
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, r *http.Request) {
		// Block until the entire test is done.
		<-ctx.Done()
	})
	s := httptest.NewServer(mux)
	defer s.Close()
	defer cancel() // if we call s.Close first, the ctx.Done above hangs

	// Navigate to a page that navigates, but never finishes loading. When the
	// page has the HTML and starts to load an image, cancel the Run context.
	// This must result in a context error.
	action := Func(func(ctx context.Context, t *Target) error {
		_, err := cdp.Call(ctx, t, page.Navigate, page.NavigateParams{URL: s.URL})
		loadStarted <- struct{}{}
		return err
	})
	ctx2, cancel2 := context.WithCancel(ctx)
	go func() {
		<-loadStarted
		cancel2()
	}()
	if _, err := RunResponse(ctx2, action); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error to be %q, got: %v", context.Canceled, err)
	}
}
