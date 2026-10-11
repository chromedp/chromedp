package websocket

import (
	"bufio"
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	// MaxMessageSize is the size of the largest message that a connection
	// reads. Chrome sends single-frame messages of up to 100 MiB, so the limit
	// has room above that. A frame or a message that is larger gets the close
	// code 1009.
	MaxMessageSize = 256 << 20

	// closeTimeout is how long Close waits to send its close frame.
	closeTimeout = time.Second

	// closeWait is how long Close waits for the close frame of the server, or
	// for the end of the stream, after it sent its own close frame. RFC 6455
	// section 7.1.1 lets the server close the connection first.
	closeWait = time.Second

	// maxFrames is the most frames that one call of ReadMessage reads. A server
	// can send endless empty fragments or tiny frames, and the reader
	// cannot give a message back. The limit is a limit on the shape of a message,
	// as the size limit is, so the connection fails with the close code 1009
	// and the error wraps [ErrMessageTooBig]. The code 1008 is a general policy
	// code, and 1009 says that the message is too big to process. A
	// message of a million frames is that.
	maxFrames = 1_000_000

	// retainLimit is the largest buffer that a connection keeps between two
	// messages. A larger buffer goes to the garbage collector.
	retainLimit = 4 << 20

	// readStep is the most that the reader grows its buffer in one step. A
	// peer must send the bytes before the buffer grows, so a header with a
	// large length does not make a large allocation.
	readStep = 1 << 20

	// readBufferSize is the size of the buffer for reads from the network.
	readBufferSize = 4096
)

// deadliner is the part of net.Conn that sets a write deadline. A connection
// uses it to limit the time of a close frame.
type deadliner interface {
	SetWriteDeadline(t time.Time) error
}

// Conn is a client WebSocket connection. One goroutine can call ReadMessage
// while another calls WriteText. Close can run at any time.
type Conn struct {
	// Logf, when set, receives a short line for each control frame that the
	// reader handles. Set it before the first read.
	Logf func(format string, args ...any)

	rwc         io.ReadWriteCloser
	br          *bufio.Reader
	dl          deadliner
	subprotocol string
	maxMessage  uint64
	maxFrames   int
	closeWait   time.Duration

	// The reader goroutine owns these fields.
	msg     []byte
	ctl     [maxControlPayload]byte
	readErr error

	// writeMu protects the fields below. It makes each frame one Write call,
	// so that a pong from the reader never splits a message.
	writeMu   sync.Mutex
	wbuf      []byte
	rng       *rand.ChaCha8
	writeErr  error
	closeSent bool

	// closing is set when Close starts, and closed when the network connection
	// is closed. A read that fails after either one reports net.ErrClosed, even
	// when the server closed first.
	closing atomic.Bool
	closed  atomic.Bool

	// readerActive is true from the first call of ReadMessage until a call
	// fails. Close waits for the close frame of the server only when a reader
	// can see it.
	readerActive atomic.Bool

	// done closes when the network connection is closed.
	done      chan struct{}
	closeOnce sync.Once
}

// newConn makes a connection on top of rwc. The dl argument can be nil. The
// mask keys of a connection come from a ChaCha8 generator that crypto/rand
// seeds. The keys are unpredictable, and the generator is faster than a call to
// crypto/rand for each frame. RFC 6455 section 10.3 asks for keys that a
// script cannot predict, and ChaCha8 gives that.
func newConn(rwc io.ReadWriteCloser, dl deadliner) (*Conn, error) {
	var seed [32]byte
	if _, err := crand.Read(seed[:]); err != nil {
		return nil, fmt.Errorf("seeding the mask generator: %w", err)
	}
	return &Conn{
		rwc:        rwc,
		br:         bufio.NewReaderSize(rwc, readBufferSize),
		dl:         dl,
		maxMessage: MaxMessageSize,
		maxFrames:  maxFrames,
		closeWait:  closeWait,
		done:       make(chan struct{}),
		rng:        rand.NewChaCha8(seed),
	}, nil
}

// Subprotocol returns the subprotocol that the server chose. It is empty when
// the server chose none.
func (c *Conn) Subprotocol() string {
	return c.subprotocol
}

