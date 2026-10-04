package chromedp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// closedGrace is how long WaitClosed waits for the browser process to exit
// after its context ends. A browser that closes its last window also ends the
// context, because the connection drops a moment before the process exits.
const closedGrace = 500 * time.Millisecond

// keepOpenPrefix starts the name of the default user data directory of a
// browser that KeepOpen started.
const keepOpenPrefix = "keepopen-"

// KeepOpen is an ExecAllocatorOption that leaves the browser open when the Go
// program ends or when the context is canceled. Use it to look at the page
// after the program is done. See also [KeptOpen] and [VisibleWindow].
//
// The allocator needs a dialer for a kept browser, as it connects with a
// websocket. Without one, Allocate returns [ErrNoDialer]. Add [WithDialer], or
// WebSocket of the module github.com/chromedp/chromedp/remote, to the options.
// For the default allocator of NewContext, give the option WithKeepOpen of that
// module to NewContext.
//
// The browser must outlive the program, so KeepOpen changes how the allocator
// starts it:
//   - The allocator uses the websocket and not the pipe, because the pipe
//     closes when the Go process exits, and Chrome then exits too. It starts
//     Chrome with --remote-debugging-port=0 and reads the address from the file
//     DevToolsActivePort in the user data directory. The allocator of the
//     remote module can connect to that address later.
//   - The process starts detached: in a new session on Unix, and detached from
//     the console and in a new process group on Windows. A func from
//     [ModifyCmdFunc] runs first, and the detach settings come after it.
//   - The allocator does not kill the process when the context ends. It waits
//     for the process in a goroutine, so that no zombie process stays while the
//     program runs.
//   - The allocator never removes the user data directory. Unless you set
//     [UserDataDir], the directory is "keepopen-PID" in the directory
//     "chromedp" of os.UserCacheDir, where PID is the process ID of the Go
//     program. Each program run leaves one such directory, and you must remove
//     it yourself. Two browsers of one program share the directory, so set
//     UserDataDir when a program keeps more than one browser open.
//   - [Cancel] does not close the browser. The output of the browser goes
//     nowhere, so [CombinedOutput] has no effect. The library prints nothing.
//     [KeptOpen] returns the address and the directory, so that the program can
//     print them.
//
// To close a kept browser, close its window, or kill its process. The process
// is available as Process of the Browser.
func KeepOpen(a *ExecAllocator) {
	a.keepOpen = true
}

// KeptOpen returns the websocket address of the browser of the context and the
// user data directory of the browser, so that a program can print them. Another
// program can attach to the address with the allocator of the module
// github.com/chromedp/chromedp/remote. The user can delete the directory when
// the browser is closed.
//
// KeptOpen returns two empty strings when the context has no browser yet, or
// when the allocator did not start the browser with [KeepOpen]. The browser
// starts at the first Run, so call KeptOpen after it.
func KeptOpen(ctx context.Context) (wsURL, userDataDir string) {
	c := FromContext(ctx)
	if c == nil || c.Browser == nil || !c.Browser.keptOpen {
		return "", ""
	}
	return c.Browser.wsURL, c.Browser.userDataDir
}

// WaitClosed blocks until the browser process of the context exits, for
// example when the user closes the last window, or until ctx ends. It returns
// nil when the process exited, and the error of ctx otherwise. Use it in a
// program that must stay alive while the user looks at the window.
//
// If the context has no browser yet, WaitClosed starts one, as Run does. For a
// browser that an allocator of the remote module reaches, WaitClosed waits
// until the connection drops, because there is no process to watch.
func WaitClosed(ctx context.Context) error {
	c, err := initContextBrowser(ctx)
	if err != nil {
		return err
	}
	closed := c.Browser.exited
	if closed == nil {
		closed = c.Browser.LostConnection
	}
	select {
	case <-closed:
		return nil
	case <-ctx.Done():
	}
	// The connection of a browser that exits can end the context just before
	// the process exits. Give the process a short time before reporting it.
	select {
	case <-closed:
		return nil
	case <-time.After(closedGrace):
		return ctx.Err()
	}
}

// keptDirs holds the user data directories of the browsers that this program
// started with KeepOpen.
var keptDirs sync.Map

// keepOpenRoot returns the directory that holds the default user data
// directories of KeepOpen.
func keepOpenRoot() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding the cache directory: %w", err)
	}
	return filepath.Join(cache, "chromedp"), nil
}

