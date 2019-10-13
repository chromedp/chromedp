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

	easyjson "github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
)

// Browser is the high-level Chrome DevTools Protocol browser manager, handling
// the browser process runner, WebSocket clients, associated targets, and
// network, page, and DOM events.
type Browser struct {
	// LostConnection is closed when the websocket connection to Chrome is
	// dropped. This can be useful to make sure that Browser's context is
	// cancelled (and the handler stopped) once the connection has failed.
	LostConnection chan struct{}

	dialTimeout time.Duration

	listenersMu sync.Mutex
	listeners   []cancelableListener

	conn Transport

	// next is the next message id.
	next int64

	// newTabQueue is the queue used to create new target handlers, once a new
	// tab is created and attached to. The newly created Target is sent back
	// via newTabResult.
	newTabQueue chan *Target

	// cmdQueue is the outgoing command queue.
	cmdQueue chan *cdproto.Message

	// logging funcs
	logf func(string, ...interface{})
	errf func(string, ...interface{})
	dbgf func(string, ...interface{})

	// The optional fields below are helpful for some tests.

	// process can be initialized by the allocators which start a process
	// when allocating a browser.
	process *os.Process

	// userDataDir can be initialized by the allocators which set up user
	// data dirs directly.
	userDataDir string
}

// NewBrowser creates a new browser. Typically, this function wouldn't be called
// directly, as the Allocator interface takes care of it.
func NewBrowser(ctx context.Context, urlstr string, opts ...BrowserOption) (*Browser, error) {
	b := &Browser{
		LostConnection: make(chan struct{}),

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
	// ensure errf is set
	if b.errf == nil {
		b.errf = func(s string, v ...interface{}) { b.logf("ERROR: "+s, v...) }
	}

	dialCtx := ctx
	if b.dialTimeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, b.dialTimeout)
		defer cancel()
	}

	var err error
	urlstr = forceIP(urlstr)
	b.conn, err = DialContext(dialCtx, urlstr, WithConnDebugf(b.dbgf))
	if err != nil {
		return nil, fmt.Errorf("could not dial %q: %v", urlstr, err)
	}

	go b.run(ctx)
	return b, nil
}

func (b *Browser) newExecutorForTarget(ctx context.Context, targetID target.ID, sessionID target.SessionID) (*Target, error) {
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

		messageQueue: make(chan *cdproto.Message, 1024),
		frames:       make(map[cdp.FrameID]*cdp.Frame),

		logf: b.logf,
		errf: b.errf,
	}

	// This send should be blocking, to ensure the tab is inserted into the
	// map before any more target events are routed.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b.newTabQueue <- t:
	}
	return t, nil
}

func (b *Browser) Execute(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	if method == browser.CommandClose {
		return fmt.Errorf("to close the browser, cancel its context or use chromedp.Cancel")
	}

	id := atomic.AddInt64(&b.next, 1)
	lctx, cancel := context.WithCancel(ctx)
	ch := make(chan *cdproto.Message, 1)
	fn := func(ev interface{}) {
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
		buf, err = easyjson.Marshal(params)
		if err != nil {
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
			return easyjson.Unmarshal(msg.Result, res)
		}
	}
	return nil
}

func (b *Browser) run(ctx context.Context) {
	defer b.conn.Close()

	// incomingQueue is the queue of incoming target events, to be routed by
	// their session ID.
	incomingQueue := make(chan *cdproto.Message, 1)

	// This goroutine continuously reads events from the websocket
	// connection. The separate goroutine is needed since a websocket read
	// is blocking, so it cannot be used in a select statement.
	go func() {
		// Reuse the space for the read message, since in some cases
		// like EventTargetReceivedMessageFromTarget we throw it away.
		lexer := new(jlexer.Lexer)
		for {
			msg := new(cdproto.Message)
			if err := b.conn.Read(ctx, msg); err != nil {
				// If the websocket failed, most likely Chrome was closed or
				// crashed. Signal that so the entire browser handler can be
				// stopped.
				close(b.LostConnection)
				return
			}
			if msg.Method == cdproto.EventRuntimeExceptionThrown {
				ev := new(runtime.EventExceptionThrown)
				*lexer = jlexer.Lexer{Data: msg.Params}
				ev.UnmarshalJSON(msg.Params)
				if err := lexer.Error(); err != nil {
					b.errf("%s", err)
					continue
				}
				b.errf("%+v\n", ev.ExceptionDetails)
				continue
			}

			switch {
			case msg.SessionID != "" && (msg.Method != "" || msg.ID != 0):
				select {
				case <-ctx.Done():
					return
				case incomingQueue <- msg:
				}

			case msg.Method != "":
				ev, err := cdproto.UnmarshalMessage(msg)
				if err != nil {
					b.errf("%s", err)
					continue
				}
				b.listenersMu.Lock()
				b.listeners = runListeners(b.listeners, ev)
				b.listenersMu.Unlock()

			case msg.ID != 0:
				b.listenersMu.Lock()
				b.listeners = runListeners(b.listeners, msg)
				b.listenersMu.Unlock()

			default:
				b.errf("ignoring malformed incoming message (missing id or method): %#v", msg)
			}
		}
	}()

	pages := make(map[target.SessionID]*Target, 32)
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
			if _, ok := pages[t.SessionID]; ok {
				b.errf("executor for %q already exists", t.SessionID)
			}
			pages[t.SessionID] = t

		case m := <-incomingQueue:
			page, ok := pages[m.SessionID]
			if !ok {
				// A page we recently closed still sending events.
				continue
			}
			page.messageQueue <- m
			if m.Method == cdproto.EventTargetDetachedFromTarget {
				if _, ok := pages[m.SessionID]; !ok {
					b.errf("executor for %q doesn't exist", m.SessionID)
				}
				delete(pages, m.SessionID)
			}

		case <-b.LostConnection:
			return // to avoid "write: broken pipe" errors
		}
	}
}

// BrowserOption is a browser option.
type BrowserOption = func(*Browser)

// WithBrowserLogf is a browser option to specify a func to receive general logging.
func WithBrowserLogf(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) { b.logf = f }
}

// WithBrowserErrorf is a browser option to specify a func to receive error logging.
func WithBrowserErrorf(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) { b.errf = f }
}

// WithBrowserDebugf is a browser option to specify a func to log actual
// websocket messages.
func WithBrowserDebugf(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) { b.dbgf = f }
}

// WithConsolef is a browser option to specify a func to receive chrome log events.
//
// Note: NOT YET IMPLEMENTED.
func WithConsolef(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) {}
}

// WithDialTimeout is a browser option to specify the timeout when dialing a
// browser's websocket address. The default is ten seconds; use a zero duration
// to not use a timeout.
func WithDialTimeout(d time.Duration) BrowserOption {
	return func(b *Browser) { b.dialTimeout = d }
}
