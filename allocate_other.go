//go:build !linux

package chromedp

import "os/exec"

func allocateCmdOptions(cmd *exec.Cmd) {
}

// killLeftovers does nothing on this platform.
func killLeftovers(dir string) {
}