// isKeepOpenDir reports whether dir is the user data directory of a browser
// that KeepOpen started, in this program or in another one.
func isKeepOpenDir(dir string) bool {
	if _, ok := keptDirs.Load(dir); ok {
		return true
	}
	root, err := keepOpenRoot()
	if err != nil {
		return false
	}
	return filepath.Dir(dir) == root && strings.HasPrefix(filepath.Base(dir), keepOpenPrefix)
}

// allocateKeepOpen is Allocate for an allocator with KeepOpen.
func (a *ExecAllocator) allocateKeepOpen(ctx context.Context, c *Context, opts []BrowserOption) (*Browser, error) {
	args, err := a.flagArgs()
	if err != nil {
		return nil, err
	}

	dataDir, ok := a.initFlags["user-data-dir"].(string)
	if !ok {
		root, err := keepOpenRoot()
		if err != nil {
			return nil, err
		}
		dataDir = filepath.Join(root, fmt.Sprintf("%s%d", keepOpenPrefix, os.Getpid()))
		args = append(args, "--user-data-dir="+dataDir)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating the user data directory: %w", err)
	}
	keptDirs.Store(dataDir, struct{}{})
	// A file from an earlier browser has an address that does not work.
	portFile := filepath.Join(dataDir, "DevToolsActivePort")
	if err := os.Remove(portFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("removing the old address file: %w", err)
	}

	if _, ok := a.initFlags["no-sandbox"]; !ok && os.Getuid() == 0 {
		// See Allocate.
		args = append(args, "--no-sandbox")
	}
	if _, ok := a.initFlags["remote-debugging-port"]; !ok {
		args = append(args, "--remote-debugging-port=0")
	}
	args = append(args, "about:blank")

	// Do not use exec.CommandContext. It kills the process when ctx ends.
	cmd := exec.Command(a.execPath, args...)
	if a.modifyCmdFunc != nil {
		a.modifyCmdFunc(cmd)
	}
	detachCmd(cmd)
	a.setCmdEnv(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// Reap the process, so that it does not stay as a zombie.
	exited := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(exited)
	}()
	stop := func() {
		cmd.Process.Kill()
		<-exited
	}

	select {
	case <-ctx.Done():
		stop()
		return nil, ctx.Err()
	case <-c.allocated: // for this browser's root context
	}
	a.wg.Add(1) // for the entire allocator
	go func() {
		// The process keeps running after the context ends. So the browser is
		// done for the Go program when the context ends or the process exits.
		select {
		case <-exited:
		case <-ctx.Done():
		}
		a.wg.Done()
		close(c.allocated)
	}()

	wsURL, err := a.waitActivePort(ctx, portFile, exited)
	if err != nil {
		stop()
		return nil, err
	}
	browser, err := a.dial(ctx, wsURL, opts)
	if err != nil {
		stop()
		return nil, err
	}
	go func() {
		// See Allocate. The browser keeps running, so only the handler stops.
		<-browser.LostConnection
		select {
		case <-browser.closingGracefully:
		default:
			browser.noteLost(ctx)
			c.cancel()
		}
	}()
	browser.process = cmd.Process
	browser.userDataDir = dataDir
	browser.exited = exited
	browser.reaped = exited
	browser.exitErr = &waitErr
	browser.keptOpen = true
	browser.wsURL = wsURL
	return browser, nil
}

// waitActivePort waits until the browser writes the file DevToolsActivePort,
// and returns the websocket address in it. The first line of the file is the
// port, and the second line is the path.
func (a *ExecAllocator) waitActivePort(ctx context.Context, file string, exited <-chan struct{}) (string, error) {
	timeout := time.NewTimer(a.wsURLReadTimeout)
	defer timeout.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		if data, err := os.ReadFile(file); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) >= 2 {
				port := strings.TrimSpace(lines[0])
				path := strings.TrimSpace(lines[1])
				if port != "" && path != "" {
					return "ws://127.0.0.1:" + port + path, nil
				}
			}
		}
		select {
		case <-tick.C:
		case <-exited:
			return "", fmt.Errorf("chrome exited before it wrote %s", file)
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout.C:
			return "", fmt.Errorf("address file timeout reached after %v: %s", a.wsURLReadTimeout, file)
		}
	}
}
