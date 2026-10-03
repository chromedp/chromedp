package chromedp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
)

var (
	// DefaultUnmarshalOptions are default unmarshal options.
	DefaultUnmarshalOptions = jsonv2.JoinOptions(
		jsonv2.DefaultOptionsV2(),
		jsontext.AllowInvalidUTF8(true),
	)
	// DefaultMarshalOptions are default marshal options.
	DefaultMarshalOptions = jsonv2.JoinOptions(
		jsonv2.DefaultOptionsV2(),
		jsontext.AllowInvalidUTF8(true),
	)
)

// Browser manages a browser through the Chrome DevTools Protocol. It handles
// the browser process runner, the WebSocket clients, the targets, and the
// network, page, and DOM events.
type Browser struct {
	// next is the next message id.
	// NOTE: it must be 64-bit aligned on 32-bit targets too, so be careful when you move this field.
	// The compiler will do this when https://github.com/golang/go/issues/599 is fixed.
	next int64

	// LostConnection is closed when the websocket connection to Chrome drops.
	// Use it to make sure that the context of the Browser is canceled (and the
	// handler stopped) after the connection fails.
	LostConnection chan struct{}

	// closingGracefully is closed by Close before it shuts the browser down
	// gracefully. If the connection to the browser is lost and LostConnection
	// is closed, we then know not to kill the Chrome process at once. This is
	// important, because the browser must shut itself off and save its state
	// to disk.
	closingGracefully chan struct{}

	dialTimeout time.Duration

	// pages tracks the attached targets by session ID. It is a field only so
	// that the tests can read the map after a browser closes.
	pages map[target.SessionID]*Target

	listenersMu sync.Mutex
	listeners   []cancelableListener

	// events holds the subscriptions made with Subscribe.
	events subscribers

	conn Transport

	// newTabQueue is the queue of requests to create new target handlers,
	// after a new tab is created and attached. The new Target is sent back on
	// newTabResult.
	newTabQueue chan *Target

	// cmdQueue is the outgoing command queue.
	cmdQueue chan *cdproto.Message

	// logging funcs
	logf func(string, ...any)
	errf func(string, ...any)
	dbgf func(string, ...any)

	// The optional fields below help some tests.

	// process can be set by the allocators that start a process when they
	// allocate a browser.
	process *os.Process

	// userDataDir can be set by the allocators that set user data directories
	// directly.
	userDataDir string
}

// NewBrowser creates a new browser. Typically you do not call it directly,
// because the Allocator interface does it.
//
// It dials the websocket address urlstr. To use a connection that is already
// open, see [NewBrowserTransport].
func NewBrowser(ctx context.Context, urlstr string, opts ...BrowserOption) (*Browser, error) {
	// Apply the options once here, as the dial needs the timeout and the
	// debug logger.
	b := newBrowser(opts)

	dialCtx := ctx
	if b.dialTimeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, b.dialTimeout)
		defer cancel()
	}

	conn, err := DialContext(dialCtx, urlstr, WithConnDebugf(b.dbgf))
	if err != nil {
		return nil, fmt.Errorf("could not dial %q: %w", urlstr, err)
	}
	return NewBrowserTransport(ctx, conn, opts...)
}

// NewBrowserTransport creates a new browser that uses tr, a connection to a
// browser that is already open, such as a [*PipeConn]. The browser closes tr
// when it stops.
func NewBrowserTransport(ctx context.Context, tr Transport, opts ...BrowserOption) (*Browser, error) {
	b := newBrowser(opts)
	if s, ok := tr.(interface{ setDebugf(func(string, ...any)) }); ok {
		s.setDebugf(b.dbgf)
	}
	b.conn = tr

	go b.run(ctx)
	return b, nil
}

