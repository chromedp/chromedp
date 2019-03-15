package chromedp

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Allocator interface {
	// Allocate creates a new browser from the pool. It can be cancelled via
	// the provided context, at which point all the resources used by the
	// browser (such as temporary directories) will be cleaned up.
	Allocate(context.Context) (*Browser, error)

	// Wait can be called after cancelling a pool's context, to block until
	// all the pool's resources have been cleaned up.
	Wait()
}

func NewAllocator(parent context.Context, opts ...AllocatorOption) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	c := &Context{}

	for _, o := range opts {
		o(&c.Allocator)
	}

	ctx = context.WithValue(ctx, contextKey{}, c)
	return ctx, cancel
}

type AllocatorOption func(*Allocator)

func WithExecAllocator(opts ...ExecAllocatorOption) func(*Allocator) {
	return func(p *Allocator) {
		ep := &ExecAllocator{
			initFlags: make(map[string]interface{}),
		}
		for _, o := range opts {
			o(ep)
		}
		if ep.execPath == "" {
			ep.execPath = findExecPath()
		}
		*p = ep
	}
}

type ExecAllocatorOption func(*ExecAllocator)

type ExecAllocator struct {
	execPath  string
	initFlags map[string]interface{}

	wg sync.WaitGroup
}

func (p *ExecAllocator) Allocate(ctx context.Context) (*Browser, error) {
	removeDir := false
	var cmd *exec.Cmd

	// TODO: figure out a nicer way to do this
	flags := make(map[string]interface{})
	for name, value := range p.initFlags {
		flags[name] = value
	}

	dataDir, ok := flags["user-data-dir"].(string)
	if !ok {
		tempDir, err := ioutil.TempDir("", "chromedp-runner")
		if err != nil {
			return nil, err
		}
		flags["user-data-dir"] = tempDir
		dataDir = tempDir
		removeDir = true
	}

	p.wg.Add(1)
	go func() {
		<-ctx.Done()
		// First wait for the process to be finished.
		if cmd != nil {
			cmd.Wait()
		}
		// Then delete the temporary user data directory, if needed.
		if removeDir {
			os.RemoveAll(dataDir)
		}
		p.wg.Done()
	}()

	flags["remote-debugging-port"] = "0"

	args := []string{}
	for name, value := range flags {
		switch value := value.(type) {
		case string:
			args = append(args, fmt.Sprintf("--%s=%s", name, value))
		case bool:
			if value {
				args = append(args, fmt.Sprintf("--%s", name))
			}
		default:
			return nil, fmt.Errorf("invalid exec pool flag")
		}
	}

	cmd = exec.CommandContext(ctx, p.execPath, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Pick up the browser's websocket URL from stderr.
	wsURL := ""
	scanner := bufio.NewScanner(stderr)
	prefix := "DevTools listening on"
	for scanner.Scan() {
		line := scanner.Text()
		if s := strings.TrimPrefix(line, prefix); s != line {
			wsURL = strings.TrimSpace(s)
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	stderr.Close()

	browser, err := NewBrowser(ctx, wsURL)
	if err != nil {
		return nil, err
	}
	browser.UserDataDir = dataDir
	browser.Start(ctx)
	return browser, nil
}

func (p *ExecAllocator) Wait() {
	p.wg.Wait()
}

func ExecPath(path string) ExecAllocatorOption {
	return func(p *ExecAllocator) {
		if fullPath, _ := exec.LookPath(path); fullPath != "" {
			// Convert to an absolute path if possible, to avoid
			// repeated LookPath calls in each Allocate.
			path = fullPath
		}
		p.execPath = path
	}
}

// findExecPath tries to find the Chrome browser somewhere in the current
// system. It performs a rather agressive search, which is the same in all
// systems. That may make it a bit slow, but it will only be run when creating a
// new ExecAllocator.
func findExecPath() string {
	for _, path := range [...]string{
		// Unix-like
		"headless_shell",
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"google-chrome-beta",
		"google-chrome-unstable",
		"/usr/bin/google-chrome",

		// Windows
		"chrome",
		"chrome.exe", // in case PATHEXT is misconfigured
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,

		// Mac
		`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`,
	} {
		found, err := exec.LookPath(path)
		if err == nil {
			return found
		}
	}
	// Fall back to something simple and sensible, to give a useful error
	// message.
	return "google-chrome"
}

// Flag is a generic command line option to pass a flag to Chrome. If the value
// is a string, it will be passed as --name=value. If it's a boolean, it will be
// passed as --name if value is true.
func Flag(name string, value interface{}) ExecAllocatorOption {
	return func(p *ExecAllocator) {
		p.initFlags[name] = value
	}
}

// UserDataDir is the command line option to set the user data dir.
//
// Note: set this option to manually set the profile directory used by Chrome.
// When this is not set, then a default path will be created in the /tmp
// directory.
func UserDataDir(dir string) ExecAllocatorOption {
	return Flag("user-data-dir", dir)
}

// ProxyServer is the command line option to set the outbound proxy server.
func ProxyServer(proxy string) ExecAllocatorOption {
	return Flag("proxy-server", proxy)
}

// WindowSize is the command line option to set the initial window size.
func WindowSize(width, height int) ExecAllocatorOption {
	return Flag("window-size", fmt.Sprintf("%d,%d", width, height))
}

// UserAgent is the command line option to set the default User-Agent
// header.
func UserAgent(userAgent string) ExecAllocatorOption {
	return Flag("user-agent", userAgent)
}

// NoSandbox is the Chrome comamnd line option to disable the sandbox.
func NoSandbox(p *ExecAllocator) {
	Flag("no-sandbox", true)(p)
}

// NoFirstRun is the Chrome comamnd line option to disable the first run
// dialog.
func NoFirstRun(p *ExecAllocator) {
	Flag("no-first-run", true)(p)
}

// NoDefaultBrowserCheck is the Chrome comamnd line option to disable the
// default browser check.
func NoDefaultBrowserCheck(p *ExecAllocator) {
	Flag("no-default-browser-check", true)(p)
}

// Headless is the command line option to run in headless mode.
func Headless(p *ExecAllocator) {
	Flag("headless", true)(p)
}

// DisableGPU is the command line option to disable the GPU process.
func DisableGPU(p *ExecAllocator) {
	Flag("disable-gpu", true)(p)
}
