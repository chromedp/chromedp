// +build linux

package chromedp

import (
	"os"
	"os/exec"
	"syscall"
)

func allocateCmdOptions(cmd *exec.Cmd) {
	_, isLambda := os.LookupEnv("LAMBDA_TASK_ROOT")
	if isLambda {
		// do nothing on AWS Lambda
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	// When the parent process dies (Go), kill the child as well.
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