// newBrowser returns a browser with the options applied, and without a
// connection.
func newBrowser(opts []BrowserOption) *Browser {
	b := &Browser{
		LostConnection:    make(chan struct{}),
		closingGracefully: make(chan struct{}),

		dialTimeout: 10 * time.Second,

		newTabQueue: make(chan *Target),

		// Fit some jobs without blocking, to reduce blocking in Execute.
		cmdQueue: make(chan *cdproto.Message, 32),

		logf: log.Printf,
	}
	// apply options
	for _, o := range opts {
		o(b)
	}
	// make sure that errf is set
	if b.errf == nil {
		b.errf = func(s string, v ...any) { b.logf("ERROR: "+s, v...) }
	}
	return b
}

// Process returns the process object of the browser.
//
// It is nil when the browser was allocated with RemoteAllocator. A monitoring
// system can use it to collect process metrics of the browser process (see
// [prometheus.NewProcessCollector] for an example).
//
// Example:
//
//	if process := chromedp.FromContext(ctx).Browser.Process(); process != nil {
//		fmt.Printf("Browser PID: %v", process.Pid)
//	}
//
// [prometheus.NewProcessCollector]: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#NewProcessCollector
func (b *Browser) Process() *os.Process {
	return b.process
}

func (b *Browser) newTarget(ctx context.Context, targetID target.ID, sessionID target.SessionID) (*Target, error) {
	if targetID == "" {
		return nil, errors.New("empty target ID")
	}
	if sessionID == "" {
		return nil, errors.New("empty session ID")
	}
	t := &Target{
		browser:   b,
		TargetID:  targetID,
		SessionID: sessionID,

		messageQueue:  make(chan *cdproto.Message, 1024),
		frames:        make(map[cdp.FrameID]*Frame),
		execContexts:  make(map[cdp.FrameID]runtime.ExecutionContextID),
		execUniqueIDs: make(map[cdp.FrameID]string),
		cur:           cdp.FrameID(targetID),

		logf: b.logf,
		errf: b.errf,
	}

	// This send must block, so that the tab is in the map before any more
	// target events are routed.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b.newTabQueue <- t:
	}
	return t, nil
}

// Call sends the command to the browser, waits for the response, and decodes
// the result into res. It satisfies [cdp.Session].
//
// Call returns a browser error as a [*cdproto.Error].
func (b *Browser) Call(ctx context.Context, method string, params, res any) error {
	// Certain methods are not available to the user directly.
	if method == browser.CommandClose {
		return fmt.Errorf("to close the browser gracefully, use chromedp.Cancel")
	}
	return b.execute(ctx, method, params, res)
}

// Subscribe starts to buffer the browser events with the method, and returns
// the channel that the raw event parameters arrive on. It satisfies
// [cdp.Session].
func (b *Browser) Subscribe(method string) (<-chan jsontext.Value, func()) {
	return b.events.subscribe(method)
}

func (b *Browser) execute(ctx context.Context, method string, params, res any) error {
	id := atomic.AddInt64(&b.next, 1)
	lctx, cancel := context.WithCancel(ctx)
	ch := make(chan *cdproto.Message, 1)
	fn := func(ev any) {
		if msg, ok := ev.(*cdproto.Message); ok && msg.ID == id {
			select {
			case <-ctx.Done():
			case ch <- msg:
			}
			cancel()
		}
	}
	b.listenersMu.Lock()
	b.listeners = append(b.listeners, cancelableListener{lctx, fn})
	b.listenersMu.Unlock()

	// send command
	var buf []byte
	if params != nil {
		var err error
		if buf, err = jsonv2.Marshal(params, DefaultMarshalOptions); err != nil {
			return err
		}
	}
	cmd := &cdproto.Message{
		ID:     id,
		Method: cdproto.MethodType(method),
		Params: buf,
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.cmdQueue <- cmd:
	}

	// wait for result
	select {
	case <-ctx.Done():
		return ctx.Err()
	case msg := <-ch:
		switch {
		case msg == nil:
			return ErrChannelClosed
		case msg.Error != nil:
			return msg.Error
		case res != nil:
			return jsonv2.Unmarshal(msg.Result, res, DefaultUnmarshalOptions)
		}
	}
	return nil
}

