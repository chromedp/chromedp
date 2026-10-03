//go:build !linux

package chromedp

// killProfileProcesses does nothing on this platform. The tests kill the main
// browser process, and its children exit with it.
func killProfileProcesses(dir string) {}
