//go:build windows

package chromedp

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"syscall"
)

// setChildPipes gives the ends of the pipes that the browser uses to cmd. The
// browser has no file descriptors 3 and 4 on Windows. Chromium takes the
// handles from the switch --remote-debugging-io-pipes. The switch lists the
// handle that the browser reads the commands from, and then the handle that it
// writes the responses and the events to. A handle has the same value in the
// browser process as in this process.
//
// The function os.Pipe makes inheritable handles. Only the two ends of the
// browser must pass to the process, so setChildPipes makes the other ends
// non-inheritable. Then no other process that this program starts can get them.
func setChildPipes(cmd *exec.Cmd, p *pipeFiles) error {
	for _, f := range []*os.File{p.parentR, p.parentW, p.outR} {
		h := syscall.Handle(f.Fd())
		if err := syscall.SetHandleInformation(h, syscall.HANDLE_FLAG_INHERIT, 0); err != nil {
			return fmt.Errorf("making the parent end of a pipe private: %w", err)
		}
	}
	hr := syscall.Handle(p.childR.Fd())
	hw := syscall.Handle(p.childW.Fd())
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	cmd.SysProcAttr.AdditionalInheritedHandles = append(cmd.SysProcAttr.AdditionalInheritedHandles, hr, hw)
	flag := "--remote-debugging-io-pipes=" + strconv.FormatUint(uint64(hr), 10) + "," + strconv.FormatUint(uint64(hw), 10)
	// Put the switch first, before the URL, so that every parser reads it as a
	// switch.
	cmd.Args = slices.Insert(cmd.Args, 1, flag)
	return nil
}
