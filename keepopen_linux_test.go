//go:build linux

package chromedp

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// killProfileProcesses kills the browser processes that still use dir. A test
// calls it to make sure that a kept browser leaves nothing behind.
func killProfileProcesses(dir string) {
	for range 3 {
		killProcessesUsing(dir)
		time.Sleep(10 * time.Millisecond)
	}
}

// startMarked starts a process that has --user-data-dir=dir in its command
// line, as a Chrome process has. The channel closes when the process ends.
func startMarked(t *testing.T, dir string) (*os.Process, <-chan struct{}) {
	t.Helper()
	cmd := exec.Command("sh", "-c", "sleep 60; true", "sh", "--user-data-dir="+dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		cmd.Process.Kill()
		<-done
	})
	return cmd.Process, done
}

func TestKillLeftoversSkipsKeepOpen(t *testing.T) {
	// The test starts processes with sh and reads their command lines. It is
	// not stable on a CI runner, and the headless-shell image has no sh.
	if os.Getenv("CI") != "" || os.Getenv("HEADLESS_SHELL") != "" {
		t.Skip("not stable on a CI runner, and the headless-shell image has no sh")
	}
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	keptDir := filepath.Join(cache, "chromedp", keepOpenPrefix+"12345")
	otherDir := filepath.Join(cache, "other")

	_, keptDone := startMarked(t, keptDir)
	_, otherDone := startMarked(t, otherDir)
	// Let sh start, so that its command line is in place.
	time.Sleep(100 * time.Millisecond)

	killLeftovers(keptDir)
	killLeftovers(otherDir)

	select {
	case <-otherDone:
	case <-time.After(5 * time.Second):
		t.Fatal("killLeftovers did not kill the process of an ordinary directory")
	}
	select {
	case <-keptDone:
		t.Fatal("killLeftovers killed the process of a kept browser")
	default:
	}
}