// ReadMessage reads the next text or binary message. It joins the fragments of
// a message, answers a ping with a pong, and skips a pong. The returned bytes
// stay valid until the next call.
//
// A close frame from the server gets a close frame in answer. Then the
// method returns io.EOF for a normal close, or a [*CloseError]. A frame that
// breaks the protocol gets a close frame with the matching code, and the
// connection closes. The method returns an error that wraps [ErrProtocol],
// [ErrInvalidPayload] or [ErrMessageTooBig]. After an error, every call returns
// the same error. If the stream ends between two frames, the error is io.EOF.
// If a call fails because Close ran, the error wraps net.ErrClosed.
func (c *Conn) ReadMessage() (Opcode, []byte, error) {
	if c.readErr != nil {
		return 0, nil, c.readErr
	}
	c.readerActive.Store(true)
	op, msg, err := c.readMessage()
	if err != nil {
		if code := failCode(err); code != 0 {
			c.fail(code)
		}
		c.readErr = err
		c.readerActive.Store(false)
		// A Close that waits for the server needs no more time when the
		// stream is at its end.
		if c.closing.Load() {
			c.closeConn()
		}
		return 0, nil, err
	}
	return op, msg, nil
}

// readMessage reads frames until it has a whole message. RFC 6455 section 5.4
// describes fragmentation: the first frame has the type, the later frames are
// continuation frames, and the last one has the FIN bit. A control frame can
// come between two fragments.
func (c *Conn) readMessage() (Opcode, []byte, error) {
	if len(c.msg) > retainLimit {
		c.msg = nil
	}
	c.msg = c.msg[:0]

	var (
		op     Opcode
		inMsg  bool
		frames int
	)
	for {
		// A Close that waits for the close frame of the server keeps the
		// reader going, so only a closed connection stops it here.
		if c.closed.Load() {
			return 0, nil, fmt.Errorf("reading from a closed connection: %w", net.ErrClosed)
		}
		h, err := readFrameHeader(c.br)
		if err != nil {
			return 0, nil, c.ioError(err)
		}
		if frames++; frames > c.maxFrames {
			return 0, nil, fmt.Errorf("%w: a message has more than %d frames", ErrMessageTooBig, c.maxFrames)
		}
		if h.opcode.isControl() {
			if err := c.handleControl(h); err != nil {
				return 0, nil, err
			}
			continue
		}

		switch {
		case h.opcode == OpContinuation && !inMsg:
			return 0, nil, protocolError("a continuation frame has no message to continue")
		case h.opcode != OpContinuation && inMsg:
			return 0, nil, protocolError("a %s frame came while a message was open", h.opcode)
		case h.opcode != OpContinuation:
			op, inMsg = h.opcode, true
		}

		// Check the size before any read, so that a large length makes no
		// large allocation.
		if h.length > c.maxMessage || uint64(len(c.msg)) > c.maxMessage-h.length {
			return 0, nil, fmt.Errorf("%w: the limit is %d bytes", ErrMessageTooBig, c.maxMessage)
		}
		if err := c.readPayload(h.length); err != nil {
			return 0, nil, err
		}
		if !h.fin {
			continue
		}
		// RFC 6455 section 5.6: the text of a message must be UTF-8. A rune can
		// span two fragments, so the check runs on the whole message.
		if op == OpText && !utf8.Valid(c.msg) {
			return 0, nil, fmt.Errorf("%w: a text message is not UTF-8", ErrInvalidPayload)
		}
		return op, c.msg, nil
	}
}

// readPayload appends n bytes from the stream to c.msg. It grows the buffer
// in steps, so the buffer follows the bytes that arrived.
func (c *Conn) readPayload(n uint64) error {
	for n > 0 {
		step := int(min(n, readStep))
		c.msg = slices.Grow(c.msg, step)
		start := len(c.msg)
		c.msg = c.msg[:start+step]
		if _, err := io.ReadFull(c.br, c.msg[start:]); err != nil {
			return c.ioError(unexpectedEOF(err))
		}
		n -= uint64(step)
	}
	return nil
}

// handleControl reads and handles one control frame. It returns nil when the
// message read can go on. Otherwise it returns the error that ends the read.
func (c *Conn) handleControl(h frameHeader) error {
	p := c.ctl[:h.length]
	if _, err := io.ReadFull(c.br, p); err != nil {
		return c.ioError(unexpectedEOF(err))
	}

	switch h.opcode {
	case OpPing:
		// RFC 6455 section 5.5.2: answer with a pong that has the same
		// payload.
		c.logf("received ping frame, sending pong")
		if err := c.writeFrame(OpPong, p); err != nil {
			// After Close started, no frame can follow the close frame. The
			// pong is dropped, so that it does not hide the real error.
			if c.isClosed() || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("writing a pong frame: %w", err)
		}
	case OpPong:
		// RFC 6455 section 5.5.3: a pong can come without a ping.
		c.logf("received pong frame, ignoring")
	case OpClose:
		c.logf("received close frame")
		code, reason, err := parseClose(p)
		if err != nil {
			return err
		}
		// RFC 6455 section 5.5.1: answer with the same status code.
		var reply []byte
		if code != closeNoStatusReceived {
			reply = p[:2]
		}
		// The answer is best effort. The server wants to close, so a failed
		// write must not hide the result. Then close the network connection
		// at once, because the server closes its side after the frame.
		c.writeClose(reply)
		c.closeConn()
		return closeResult(code, reason)
	}
	return nil
}

