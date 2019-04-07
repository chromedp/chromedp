package chromedp

import (
	"context"
	"testing"
)

func TestTargets(t *testing.T) {
	t.Parallel()

	// Start one browser with one tab.
	ctx1, cancel1 := NewContext(context.Background())
	defer cancel1()
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}

	wantTargets := func(ctx context.Context, want int) {
		t.Helper()
		infos, err := Targets(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(infos); want != got {
			t.Fatalf("want %d targets, got %d", want, got)
		}
	}
	wantTargets(ctx1, 1)

	// Start a second tab on the same browser.
	ctx2, cancel2 := NewContext(ctx1)
	defer cancel2()
	if err := Run(ctx2); err != nil {
		t.Fatal(err)
	}
	wantTargets(ctx2, 2)

	// The first context should also see both targets.
	wantTargets(ctx1, 2)

	// Cancelling the second context should close the second tab alone.
	cancel2()
	wantTargets(ctx1, 1)
}
