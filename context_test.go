package chromedp

import (
	"context"
	"testing"
)

func TestTargets(t *testing.T) {
	t.Parallel()

	// Start one browser with one tab.
	ctx1, cancel := NewContext(context.Background())
	defer cancel()
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}

	{
		infos, err := Targets(ctx1)
		if err != nil {
			t.Fatal(err)
		}
		if want, got := 1, len(infos); want != got {
			t.Fatalf("want %d targets, got %d", want, got)
		}
	}

	// Start a second tab on the same browser
	ctx2, cancel := NewContext(ctx1)
	defer cancel()
	if err := Run(ctx2); err != nil {
		t.Fatal(err)
	}

	{
		infos, err := Targets(ctx2)
		if err != nil {
			t.Fatal(err)
		}
		if want, got := 2, len(infos); want != got {
			t.Fatalf("want %d targets, got %d", want, got)
		}
	}

	// The first context should also see both targets.
	{
		infos, err := Targets(ctx1)
		if err != nil {
			t.Fatal(err)
		}
		if want, got := 2, len(infos); want != got {
			t.Fatalf("want %d targets, got %d", want, got)
		}
	}
}
