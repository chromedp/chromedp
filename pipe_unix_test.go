//go:build !windows

package chromedp

import "os/exec"

// dropChildPipes takes the pipes of the browser away from cmd, so that the
// browser starts without the file descriptors 3 and 4.
func dropChildPipes(cmd *exec.Cmd) {
	cmd.ExtraFiles = nil
}
