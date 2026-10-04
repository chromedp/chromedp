//go:build !linux && !darwin

package chromedp

import "os/exec"

// checkDisplay does nothing on this platform.
func checkDisplay() error {
	return nil
}

func allocateCmdOptions(cmd *exec.Cmd) {
}

// killLeftovers does nothing on this platform.
func killLeftovers(dir string) {
}
