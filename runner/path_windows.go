// +build windows

package runner

const (
	// DefaultChromePath is the default path to use for Chrome if the
	// executable is not in %PATH%.
	DefaultChromePath = `C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`
)

// DefaultChromeNames are the default Chrome executable names to look for in
// %PATH%.
var DefaultChromeNames = []string{`chrome.exe`}
