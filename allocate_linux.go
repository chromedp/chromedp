//go:build linux

package chromedp

import (
	"os"
	"os/exec"
	"syscall"
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

// killProcessGroup kills the processes that are still in the process group of
// cmd. Chrome child processes, such as the network service, can outlive the
// main process when it is killed. They can write files in the user data
// directory after it is removed.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil || cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid || cmd.SysProcAttr.Pgid != 0 {
		return
	}
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
