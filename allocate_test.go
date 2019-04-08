package chromedp

import (
	"context"
	"os"
	"testing"
)

func TestExecAllocator(t *testing.T) {
	t.Parallel()

	poolCtx, poolCancel := NewExecAllocator(context.Background(), allocOpts...)
	defer poolCancel()

	// TODO: test that multiple child contexts are run in different
	// processes and browsers.

	taskCtx, cancel := NewContext(poolCtx)
	defer cancel()

	want := "insert"
	var got string
	if err := Run(taskCtx,
		Navigate(testdataDir+"/form.html"),
		Text("#foo", &got, ByID),
	); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("wanted %q, got %q", want, got)
	}

	cancel()

	tempDir := FromContext(taskCtx).Browser.userDataDir
	if _, err := os.Lstat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("temporary user data dir %q not deleted", tempDir)
	}
}

func TestExecAllocatorCancelParent(t *testing.T) {
	t.Parallel()

	poolCtx, poolCancel := NewExecAllocator(context.Background(), allocOpts...)
	defer poolCancel()

	// TODO: test that multiple child contexts are run in different
	// processes and browsers.

	taskCtx, _ := NewContext(poolCtx)
	if err := Run(taskCtx); err != nil {
		t.Fatal(err)
	}

	// Canceling the pool context should stop all browsers too.
	poolCancel()

	tempDir := FromContext(taskCtx).Browser.userDataDir
	if _, err := os.Lstat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("temporary user data dir %q not deleted", tempDir)
	}
}
