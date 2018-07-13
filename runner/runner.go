// Package runner provides a Chrome process runner.
package runner

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sync"
	"syscall"

	"github.com/chromedp/chromedp/client"
)

const (
	// DefaultUserDataDirPrefix is the default user data directory prefix.
	DefaultUserDataDirPrefix = "chromedp-runner.%d."
)

// Error is a runner error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// Error values.
const (
	// ErrAlreadyStarted is the already started error.
	ErrAlreadyStarted Error = "already started"

	// ErrAlreadyWaiting is the already waiting error.
	ErrAlreadyWaiting Error = "already waiting"

	// ErrInvalidURLs is the invalid url-opts error.
	ErrInvalidURLOpts Error = "invalid url-opts"

	// ErrInvalidCmdOpts is the invalid cmd-opts error.
	ErrInvalidCmdOpts Error = "invalid cmd-opts"

	// ErrInvalidProcessOpts is the invalid process-opts error.
	ErrInvalidProcessOpts Error = "invalid process-opts"

	// ErrInvalidExecPath is the invalid exec-path error.
	ErrInvalidExecPath Error = "invalid exec-path"
)

// Runner holds information about a running Chrome process.
type Runner struct {
	opts    map[string]interface{}
	cmd     *exec.Cmd
	waiting bool
	rw      sync.RWMutex
}

// New creates a new Chrome process using the supplied command line options.
func New(opts ...CommandLineOption) (*Runner, error) {
	var err error

	cliOpts := make(map[string]interface{})

	// apply opts
	for _, o := range opts {
		if err = o(cliOpts); err != nil {
			return nil, err
		}
	}

	// set default Chrome options if exec-path not provided
	if _, ok := cliOpts["exec-path"]; !ok {
		cliOpts["exec-path"] = LookChromeNames()
		for k, v := range map[string]interface{}{
			"no-first-run":             true,
			"no-default-browser-check": true,
			"remote-debugging-port":    9222,
		} {
			if _, ok := cliOpts[k]; !ok {
				cliOpts[k] = v
			}
		}
	}

	// add KillProcessGroup and ForceKill if no other cmd opts provided
	if _, ok := cliOpts["cmd-opts"]; !ok {
		for _, o := range []CommandLineOption{KillProcessGroup, ForceKill} {
			if err = o(cliOpts); err != nil {
				return nil, err
			}
		}
	}

	return &Runner{
		opts: cliOpts,
	}, nil
}

// cliOptRE is a regular expression to validate a chrome cli option.
var cliOptRE = regexp.MustCompile(`^[a-z0-9\-]+$`)

// buildOpts generates the command line options for Chrome.
func (r *Runner) buildOpts() []string {
	var opts []string
	var urls []string

	// process opts
	for k, v := range r.opts {
		if !cliOptRE.MatchString(k) || v == nil {
			continue
		}

		switch k {
		case "exec-path", "cmd-opts", "process-opts":
			continue

		case "url-opts":
			urls = v.([]string)

		default:
			switch z := v.(type) {
			case bool:
				if z {
					opts = append(opts, "--"+k)
				}

			case string:
				opts = append(opts, "--"+k+"="+z)

			default:
				opts = append(opts, "--"+k+"="+fmt.Sprintf("%v", v))
			}
		}
	}

	if urls == nil {
		urls = append(urls, "about:blank")
	}

	return append(opts, urls...)
}

