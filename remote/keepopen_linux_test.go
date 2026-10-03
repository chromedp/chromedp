//go:build linux

package remote

import (
	"bytes"
	"os"
	"strconv"
	"syscall"
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

// killProcessesUsing kills the processes that have the user data directory dir
// in their command line.
func killProcessesUsing(dir string) {
	want := "--user-data-dir=" + dir
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	self := os.Getpid()
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		cmdline, err := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if err != nil {
			continue
		}
		for arg := range bytes.SplitSeq(cmdline, []byte{0}) {
			if string(arg) == want {
				syscall.Kill(pid, syscall.SIGKILL)
				break
			}
		}
	}
}
