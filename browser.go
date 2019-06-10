package chromedp

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	easyjson "github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
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

// forceIP forces the host component in urlstr to be an IP address.
//
// Since Chrome 66+, Chrome DevTools Protocol clients connecting to a browser
// must send the "Host:" header as either an IP address, or "localhost".
func forceIP(urlstr string) string {
	if i := strings.Index(urlstr, "://"); i != -1 {
		scheme := urlstr[:i+3]
		host, port, path := urlstr[len(scheme)+3:], "", ""
		if i := strings.Index(host, "/"); i != -1 {
			host, path = host[:i], host[i:]
		}
		if i := strings.Index(host, ":"); i != -1 {
			host, port = host[:i], host[i:]
		}
		if addr, err := net.ResolveIPAddr("ip", host); err == nil {
			urlstr = scheme + addr.IP.String() + port + path
		}
	}
	return urlstr
}

func (b *Browser) newExecutorForTarget(targetID target.ID, sessionID target.SessionID) *Target {
	if targetID == "" {
		panic("empty target ID")
	}
	if sessionID == "" {
		panic("empty session ID")
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
	b.newTabQueue <- t
	return t
}

func rawMarshal(v easyjson.Marshaler) easyjson.RawMessage {
	if v == nil {
		return nil
	}
	buf, err := easyjson.Marshal(v)
	if err != nil {
		panic(err)
	}
	return buf
}

func (b *Browser) Execute(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	id := atomic.AddInt64(&b.next, 1)
	lctx, cancel := context.WithCancel(ctx)
	ch := make(chan *cdproto.Message, 1)
	fn := func(ev interface{}) {
		if msg, ok := ev.(*cdproto.Message); ok && msg.ID == id {
			ch <- msg
			cancel()
		}
	}
	b.listenersMu.Lock()
	b.listeners = append(b.listeners, cancelableListener{lctx, fn})
	b.listenersMu.Unlock()

	b.cmdQueue <- &cdproto.Message{
		ID:     id,
		Method: cdproto.MethodType(method),
		Params: rawMarshal(params),
	}
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

type tabMessage struct {
	sessionID target.SessionID
	msg       *cdproto.Message
}

//go:generate easyjson browser.go

//easyjson:json
type eventReceivedMessageFromTarget struct {
	SessionID target.SessionID `json:"sessionId"`
	Message   decMessageString `json:"message"`
}

type decMessageString struct {
	lexer jlexer.Lexer // to avoid an alloc
	m     cdproto.Message
}

func (m *decMessageString) UnmarshalEasyJSON(l *jlexer.Lexer) {
	if l.IsNull() {
		l.Skip()
	} else {
		l.AddError(unmarshal(&m.lexer, l.UnsafeBytes(), &m.m))
	}
}

//easyjson:json
type sendMessageToTargetParams struct {
	Message   encMessageString `json:"message"`
	SessionID target.SessionID `json:"sessionId,omitempty"`
}

type encMessageString struct {
	Message cdproto.Message
}

func (m encMessageString) MarshalEasyJSON(w *jwriter.Writer) {
	var w2 jwriter.Writer
	m.Message.MarshalEasyJSON(&w2)
	w.RawText(w2.BuildBytes(nil))
}

func (b *Browser) run(ctx context.Context) {
	defer b.conn.Close()

	// tabMessageQueue is the queue of incoming target events, to be routed by
	// their session ID.
	tabMessageQueue := make(chan tabMessage, 1)

	// This goroutine continuously reads events from the websocket
	// connection. The separate goroutine is needed since a websocket read
	// is blocking, so it cannot be used in a select statement.
	go func() {
		// Reuse the space for the read message, since in some cases
		// like EventTargetReceivedMessageFromTarget we throw it away.
		lexer := new(jlexer.Lexer)
		readMsg := new(cdproto.Message)
		for {
			*readMsg = cdproto.Message{}
			if err := b.conn.Read(ctx, readMsg); err != nil {
				// If the websocket failed, most likely Chrome
				// was closed or crashed. Signal that so the
				// entire browser handler can be stopped.
				close(b.LostConnection)
				return
			}
			if readMsg.Method == cdproto.EventRuntimeExceptionThrown {
				ev := new(runtime.EventExceptionThrown)
				if err := unmarshal(lexer, readMsg.Params, ev); err != nil {
					b.errf("%s", err)
					continue
				}
				b.errf("%+v\n", ev.ExceptionDetails)
				continue
			}

			var msg *cdproto.Message
			var sessionID target.SessionID
			if readMsg.Method == cdproto.EventTargetReceivedMessageFromTarget {
				ev := new(eventReceivedMessageFromTarget)
				if err := unmarshal(lexer, readMsg.Params, ev); err != nil {
					b.errf("%s", err)
					continue
				}
				sessionID = ev.SessionID
				msg = &ev.Message.m
			} else {
				// We're passing along readMsg to another
				// goroutine, so we must make a copy of it.
				msg = new(cdproto.Message)
				*msg = *readMsg
			}
			switch {
			case msg.Method != "":
				if sessionID == "" {
					ev, err := cdproto.UnmarshalMessage(msg)
					if err != nil {
						b.errf("%s", err)
						continue
					}
					b.listenersMu.Lock()
					b.listeners = runListeners(b.listeners, ev)
					b.listenersMu.Unlock()
					// TODO: are other browser events useful?
					if ev, ok := ev.(*target.EventDetachedFromTarget); ok {
						tabMessageQueue <- tabMessage{ev.SessionID, msg}
					}
					continue
				}
				tabMessageQueue <- tabMessage{
					sessionID: sessionID,
					msg:       msg,
				}
			case msg.ID != 0:
				if sessionID == "" {
					b.listenersMu.Lock()
					b.listeners = runListeners(b.listeners, msg)
					b.listenersMu.Unlock()
					continue
				}
				tabMessageQueue <- tabMessage{sessionID, msg}

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

		case tm := <-tabMessageQueue:
			page, ok := pages[tm.sessionID]
			if !ok {
				// A page we recently closed still sending events.
				continue
			}
			page.messageQueue <- tm.msg
			if tm.msg.Method == cdproto.EventTargetDetachedFromTarget {
				if _, ok := pages[tm.sessionID]; !ok {
					b.errf("executor for %q doesn't exist", tm.sessionID)
				}
				delete(pages, tm.sessionID)
			}

		case <-b.LostConnection:
			return // to avoid "write: broken pipe" errors
		}
	}
}

// BrowserOption is a browser option.
type BrowserOption func(*Browser)

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