// Start starts a Chrome process using the specified context. The Chrome
// process can be terminated by closing the passed context.
func (r *Runner) Start(ctxt context.Context, opts ...string) error {
	var err error
	var ok bool

	r.rw.RLock()
	cmd := r.cmd
	r.rw.RUnlock()

	if cmd != nil {
		return ErrAlreadyStarted
	}

	// set user data dir, if not provided
	_, ok = r.opts["user-data-dir"]
	if !ok {
		r.opts["user-data-dir"], err = ioutil.TempDir(
			defaultUserDataTmpDir, fmt.Sprintf(DefaultUserDataDirPrefix, r.Port()),
		)
		if err != nil {
			return err
		}
	}

	// get exec path
	var execPath string
	if p, ok := r.opts["exec-path"]; ok {
		execPath, ok = p.(string)
		if !ok {
			return ErrInvalidExecPath
		}
	}

	// ensure execPath is valid
	if execPath == "" {
		return ErrInvalidExecPath
	}

	// create cmd
	r.cmd = exec.CommandContext(ctxt, execPath, append(r.buildOpts(), opts...)...)

	// apply cmd opts
	if cmdOpts, ok := r.opts["cmd-opts"]; ok {
		for _, co := range cmdOpts.([]func(*exec.Cmd) error) {
			if err = co(r.cmd); err != nil {
				return err
			}
		}
	}

	// start process
	if err = r.cmd.Start(); err != nil {
		return err
	}

	// apply process opts
	if processOpts, ok := r.opts["process-opts"]; ok {
		for _, po := range processOpts.([]func(*os.Process) error) {
			if err = po(r.cmd.Process); err != nil {
				// TODO: do something better here, as we want to kill
				// the child process, do cleanup, etc.
				panic(err)
				//return err
			}
		}
	}

	return nil
}

// Shutdown shuts down the Chrome process.
func (r *Runner) Shutdown(ctxt context.Context, opts ...client.Option) error {
	var err error

	cl := r.Client(opts...)

	targets, err := cl.ListPageTargets(ctxt)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, t := range targets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, i int, t client.Target) {
			defer wg.Done()
			errs[i] = cl.CloseTarget(ctxt, t)
		}(&wg, i, t)
	}
	wg.Wait()

	for _, e := range errs {
		if e != nil {
			return e
		}
	}

	// osx applications do not automatically exit when all windows (ie, tabs)
	// closed, so send SIGTERM.
	//
	// TODO: add other behavior here for more process options on shutdown?
	if runtime.GOOS == "darwin" && r.cmd != nil && r.cmd.Process != nil {
		return r.cmd.Process.Signal(syscall.SIGTERM)
	}

	return nil
}

// Wait waits for the previously started Chrome process to terminate, returning
// any encountered error.
func (r *Runner) Wait() error {
	r.rw.RLock()
	waiting := r.waiting
	r.rw.RUnlock()

	if waiting {
		return ErrAlreadyWaiting
	}

	r.rw.Lock()
	r.waiting = true
	r.rw.Unlock()

	defer func() {
		r.rw.Lock()
		r.waiting = false
		r.rw.Unlock()
	}()

	return r.cmd.Wait()
}

// Port returns the port the process was launched with.
func (r *Runner) Port() int {
	var port interface{}
	var ok bool
	port, ok = r.opts["remote-debugging-port"]
	if !ok {
		port, ok = r.opts["port"]
	}
	if !ok {
		panic("expected either remote-debugging-port or port to be specified in command line options")
	}

	var p int
	p, ok = port.(int)
	if !ok {
		panic("expected port to be type int")
	}

	return p
}

// Client returns a Chrome DevTools Protocol client for the running Chrome
// process.
func (r *Runner) Client(opts ...client.Option) *client.Client {
	return client.New(append(opts,
		client.URL(fmt.Sprintf("http://localhost:%d/json", r.Port())),
	)...)
}

// Run starts a new Chrome process runner, using the provided context and
// command line options.
func Run(ctxt context.Context, opts ...CommandLineOption) (*Runner, error) {
	var err error

	// create
	r, err := New(opts...)
	if err != nil {
		return nil, err
	}

	// start
	if err = r.Start(ctxt); err != nil {
		return nil, err
	}

	return r, nil
}

// CommandLineOption is a runner command line option.
//
// see: http://peter.sh/experiments/chromium-command-line-switches/
type CommandLineOption func(map[string]interface{}) error

// Flag is a generic command line option to pass a name=value flag to
// Chrome.
func Flag(name string, value interface{}) CommandLineOption {
	return func(m map[string]interface{}) error {
		m[name] = value
		return nil
	}
}

// Path sets the path to the Chrome executable and sets default run options for
// Chrome. This will also set the remote debugging port to 9222, and disable
// the first run / default browser check.
//
// Note: use ExecPath if you do not want to set other options.
func Path(path string) CommandLineOption {
	return func(m map[string]interface{}) error {
		m["exec-path"] = path
		m["no-first-run"] = true
		m["no-default-browser-check"] = true
		m["remote-debugging-port"] = 9222
		return nil
	}
}

