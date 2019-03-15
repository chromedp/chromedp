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

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/mailru/easyjson"
)

// Browser is the high-level Chrome DevTools Protocol browser manager, handling
// the browser process runner, WebSocket clients, associated targets, and
// network, page, and DOM events.
type Browser struct {
	UserDataDir string

	pages map[target.SessionID]*Target

	conn Transport

	// next is the next message id.
	next int64

	cmdQueue chan cmdJob

	// qres is the incoming command result queue.
	qres chan *cdproto.Message

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
		conn:  conn,
		pages: make(map[target.SessionID]*Target, 1024),
		logf:  log.Printf,
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

func (b *Browser) executorForTarget(ctx context.Context, sessionID target.SessionID) *Target {
	if sessionID == "" {
		panic("empty session ID")
	}
	if t, ok := b.pages[sessionID]; ok {
		return t
	}
	t := &Target{
		browser:   b,
		sessionID: sessionID,

		eventQueue: make(chan *cdproto.Message, 1024),
		waitQueue:  make(chan func(cur *cdp.Frame) bool, 1024),
		frames:     make(map[cdp.FrameID]*cdp.Frame),

		logf: b.logf,
		errf: b.errf,
	}
	go t.run(ctx)
	b.pages[sessionID] = t
	return t
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

func (b *Browser) Start(ctx context.Context) {
	b.cmdQueue = make(chan cmdJob)
	b.qres = make(chan *cdproto.Message)

	go b.run(ctx)
}

func (b *Browser) run(ctx context.Context) {
	defer b.conn.Close()

	// add cancel to context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// continue below
			}
			msg, err := b.conn.Read()
			if err != nil {
				return
			}
			var sessionID target.SessionID
			if msg.Method == cdproto.EventTargetReceivedMessageFromTarget {
				recv := new(target.EventReceivedMessageFromTarget)
				if err := json.Unmarshal(msg.Params, recv); err != nil {
					b.errf("%s", err)
					continue
				}
				sessionID = recv.SessionID
				msg = new(cdproto.Message)
				if err := json.Unmarshal([]byte(recv.Message), msg); err != nil {
					b.errf("%s", err)
					continue
				}
			}

			switch {
			case msg.Method != "":
				if sessionID == "" {
					// TODO: are we interested in
					// these events?
					continue
				}

				page, ok := b.pages[sessionID]
				if !ok {
					b.errf("unknown session ID %q", sessionID)
					continue
				}
				select {
				case page.eventQueue <- msg:
				default:
					panic("eventQueue is full")
				}

			case msg.ID != 0:
				b.qres <- msg

			default:
				b.errf("ignoring malformed incoming message (missing id or method): %#v", msg)
			}
		}
	}()

	respByID := make(map[int64]chan *cdproto.Message)

	// process queues
	for {
		select {
		case res := <-b.qres:
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
