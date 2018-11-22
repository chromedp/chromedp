package chromedp

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/chromedp/chromedp/runner"
)

// Pool manages a pool of running Chrome processes.
type Pool struct {
	// start is the start port.
	start int

	// end is the end port.
	end int

	// res are the running chrome resources.
	res map[int]*Res

	// logging funcs
	logf, debugf, errf func(string, ...interface{})

	rw sync.RWMutex
}

// NewPool creates a new Chrome runner pool.
func NewPool(opts ...PoolOption) (*Pool, error) {
	p := &Pool{
		start:  DefaultPoolStartPort,
		end:    DefaultPoolEndPort,
		res:    make(map[int]*Res),
		logf:   log.Printf,
		debugf: func(string, ...interface{}) {},
	}

	// apply opts
	for _, o := range opts {
		if err := o(p); err != nil {
			return nil, err
		}
	}

	if p.errf == nil {
		p.errf = func(s string, v ...interface{}) {
			p.logf("ERROR: "+s, v...)
		}
	}

	return p, nil
}

// Shutdown releases all the pool resources.
func (p *Pool) Shutdown() error {
	p.rw.Lock()
	defer p.rw.Unlock()

	for _, r := range p.res {
		r.cancel()
	}

	return nil
}

// Allocate creates a new process runner and returns it.
func (p *Pool) Allocate(ctxt context.Context, opts ...runner.CommandLineOption) (*Res, error) {
	var err error

	r := p.next(ctxt)

	// Check if the port is available first. If it's not, Chrome will print
	// an "address already in use" error, but it will otherwise keep
	// running. This can lead to Allocate succeeding, while the chrome
	// process isn't actually listening on the port we need.
	l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", r.port))
	if err != nil {
		// we can't use this port, e.g. address already in use
		p.errf("pool could not allocate runner on port %d: %v", r.port, err)
		return nil, err
	}
	l.Close()

	p.debugf("pool allocating %d", r.port)

	// create runner
	r.r, err = runner.New(append([]runner.CommandLineOption{
		runner.ExecPath(runner.LookChromeNames("headless_shell")),
		runner.RemoteDebuggingPort(r.port),
		runner.NoDefaultBrowserCheck,
		runner.NoFirstRun,
		runner.Headless,
	}, opts...)...)
	if err != nil {
		defer r.Release()
		p.errf("pool could not allocate runner on port %d: %v", r.port, err)
		return nil, err
	}

	// start runner
	err = r.r.Start(r.ctxt)
	if err != nil {
		defer r.Release()
		p.errf("pool could not start runner on port %d: %v", r.port, err)
		return nil, err
	}

	// setup cdp
	r.c, err = New(
		r.ctxt, WithRunner(r.r),
		WithLogf(p.logf), WithDebugf(p.debugf), WithErrorf(p.errf),
	)
	if err != nil {
		defer r.Release()
		p.errf("pool could not connect to %d: %v", r.port, err)
		return nil, err
	}

	return r, nil
}

// next returns the next available res.
func (p *Pool) next(ctxt context.Context) *Res {
	p.rw.Lock()
	defer p.rw.Unlock()

	var found bool
	var i int
	for i = p.start; i < p.end; i++ {
		if _, ok := p.res[i]; !ok {
			found = true
			break
		}
	}

	if !found {
		panic("no ports available")
	}

	r := &Res{
		p:    p,
		port: i,
	}
	r.ctxt, r.cancel = context.WithCancel(ctxt)

	p.res[i] = r

	return r
}

// Res is a pool resource.
type Res struct {
	p      *Pool
	ctxt   context.Context
	cancel func()
	port   int
	r      *runner.Runner
	c      *CDP
}

// Release releases the pool resource.
func (r *Res) Release() error {
	r.cancel()

	var err error
	if r.c != nil {
		err = r.c.Wait()
	}

	defer r.p.debugf("pool released %d", r.port)

	r.p.rw.Lock()
	defer r.p.rw.Unlock()
	delete(r.p.res, r.port)

	return err
}

// Port returns the allocated port for the pool resource.
func (r *Res) Port() int {
	return r.port
}

// URL returns a formatted URL for the pool resource.
func (r *Res) URL() string {
	return fmt.Sprintf("http://localhost:%d/json", r.port)
}

// CDP returns the actual CDP instance.
func (r *Res) CDP() *CDP {
	return r.c
}

// Run runs an action.
func (r *Res) Run(ctxt context.Context, a Action) error {
	return r.c.Run(ctxt, a)
}

// PoolOption is a pool option.
type PoolOption func(*Pool) error

// PortRange is a pool option to set the port range to use.
func PortRange(start, end int) PoolOption {
	return func(p *Pool) error {
		p.start = start
		p.end = end
		return nil
	}
}

// PoolLog is a pool option to set the logging to use for the pool.
func PoolLog(logf, debugf, errf func(string, ...interface{})) PoolOption {
	return func(p *Pool) error {
		p.logf, p.debugf, p.errf = logf, debugf, errf
		return nil
	}
}
