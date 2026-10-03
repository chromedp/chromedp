package chromedp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
)

const (
	// maxStartOutput is the most output of the browser that an ExecAllocator
	// keeps to show in the error of a failed start.
	maxStartOutput = 64 << 10

	// outputGrace is how long an ExecAllocator waits for the rest of the
	// output of the browser after the process exits. A leftover child
	// process can hold the pipe open for much longer.
	outputGrace = time.Second

	// exitGrace is how long an ExecAllocator waits for the browser process
	// to exit after the pipe closed during the start. A busy machine can need
	// several seconds to load the browser, and the process prints its reason
	// before it exits. A browser that does not exit in this time is killed, and
	// then the error has no reason.
	exitGrace = 10 * time.Second
)

// pipeFiles holds the files of the pipe transport of an ExecAllocator.
type pipeFiles struct {
	// childR and childW are the ends that the browser uses. On Unix they are
	// its file descriptors 3 and 4.
	childR, childW *os.File

	// parentW writes the commands, and parentR reads the responses and the
	// events.
	parentW, parentR *os.File

	// outW and outR are the pipe of the standard output and the standard
	// error of the browser.
	outW, outR *os.File
}

// newPipeFiles creates the pipes and gives the ends of the browser and the
// output of the browser to cmd.
// The caller must call closeChild after the process starts, or when it does
// not start, and closeParent when it does not use the pipes.
func newPipeFiles(cmd *exec.Cmd) (*pipeFiles, error) {
	p := new(pipeFiles)
	var err error
	if p.childR, p.parentW, err = os.Pipe(); err != nil {
		return nil, fmt.Errorf("creating the command pipe: %w", err)
	}
	if p.parentR, p.childW, err = os.Pipe(); err != nil {
		p.closeChild()
		p.closeParent()
		return nil, fmt.Errorf("creating the response pipe: %w", err)
	}
	if p.outR, p.outW, err = os.Pipe(); err != nil {
		p.closeChild()
		p.closeParent()
		return nil, fmt.Errorf("creating the output pipe: %w", err)
	}
	cmd.Stdout = p.outW
	cmd.Stderr = p.outW
	if err := setChildPipes(cmd, p); err != nil {
		p.closeChild()
		p.closeParent()
		return nil, err
	}
	return p, nil
}

// closeChild closes the ends that only the browser process needs. Calling it
// more than once is safe.
func (p *pipeFiles) closeChild() {
	for _, f := range []*os.File{p.childR, p.childW, p.outW} {
		if f != nil {
			f.Close()
		}
	}
}

// closeParent closes the ends of the parent process. Calling it more than once
// is safe.
func (p *pipeFiles) closeParent() {
	for _, f := range []*os.File{p.parentR, p.parentW, p.outR} {
		if f != nil {
			f.Close()
		}
	}
}

// limitedBuffer is a buffer that is safe to use from more than one goroutine.
// It keeps the first max bytes that are written to it and drops the rest.
type limitedBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

// Write satisfies the io.Writer interface. It never fails.
func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := b.max - len(b.buf); room > 0 {
		b.buf = append(b.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

// String returns the text that was kept.
func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// browserOutput copies the output of the browser to a buffer and to the
// combined output writer.
type browserOutput struct {
	src  io.ReadCloser
	buf  limitedBuffer
	dst  io.Writer
	done chan struct{}
}

// startBrowserOutput starts to copy src. The goroutine is part of wg, so that
// the allocator waits for it.
func startBrowserOutput(wg *sync.WaitGroup, src io.ReadCloser, dst io.Writer) *browserOutput {
	o := &browserOutput{
		src:  src,
		buf:  limitedBuffer{max: maxStartOutput},
		dst:  dst,
		done: make(chan struct{}),
	}
	wg.Go(o.copy)
	return o
}

func (o *browserOutput) copy() {
	defer close(o.done)
	defer o.src.Close()
	p := make([]byte, 32<<10)
	for {
		n, err := o.src.Read(p)
		if n > 0 {
			o.buf.Write(p[:n])
			if o.dst != nil {
				if _, werr := o.dst.Write(p[:n]); werr != nil {
					// Keep reading, so that the browser does
					// not block on a full pipe.
					o.dst = nil
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// finish waits for the rest of the output after the browser process exited.
// A leftover child process can keep the pipe open, so it gives up after
// outputGrace and closes the pipe.
func (o *browserOutput) finish() {
	select {
	case <-o.done:
	case <-time.After(outputGrace):
		o.src.Close()
		<-o.done
	}
}

// String returns the output that was kept.
func (o *browserOutput) String() string {
	return o.buf.String()
}

// connectPipe makes a browser that talks to the process through pf, and waits
// for the first answer of the process. If the pipe closes first, it returns an
// error with the output of the process.
//
// exited is closed when the process exited and out is complete.
func connectPipe(ctx context.Context, pf *pipeFiles, cmd *exec.Cmd, out *browserOutput, exited <-chan struct{}, opts []BrowserOption) (*Browser, error) {
	tr := NewPipeConn(pf.parentR, pf.parentW)
	b, err := NewBrowserTransport(ctx, tr, opts...)
	if err != nil {
		tr.Close()
		return nil, err
	}

	first := make(chan error, 1)
	go func() {
		first <- b.execute(ctx, browser.CommandGetVersion, nil, nil)
	}()
	select {
	case err := <-first:
		if err != nil {
			return nil, fmt.Errorf("starting the browser: %w", err)
		}
		return b, nil
	case <-b.LostConnection:
	}

	// The answer and the end of the pipe can come at almost the same time.
	select {
	case err := <-first:
		if err == nil {
			return b, nil
		}
	case <-time.After(100 * time.Millisecond):
	}

	// Let the process exit, so that its output is complete.
	select {
	case <-exited:
	case <-time.After(exitGrace):
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return nil, fmt.Errorf("killing the browser that did not start: %w", err)
		}
		<-exited
	}
	// A canceled or expired context kills the process. The error of the
	// context is the cause then, and the output only shows the processes
	// that notice the death of their parent.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("starting the browser: %w", err)
	}
	return nil, fmt.Errorf("chrome failed to start:\n%s", out.String())
}
