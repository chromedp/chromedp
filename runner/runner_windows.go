// +build windows

package runner

import "os"

var (
	defaultUserDataTmpDir = os.Getenv("USERPROFILE") + `\AppData\Local`
)

// KillProcessGroup is a Chrome command line option that will instruct the
// invoked child Chrome process to terminate when the parent process (ie, the
// Go application) dies.
//
// Note: sets exec.Cmd.SysProcAttr.Setpgid = true and does nothing on Windows.
func KillProcessGroup(m map[string]interface{}) error {
	return nil
}

// ForceKill is a Chrome command line option that forces Chrome to be killed
// when the parent is killed.
//
// Note: sets exec.Cmd.SysProcAttr.Setpgid = true (only for Linux)
func ForceKill(m map[string]interface{}) error {
	return nil
}
