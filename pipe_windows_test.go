//go:build windows

package chromedp

import (
	"os/exec"
	"slices"
	"strings"
)

// dropChildPipes takes the pipes of the browser away from cmd, so that the
// browser starts without the handles that the switch names.
func dropChildPipes(cmd *exec.Cmd) {
	cmd.Args = slices.DeleteFunc(cmd.Args, func(arg string) bool {
		return strings.HasPrefix(arg, "--remote-debugging-io-pipes=")
	})
	cmd.SysProcAttr.AdditionalInheritedHandles = nil
}
