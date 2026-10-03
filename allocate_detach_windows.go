//go:build windows

package chromedp

import (
	"os/exec"
	"syscall"
)

// detachedProcess is the Windows process creation flag DETACHED_PROCESS. The
// syscall package does not define it.
const detachedProcess = 0x00000008

// detachCmd makes the process of cmd start detached from the console and in a
// new process group. The browser then keeps running when the Go program exits.
func detachCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess
}