func (b *Browser) run(ctx context.Context) {
	defer b.conn.Close()
	defer b.events.close()

	// incomingQueue is the queue of incoming target events, to be routed by
	// their session ID.
	incomingQueue := make(chan *cdproto.Message, 1)

	delTabQueue := make(chan target.SessionID, 1)

	// This goroutine reads events from the websocket connection all the time.
	// It needs its own goroutine, because a websocket read blocks and cannot
	// be part of a select statement.
	go func() {
		// Signal to run and exit the browser cleanup goroutine.
		defer close(b.LostConnection)

		for {
			msg := new(cdproto.Message)
			if err := b.conn.Read(ctx, msg); err != nil {
				if _, ok := errors.AsType[*jsontext.SyntacticError](err); ok {
					b.errf("%s", err)
				}
				return
			}

			switch {
			case msg.SessionID != "" && (msg.Method != "" || msg.ID != 0):
				select {
				case <-ctx.Done():
					return
				case incomingQueue <- msg:
				}

			case msg.Method != "":
				b.events.publish(string(msg.Method), msg.Params)
				ev, err := cdproto.UnmarshalMessage(msg, DefaultUnmarshalOptions)
				if err != nil {
					b.errf("%s", err)
					continue
				}
				b.listenersMu.Lock()
				b.listeners = runListeners(b.listeners, ev)
				b.listenersMu.Unlock()

				if ev, ok := ev.(*target.EventDetachedFromTarget); ok {
					delTabQueue <- ev.SessionID
				}

			case msg.ID != 0:
				b.listenersMu.Lock()
				b.listeners = runListeners(b.listeners, msg)
				b.listenersMu.Unlock()

			default:
				b.errf("ignoring malformed incoming message (missing id or method): %#v", msg)
			}
		}
	}()

	b.pages = make(map[target.SessionID]*Target, 32)
	for {
		select {
		case <-ctx.Done():
			return

		case msg := <-b.cmdQueue:
			if err := b.conn.Write(ctx, msg); err != nil {
				b.errf("%s", err)
				continue
			}

		case t := <-b.newTabQueue:
			if _, ok := b.pages[t.SessionID]; ok {
				b.errf("executor for %q already exists", t.SessionID)
			}
			b.pages[t.SessionID] = t

		case sessionID := <-delTabQueue:
			if _, ok := b.pages[sessionID]; !ok {
				b.errf("executor for %q doesn't exist", sessionID)
			}
			delete(b.pages, sessionID)

		case m := <-incomingQueue:
			page, ok := b.pages[m.SessionID]
			if !ok {
				// A page that we closed recently still sends events.
				continue
			}

			select {
			case <-ctx.Done():
				return
			case page.messageQueue <- m:
			}

		case <-b.LostConnection:
			return // to avoid "write: broken pipe" errors
		}
	}
}

// BrowserOption is a browser option.
type BrowserOption = func(*Browser)

// WithBrowserLogf is a browser option that sets the func that receives general
// log messages.
func WithBrowserLogf(f func(string, ...any)) BrowserOption {
	return func(b *Browser) { b.logf = f }
}

// WithBrowserErrorf is a browser option that sets the func that receives error
// messages.
func WithBrowserErrorf(f func(string, ...any)) BrowserOption {
	return func(b *Browser) { b.errf = f }
}

// WithBrowserDebugf is a browser option that sets the func that receives the
// websocket messages.
func WithBrowserDebugf(f func(string, ...any)) BrowserOption {
	return func(b *Browser) { b.dbgf = f }
}

// WithConsolef is a browser option that sets the func that receives chrome log
// events.
//
// Note: it is not implemented yet.
func WithConsolef(f func(string, ...any)) BrowserOption {
	return func(b *Browser) {}
}

// WithDialTimeout is a browser option that sets the timeout for dialing the
// websocket address of the browser. The default is ten seconds. Use a zero
// duration for no timeout.
func WithDialTimeout(d time.Duration) BrowserOption {
	return func(b *Browser) { b.dialTimeout = d }
}
