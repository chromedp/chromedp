package chromedp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// An Allocator creates and manages a number of browsers.
//
// This interface hides how the browser process runs. For example, an Allocator
// can reuse browser processes, or connect to browsers that already run on
// remote machines.
type Allocator interface {
	// Allocate creates a new browser. Canceling the provided context frees all
	// resources of the browser, such as temporary directories.
	Allocate(context.Context, ...BrowserOption) (*Browser, error)

	// Wait blocks until the allocator has freed all of its resources.
	// Canceling the allocator context does this too, so normally you do not
	// need to call Wait.
	Wait()
}

// setupExecAllocator is like NewExecAllocator, but NewContext uses it to
// create the allocator without an extra context layer.
func setupExecAllocator(opts ...ExecAllocatorOption) *ExecAllocator {
	ep := &ExecAllocator{
		initFlags:        make(map[string]any),
		wsURLReadTimeout: 20 * time.Second,
	}
	for _, o := range opts {
		o(ep)
	}
	if ep.execPath == "" {
		ep.execPath = findExecPath()
	}
	return ep
}

// DefaultExecAllocatorOptions are the ExecAllocator options that NewContext
// uses when the parent context has no allocator. Do not modify this global.
// Use NewExecAllocator instead. See [ExampleExecAllocator].
//
// The list runs the browser in headless mode. To get a visible window, add
// [VisibleWindow] to a copy of the list, or give [WithVisibleWindow] to
// NewContext.
//
// [ExampleExecAllocator]: https://pkg.go.dev/github.com/chromedp/chromedp#example-ExecAllocator
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
	Flag("disable-features", "site-per-process,Translate,BlinkGenPropertyTrees"),
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

// defaultExecAllocatorOptions returns the options that NewContext uses to make
// the default allocator. It is the only place that builds that list from
// DefaultExecAllocatorOptions, so that the options of NewContext and the options
// that a caller of NewExecAllocator builds from the same list cannot drift.
func defaultExecAllocatorOptions(visibleWindow, keepOpen bool) []ExecAllocatorOption {
	opts := slices.Clone(DefaultExecAllocatorOptions[:])
	if visibleWindow {
		opts = append(opts, VisibleWindow)
	}
	if keepOpen {
		opts = append(opts, KeepOpen)
	}
	return opts
}

// NewExecAllocator creates a new context with an ExecAllocator. Use it with
// NewContext.
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

// ExecAllocatorOption is an exec allocator option.
type ExecAllocatorOption = func(*ExecAllocator)

// ExecAllocator is an Allocator which starts new browser processes on the host
// machine.
type ExecAllocator struct {
	execPath  string
	initFlags map[string]any
	initEnv   []string

	// webSocket makes the allocator connect to the browser with a websocket
	// instead of a pipe. See WebSocket.
	webSocket bool

	// Chrome sometimes does not print the websocket address, or runs for a
	// long time without exit. Give up after a timeout, so that we do not block
	// forever. It only applies to the websocket mode.
	wsURLReadTimeout time.Duration

	modifyCmdFunc func(cmd *exec.Cmd)

	// keepOpen is set by KeepOpen. See keepopen.go.
	keepOpen bool

	// visibleWindow is set by VisibleWindow. Allocate then checks for a
	// display before it starts the browser.
	visibleWindow bool

	wg sync.WaitGroup

	combinedOutputWriter io.Writer
}

// allocTempDir is the location of all temporary user data directories of
// ExecAllocator. The tests use it. If it is empty, ExecAllocator uses the
// default temporary directory of the system.
var allocTempDir string

