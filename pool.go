package chromedp

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/knq/chromedp/runner"
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
	logf, debugf, errorf LogFunc

	rw sync.RWMutex
}

// NewPool creates a new Chrome runner pool.
func NewPool(opts ...PoolOption) (*Pool, error) {
	var err error

	p := &Pool{
		start:  DefaultPoolStartPort,
		end:    DefaultPoolEndPort,
		res:    make(map[int]*Res),
		logf:   log.Printf,
		debugf: func(string, ...interface{}) {},
		errorf: func(s string, v ...interface{}) { log.Printf("error: "+s, v...) },
	}

	// apply opts
	for _, o := range opts {
		err = o(p)
		if err != nil {
			return nil, err
		}
	}

	return p, err
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

	ctxt, cancel := context.WithCancel(ctxt)

	r := &Res{
		p:      p,
		ctxt:   ctxt,
		cancel: cancel,
		port:   p.next(),
	}

	// create runner
	r.r, err = runner.New(append([]runner.CommandLineOption{
		runner.Headless("", r.port),
	}, opts...)...)
	if err != nil {
		cancel()
		return nil, err
	}

	// start runner
	err = r.r.Start(ctxt)
	if err != nil {
		cancel()
		return nil, err
	}

	// setup cdp
	r.c, err = New(
		ctxt, WithRunner(r.r),
		WithLogf(p.logf), WithDebugf(p.debugf), WithErrorf(p.errorf),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	p.rw.Lock()
	defer p.rw.Unlock()

	p.res[r.port] = r

	return r, nil
}

// next returns the next available port number.
func (p *Pool) next() int {
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

	return i
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

	err := r.c.Wait()

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
func PoolLog(logf, debugf, errorf LogFunc) PoolOption {
	return func(p *Pool) error {
		p.logf = logf
		p.debugf = debugf
		p.errorf = errorf
		return nil
	}
}
