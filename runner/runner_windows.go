// +build windows

package runner

import "os"
import "os/exec"

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

// EdgeDiagnosticsAdapterWithPath is a command line option to specify using the
// Microsoft Edge Diagnostics adapter at the specified path.
func EdgeDiagnosticsAdapterWithPathAndPort(path string, port int) CommandLineOption {
	return func(m map[string]interface{}) error {
		m["exec-path"] = path
		m["port"] = port
		return nil
	}
}

// DefaultEdgeDiagnosticsAdapterPath is the default path to use for the
// Microsoft Edge Diagnostics Adapter if the executable is not in %PATH%.
const defaultEdgeDiagnosticsAdapterPath = `c:\Edge\EdgeDiagnosticsAdapter\x64\EdgeDiagnosticsAdapter.exe`

// EdgeDiagnosticsAdapter is a command line option to specify using the
// Microsoft Edge Diagnostics adapter found on the path.
//
// If EdgeDiagnosticsAdapter.exe is not found in %PATH%, this returns
// a hard-coded default path as a guess.
func EdgeDiagnosticsAdapter() CommandLineOption {
	path, err := exec.LookPath(`EdgeDiagnosticsAdapter.exe`)
	if err != nil {
		path = defaultEdgeDiagnosticsAdapterPath
	}
	return EdgeDiagnosticsAdapterWithPathAndPort(path, 9222)
}
