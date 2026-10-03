//go:build !linux

package remote

// killProfileProcesses does nothing on this platform. The tests kill the main
// browser process, and its children exit with it.
func killProfileProcesses(dir string) {}
