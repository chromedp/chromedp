//go:build !windows

package chromedp

import (
	"os"
	"os/exec"
)

// setChildPipes gives the ends of the pipes that the browser uses to cmd. On
// Unix, the browser reads the commands from its file descriptor 3 and writes
// the responses and the events to its file descriptor 4. The os/exec package
// numbers the files of ExtraFiles from 3.
func setChildPipes(cmd *exec.Cmd, p *pipeFiles) error {
	cmd.ExtraFiles = []*os.File{p.childR, p.childW}
	return nil
}