// ioError maps the error of a network read. A read that fails after Close
// reports net.ErrClosed.
func (c *Conn) ioError(err error) error {
	if c.isClosed() {
		return fmt.Errorf("reading from a closed connection: %w", net.ErrClosed)
	}
	return err
}

// isClosed reports whether Close started.
func (c *Conn) isClosed() bool {
	return c.closing.Load() || c.closed.Load()
}

func (c *Conn) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// WriteText sends p as one text frame. RFC 6455 section 5.6 asks the caller
// for valid UTF-8. Chrome does not support incoming fragments, so the method
// never splits a message. It makes one Write call on the connection.
func (c *Conn) WriteText(p []byte) error {
	return c.writeFrame(OpText, p)
}

// writeFrame writes one masked frame with the FIN bit set. It copies the
// payload into the buffer of the connection and masks it there, so the bytes
// of the caller stay as they are. The header and the payload go out in one
// Write call.
func (c *Conn) writeFrame(op Opcode, p []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.writeFrameLocked(op, p)
}

// writeFrameLocked is writeFrame for a caller that holds writeMu.
func (c *Conn) writeFrameLocked(op Opcode, p []byte) error {
	if c.writeErr != nil {
		return c.writeErr
	}
	// This check and the write below run under writeMu, together with the
	// write of the close frame. So no data frame can follow a close frame.
	if c.closeSent || c.closed.Load() {
		return fmt.Errorf("writing after the close frame: %w", net.ErrClosed)
	}
	if len(c.wbuf) > retainLimit {
		c.wbuf = nil
	}
	key := c.maskKey()
	b := slices.Grow(c.wbuf[:0], maxHeaderLen+len(p))
	b = appendHeader(b, op, len(p), key)
	hdr := len(b)
	b = append(b, p...)
	maskBytes(key, b[hdr:])
	c.wbuf = b
	if _, err := c.rwc.Write(b); err != nil {
		// A part of the frame can be on the wire, so no later frame is safe.
		c.writeErr = err
		return err
	}
	return nil
}

// maskKey returns a new random mask key. The caller holds writeMu.
func (c *Conn) maskKey() [4]byte {
	v := c.rng.Uint64()
	return [4]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

// writeClose sends a close frame once, with the given payload. A later call
// does nothing. The write has a time limit when the connection has a
// deadline. RFC 6455 section 5.5.1 describes the frame.
func (c *Conn) writeClose(payload []byte) error {
	// A write that blocks holds the mutex. The deadline ends it.
	c.setWriteDeadline()
	if c.dl == nil {
		if !c.writeMu.TryLock() {
			return nil
		}
	} else {
		c.writeMu.Lock()
	}
	defer c.writeMu.Unlock()
	if c.closeSent {
		return nil
	}
	c.setWriteDeadline()
	err := c.writeFrameLocked(OpClose, payload)
	c.closeSent = true
	return err
}

func (c *Conn) setWriteDeadline() {
	if c.dl != nil {
		c.dl.SetWriteDeadline(time.Now().Add(closeTimeout))
	}
}

// fail ends the connection after a protocol error. RFC 6455 section 7.1.7
// says to send a close frame with the code, and then to close the connection.
func (c *Conn) fail(code uint16) {
	c.writeClose([]byte{byte(code >> 8), byte(code)})
	c.closeConn()
}

// Close sends a close frame with the code 1000 if it can do that in a short
// time. Then it waits for at most one second for the close frame of the server
// or for the end of the stream. The goroutine that reads sees both, so the
// wait happens only when a reader runs. At the end, Close closes the network
// connection. A Write that runs after the close frame fails. It is safe to
// call Close more than once, and while another goroutine reads or writes. A
// call after the connection is closed returns nil.
//
// Close holds no lock while it waits, so a Write or a ReadMessage can return
// at any time.
func (c *Conn) Close() error {
	c.closing.Store(true)
	err := c.writeClose([]byte{byte(CloseNormal >> 8), byte(CloseNormal & 0xff)})
	if err == nil && c.readerActive.Load() {
		t := time.NewTimer(c.closeWait)
		defer t.Stop()
		select {
		case <-c.done:
		case <-t.C:
		}
	}
	return c.closeConn()
}

// closeConn closes the network connection once. It returns the error of the
// close for the first call, and nil for later calls.
func (c *Conn) closeConn() error {
	var err error
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		err = c.rwc.Close()
		close(c.done)
	})
	return err
}
