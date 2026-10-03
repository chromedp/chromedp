//go:build linux

package chromedp

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func allocateCmdOptions(cmd *exec.Cmd) {
	if _, ok := os.LookupEnv("LAMBDA_TASK_ROOT"); ok {
		// do nothing on AWS Lambda
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	// When the parent process dies (Go), kill the child as well.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}

// killLeftovers kills the processes that still use dir as the user data
// directory of a Chrome that was started with it.
//
// Chrome child processes, such as the network service and the zygote, can
// outlive the main process when it is killed, and they can write files in the
// user data directory after it is removed. A child can also be forked just as
// the main process dies, and show up after the first look, so killLeftovers
// looks again until it finds nothing for a few passes in a row. Chrome passes
// --user-data-dir to every child, and the directory of a temporary allocation
// is unique, so the command line finds the leftovers without a process group. A
// process group does not work: on the CI runner Chrome did not start in its own
// group.
func killLeftovers(dir string) {
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
// in their command line, and returns how many it found.
func killProcessesUsing(dir string) int {
	want := "--user-data-dir=" + dir
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	self := os.Getpid()
	var n int
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
				n++
				break
			}
		}
	}
	return n
}
