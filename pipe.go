package chromedp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/chromedp/cdproto"
	jsonv2 "github.com/chromedp/cdproto/cdp/jsonv2"
)

// PipeConn implements Transport with the two pipes of a browser that was
// started with the --remote-debugging-pipe flag.
//
// On Unix, the browser reads the commands from its file descriptor 3 and writes
// the responses and the events to its file descriptor 4. On Windows, it uses two
// handles. Each JSON message ends with one zero byte, in both directions.
type PipeConn struct {
	// r is the end of the pipe that the browser writes to.
	r io.ReadCloser
	// w is the end of the pipe that the browser reads from.
	w io.WriteCloser

	// br buffers r. Only one goroutine can call Read at a time.
	br *bufio.Reader

	// decoder is reused by Read.
	decoder jsonv2.Decoder

	// writeMu makes Write safe to call from more than one goroutine. It
	// also protects encoder and wbuf.
	writeMu sync.Mutex
	encoder jsonv2.Encoder
	wbuf    bytes.Buffer

	closeOnce sync.Once
	closeErr  error

	debugf func(string, ...any)
}

// NewPipeConn returns a Transport that reads the messages of the browser from
// r and writes the messages for the browser to w. Close closes both r and w.
func NewPipeConn(r io.ReadCloser, w io.WriteCloser, opts ...PipeOption) *PipeConn {
	c := &PipeConn{
		r:  r,
		w:  w,
		br: bufio.NewReader(r),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Close satisfies the io.Closer interface. It closes both ends of the pipe,
// which makes the browser exit.
func (c *PipeConn) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = errors.Join(c.r.Close(), c.w.Close())
	})
	return c.closeErr
}

// Read reads the next message. It returns io.EOF when the browser closes its
// end of the pipe.
func (c *PipeConn) Read(_ context.Context, msg *cdproto.Message) error {
	for {
		// A message can be larger than the buffer of c.br, and a read
		// can return part of a message, so read up to the zero byte.
		b, err := c.br.ReadBytes(0)
		if err != nil {
			if err == io.EOF && len(b) > 0 {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		b = bytes.TrimSuffix(b, []byte{0})
		if len(bytes.TrimSpace(b)) == 0 {
			continue
		}

		if c.debugf != nil {
			c.debugf("<- %s", b)
		}

		c.decoder.Reset(bytes.NewReader(b), DefaultUnmarshalOptions)
		return jsonv2.UnmarshalDecode(&c.decoder, msg, DefaultUnmarshalOptions)
	}
}

// Write writes a message. It is safe to call from more than one goroutine.
func (c *PipeConn) Write(_ context.Context, msg *cdproto.Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.wbuf.Reset()
	c.encoder.Reset(&c.wbuf, DefaultMarshalOptions)
	if err := jsonv2.MarshalEncode(&c.encoder, msg, DefaultMarshalOptions); err != nil {
		return fmt.Errorf("encoding message: %w", err)
	}
	// The encoder ends a value with a newline. The pipe protocol needs one
	// zero byte instead.
	b := bytes.TrimRight(c.wbuf.Bytes(), "\n")
	if c.debugf != nil {
		c.debugf("-> %s", b)
	}
	c.wbuf.Truncate(len(b))
	c.wbuf.WriteByte(0)

	// One Write call, so that the message is not split by another writer.
	if _, err := c.w.Write(c.wbuf.Bytes()); err != nil {
		return fmt.Errorf("writing message: %w", err)
	}
	return nil
}

// SetDebugf sets the protocol logger. NewBrowserTransport calls it with the
// logger of the browser.
func (c *PipeConn) SetDebugf(f func(string, ...any)) {
	c.debugf = f
}

// PipeOption is an option of NewPipeConn.
type PipeOption = func(*PipeConn)

// WithPipeDebugf is a pipe option to set a protocol logger.
func WithPipeDebugf(f func(string, ...any)) PipeOption {
	return func(c *PipeConn) {
		c.debugf = f
	}
}
