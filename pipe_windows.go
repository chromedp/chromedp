//go:build windows

package chromedp

// usePipe reports whether the platform can pass extra file descriptors to a
// child process. The os/exec package does not support ExtraFiles on Windows, so
// an ExecAllocator always uses the websocket there.
func usePipe() bool {
	return false
}