// Allocate satisfies the Allocator interface.
func (a *ExecAllocator) Allocate(ctx context.Context, opts ...BrowserOption) (*Browser, error) {
	c := FromContext(ctx)
	if c == nil {
		return nil, ErrInvalidContext
	}

	if a.visibleWindow {
		if err := checkDisplay(); err != nil {
			return nil, err
		}
	}

	if a.keepOpen {
		return a.allocateKeepOpen(ctx, c, opts)
	}

	args, err := a.flagArgs()
	if err != nil {
		return nil, err
	}

	removeDir := false
	dataDir, ok := a.initFlags["user-data-dir"].(string)
	if !ok {
		tempDir, err := os.MkdirTemp(allocTempDir, "chromedp-runner")
		if err != nil {
			return nil, err
		}
		args = append(args, "--user-data-dir="+tempDir)
		dataDir = tempDir
		removeDir = true
	}
	if _, ok := a.initFlags["no-sandbox"]; !ok && os.Getuid() == 0 {
		// We run as root, for example in a Linux container. Chrome needs
		// --no-sandbox as root, so make that the default, unless the user set
		// Flag("no-sandbox", false).
		args = append(args, "--no-sandbox")
	}
	pipe := a.usesPipe()
	if pipe {
		if v, _ := a.initFlags["remote-debugging-pipe"].(bool); !v {
			args = append(args, "--remote-debugging-pipe")
		}
	} else if _, ok := a.initFlags["remote-debugging-port"]; !ok {
		args = append(args, "--remote-debugging-port=0")
	}

	// Force the first page to be blank, and not the welcome page.
	// --no-first-run does not do that.
	args = append(args, "about:blank")

	cmd := exec.CommandContext(ctx, a.execPath, args...)
	defer func() {
		if removeDir && cmd.Process == nil {
			// The process did not start, so we do not reach the goroutine that
			// calls RemoveAll below. Remove the directory here, so that no
			// empty directory stays.
			os.RemoveAll(dataDir)
		}
	}()

	// In the pipe mode, the pipes are set up before ModifyCmdFunc runs, so
	// that the function can see them.
	var pf *pipeFiles
	connected := false
	if pipe {
		var err error
		if pf, err = newPipeFiles(cmd); err != nil {
			return nil, err
		}
		// The browser process has its own copies after it starts.
		defer pf.closeChild()
		defer func() {
			if !connected {
				pf.closeParent()
			}
		}()
	}

	if a.modifyCmdFunc != nil {
		a.modifyCmdFunc(cmd)
	} else {
		allocateCmdOptions(cmd)
	}

	var stdout io.ReadCloser
	if !pipe {
		var err error
		if stdout, err = cmd.StdoutPipe(); err != nil {
			return nil, err
		}
		cmd.Stderr = cmd.Stdout
	}

	// Preserve environment variables set in the (lowest priority) existing
	// environment, OverrideCmdFunc(), and Env (highest priority)
	if len(a.initEnv) > 0 || len(cmd.Env) > 0 {
		cmd.Env = append(os.Environ(), cmd.Env...)
		cmd.Env = append(cmd.Env, a.initEnv...)
	}

	// We must start the cmd before we call cmd.Wait. Otherwise the two can
	// race.
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		// Chrome started, but nothing else will wait for it or remove
		// its directory. The context is done, so Chrome is killed.
		cmd.Wait()
		if removeDir {
			killLeftovers(dataDir)
			removeAllRetry(dataDir)
		}
		return nil, ctx.Err()
	case <-c.allocated: // for this browser's root context
	}
	var out *browserOutput
	if pipe {
		// The browser has its own copy of the child ends now.
		pf.closeChild()
		out = startBrowserOutput(&a.wg, pf.outR, a.combinedOutputWriter)
	}
	exited := make(chan struct{})
	a.wg.Add(1) // for the entire allocator
	go func() {
		// First wait for the process to finish.
		// TODO: do we care about this error in any scenario? If the user
		// canceled the context and killed chrome, it is most likely
		// "signal: killed", which is not interesting.
		cmd.Wait()
		if out != nil {
			out.finish()
		}
		close(exited)

		// Then delete the temporary user data directory, if needed.
		if removeDir {
			// Child processes of Chrome can outlive it, and write in the
			// directory after it is removed.
			killLeftovers(dataDir)
			// Sometimes Chrome still creates files or directories in the user
			// data directory at this point. We cannot reproduce it with strace,
			// so the reason is unknown. As a workaround, wait a short time
			// before we remove the directory.
			<-time.After(10 * time.Millisecond)
			if err := removeAllRetry(dataDir); c.cancelErr == nil {
				c.cancelErr = err
			}
		}
		a.wg.Done()
		close(c.allocated)
	}()

	var browser *Browser
	if pipe {
		var err error
		browser, err = connectPipe(ctx, pf, cmd, out, exited, opts)
		if err != nil {
			return nil, err
		}
		connected = true
	} else {
		var err error
		if browser, err = a.connectWebSocket(ctx, stdout, opts); err != nil {
			return nil, err
		}
	}
	go func() {
		// If the browser loses the connection, kill the whole process and the
		// handler at once. Do not use Cancel, because Cancel tries to close the
		// browser gracefully, and that hangs.
		// Do not cancel in the middle of a graceful Close, because Chrome must
		// shut itself down when it finishes.
		<-browser.LostConnection
		select {
		case <-browser.closingGracefully:
		default:
			c.cancel()
		}
	}()
	browser.process = cmd.Process
	browser.userDataDir = dataDir
	browser.exited = exited
	return browser, nil
}

