// Package chromedp is a high level Chrome DevTools Protocol client that
// simplifies driving browsers for scraping, unit testing, or profiling web
// pages using the CDP.
//
// chromedp requires no third-party dependencies, implementing the async Chrome
// DevTools Protocol entirely in Go.
package chromedp

import (
	"context"
	"encoding/json"
	"log"
	"sync/atomic"

	"github.com/mailru/easyjson"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
)

// Browser is the high-level Chrome DevTools Protocol browser manager, handling
// the browser process runner, WebSocket clients, associated targets, and
// network, page, and DOM events.
type Browser struct {
	userDataDir string

	conn Transport

	// next is the next message id.
	next int64

	// tabQueue is the queue used to create new target handlers, once a new
	// tab is created and attached to. The newly created Target is sent back
	// via tabResult.
	tabQueue  chan target.SessionID
	tabResult chan *Target

	// cmdQueue is the outgoing command queue.
	cmdQueue chan cmdJob

	// logging funcs
	logf func(string, ...interface{})
	errf func(string, ...interface{})
}

type cmdJob struct {
	msg  *cdproto.Message
	resp chan *cdproto.Message
}

// NewBrowser creates a new browser.
func NewBrowser(ctx context.Context, urlstr string, opts ...BrowserOption) (*Browser, error) {
	conn, err := DialContext(ctx, ForceIP(urlstr))
	if err != nil {
		return nil, err
	}

	b := &Browser{
		conn: conn,

		tabQueue:  make(chan target.SessionID, 1),
		tabResult: make(chan *Target, 1),

		cmdQueue: make(chan cmdJob),

		logf: log.Printf,
	}

	// apply options
	for _, o := range opts {
		if err := o(b); err != nil {
			return nil, err
		}
	}

	// ensure errf is set
	if b.errf == nil {
		b.errf = func(s string, v ...interface{}) { b.logf("ERROR: "+s, v...) }
	}

	go b.run(ctx)
	return b, nil
}

// Shutdown shuts down the browser.
func (b *Browser) Shutdown() error {
	if b.conn != nil {
		if err := b.send(cdproto.CommandBrowserClose, nil); err != nil {
			b.errf("could not close browser: %v", err)
		}
		return b.conn.Close()
	}
	return nil
}

// send writes the supplied message and params.
func (b *Browser) send(method cdproto.MethodType, params easyjson.RawMessage) error {
	msg := &cdproto.Message{
		ID:     atomic.AddInt64(&b.next, 1),
		Method: method,
		Params: params,
	}
	return b.conn.Write(msg)
}

func (b *Browser) newExecutorForTarget(ctx context.Context, sessionID target.SessionID) *Target {
	if sessionID == "" {
		panic("empty session ID")
	}
	b.tabQueue <- sessionID
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

func (b *Browser) run(ctx context.Context) {
	defer b.conn.Close()

	// add cancel to context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// tabEventQueue is the queue of incoming target events, to be routed by
	// their session ID.
	tabEventQueue := make(chan tabEvent, 1)

	// resQueue is the incoming command result queue.
	resQueue := make(chan *cdproto.Message, 1)

	// This goroutine continuously reads events from the websocket
	// connection. The separate goroutine is needed since a websocket read
	// is blocking, so it cannot be used in a select statement.
	go func() {
		defer cancel()
		for {
			msg, err := b.conn.Read()
			if err != nil {
				return
			}
			if msg.Method == cdproto.EventRuntimeExceptionThrown {
				ev := new(runtime.EventExceptionThrown)
				if err := json.Unmarshal(msg.Params, ev); err != nil {
					b.errf("%s", err)
					continue
				}
				b.errf("%+v\n", ev.ExceptionDetails)
				continue
			}

			var sessionID target.SessionID
			if msg.Method == cdproto.EventTargetReceivedMessageFromTarget {
				event := new(target.EventReceivedMessageFromTarget)
				if err := json.Unmarshal(msg.Params, event); err != nil {
					b.errf("%s", err)
					continue
				}
				sessionID = event.SessionID
				msg = new(cdproto.Message)
				if err := json.Unmarshal([]byte(event.Message), msg); err != nil {
					b.errf("%s", err)
					continue
				}
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
		defer cancel()

		// This map is only safe for use within this goroutine, so don't
		// declare it as a Browser field.
		pages := make(map[target.SessionID]*Target, 1024)
		for {
			select {
			case sessionID := <-b.tabQueue:
				if _, ok := pages[sessionID]; ok {
					b.errf("executor for %q already exists", sessionID)
				}
				t := &Target{
					browser:   b,
					SessionID: sessionID,

					eventQueue: make(chan *cdproto.Message, 1024),
					waitQueue:  make(chan func(cur *cdp.Frame) bool, 1024),
					frames:     make(map[cdp.FrameID]*cdp.Frame),

					logf: b.logf,
					errf: b.errf,
				}
				go t.run(ctx)
				pages[sessionID] = t
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
type BrowserOption func(*Browser) error

// WithLogf is a browser option to specify a func to receive general logging.
func WithLogf(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) error {
		b.logf = f
		return nil
	}
}

// WithErrorf is a browser option to specify a func to receive error logging.
func WithErrorf(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) error {
		b.errf = f
		return nil
	}
}

// WithConsolef is a browser option to specify a func to receive chrome log events.
//
// Note: NOT YET IMPLEMENTED.
func WithConsolef(f func(string, ...interface{})) BrowserOption {
	return func(b *Browser) error {
		return nil
	}
}
