package chromedp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// An Allocator is responsible for creating and managing a number of browsers.
//
// This interface abstracts away how the browser process is actually run. For
// example, an Allocator implementation may reuse browser processes, or connect
// to already-running browsers on remote machines.
type Allocator interface {
	// Allocate creates a new browser. It can be cancelled via the provided
	// context, at which point all the resources used by the browser (such
	// as temporary directories) will be freed.
	Allocate(context.Context, ...BrowserOption) (*Browser, error)

	// Wait blocks until an allocator has freed all of its resources.
	// Cancelling the allocator context will already perform this operation,
	// so normally there's no need to call Wait directly.
	Wait()
}

// setupExecAllocator is similar to NewExecAllocator, but it allows NewContext
// to create the allocator without the unnecessary context layer.
func setupExecAllocator(opts ...ExecAllocatorOption) *ExecAllocator {
	ep := &ExecAllocator{
		initFlags: make(map[string]interface{}),
	}
	for _, o := range opts {
		o(ep)
	}
	if ep.execPath == "" {
		ep.execPath = findExecPath()
	}
	return ep
}

// DefaultExecAllocatorOptions are the ExecAllocator options used by NewContext
// if the given parent context doesn't have an allocator set up. Do not modify
// this global; instead, use NewExecAllocator. See ExampleExecAllocator.
var DefaultExecAllocatorOptions = [...]ExecAllocatorOption{
	NoFirstRun,
	NoDefaultBrowserCheck,
	Headless,

	// After Puppeteer's default behavior.
	Flag("disable-background-networking", true),
	Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
	Flag("disable-background-timer-throttling", true),
	Flag("disable-backgrounding-occluded-windows", true),
	Flag("disable-breakpad", true),
	Flag("disable-client-side-phishing-detection", true),
	Flag("disable-default-apps", true),
	Flag("disable-dev-shm-usage", true),
	Flag("disable-extensions", true),
	Flag("disable-features", "site-per-process,TranslateUI,BlinkGenPropertyTrees"),
	Flag("disable-hang-monitor", true),
	Flag("disable-ipc-flooding-protection", true),
	Flag("disable-popup-blocking", true),
	Flag("disable-prompt-on-repost", true),
	Flag("disable-renderer-backgrounding", true),
	Flag("disable-sync", true),
	Flag("force-color-profile", "srgb"),
	Flag("metrics-recording-only", true),
	Flag("safebrowsing-disable-auto-update", true),
	Flag("enable-automation", true),
	Flag("password-store", "basic"),
	Flag("use-mock-keychain", true),
}

// NewExecAllocator creates a new context set up with an ExecAllocator, suitable
// for use with NewContext.
func NewExecAllocator(parent context.Context, opts ...ExecAllocatorOption) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	c := &Context{Allocator: setupExecAllocator(opts...)}

	ctx = context.WithValue(ctx, contextKey{}, c)
	cancelWait := func() {
		cancel()
		c.Allocator.Wait()
	}
	return ctx, cancelWait
}

// ExecAllocatorOption is a exec allocator option.
type ExecAllocatorOption = func(*ExecAllocator)

// ExecAllocator is an Allocator which starts new browser processes on the host
// machine.
type ExecAllocator struct {
	execPath  string
	initFlags map[string]interface{}
	initEnv   []string

	wg sync.WaitGroup

	combinedOutputWriter io.Writer
}

// allocTempDir is used to group all ExecAllocator temporary user data dirs in
// the same location, useful for the tests. If left empty, the system's default
// temporary directory is used.
var allocTempDir string

