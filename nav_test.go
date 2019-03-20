package chromedp

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
)

func TestNavigate(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	if err := Run(ctx, WaitVisible(`#icon-brankas`, ByID)); err != nil {
		t.Fatal(err)
	}

	var urlstr string
	if err := Run(ctx, Location(&urlstr)); err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(urlstr, "image.html") {
		t.Errorf("expected to be on image.html, at: %s", urlstr)
	}

	var title string
	if err := Run(ctx, Title(&title)); err != nil {
		t.Fatal(err)
	}

	exptitle := "this is title"
	if title != exptitle {
		t.Errorf("expected title to contain google, instead title is: %s", title)
	}
}

func TestNavigationEntries(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()
	time.Sleep(50 * time.Millisecond)

	tests := []string{
		"form.html",
		"image.html",
	}

	var entries []*page.NavigationEntry
	var index int64
	if err := Run(ctx, NavigationEntries(&index, &entries)); err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Errorf("expected to have 2 navigation entry: got %d", len(entries))
	}
	if index != 1 {
		t.Errorf("expected navigation index is 1, got: %d", index)
	}

	expIdx, expEntries := 2, 3
	for i, url := range tests {
		if err := Run(ctx, Navigate(testdataDir+"/"+url)); err != nil {
			t.Fatal(err)
		}

		time.Sleep(50 * time.Millisecond)
		if err := Run(ctx, NavigationEntries(&index, &entries)); err != nil {
			t.Fatal(err)
		}

		if len(entries) != expEntries {
			t.Errorf("test %d expected to have %d navigation entry: got %d", i, expEntries, len(entries))
		}
		if want := int64(i + 2); index != want {
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
	time.Sleep(50 * time.Millisecond)
	if err := Run(ctx, NavigationEntries(&index, &entries)); err != nil {
		t.Fatal(err)
	}

	if err := Run(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
	if err := Run(ctx, NavigateToHistoryEntry(entries[index].ID)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	var title string
	if err := Run(ctx, Title(&title)); err != nil {
		t.Fatal(err)
	}

	if title != entries[index].Title {
		t.Errorf("expected title to be %s, instead title is: %s", entries[index].Title, title)
	}
}

func TestNavigateBack(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()
	time.Sleep(50 * time.Millisecond)

	var exptitle string
	if err := Run(ctx, Title(&exptitle)); err != nil {
		t.Fatal(err)
	}

	if err := Run(ctx, Navigate(testdataDir+"/image.html")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
	if err := Run(ctx, NavigateBack()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	var title string
	if err := Run(ctx, Title(&title)); err != nil {
		t.Fatal(err)
	}

	if title != exptitle {
		t.Errorf("expected title to be %s, instead title is: %s", exptitle, title)
	}
}

func TestNavigateForward(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()
	time.Sleep(50 * time.Millisecond)
	if err := Run(ctx, Navigate(testdataDir+"/image.html")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	var exptitle string
	if err := Run(ctx, Title(&exptitle)); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, NavigateBack()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
	if err := Run(ctx, NavigateForward()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	var title string
	if err := Run(ctx, Title(&title)); err != nil {
		t.Fatal(err)
	}

	if title != exptitle {
		t.Errorf("expected title to be %s, instead title is: %s", exptitle, title)
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

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	time.Sleep(50 * time.Millisecond)

	var exptitle string
	if err := Run(ctx, Title(&exptitle)); err != nil {
		t.Fatal(err)
	}

	if err := Run(ctx, Reload()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	var title string
	if err := Run(ctx, Title(&title)); err != nil {
		t.Fatal(err)
	}

	if title != exptitle {
		t.Errorf("expected title to be %s, instead title is: %s", exptitle, title)
	}
}

func TestCaptureScreenshot(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	time.Sleep(50 * time.Millisecond)

	var buf []byte
	if err := Run(ctx, CaptureScreenshot(&buf)); err != nil {
		t.Fatal(err)
	}

	if len(buf) == 0 {
		t.Fatal("failed to capture screenshot")
	}
	//TODO: test image
}

/*func TestAddOnLoadScript(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	var scriptID page.ScriptIdentifier
	err = Run(ctx, AddOnLoadScript(`window.alert("TEST")`, &scriptID))
	if err != nil {
		t.Fatal(err)
	}

	err = Run(ctx, Navigate(testdataDir+"/form.html"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

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
	err = Run(ctx, AddOnLoadScript(`window.alert("TEST")`, &scriptID))
	if err != nil {
		t.Fatal(err)
	}

	if scriptID == "" {
		t.Fatal("got empty script ID")
	}

	err = Run(ctx, RemoveOnLoadScript(scriptID))
	if err != nil {
		t.Fatal(err)
	}

	err = Run(ctx, Navigate(testdataDir+"/form.html"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
}*/

func TestLocation(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	time.Sleep(50 * time.Millisecond)

	var urlstr string
	if err := Run(ctx, Location(&urlstr)); err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(urlstr, "form.html") {
		t.Fatalf("expected to be on form.html, got: %s", urlstr)
	}
}

func TestTitle(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	time.Sleep(50 * time.Millisecond)

	var title string
	if err := Run(ctx, Title(&title)); err != nil {
		t.Fatal(err)
	}

	exptitle := "this is title"
	if title != exptitle {
		t.Fatalf("expected title to be %s, got: %s", exptitle, title)
	}
}
