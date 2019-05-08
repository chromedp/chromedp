package chromedp

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
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

func TestCaptureScreenshot(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	// set the viewport size, to know what screenshot size to expect
	width, height := 650, 450
	var buf []byte
	if err := Run(ctx,
		emulation.SetDeviceMetricsOverride(int64(width), int64(height), 1.0, false),
		CaptureScreenshot(&buf),
	); err != nil {
		t.Fatal(err)
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if want := "png"; format != want {
		t.Fatalf("expected format to be %q, got %q", want, format)
	}
	if config.Width != width || config.Height != height {
		t.Fatalf("expected dimensions to be %d*%d, got %d*%d",
			width, height, config.Width, config.Height)
	}
}

/*func TestAddOnLoadScript(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	var scriptID page.ScriptIdentifier
	if err := Run(ctx,
		AddOnLoadScript(`window.alert("TEST")`, &scriptID),
		Navigate(testdataDir+"/form.html"),
	); err != nil {
		t.Fatal(err)
	}

	if scriptID == "" {
		t.Fatal("got empty script ID")
	}
	// TODO: Handle javascript dialog.
}

func TestRemoveOnLoadScript(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	var scriptID page.ScriptIdentifier
	if err := Run(ctx, AddOnLoadScript(`window.alert("TEST")`, &scriptID)); err != nil {
		t.Fatal(err)
	}
	if scriptID == "" {
		t.Fatal("got empty script ID")
	}

	if err := Run(ctx,
		RemoveOnLoadScript(scriptID),
		Navigate(testdataDir+"/form.html"),
	); err != nil {
		t.Fatal(err)
	}
}*/

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

func TestLoadIframe(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "iframe.html")
	defer cancel()

	if err := Run(ctx, Tasks{
		// TODO: remove the sleep once we have better support for
		// iframes.
		Sleep(10 * time.Millisecond),
		// WaitVisible(`#form`, ByID), // for the nested form.html
		WaitVisible(`#parent`, ByID), // for iframe.html
	}); err != nil {
		t.Fatal(err)
	}
}
