package chromedp

import (
	"context"
	"fmt"
	"sync"

	"github.com/knq/chromedp/runner"
)

const (
	// DefaultStartPort is the default start port number.
	DefaultStartPort = 9000

	// DefaultEndPort is the default end port number.
	DefaultEndPort = 10000
)

// Pool manages a pool of running Chrome processes.
type Pool struct {
	// start is the start port.
	start int

	// end is the end port.
	end int

	// res are the running chrome resources.
	res map[int]*Res

	rw sync.RWMutex
}

// NewPool creates a new Chrome runner pool.
func NewPool(opts ...PoolOption) (*Pool, error) {
	var err error

	p := &Pool{
		start: DefaultStartPort,
		end:   DefaultEndPort,
		res:   make(map[int]*Res),
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
	r.c, err = New(ctxt, WithRunner(r.r))
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

	r.p.rw.Lock()
	defer r.p.rw.Unlock()

	delete(r.p.res, r.port)

	return nil
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