// flagArgs returns the command line arguments for the flags of the allocator.
func (a *ExecAllocator) flagArgs() ([]string, error) {
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
	return args, nil
}

// connectWebSocket waits for the websocket address in the output of the
// browser, and connects to it.
func (a *ExecAllocator) connectWebSocket(ctx context.Context, stdout io.ReadCloser, opts []BrowserOption) (*Browser, error) {
	var wsURL string
	wsURLChan := make(chan struct{})
	var copy func()
	var output syncBuffer
	var err error
	go func() {
		wsURL, copy, err = readOutputTo(stdout, a.combinedOutputWriter, &output)
		close(wsURLChan)
	}()
	select {
	case <-wsURLChan:
		if err != nil {
			return nil, err
		}

	case <-time.After(a.wsURLReadTimeout):
		return nil, fmt.Errorf("websocket url timeout reached after %v, chrome printed:\n%s", a.wsURLReadTimeout, output.String())
	}

	if a.combinedOutputWriter != nil && copy != nil {
		a.wg.Go(func() {
			copy()
		})
	}

	return NewBrowser(ctx, wsURL, opts...)
}

// usesPipe reports whether Allocate connects to the browser with a pipe. It
// does not when the WebSocket option or the KeepOpen option is set, when the
// platform cannot pass the pipe to the process, or when the flags ask for a
// debugging port or address.
func (a *ExecAllocator) usesPipe() bool {
	if a.webSocket || a.keepOpen || !usePipe() {
		return false
	}
	for _, name := range []string{"remote-debugging-port", "remote-debugging-address"} {
		if _, ok := a.initFlags[name]; ok {
			return false
		}
	}
	return true
}

// removeAllRetry removes dir like os.RemoveAll. Chrome child processes, such as
// the crashpad handler, can still write files in dir after the main process
// exits. Then os.RemoveAll fails with "directory not empty". So
// removeAllRetry tries again for a few seconds.
func removeAllRetry(dir string) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.RemoveAll(dir)
		if err == nil || !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// readOutput reads the websocket address from the output of Chrome and returns
// as soon as it finds the address. It forwards all output that it reads to
// forward, if forward is not nil. It signals on done when the asynchronous
// io.Copy ends, if there is one.
func readOutput(rc io.ReadCloser, forward io.Writer) (wsURL string, _ func(), _ error) {
	return readOutputTo(rc, forward, new(syncBuffer))
}