// Allocate satisfies the Allocator interface.
func (a *ExecAllocator) Allocate(ctx context.Context, opts ...BrowserOption) (*Browser, error) {
	c := FromContext(ctx)
	if c == nil {
		return nil, ErrInvalidContext
	}

	var args []string
	for name, value := range a.initFlags {
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

	removeDir := false
	dataDir, ok := a.initFlags["user-data-dir"].(string)
	if !ok {
		tempDir, err := ioutil.TempDir(allocTempDir, "chromedp-runner")
		if err != nil {
			return nil, err
		}
		args = append(args, "--user-data-dir="+tempDir)
		dataDir = tempDir
		removeDir = true
	}
	if _, ok := a.initFlags["no-sandbox"]; !ok && os.Getuid() == 0 {
		// Running as root, for example in a Linux container. Chrome
		// needs --no-sandbox when running as root, so make that the
		// default, unless the user set Flag("no-sandbox", false).
		args = append(args, "--no-sandbox")
	}
	if _, ok := a.initFlags["remote-debugging-port"]; !ok {
		args = append(args, "--remote-debugging-port=0")
	}

	// Force the first page to be blank, instead of the welcome page;
	// --no-first-run doesn't enforce that.
	args = append(args, "about:blank")

	cmd := exec.CommandContext(ctx, a.execPath, args...)
	defer func() {
		if removeDir && cmd.Process == nil {
			// We couldn't start the process, so we didn't get to
			// the goroutine that handles RemoveAll below. Remove it
			// to not leave an empty directory.
			os.RemoveAll(dataDir)
		}
	}()
	allocateCmdOptions(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout

	if len(a.initEnv) > 0 {
		cmd.Env = append(os.Environ(), a.initEnv...)
	}

	// We must start the cmd before calling cmd.Wait, as otherwise the two
	// can run into a data race.
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.allocated: // for this browser's root context
	}
	a.wg.Add(1) // for the entire allocator
	if a.combinedOutputWriter != nil {
		a.wg.Add(1) // for the io.Copy in a separate goroutine
	}
	go func() {
		// First wait for the process to be finished.
		// TODO: do we care about this error in any scenario? if the
		// user cancelled the context and killed chrome, this will most
		// likely just be "signal: killed", which isn't interesting.
		cmd.Wait()

		// Then delete the temporary user data directory, if needed.
		if removeDir {
			if err := os.RemoveAll(dataDir); c.cancelErr == nil {
				c.cancelErr = err
			}
		}
		a.wg.Done()
		close(c.allocated)
	}()

	// Chrome will sometimes fail to print the websocket, or run for a long
	// time, without properly exiting. To avoid blocking forever in those
	// cases, give up after ten seconds.
	const wsURLReadTimeout = 20 * time.Second

	var wsURL string
	wsURLChan := make(chan struct{}, 1)
	go func() {
		wsURL, err = readOutput(stdout, a.combinedOutputWriter, a.wg.Done)
		wsURLChan <- struct{}{}
	}()
	select {
	case <-wsURLChan:
	case <-time.After(wsURLReadTimeout):
		err = errors.New("websocket url timeout reached")
	}
	if err != nil {
		if a.combinedOutputWriter != nil {
			// There's no io.Copy goroutine to call the done func.
			// TODO: a cleaner way to deal with this edge case?
			a.wg.Done()
		}
		return nil, err
	}

	browser, err := NewBrowser(ctx, wsURL, opts...)
	if err != nil {
		return nil, err
	}
	go func() {
		// If the browser loses connection, kill the entire process and
		// handler at once. Don't use Cancel, as that will attempt to
		// gracefully close the browser, which will hang.
		// Don't cancel if we're in the middle of a graceful Close,
		// since we want to let Chrome shut itself when it is fully
		// finished.
		<-browser.LostConnection
		select {
		case <-browser.closingGracefully:
		default:
			c.cancel()
		}
	}()
	browser.process = cmd.Process
	browser.userDataDir = dataDir
	return browser, nil
}

// readOutput grabs the websocket address from chrome's output, returning as
// soon as it is found. All read output is forwarded to forward, if non-nil.
// done is used to signal that the asynchronous io.Copy is done, if any.
func readOutput(rc io.ReadCloser, forward io.Writer, done func()) (wsURL string, _ error) {
	prefix := []byte("DevTools listening on")
	var accumulated bytes.Buffer
	bufr := bufio.NewReader(rc)
readLoop:
	for {
		line, err := bufr.ReadBytes('\n')
		if err != nil {
			return "", fmt.Errorf("chrome failed to start:\n%s",
				accumulated.Bytes())
		}
		if forward != nil {
			if _, err := forward.Write(line); err != nil {
				return "", err
			}
		}

		if bytes.HasPrefix(line, prefix) {
			line = line[len(prefix):]
			// use TrimSpace, to also remove \r on Windows
			line = bytes.TrimSpace(line)
			wsURL = string(line)
			break readLoop
		}
		accumulated.Write(line)
	}
	if forward == nil {
		// We don't need the process's output anymore.
		rc.Close()
	} else {
		// Copy the rest of the output in a separate goroutine, as we
		// need to return with the websocket URL.
		go func() {
			io.Copy(forward, bufr)
			done()
		}()
	}
	return wsURL, nil
}

// Wait satisfies the Allocator interface.
func (a *ExecAllocator) Wait() {
	a.wg.Wait()
}

// ExecPath returns an ExecAllocatorOption which uses the given path to execute
// browser processes. The given path can be an absolute path to a binary, or
// just the name of the program to find via exec.LookPath.
func ExecPath(path string) ExecAllocatorOption {
	return func(a *ExecAllocator) {
		// Convert to an absolute path if possible, to avoid
		// repeated LookPath calls in each Allocate.
		if fullPath, _ := exec.LookPath(path); fullPath != "" {
			a.execPath = fullPath
		} else {
			a.execPath = path
		}
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
		"headless-shell",
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
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Google\Chrome\Application\chrome.exe`),

		// Mac
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
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
	return func(a *ExecAllocator) {
		a.initFlags[name] = value
	}
}

// Env is a list of generic environment variables in the form NAME=value
// to pass into the new Chrome process. These will be appended to the
// environment of the golang process as retrieved by os.Environ.
func Env(vars ...string) ExecAllocatorOption {
	return func(a *ExecAllocator) {
		a.initEnv = append(a.initEnv, vars...)
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
func NoSandbox(a *ExecAllocator) {
	Flag("no-sandbox", true)(a)
}

// NoFirstRun is the Chrome comamnd line option to disable the first run
// dialog.
func NoFirstRun(a *ExecAllocator) {
	Flag("no-first-run", true)(a)
}

// NoDefaultBrowserCheck is the Chrome comamnd line option to disable the
// default browser check.
func NoDefaultBrowserCheck(a *ExecAllocator) {
	Flag("no-default-browser-check", true)(a)
}

// Headless is the command line option to run in headless mode. On top of
// setting the headless flag, it also hides scrollbars and mutes audio.
func Headless(a *ExecAllocator) {
	Flag("headless", true)(a)
	// Like in Puppeteer.
	Flag("hide-scrollbars", true)(a)
	Flag("mute-audio", true)(a)
}

// DisableGPU is the command line option to disable the GPU process.
func DisableGPU(a *ExecAllocator) {
	Flag("disable-gpu", true)(a)
}

// CombinedOutput is used to set an io.Writer where stdout and stderr
// from the browser will be sent
func CombinedOutput(w io.Writer) ExecAllocatorOption {
	return func(a *ExecAllocator) {
		a.combinedOutputWriter = w
	}
}

// NewRemoteAllocator creates a new context set up with a RemoteAllocator,
// suitable for use with NewContext. The url should point to the browser's
// websocket address, such as "ws://127.0.0.1:$PORT/devtools/browser/...".
func NewRemoteAllocator(parent context.Context, url string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	c := &Context{Allocator: &RemoteAllocator{
		wsURL: url,
	}}
	ctx = context.WithValue(ctx, contextKey{}, c)
	return ctx, cancel
}

// RemoteAllocator is an Allocator which connects to an already running Chrome
// process via a websocket URL.
type RemoteAllocator struct {
	wsURL string

	wg sync.WaitGroup
}

// Allocate satisfies the Allocator interface.
func (a *RemoteAllocator) Allocate(ctx context.Context, opts ...BrowserOption) (*Browser, error) {
	c := FromContext(ctx)
	if c == nil {
		return nil, ErrInvalidContext
	}

	// Use a different context for the websocket, so we can have a chance at
	// closing the relevant pages before closing the websocket connection.
	wctx, cancel := context.WithCancel(context.Background())

	close(c.allocated)
	a.wg.Add(1) // for the entire allocator
	go func() {
		<-ctx.Done()
		Cancel(ctx) // block until all pages are closed
		cancel()    // close the websocket connection
		a.wg.Done()
	}()
	browser, err := NewBrowser(wctx, a.wsURL, opts...)
	if err != nil {
		return nil, err
	}
	go func() {
		// If the browser loses connection, kill the entire process and
		// handler at once.
		<-browser.LostConnection
		select {
		case <-browser.closingGracefully:
		default:
			Cancel(ctx)
		}
	}()
	return browser, nil
}

// Wait satisfies the Allocator interface.
func (a *RemoteAllocator) Wait() {
	a.wg.Wait()
}
