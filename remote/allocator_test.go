package remote

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/internal/chromedptest"
)

// readWebSocketURL reads the websocket address from the output of a browser
// that started with a debugging port. It returns as soon as it finds the
// address, and it leaves the rest of the output in r.
func readWebSocketURL(r io.Reader) (string, error) {
	const prefix = "DevTools listening on"
	var out strings.Builder
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("chrome failed to start:\n%s", out.String())
		}
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(rest), nil
		}
		out.WriteString(line)
	}
}

func TestAllocator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		modifyURL func(wsURL string) string
		opts      []Option
		wantErr   string
	}{
		{
			name:      "original wsURL",
			modifyURL: func(wsURL string) string { return wsURL },
		},
		{
			name: "detect from ws",
			modifyURL: func(wsURL string) string {
				return wsURL[0:strings.Index(wsURL, "devtools")]
			},
		},
		{
			name: "detect from http",
			modifyURL: func(wsURL string) string {
				return "http" + wsURL[2:strings.Index(wsURL, "devtools")]
			},
		},
		{
			name: "hostname",
			modifyURL: func(wsURL string) string {
				// Chrome ignores "remote-debugging-address" and
				// listens on the loopback interface only, so the
				// machine hostname is not reachable.
				h := "localhost"
				u, err := url.Parse(wsURL)
				if err != nil {
					t.Fatal(err)
				}
				_, port, err := net.SplitHostPort(u.Host)
				if err != nil {
					t.Fatal(err)
				}
				u.Host = net.JoinHostPort(h, port)
				u.Path = "/"
				return u.String()
			},
		},
		{
			name: "NoModifyURL",
			modifyURL: func(wsURL string) string {
				return wsURL[0:strings.Index(wsURL, "devtools")]
			},
			opts:    []Option{NoModifyURL},
			wantErr: "could not dial",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testAllocator(t, test.modifyURL, test.wantErr, test.opts)
		})
	}
}

func testAllocator(t *testing.T, modifyURL func(wsURL string) string, wantErr string, opts []Option) {
	tempDir := t.TempDir()

	procCtx, procCancel := context.WithCancel(context.Background())
	defer procCancel()
	cmd := exec.CommandContext(procCtx, chromedptest.ExecPath,
		// TODO: deduplicate these with allocOpts in chromedp_test.go
		"--no-first-run",
		"--no-default-browser-check",
		"--headless",
		"--disable-gpu",
		"--no-sandbox",

		// TODO: perhaps deduplicate this code with ExecAllocator
		"--user-data-dir="+tempDir,
		"--remote-debugging-address=0.0.0.0",
		"--remote-debugging-port=0",
		"about:blank",
	)

	// Kill is too abrupt: the child processes of Chrome can still write to
	// the temporary directory after the test, and then the cleanup fails.
	// Ask Chrome to exit, and kill it only if it does not.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 10 * time.Second

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	wsURL, err := readWebSocketURL(stderr)
	if err != nil {
		t.Fatal(err)
	}
	allocCtx, allocCancel := NewAllocator(context.Background(), modifyURL(wsURL), opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx,
		// This used to crash when used with the remote allocator.
		chromedp.WithLogf(func(format string, args ...any) {}),
	)

	{
		infos, err := chromedp.Targets(taskCtx)
		if len(wantErr) > 0 {
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("\ngot error:\n\t%v\nwant error contains:\n\t%s", err, wantErr)
			}

			procCancel()
			cmd.Wait()
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		// Current Chrome also lists targets that are not pages, such as
		// "browser_ui", "service_worker" and "background_page".
		var pages int
		for _, info := range infos {
			if info.Type == "page" {
				pages++
			}
		}
		if pages > 1 {
			t.Fatalf("expected Targets on a new remote allocator context to return at most one page, got: %d", pages)
		}
	}

	defer taskCancel()
	want := "insert"
	var got string
	if err := chromedp.Do(taskCtx,
		chromedp.Navigate(chromedptest.TestdataDir+"/form.html"),
		chromedptest.Into(&got, chromedp.Text(chromedp.ID("foo"))),
	); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
	targetID := chromedp.FromContext(taskCtx).Target.TargetID
	if err := chromedp.Cancel(taskCtx); err != nil {
		t.Fatal(err)
	}

	// Make sure that cancel closed the tabs. Do not just count the
	// number of targets, as perhaps the initial blank tab has not
	// come up yet.
	targetsCtx, targetsCancel := chromedp.NewContext(allocCtx)
	defer targetsCancel()
	infos, err := chromedp.Targets(targetsCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		if info.TargetID == targetID {
			t.Fatalf("target from previous iteration wasn't closed: %v", targetID)
		}
	}
	targetsCancel()

	// Finally, if we kill the browser and the websocket connection drops,
	// Run must return an error well before the 5s timeout.
	// TODO: a "defer cancel()" here adds a 1s timeout, because we try to
	// close the target twice. Fix that.
	ctx, _ := chromedp.NewContext(allocCtx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Connect to the browser, then kill it.
	if err := chromedp.Do(ctx); err != nil {
		t.Fatal(err)
	}
	procCancel()
	switch err := chromedp.Do(ctx, chromedp.Navigate(chromedptest.TestdataDir+"/form.html")); err {
	case nil:
		// TODO: figure out why this happens sometimes on Travis
		// t.Fatal("did not expect a nil error")
	case context.DeadlineExceeded:
		t.Fatalf("did not expect a standard context error: %v", err)
	}
	cmd.Wait()
}
