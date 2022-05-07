//go:build !linux
// +build !linux

package chromedp

import "os/exec"

func allocateCmdOptions(cmd *exec.Cmd) {
}
