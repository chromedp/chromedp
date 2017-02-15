package chromedp

import (
	"strings"
	"testing"
)

func TestNavigate(t *testing.T) {
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
