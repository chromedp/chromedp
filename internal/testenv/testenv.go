// Package testenv reads the environment variables that change the tests of
// chromedp. The tests of the core module and the tests of the other modules of
// the repository share it. It imports no chromedp package, so that the tests of
// the core package can import it too.
package testenv

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ExecPath returns the browser binary that the tests use. The variable
// CHROMEDP_TEST_RUNNER names it. Without the variable, ExecPath looks for a
// browser in the usual places and returns the empty string when it finds none.
func ExecPath() string {
	if path := os.Getenv("CHROMEDP_TEST_RUNNER"); path != "" {
		return path
	}
	var names []string
	switch runtime.GOOS {
	case "darwin":
		names = []string{
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		}
	case "windows":
		names = []string{
			"chrome",
			"chrome.exe",
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		}
	default:
		names = []string{
			"headless_shell",
			"headless-shell",
			"chromium",
			"chromium-browser",
			"google-chrome",
			"google-chrome-stable",
			"google-chrome-beta",
			"google-chrome-unstable",
			"/usr/bin/google-chrome",
			"/usr/local/bin/chrome",
			"/snap/bin/chromium",
			"chrome",
		}
	}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// VisibleWindow reports whether the tests must show the window of the browser.
// The variable CHROMEDP_VISIBLEWINDOW asks for it, and so does the old name
// CHROMEDP_NO_HEADLESS. Any value other than the empty string, "false" and "0"
// is true for the first variable, and any value other than the empty string and
// "false" is true for the second.
func VisibleWindow() bool {
	if v := os.Getenv("CHROMEDP_VISIBLEWINDOW"); v != "" && v != "0" && !strings.EqualFold(v, "false") {
		return true
	}
	v := os.Getenv("CHROMEDP_NO_HEADLESS")
	return v != "" && v != "false"
}

// NoSandbox reports whether the tests must start the browser without its
// sandbox. It is true unless CHROMEDP_NO_SANDBOX is "false".
func NoSandbox() bool {
	return os.Getenv("CHROMEDP_NO_SANDBOX") != "false"
}

// Debug reports whether the tests must log every protocol message. The
// variable CHROMEDP_DEBUG turns it on.
func Debug() bool {
	v := os.Getenv("CHROMEDP_DEBUG")
	return v != "" && v != "false"
}
