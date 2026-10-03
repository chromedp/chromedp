package chromedp

import (
	"slices"
	"strings"
	"testing"
)

func TestExecLocationsWindowsFallsBackToEdge(t *testing.T) {
	t.Parallel()

	locations := execLocations("windows")
	lastChrome, firstEdge := -1, -1
	for i, location := range locations {
		switch {
		case strings.Contains(location, "chrome"):
			lastChrome = i
		case strings.Contains(location, "msedge"):
			if firstEdge < 0 {
				firstEdge = i
			}
		}
	}
	if lastChrome < 0 || firstEdge < 0 {
		t.Fatalf("expected Chrome and Edge in %q", locations)
	}
	if firstEdge < lastChrome {
		t.Errorf("expected every Chrome location before the first Edge location, got %q", locations)
	}
	for _, want := range []string{"msedge", "msedge.exe"} {
		if !slices.Contains(locations, want) {
			t.Errorf("expected %q in the locations", want)
		}
	}
}

func TestExecLocationsOtherSystemsHaveNoEdge(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"linux", "darwin"} {
		for _, location := range execLocations(goos) {
			if strings.Contains(location, "msedge") {
				t.Errorf("expected no Edge on %s, got %q", goos, location)
			}
		}
	}
}
