//go:build darwin

package chromedp

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// checkDisplay does nothing on this platform.
func checkDisplay() error {
	return nil
}

func allocateCmdOptions(cmd *exec.Cmd) {
}

// killLeftovers kills the processes that still have the user data directory
// dir in their command line, after the main process of a browser exited.
//
// Chrome starts helper processes. They can outlive the main process, for
// example when a program kills it, and they create files in the directory
// again. The function repeats until it finds none for a few passes.
//
// It never kills the processes of a browser that started with KeepOpen. That
// browser must outlive the program, and a later run must not kill it.
func killLeftovers(dir string) {
	if isKeepOpenDir(dir) {
		return
	}
	const (
		quietPasses = 3
		pause       = 10 * time.Millisecond
		limit       = 2 * time.Second
	)
	deadline := time.Now().Add(limit)
	for quiet := 0; quiet < quietPasses && time.Now().Before(deadline); time.Sleep(pause) {
		if killProcessesUsing(dir) == 0 {
			quiet++
		} else {
			quiet = 0
		}
	}
}

// killProcessesUsing kills the processes that have the user data directory dir
// in their command line, and returns how many it found. macOS has no /proc, so
// it reads the list of processes from the command ps.
func killProcessesUsing(dir string) int {
	out, err := exec.Command("ps", "-ax", "-o", "pid=,command=").Output()
	if err != nil {
		return 0
	}
	// The command line of a process is one string. An argument ends at a space
	// or at the end of the line.
	want := "--user-data-dir=" + dir
	self := os.Getpid()
	var n int
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		pidText, command, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid == self {
			continue
		}
		if !strings.Contains(command+" ", want+" ") {
			continue
		}
		if syscall.Kill(pid, syscall.SIGKILL) == nil {
			n++
		}
	}
	return n
}
