package chromedp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"
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

	// We used to have a bug where Run would reset the first context as if
	// it weren't the first, breaking its cancellation.
	if err := Run(ctx1); err != nil {
		t.Fatal(err)
	}
}

func TestBrowserQuit(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt isn't supported on Windows")
	}

	// Simulate a scenario where we navigate to a page that's slow to
	// respond, and the browser is closed before we can finish the
	// navigation.
	serve := make(chan bool, 1)
	close := make(chan bool, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close <- true
		<-serve
		fmt.Fprintf(w, "response")
	}))
	defer s.Close()

	ctx, cancel := NewContext(context.Background())
	defer cancel()
	if err := Run(ctx); err != nil {
		t.Fatal(err)
	}

	go func() {
		<-close
		b := FromContext(ctx).Browser
		if err := b.process.Signal(os.Interrupt); err != nil {
			t.Error(err)
		}
		serve <- true
	}()

	// Run should error with something other than "deadline exceeded" in
	// much less than 5s.
	ctx2, _ := context.WithTimeout(ctx, 5*time.Second)
	switch err := Run(ctx2, Navigate(s.URL)); err {
	case nil:
		t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v", err)
	}
}
