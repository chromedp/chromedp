package chromedp

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestBrowserOutput(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	allocCtx, cancel := NewExecAllocator(context.Background(), append(allocOpts, CombinedOutput(buf))...)
	defer cancel()

	taskCtx, cancel := NewContext(allocCtx)
	defer cancel()

	if err := Run(taskCtx,
		Navigate(testdataDir+"/image.html"),
	); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	found := false
	prefix := "DevTools listening on"
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			found = true
		}
	}
	if !found {
		t.Fatal("Failed to find websocket string in browser output test")
	}
}
