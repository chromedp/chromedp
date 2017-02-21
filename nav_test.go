package chromedp

import (
	"strings"
	"testing"
	"time"

	"github.com/knq/chromedp/cdp/page"
)

func TestNavigate(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://www.google.com/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, WaitVisible(`#hplogo`, ByID))
	if err != nil {
		t.Fatal(err)
	}

	var urlstr string
	err = c.Run(defaultContext, Location(&urlstr))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(urlstr, "https://www.google.") {
		t.Errorf("expected to be on google domain, at: %s", urlstr)
	}

	var title string
	err = c.Run(defaultContext, Title(&title))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(title), "google") {
		t.Errorf("expected title to contain google, instead title is: %s", title)
	}
}

func TestNavigationEntries(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	tests := []string{
		"https://godoc.org/",
		"https://golang.org/",
	}

	var entries []*page.NavigationEntry
	var index int64

	err = c.Run(defaultContext, NavigationEntries(&index, &entries))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Errorf("expected to have 1 navigation entry: got %d", len(entries))
	}
	if index != 0 {
		t.Errorf("expected navigation index is 0, got: %d", index)
	}

	expIdx, expEntries := 1, 2
	for i, url := range tests {
		err = c.Run(defaultContext, Navigate(url))
		if err != nil {
			t.Fatal(err)
		}

		err = c.Run(defaultContext, Sleep(time.Second*1))
		if err != nil {
			t.Fatal(err)
		}

		err = c.Run(defaultContext, NavigationEntries(&index, &entries))
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != expEntries {
			t.Errorf("test %d expected to have %d navigation entry: got %d", i, expEntries, len(entries))
		}
		if index != int64(i+1) {
			t.Errorf("test %d expected navigation index is %d, got: %d", i, i, index)
		}

		expIdx++
		expEntries++
	}
}

func TestNavigateToHistoryEntry(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	var entries []*page.NavigationEntry
	var index int64
	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, NavigationEntries(&index, &entries))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Navigate("https://golang.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, NavigateToHistoryEntry(entries[index].ID))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var title string
	err = c.Run(defaultContext, Title(&title))
	if err != nil {
		t.Fatal(err)
	}
	if title != entries[index].Title {
		t.Errorf("expected title to be %s, instead title is: %s", entries[index].Title, title)
	}
}

func TestNavigateBack(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var expTitle string
	err = c.Run(defaultContext, Title(&expTitle))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Navigate("https://golang.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, NavigateBack())
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var title string
	err = c.Run(defaultContext, Title(&title))
	if err != nil {
		t.Fatal(err)
	}
	if title != expTitle {
		t.Errorf("expected title to be %s, instead title is: %s", expTitle, title)
	}
}

func TestNavigateForward(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Navigate("https://golang.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var expTitle string
	err = c.Run(defaultContext, Title(&expTitle))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, NavigateBack())
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, NavigateForward())
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var title string
	err = c.Run(defaultContext, Title(&title))
	if err != nil {
		t.Fatal(err)
	}
	if title != expTitle {
		t.Errorf("expected title to be %s, instead title is: %s", expTitle, title)
	}
}

func TestStop(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Stop())
	if err != nil {
		t.Fatal(err)
	}
}

func TestReload(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var expTitle string
	err = c.Run(defaultContext, Title(&expTitle))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Reload())
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var title string
	err = c.Run(defaultContext, Title(&title))
	if err != nil {
		t.Fatal(err)
	}
	if title != expTitle {
		t.Errorf("expected title to be %s, instead title is: %s", expTitle, title)
	}
}

func TestCaptureScreenshot(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var buf []byte
	err = c.Run(defaultContext, CaptureScreenshot(&buf))
	if err != nil {
		t.Fatal(err)
	}

	if len(buf) == 0 {
		t.Fatal("failed to capture screenshoot")
	}
	//TODO: test image
}

func TestAddOnLoadScript(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	var scriptID page.ScriptIdentifier
	err = c.Run(defaultContext, AddOnLoadScript(`window.alert("TEST")`, &scriptID))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	if scriptID == "" {
		t.Fatal("got empty script ID")
	}
	// TODO: Handle javascript dialog.
}

func TestRemoveOnLoadScript(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	var scriptID page.ScriptIdentifier
	err = c.Run(defaultContext, AddOnLoadScript(`window.alert("TEST")`, &scriptID))
	if err != nil {
		t.Fatal(err)
	}

	if scriptID == "" {
		t.Fatal("got empty script ID")
	}

	err = c.Run(defaultContext, RemoveOnLoadScript(scriptID))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}
}

func TestLocation(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var url string
	err = c.Run(defaultContext, Location(&url))
	if err != nil {
		t.Fatal(err)
	}

	if url != "https://godoc.org/" {
		t.Fatalf("expected url to be https://godoc.org/ ,got: %s", url)
	}
}

func TestTitle(t *testing.T) {
	t.Parallel()

	var err error

	c := testAllocate(t, "")
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://godoc.org/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, Sleep(time.Second*1))
	if err != nil {
		t.Fatal(err)
	}

	var title string
	err = c.Run(defaultContext, Title(&title))
	if err != nil {
		t.Fatal(err)
	}

	if title != "GoDoc" {
		t.Fatalf("expected title to be GoDoc, got: %s", title)
	}
}