// syncBuffer is a bytes.Buffer that is safe to use from more than one
// goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write satisfies the io.Writer interface.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns the text that was written.
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// readOutputTo is like readOutput. It also writes the output that it read
// before the websocket address to accumulated. A caller that gives up can then
// show what the browser printed.
func readOutputTo(rc io.ReadCloser, forward io.Writer, accumulated *syncBuffer) (wsURL string, _ func(), _ error) {
	prefix := []byte("DevTools listening on")
	bufr := bufio.NewReader(rc)
readLoop:
	for {
		line, err := bufr.ReadBytes('\n')
		if err != nil {
			return "", nil, fmt.Errorf("chrome failed to start:\n%s",
				accumulated.String())
		}
		if forward != nil {
			if _, err := forward.Write(line); err != nil {
				return "", nil, err
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
	copy := func() {}
	if forward == nil {
		// We do not need the process's output anymore.
		rc.Close()
	} else {
		// Return a func that the caller calls later to copy the rest of the
		// output in a separate goroutine. We must return the websocket URL
		// now.
		copy = func() { io.Copy(forward, bufr) }
	}
	return wsURL, copy, nil
}

// Wait satisfies the Allocator interface.
func (a *ExecAllocator) Wait() {
	a.wg.Wait()
}

// ExecPath returns an ExecAllocatorOption that uses the given path to run
// browser processes. The path can be an absolute path to a binary, or the name
// of a program that exec.LookPath finds.
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

// findExecPath looks for the Chrome browser on the system. It looks in
// different places on different operating systems. The search can be
// aggressive and a bit slow, but it runs only when you create a new
// ExecAllocator.
func findExecPath() string {
	var locations []string
	switch runtime.GOOS {
	case "darwin":
		locations = []string{
			// Mac
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		}
	case "windows":
		locations = []string{
			// Windows
			"chrome",
			"chrome.exe", // in case PATHEXT is misconfigured
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Chromium\Application\chrome.exe`),
		}
	default:
		locations = []string{
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
			"/usr/local/bin/chrome",
			"/snap/bin/chromium",
			"chrome",
		}
	}

	for _, path := range locations {
		found, err := exec.LookPath(path)
		if err == nil {
			return found
		}
	}
	// Fall back to something simple and sensible, to give a useful error
	// message.
	return "google-chrome"
}

// Flag is a generic command line option that passes a flag to Chrome. If the
// value is a string, Flag passes --name=value. If it is a boolean, Flag passes
// --name when the value is true.
func Flag(name string, value any) ExecAllocatorOption {
	return func(a *ExecAllocator) {
		a.initFlags[name] = value
	}
}

// Env sets environment variables, in the form NAME=value, for the new Chrome
// process. They add to the environment of the Go process, which os.Environ
// returns.
func Env(vars ...string) ExecAllocatorOption {
	return func(a *ExecAllocator) {
		a.initEnv = append(a.initEnv, vars...)
	}
}

// ModifyCmdFunc runs a func on the exec.Cmd of the browser. It replaces the
// default func, which sends SIGKILL to any open browsers when the Go program
// exits.
func ModifyCmdFunc(f func(cmd *exec.Cmd)) ExecAllocatorOption {
	return func(a *ExecAllocator) {
		a.modifyCmdFunc = f
	}
}

// UserDataDir is the command line option to set the user data directory. Use
// it to choose the profile directory of Chrome. Without it, ExecAllocator
// creates a default path in the /tmp directory.
func UserDataDir(dir string) ExecAllocatorOption {
	return Flag("user-data-dir", dir)
}

// ProxyServer is the command line option to set the outbound proxy server.
func ProxyServer(proxy string) ExecAllocatorOption {
	return Flag("proxy-server", proxy)
}

// IgnoreCertErrors is the command line option to ignore certificate errors.
// Use it to access an HTTPS website through a proxy.
func IgnoreCertErrors(a *ExecAllocator) {
	Flag("ignore-certificate-errors", true)(a)
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

// NoSandbox is the Chrome command line option to disable the sandbox.
func NoSandbox(a *ExecAllocator) {
	Flag("no-sandbox", true)(a)
}

// NoFirstRun is the Chrome command line option to disable the first run
// dialog.
func NoFirstRun(a *ExecAllocator) {
	Flag("no-first-run", true)(a)
}

// NoDefaultBrowserCheck is the Chrome command line option to disable the
// default browser check.
func NoDefaultBrowserCheck(a *ExecAllocator) {
	Flag("no-default-browser-check", true)(a)
}

// Headless is the command line option to run in headless mode. It sets the
// headless flag, hides scrollbars, and mutes audio.
func Headless(a *ExecAllocator) {
	Flag("headless", true)(a)
	// Like in Puppeteer.
	Flag("hide-scrollbars", true)(a)
	Flag("mute-audio", true)(a)
}

// VisibleWindow is the command line option to run the browser with a visible
// window. It removes the flags that make the browser quiet or hidden, and it
// starts the window maximized.
//
// It removes these flags: headless, hide-scrollbars, mute-audio,
// enable-automation (the infobar that says that software controls the
// browser) and disable-extensions. It keeps every other flag, and it adds
// start-maximized. It does not open the developer tools. To open them for every
// tab, add Flag("auto-open-devtools-for-tabs", true).
//
// Add VisibleWindow after the options that it must change. It only removes the
// flags that exist when it runs. For example:
//
//	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.VisibleWindow)
//	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
//
// On Linux, Allocate returns ErrNoDisplay when the environment variables
// DISPLAY and WAYLAND_DISPLAY are both empty. The allocator never falls back to
// headless mode. Allocate does no such check on macOS and Windows.
func VisibleWindow(a *ExecAllocator) {
	for _, name := range []string{
		"headless",
		"hide-scrollbars",
		"mute-audio",
		"enable-automation",
		"disable-extensions",
	} {
		delete(a.initFlags, name)
	}
	Flag("start-maximized", true)(a)
	a.visibleWindow = true
}

// visibleWindowEnv is the name of the environment variable that has the same
// effect as WithVisibleWindow.
const visibleWindowEnv = "CHROMEDP_VISIBLEWINDOW"

// visibleWindowFromEnv reports whether the environment variable
// CHROMEDP_VISIBLEWINDOW asks for a visible window. Any value other than the
// empty string, "false" and "0" does.
func visibleWindowFromEnv() bool {
	v := os.Getenv(visibleWindowEnv)
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

// DisableGPU is the command line option to disable the GPU process.
//
// The --disable-gpu option is a temporary workaround for a few bugs in
// headless mode. The references below say that it is no longer required:
//   - https://bugs.chromium.org/p/chromium/issues/detail?id=737678
//   - https://github.com/puppeteer/puppeteer/pull/2908
//   - https://github.com/puppeteer/puppeteer/pull/4523
//
// But this issue says that it is still required in some cases:
//   - https://github.com/chromedp/chromedp/issues/904
//
// Chromium 139 and later has no fallback to Swiftshader, unless you pass the
// --enable-unsafe-swiftshader option:
//   - https://chromestatus.com/feature/5166674414927872
func DisableGPU(a *ExecAllocator) {
	Flag("disable-gpu", true)(a)
	Flag("enable-unsafe-swiftshader", true)(a)
}

// CombinedOutput sets the io.Writer that receives the stdout and stderr of the
// browser.
func CombinedOutput(w io.Writer) ExecAllocatorOption {
	return func(a *ExecAllocator) {
		a.combinedOutputWriter = w
	}
}

// WebSocket is an ExecAllocatorOption that makes the allocator connect to the
// browser with a websocket, as chromedp did before. The default is a pipe: the
// allocator starts the browser with --remote-debugging-pipe, and the browser
// reads the commands from the file descriptor 3 and writes the responses and
// the events to the file descriptor 4. A pipe needs no port.
//
// Use WebSocket when the browser must open a debugging port, for example to
// let a second program connect to it. The Flag options "remote-debugging-port"
// and "remote-debugging-address" also select the websocket. On Windows the
// allocator always uses the websocket, because os/exec cannot pass extra file
// descriptors there.
func WebSocket(a *ExecAllocator) {
	a.webSocket = true
}

// WSURLReadTimeout sets how long ExecAllocator waits to read the WebSocket
// URL. The default is 20 seconds. It only applies to the websocket mode, see
// [WebSocket].
func WSURLReadTimeout(t time.Duration) ExecAllocatorOption {
	return func(a *ExecAllocator) {
		a.wsURLReadTimeout = t
	}
}

// NewRemoteAllocator creates a new context with a RemoteAllocator. Use it with
// NewContext. The url must point to the websocket address of the browser, such
// as "ws://127.0.0.1:$PORT/devtools/browser/...".
//
// If the url does not contain "/devtools/browser/", NewRemoteAllocator tries
// to find the correct url. It sends a request to
// "http://$HOST:$PORT/json/version".
//
// NewRemoteAllocator accepts urls of these formats:
//   - ws://127.0.0.1:9222/
//   - http://127.0.0.1:9222/
//
// It does not accept "ws://127.0.0.1:9222/devtools/browser/", because the
// allocator does not try to modify it and it is invalid.
//
// Use NoModifyURL to stop NewRemoteAllocator from changing the url.
func NewRemoteAllocator(parent context.Context, url string, opts ...RemoteAllocatorOption) (context.Context, context.CancelFunc) {
	a := &RemoteAllocator{
		wsURL:         url,
		modifyURLFunc: modifyURL,
	}
	for _, o := range opts {
		o(a)
	}
	c := &Context{Allocator: a}

	ctx, cancel := context.WithCancel(parent)
	ctx = context.WithValue(ctx, contextKey{}, c)
	return ctx, cancel
}

// RemoteAllocatorOption is a remote allocator option.
type RemoteAllocatorOption = func(*RemoteAllocator)

// RemoteAllocator is an Allocator which connects to an already running Chrome
// process through a websocket URL.
type RemoteAllocator struct {
	wsURL         string
	modifyURLFunc func(ctx context.Context, wsURL string) (string, error)

	wg sync.WaitGroup
}

// Allocate satisfies the Allocator interface.
func (a *RemoteAllocator) Allocate(ctx context.Context, opts ...BrowserOption) (*Browser, error) {
	c := FromContext(ctx)
	if c == nil {
		return nil, ErrInvalidContext
	}

	wsURL := a.wsURL
	var err error
	if a.modifyURLFunc != nil {
		wsURL, err = a.modifyURLFunc(ctx, wsURL)
		if err != nil {
			return nil, fmt.Errorf("failed to modify wsURL: %w", err)
		}
	}

	// Use a different context for the websocket, so that we can close the
	// relevant pages before we close the websocket connection.
	wctx, cancel := context.WithCancel(context.Background())

	close(c.allocated)
	// for the entire allocator
	a.wg.Go(func() {
		<-ctx.Done()
		Cancel(ctx) // block until all pages are closed
		cancel()    // close the websocket connection
	})

	browser, err := NewBrowser(wctx, wsURL, opts...)
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

// NoModifyURL is a RemoteAllocatorOption that prevents the remote allocator
// from modifying the websocket debugger URL passed to it.
func NoModifyURL(a *RemoteAllocator) {
	a.modifyURLFunc = nil
}
