package chromedp

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync/atomic"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"
)

// Browser is the high-level Chrome DevTools Protocol browser manager, handling
// the browser process runner, WebSocket clients, associated targets, and
// network, page, and DOM events.
type Browser struct {
	conn Transport

	// next is the next message id.
	next int64

	// tabQueue is the queue used to create new target handlers, once a new
	// tab is created and attached to. The newly created Target is sent back
	// via tabResult.
	tabQueue  chan newTab
	tabResult chan *Target

	// cmdQueue is the outgoing command queue.
	cmdQueue chan cmdJob

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

type newTab struct {
	targetID  target.ID
	sessionID target.SessionID
}

type cmdJob struct {
	msg  *cdproto.Message
	resp chan *cdproto.Message
}

// NewBrowser creates a new browser.
func NewBrowser(ctx context.Context, urlstr string, opts ...BrowserOption) (*Browser, error) {
	b := &Browser{
		tabQueue:  make(chan newTab, 1),
		tabResult: make(chan *Target, 1),
		cmdQueue:  make(chan cmdJob),
		logf:      log.Printf,
	}
	// apply options
	for _, o := range opts {
		o(b)
	}
	// ensure errf is set
	if b.errf == nil {
		b.errf = func(s string, v ...interface{}) { b.logf("ERROR: "+s, v...) }
	}

	// dial
	var err error
	b.conn, err = DialContext(ctx, ForceIP(urlstr), WithConnDebugf(b.dbgf))
	if err != nil {
		return nil, err
	}

	go b.run(ctx)
	return b, nil
}

func (b *Browser) newExecutorForTarget(ctx context.Context, targetID target.ID, sessionID target.SessionID) *Target {
	if targetID == "" {
		panic("empty target ID")
	}
	if sessionID == "" {
		panic("empty session ID")
	}
	b.tabQueue <- newTab{targetID, sessionID}
	return <-b.tabResult
}

func (b *Browser) Execute(ctx context.Context, method string, params json.Marshaler, res json.Unmarshaler) error {
	paramsMsg := emptyObj
	if params != nil {
		var err error
		if paramsMsg, err = json.Marshal(params); err != nil {
			return err
		}
	}

	id := atomic.AddInt64(&b.next, 1)
	ch := make(chan *cdproto.Message, 1)
	b.cmdQueue <- cmdJob{
		msg: &cdproto.Message{
			ID:     id,
			Method: cdproto.MethodType(method),
			Params: paramsMsg,
		},
		resp: ch,
	}
	select {
	case msg := <-ch:
		switch {
		case msg == nil:
			return ErrChannelClosed
		case msg.Error != nil:
			return msg.Error
		case res != nil:
			return json.Unmarshal(msg.Result, res)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

type tabEvent struct {
	sessionID target.SessionID
	msg       *cdproto.Message
}

//go:generate easyjson browser.go

//easyjson:json
type eventReceivedMessageFromTarget struct {
	SessionID target.SessionID `json:"sessionId"`
	Message   messageString    `json:"message"`
}

type messageString struct {
	M cdproto.Message
}

func (m *messageString) UnmarshalEasyJSON(l *jlexer.Lexer) {
	if l.IsNull() {
		l.Skip()
	} else {
		easyjson.Unmarshal(l.UnsafeBytes(), &m.M)
	}
}

func (b *Browser) run(ctx context.Context) {
	defer b.conn.Close()

	cancel := FromContext(ctx).cancel

	// tabEventQueue is the queue of incoming target events, to be routed by
	// their session ID.
	tabEventQueue := make(chan tabEvent, 1)

	// resQueue is the incoming command result queue.
	resQueue := make(chan *cdproto.Message, 1)

	// This goroutine continuously reads events from the websocket
	// connection. The separate goroutine is needed since a websocket read
	// is blocking, so it cannot be used in a select statement.
	go func() {
		for {
			msg, err := b.conn.Read()
			if err != nil {
				// If the websocket failed, most likely Chrome
				// was closed or crashed. Cancel the entire
				// Browser context to stop all activity.
				cancel()
				return
			}
			if msg.Method == cdproto.EventRuntimeExceptionThrown {
				ev := new(runtime.EventExceptionThrown)
				if err := easyjson.Unmarshal(msg.Params, ev); err != nil {
					b.errf("%s", err)
					continue
				}
				b.errf("%+v\n", ev.ExceptionDetails)
				continue
			}

			var sessionID target.SessionID
			if msg.Method == cdproto.EventTargetReceivedMessageFromTarget {
				event := new(eventReceivedMessageFromTarget)
				if err := easyjson.Unmarshal(msg.Params, event); err != nil {
					b.errf("%s", err)
					continue
				}
				sessionID = event.SessionID
				msg = &event.Message.M
			}
			switch {
			case msg.Method != "":
				if sessionID == "" {
					// TODO: are we interested in browser events?
					continue
				}
				tabEventQueue <- tabEvent{
					sessionID: sessionID,
					msg:       msg,
				}
			case msg.ID != 0:
				// We can't process the response here, as it's
				// another goroutine that maintans respByID.
				resQueue <- msg
			default:
				b.errf("ignoring malformed incoming message (missing id or method): %#v", msg)
			}
		}
	}()

	// This goroutine handles tabs, as well as routing events to each tab
	// via the pages map.
	go func() {
		// This map is only safe for use within this goroutine, so don't
		// declare it as a Browser field.
		pages := make(map[target.SessionID]*Target, 1024)
		for {
			select {
			case tab := <-b.tabQueue:
				if _, ok := pages[tab.sessionID]; ok {
					b.errf("executor for %q already exists", tab.sessionID)
				}
				t := &Target{
					browser:   b,
					TargetID:  tab.targetID,
					SessionID: tab.sessionID,

					eventQueue: make(chan *cdproto.Message, 1024),
					waitQueue:  make(chan func(cur *cdp.Frame) bool, 1024),
					frames:     make(map[cdp.FrameID]*cdp.Frame),

					logf: b.logf,
					errf: b.errf,
				}
				go t.run(ctx)
				pages[tab.sessionID] = t
				b.tabResult <- t
			case event := <-tabEventQueue:
				page, ok := pages[event.sessionID]
				if !ok {
					b.errf("unknown session ID %q", event.sessionID)
					continue
				}
				select {
				case page.eventQueue <- event.msg:
				default:
					panic("eventQueue is full")
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	respByID := make(map[int64]chan *cdproto.Message)

	// This goroutine handles sending commands to the browser, and sending
	// responses back for each of these commands via respByID.
	for {
		select {
		case res := <-resQueue:
			resp, ok := respByID[res.ID]
			if !ok {
				b.errf("id %d not present in response map", res.ID)
				continue
			}
			if resp != nil {
				// resp could be nil, if we're not interested in
				// this response; for CommandSendMessageToTarget.
				resp <- res
				close(resp)
			}
			delete(respByID, res.ID)

		case q := <-b.cmdQueue:
			if _, ok := respByID[q.msg.ID]; ok {
				b.errf("id %d already present in response map", q.msg.ID)
				continue
			}
			respByID[q.msg.ID] = q.resp

			if q.msg.Method == "" {
				// Only register the chananel in respByID;
				// useful for CommandSendMessageToTarget.
				continue
			}
			if err := b.conn.Write(q.msg); err != nil {
				b.errf("%s", err)
				continue
			}

		case <-ctx.Done():
			return
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
	return func(b *Browser) {
	}
}
