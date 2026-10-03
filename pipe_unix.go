//go:build !windows

package chromedp

// usePipe reports whether the platform can pass extra file descriptors to a
// child process, so that an ExecAllocator can talk to the browser through a
// pipe.
func usePipe() bool {
	return true
}