// ExecPath is a command line option to set the exec path.
func ExecPath(path string) CommandLineOption {
	return Flag("exec-path", path)
}

// UserDataDir is the command line option to set the user data dir.
//
// Note: set this option to manually set the profile directory used by Chrome.
// When this is not set, then a default path will be created in the /tmp
// directory.
func UserDataDir(dir string) CommandLineOption {
	return Flag("user-data-dir", dir)
}

// ProxyServer is the command line option to set the outbound proxy server.
func ProxyServer(proxy string) CommandLineOption {
	return Flag("proxy-server", proxy)
}

// WindowSize is the command line option to set the initial window size.
func WindowSize(width, height int) CommandLineOption {
	return Flag("window-size", fmt.Sprintf("%d,%d", width, height))
}

// UserAgent is the command line option to set the default User-Agent
// header.
func UserAgent(userAgent string) CommandLineOption {
	return Flag("user-agent", userAgent)
}

// NoSandbox is the Chrome comamnd line option to disable the sandbox.
func NoSandbox(m map[string]interface{}) error {
	return Flag("no-sandbox", true)(m)
}

// NoFirstRun is the Chrome comamnd line option to disable the first run
// dialog.
func NoFirstRun(m map[string]interface{}) error {
	return Flag("no-first-run", true)(m)
}

// NoDefaultBrowserCheck is the Chrome comamnd line option to disable the
// default browser check.
func NoDefaultBrowserCheck(m map[string]interface{}) error {
	return Flag("no-default-browser-check", true)(m)
}

// RemoteDebuggingPort is the command line option to set the remote
// debugging port.
func RemoteDebuggingPort(port int) CommandLineOption {
	return Flag("remote-debugging-port", port)
}

// Headless is the command line option to run in headless mode.
func Headless(m map[string]interface{}) error {
	return Flag("headless", true)(m)
}

// DisableGPU is the command line option to disable the GPU process.
func DisableGPU(m map[string]interface{}) error {
	return Flag("disable-gpu", true)(m)
}

// URL is the command line option to add a URL to open on process start.
//
// Note: this can be specified multiple times, and each URL will be opened in a
// new tab.
func URL(urlstr string) CommandLineOption {
	return func(m map[string]interface{}) error {
		var urls []string
		if u, ok := m["url-opts"]; ok {
			urls, ok = u.([]string)
			if !ok {
				return ErrInvalidURLOpts
			}
		}
		m["url-opts"] = append(urls, urlstr)
		return nil
	}
}

// CmdOpt is a command line option to modify the underlying exec.Cmd
// prior to the call to exec.Cmd.Start in Run.
func CmdOpt(o func(*exec.Cmd) error) CommandLineOption {
	return func(m map[string]interface{}) error {
		var opts []func(*exec.Cmd) error
		if e, ok := m["cmd-opts"]; ok {
			opts, ok = e.([]func(*exec.Cmd) error)
			if !ok {
				return ErrInvalidCmdOpts
			}
		}
		m["cmd-opts"] = append(opts, o)
		return nil
	}
}

// ProcessOpt is a command line option to modify the child os.Process
// after the call to exec.Cmd.Start in Run.
func ProcessOpt(o func(*os.Process) error) CommandLineOption {
	return func(m map[string]interface{}) error {
		var opts []func(*os.Process) error
		if e, ok := m["process-opts"]; ok {
			opts, ok = e.([]func(*os.Process) error)
			if !ok {
				return ErrInvalidProcessOpts
			}
		}
		m["process-opts"] = append(opts, o)
		return nil
	}
}

// LookChromeNames looks for the platform's DefaultChromeNames and any
// additional names using exec.LookPath, returning the first encountered
// location or the platform's DefaultChromePath if no names are found on the
// path.
func LookChromeNames(additional ...string) string {
	for _, p := range append(additional, DefaultChromeNames...) {
		path, err := exec.LookPath(p)
		if err == nil {
			return path
		}
	}

	return DefaultChromePath
}
