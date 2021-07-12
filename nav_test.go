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
	if err := Run(ctx,
		Location(&urlstr),
		Title(&title),
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

	var entries []*page.NavigationEntry
	var index int64
	if err := Run(ctx, NavigationEntries(&index, &entries)); err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Errorf("expected to have 1 navigation entry: got %d", len(entries))
	}
	if index != 0 {
		t.Errorf("expected navigation index is 0, got: %d", index)
	}

	expIdx, expEntries := 1, 2
	for i, test := range tests {
		if err := Run(ctx,
			Navigate(testdataDir+"/"+test.file),
			NavigationEntries(&index, &entries),
		); err != nil {
			t.Fatal(err)
		}
		if len(entries) != expEntries {
			t.Errorf("test %d expected to have %d navigation entry: got %d", i, expEntries, len(entries))
		}
		if want := int64(i + 1); index != want {
			t.Errorf("test %d expected navigation index is %d, got: %d", i, want, index)
		}

		expIdx++
		expEntries++
	}
}

func TestNavigateToHistoryEntry(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	var entries []*page.NavigationEntry
	var index int64
	if err := Run(ctx,
		NavigationEntries(&index, &entries),
		Navigate(testdataDir+"/form.html"),
	); err != nil {
		t.Fatal(err)
	}

	var title string
	if err := Run(ctx,
		NavigateToHistoryEntry(entries[index].ID),
		Title(&title),
	); err != nil {
		t.Fatal(err)
	}
	if title != entries[index].Title {
		t.Errorf("expected title to be %q, instead title is %q", entries[index].Title, title)
	}
}

func TestNavigateBack(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	var title, exptitle string
	if err := Run(ctx,
		Title(&exptitle),

		Navigate(testdataDir+"/image.html"),

		NavigateBack(),
		Title(&title),
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
	if err := Run(ctx,
		Navigate(testdataDir+"/image.html"),
		Title(&exptitle),

		NavigateBack(),
		NavigateForward(),

		Title(&title),
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
	if err := Run(ctx, Stop()); err != nil {
		t.Fatal(err)
	}
}

func TestReload(t *testing.T) {
	t.Parallel()

	count := 0
	// create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
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
	if err := Run(ctx,
		Navigate(s.URL),
		Title(&firstTitle),
		Reload(),
		Title(&secondTitle),
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
	if err := Run(ctx, Location(&urlstr)); err != nil {
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
	if err := Run(ctx, Title(&title)); err != nil {
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

	var iframes, forms []*cdp.Node
	if err := Run(ctx, Nodes(`iframe`, &iframes, ByQuery)); err != nil {
		t.Fatal(err)
	}
	iframe := iframes[0]
	if err := Run(ctx, Nodes(`#form`, &forms, ByQuery, FromNode(iframe))); err != nil {
		t.Fatal(err)
	}
	form := forms[0]

	var gotFoo string
	if err := Run(ctx,
		WaitVisible(`#form`, ByQuery, FromNode(iframe)),
		Text("#foo", &gotFoo, ByQuery, FromNode(form)),

		Click("#btn1", ByQuery, FromNode(iframe)),
		Click("#btn2", ByQuery, FromNode(form)),
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
	// Navigate shouldn't block waiting for the load to finish, which may
	// not come as the target is cancelled.
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		go func() {
			time.Sleep(time.Millisecond)
			cancel()
		}()
	}))
	defer s.Close()

	if err := Run(ctx, Navigate(s.URL)); err != nil && err != context.Canceled {
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

	// First, navigate to a page that starts loading, but doesn't finish.
	// Then, tell the server to finish loading the page.
	// Immediately after, navigate to another page.
	// Finally, grab the page title, which should correspond with the last
	// page.
	//
	// This has caused problems in the past. Because the first page might
	// fire its load event just as we start the second navigate, the second
	// navigate used to get confused, either blocking forever or not waiting
	// for the right load event (the second).
	var title string
	if err := Run(ctx,
		ActionFunc(func(ctx context.Context) error {
			_, _, _, err := page.Navigate(s.URL).Do(ctx)
			return err
		}),
		ActionFunc(func(ctx context.Context) error {
			ch <- struct{}{}
			return nil
		}),
		Navigate(testdataDir+"/image.html"),
		Title(&title),
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
	if err := Run(ctx,
		ActionFunc(func(ctx context.Context) error {
			_, _, _, err := page.Navigate(testdataDir + "/form.html").Do(ctx)
			return err
		}),
		WaitVisible(`#form`, ByID), // for form.html
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

	// Navigate to a page that will navigate, but never finish loading. Once
	// it has the HTML and starts loading an image, cancel the Run context.
	// This should result in us seeing a context error.
	action := ActionFunc(func(ctx context.Context) error {
		_, _, _, err := page.Navigate(s.URL).Do(ctx)
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
