//go:build !windows

package chromedp

import (
	"os/exec"
	"syscall"
)

// detachCmd makes the process of cmd start in a new session. The browser then
// keeps running when the Go program exits or when the terminal closes.
func detachCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	cmd.SysProcAttr.Setsid = true
}
