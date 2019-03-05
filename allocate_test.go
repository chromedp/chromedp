package chromedp

import (
	"context"
	"os"
	"testing"
)

func TestExecAllocator(t *testing.T) {
	t.Parallel()

	poolCtx, cancel := NewAllocator(context.Background(), WithExecAllocator(allocOpts...))
	defer cancel()

	// TODO: test that multiple child contexts are run in different
	// processes and browsers.

	taskCtx, cancel := NewContext(poolCtx)
	defer cancel()

	want := "insert"
	var got string
	if err := Run(taskCtx, Tasks{
		Navigate(testdataDir + "/form.html"),
		Text("#foo", &got, ByID),
	}); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("wanted %q, got %q", want, got)
	}

	tempDir := FromContext(taskCtx).browser.UserDataDir
	pool := FromContext(taskCtx).Allocator

	cancel()
	pool.Wait()

	if _, err := os.Lstat(tempDir); os.IsNotExist(err) {
		return
	}
	t.Fatalf("temporary user data dir %q not deleted", tempDir)
}
