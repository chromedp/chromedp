package chromedp

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mailru/easyjson"

	"github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/cdputil"
	"github.com/knq/chromedp/cdp/css"
	"github.com/knq/chromedp/cdp/dom"
	"github.com/knq/chromedp/cdp/inspector"
	logdom "github.com/knq/chromedp/cdp/log"
	"github.com/knq/chromedp/cdp/page"
	rundom "github.com/knq/chromedp/cdp/runtime"
	"github.com/knq/chromedp/client"
	"github.com/knq/chromedp/cdp/network"
)

// TargetHandler manages a Chrome Debugging Protocol target.
type TargetHandler struct {
	conn client.Transport

	// frames is the set of encountered frames.
	frames map[cdp.FrameID]*cdp.Frame

	// lsnr is the map of listeners, which maps from cdp.MethodType to channels.
	lsnr map[cdp.MethodType][]chan interface{}

	// lsnrchs is the map of channels, which maps from channel to registered cdp.MethodType(s).
	lsnrchs map[<-chan interface{}]map[cdp.MethodType]bool

	// cur is the current top level frame.
	cur *cdp.Frame

	// qcmd is the outgoing message queue.
	qcmd chan *cdp.Message

	// qres is the incoming command result queue.
	qres chan *cdp.Message

	// qevents is the incoming event queue.
	qevents chan *cdp.Message

	// detached is closed when the detached event is received.
	detached chan *inspector.EventDetached

	// loaded is fired when the page load event is received.
	loaded chan struct{}

	pageWaitGroup, domWaitGroup *sync.WaitGroup

	// last is the last sent message identifier.
	last  int64
	lastm sync.Mutex

	// res is the id->result channel map.
	res   map[int64]chan *cdp.Message
	resrw sync.RWMutex

	// logging funcs
	logf, debugf, errorf LogFunc

	sync.RWMutex
}

// NewTargetHandler creates a new handler for the specified client target.
func NewTargetHandler(t client.Target, logf, debugf, errorf LogFunc) (*TargetHandler, error) {
	conn, err := client.Dial(t)
	if err != nil {
		return nil, err
	}

	return &TargetHandler{
		conn:   conn,
		logf:   logf,
		debugf: debugf,
		errorf: errorf,
	}, nil
}

// Run starts the processing of commands and events of the client target
// provided to NewTargetHandler.
//
// Callers can stop Run by closing the passed context.
func (h *TargetHandler) Run(ctxt context.Context) error {
	var err error

	// reset
	h.Lock()
	h.frames = make(map[cdp.FrameID]*cdp.Frame)
	h.lsnr = make(map[cdp.MethodType][]chan interface{})
	h.lsnrchs = make(map[<-chan interface{}]map[cdp.MethodType]bool)
	h.qcmd = make(chan *cdp.Message)
	h.qres = make(chan *cdp.Message)
	// The events channel needs to be big enough to buffer as many events as
	// we might recieve at once while processing one event. This is
	// necessary because the event handling may do requests which require
	// reading from the qmsg channel. This can't happen if the network
	// handler is blocked writing to qevents. See discussion in #75. In
	// practice I haven't seen the buffer get bigger than one or two. But we
	// make it large just to be safe, and panic if it is ever full, rather
	// than deadlocking.
	h.qevents = make(chan *cdp.Message, 1024)
	h.res = make(map[int64]chan *cdp.Message)
	h.detached = make(chan *inspector.EventDetached)
	h.pageWaitGroup = new(sync.WaitGroup)
	h.domWaitGroup = new(sync.WaitGroup)
	h.Unlock()

	// run
	go h.run(ctxt)

	// enable domains
	for _, a := range []Action{
		logdom.Enable(),
		rundom.Enable(),
		network.Enable(),
		inspector.Enable(),
		page.Enable(),
		dom.Enable(),
		css.Enable(),
	} {
		err = a.Do(ctxt, h)
		if err != nil {
			return fmt.Errorf("unable to execute %s: %v", reflect.TypeOf(a), err)
		}
	}

	h.Lock()

	// get page resources
	tree, err := page.GetResourceTree().Do(ctxt, h)
	if err != nil {
		return fmt.Errorf("unable to get resource tree: %v", err)
	}

	h.frames[tree.Frame.ID] = tree.Frame
	h.cur = tree.Frame

	for _, c := range tree.ChildFrames {
		h.frames[c.Frame.ID] = c.Frame
	}

	h.Unlock()

	h.documentUpdated(ctxt)

	return nil
}

// run handles the actual message processing to / from the web socket connection.
func (h *TargetHandler) run(ctxt context.Context) {
	defer h.conn.Close()

	// add cancel to context
	ctxt, cancel := context.WithCancel(ctxt)
	defer cancel()

	go func() {
		defer cancel()

		for {
			select {
			default:
				msg, err := h.read()
				if err != nil {
					return
				}

				switch {
				case msg.Method != "":
					select {
					case h.qevents <- msg:
					default:
						// See discussion in #75.
						panic("h.qevents is blocked!")
					}

				case msg.ID != 0:
					h.qres <- msg

				default:
					h.errorf("ignoring malformed incoming message (missing id or method): %#v", msg)
				}

			case <-h.detached:
				// FIXME: should log when detached, and reason
				return

			case <-ctxt.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	// process queues
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case ev := <-h.qevents:
				err := h.processEvent(ctxt, ev)
				if err != nil {
					h.errorf("could not process event %s: %v", ev.Method, err)
				}
			case <-ctxt.Done():
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case res := <-h.qres:
				err := h.processResult(res)
				if err != nil {
					h.errorf("could not process result for message %d: %v", res.ID, err)
				}
			case <-ctxt.Done():
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case cmd := <-h.qcmd:
				err := h.processCommand(cmd)
				if err != nil {
					h.errorf("could not process command message %d: %v", cmd.ID, err)
				}

			case <-ctxt.Done():
				return
			}
		}
	}()
}

// read reads a message from the client connection.
func (h *TargetHandler) read() (*cdp.Message, error) {
	// read
	buf, err := h.conn.Read()
	if err != nil {
		return nil, err
	}

	h.debugf("-> %s", string(buf))

	// unmarshal
	msg := new(cdp.Message)
	err = easyjson.Unmarshal(buf, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// WaitEventLoad returns a channel which is closed when the page load event is
// fired, or the next TargetHandler.Run takes place.
func (h *TargetHandler) WaitEventLoad() <-chan struct{} {
	h.Lock()
	defer h.Unlock()
	return h.loaded
}

// ResetEventLoad resets the event load trigger returned by WaitEventLoad.
// It is called when a navigation is requested, so that the next load can take
// place.
func (h *TargetHandler) ResetEventLoad() {
	h.Lock()
	defer h.Unlock()

	if h.loaded != nil {
		select {
		case <-h.loaded: // already closed
		default:
			// Before replacing h.loaded with a new pipeline,
			// unblock previous waiters if they are still blocked.
			// This prevents them staying deadlocked for a load
			// event which will never come, however it means that
			// they get a phantom load event.
			close(h.loaded)
			h.loaded = nil
		}
	}

	// Make a new channel for signalling the next page load event.
	h.loaded = make(chan struct{})
}

// processEvent processes an incoming event.
func (h *TargetHandler) processEvent(ctxt context.Context, msg *cdp.Message) error {
	if msg == nil {
		return cdp.ErrChannelClosed
	}

	// unmarshal
	ev, err := cdputil.UnmarshalMessage(msg)
	if err != nil {
		return err
	}

	propagate(h, msg.Method, ev)

	switch e := ev.(type) {
	case *inspector.EventDetached:
		h.Lock()
		h.detached <- e
		h.Unlock()
		return nil

	case *dom.EventDocumentUpdated:
		h.domWaitGroup.Wait()
		h.documentUpdated(ctxt)
		return nil
	}

	d := msg.Method.Domain()

	switch d {
	case "Page":
		h.pageWaitGroup.Add(1)
		h.pageEvent(ctxt, ev)

	case "DOM":
		h.domWaitGroup.Add(1)
		h.domEvent(ctxt, ev)
	}

	return nil
}

// propagate propogates event to the listeners
func propagate(h *TargetHandler, method cdp.MethodType, ev interface{}) {
	h.RLock() // prevent "send on closed channel"
	defer h.RUnlock()
	if lsnrs, ok := h.lsnr[method]; ok {
		for _, l := range lsnrs {
			l <- ev
		}
	}
}

// documentUpdated handles the document updated event, retrieving the document
// root for the root frame.
func (h *TargetHandler) documentUpdated(ctxt context.Context) {
	f, err := h.WaitFrame(ctxt, cdp.EmptyFrameID)
	if err != nil {
		h.errorf("could not get current frame: %v", err)
		return
	}

	f.Lock()
	defer f.Unlock()

	// invalidate nodes
	if f.Root != nil {
		close(f.Root.Invalidated)
	}

	f.Nodes = make(map[cdp.NodeID]*cdp.Node)
	f.Root, err = dom.GetDocument().WithPierce(true).Do(ctxt, h)
	if err != nil {
		h.errorf("could not retrieve document root for %s: %v", f.ID, err)
		return
	}
	f.Root.Invalidated = make(chan struct{})
	walk(f.Nodes, f.Root)
}

// processResult processes an incoming command result.
func (h *TargetHandler) processResult(msg *cdp.Message) error {
	h.resrw.RLock()
	defer h.resrw.RUnlock()

	ch, ok := h.res[msg.ID]
	if !ok {
		return fmt.Errorf("id %d not present in res map", msg.ID)
	}
	defer close(ch)

	ch <- msg

	return nil
}

// processCommand writes a command to the client connection.
func (h *TargetHandler) processCommand(cmd *cdp.Message) error {
	// marshal
	buf, err := easyjson.Marshal(cmd)
	if err != nil {
		return err
	}

	h.debugf("<- %s", string(buf))

	return h.conn.Write(buf)
}

// emptyObj is an empty JSON object message.
var emptyObj = easyjson.RawMessage([]byte(`{}`))

// Execute executes commandType against the endpoint passed to Run, using the
// provided context and params, decoding the result of the command to res.
func (h *TargetHandler) Execute(ctxt context.Context, commandType cdp.MethodType, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	var paramsBuf easyjson.RawMessage
	if params == nil {
		paramsBuf = emptyObj
	} else {
		var err error
		paramsBuf, err = easyjson.Marshal(params)
		if err != nil {
			return err
		}
	}

	id := h.next()

	// save channel
	ch := make(chan *cdp.Message, 1)
	h.resrw.Lock()
	h.res[id] = ch
	h.resrw.Unlock()

	// queue message
	h.qcmd <- &cdp.Message{
		ID:     id,
		Method: commandType,
		Params: paramsBuf,
	}

	errch := make(chan error, 1)
	go func() {
		defer close(errch)

		select {
		case msg := <-ch:
			switch {
			case msg == nil:
				errch <- cdp.ErrChannelClosed

			case msg.Error != nil:
				errch <- msg.Error

			case res != nil:
				errch <- easyjson.Unmarshal(msg.Result, res)
			}

		case <-ctxt.Done():
			errch <- ctxt.Err()
		}

		h.resrw.Lock()
		defer h.resrw.Unlock()

		delete(h.res, id)
	}()

	return <-errch
}

// next returns the next message id.
func (h *TargetHandler) next() int64 {
	h.lastm.Lock()
	defer h.lastm.Unlock()
	h.last++
	return h.last
}

// GetRoot returns the current top level frame's root document node.
func (h *TargetHandler) GetRoot(ctxt context.Context) (*cdp.Node, error) {
	var root *cdp.Node

loop:
	for {
		var cur *cdp.Frame
		select {
		default:
			h.RLock()
			cur = h.cur
			if cur != nil {
				cur.RLock()
				root = cur.Root
				cur.RUnlock()
			}
			h.RUnlock()

			if cur != nil && root != nil {
				break loop
			}

			time.Sleep(DefaultCheckDuration)

		case <-ctxt.Done():
			return nil, ctxt.Err()
		}
	}

	return root, nil
}

// SetActive sets the currently active frame after a successful navigation.
func (h *TargetHandler) SetActive(ctxt context.Context, id cdp.FrameID) error {
	var err error

	// get frame
	f, err := h.WaitFrame(ctxt, id)
	if err != nil {
		return err
	}

	h.Lock()
	defer h.Unlock()

	h.cur = f

	return nil
}

// WaitFrame waits for a frame to be loaded using the provided context.
func (h *TargetHandler) WaitFrame(ctxt context.Context, id cdp.FrameID) (*cdp.Frame, error) {
	// TODO: fix this
	timeout := time.After(10 * time.Second)

loop:
	for {
		select {
		default:
			var f *cdp.Frame
			var ok bool

			h.RLock()
			if id == cdp.EmptyFrameID {
				f, ok = h.cur, h.cur != nil
			} else {
				f, ok = h.frames[id]
			}
			h.RUnlock()

			if ok {
				return f, nil
			}

			time.Sleep(DefaultCheckDuration)

		case <-ctxt.Done():
			return nil, ctxt.Err()

		case <-timeout:
			break loop
		}
	}

	return nil, fmt.Errorf("timeout waiting for frame `%s`", id)
}

// WaitNode waits for a node to be loaded using the provided context.
func (h *TargetHandler) WaitNode(ctxt context.Context, f *cdp.Frame, id cdp.NodeID) (*cdp.Node, error) {
	// TODO: fix this
	timeout := time.After(10 * time.Second)

loop:
	for {
		select {
		default:
			var n *cdp.Node
			var ok bool

			f.RLock()
			n, ok = f.Nodes[id]
			f.RUnlock()

			if n != nil && ok {
				return n, nil
			}

			time.Sleep(DefaultCheckDuration)

		case <-ctxt.Done():
			return nil, ctxt.Err()

		case <-timeout:
			break loop
		}
	}

	return nil, fmt.Errorf("timeout waiting for node `%d`", id)
}

// pageEvent handles incoming page events.
func (h *TargetHandler) pageEvent(ctxt context.Context, ev interface{}) {
	defer h.pageWaitGroup.Done()

	var id cdp.FrameID
	var op frameOp

	switch e := ev.(type) {
	case *page.EventFrameNavigated:
		h.Lock()
		h.frames[e.Frame.ID] = e.Frame
		if h.cur != nil && h.cur.ID == e.Frame.ID {
			h.cur = e.Frame
		}
		h.Unlock()
		return

	case *page.EventFrameAttached:
		// NOTE(pwaller):
		//   This happens before we have the frame object for
		//   e.FrameID - that only occurs in EventFrameNavigated.
		//   so there isn't a frame to update yet.
		//   I'm not sure of the use of this state - see #75.

		//   Another issue is that events like EventFrameStoppedLoading
		//   can happen even though EventFrameNavigated *never fires*,
		//   so these events can end up calling WaitFrame on a frame
		//   that never comes into existence.

		// id, op = e.FrameID, frameAttached(e.ParentFrameID)
		return

	case *page.EventFrameDetached:
		return // See note above.
	case *page.EventFrameStartedLoading:
		return // See note above.
	case *page.EventFrameStoppedLoading:
		return // See note above.
	case *page.EventFrameScheduledNavigation:
		return // See note above.
	case *page.EventFrameClearedScheduledNavigation:
		return // See note above.

	case *page.EventDomContentEventFired:
		return
	case *page.EventLoadEventFired:
		if h.loaded != nil {
			// If anyone is listening for loaded, notify them now.
			close(h.loaded)
			h.loaded = nil
		}
		return
	case *page.EventFrameResized:
		return
	case *page.EventLifecycleEvent:
		return
	default:
		h.errorf("unhandled page event %s", reflect.TypeOf(ev))
		return
	}

	f, err := h.WaitFrame(ctxt, id)
	if err != nil {
		h.errorf("could not get frame %s: %v", id, err)
		return
	}

	h.Lock()
	defer h.Unlock()

	f.Lock()
	defer f.Unlock()

	op(f)
}

// domEvent handles incoming DOM events.
func (h *TargetHandler) domEvent(ctxt context.Context, ev interface{}) {
	defer h.domWaitGroup.Done()

	// wait current frame
	f, err := h.WaitFrame(ctxt, cdp.EmptyFrameID)
	if err != nil {
		h.errorf("could not process DOM event %s: %v", reflect.TypeOf(ev), err)
		return
	}

	var id cdp.NodeID
	var op nodeOp

	switch e := ev.(type) {
	case *dom.EventSetChildNodes:
		id, op = e.ParentID, setChildNodes(f.Nodes, e.Nodes)

	case *dom.EventAttributeModified:
		id, op = e.NodeID, attributeModified(e.Name, e.Value)

	case *dom.EventAttributeRemoved:
		id, op = e.NodeID, attributeRemoved(e.Name)

	case *dom.EventInlineStyleInvalidated:
		if len(e.NodeIds) == 0 {
			return
		}

		id, op = e.NodeIds[0], inlineStyleInvalidated(e.NodeIds[1:])

	case *dom.EventCharacterDataModified:
		id, op = e.NodeID, characterDataModified(e.CharacterData)

	case *dom.EventChildNodeCountUpdated:
		id, op = e.NodeID, childNodeCountUpdated(e.ChildNodeCount)

	case *dom.EventChildNodeInserted:
		if e.PreviousNodeID != cdp.EmptyNodeID {
			_, err = h.WaitNode(ctxt, f, e.PreviousNodeID)
			if err != nil {
				return
			}
		}
		id, op = e.ParentNodeID, childNodeInserted(f.Nodes, e.PreviousNodeID, e.Node)

	case *dom.EventChildNodeRemoved:
		id, op = e.ParentNodeID, childNodeRemoved(f.Nodes, e.NodeID)

	case *dom.EventShadowRootPushed:
		id, op = e.HostID, shadowRootPushed(f.Nodes, e.Root)

	case *dom.EventShadowRootPopped:
		id, op = e.HostID, shadowRootPopped(f.Nodes, e.RootID)

	case *dom.EventPseudoElementAdded:
		id, op = e.ParentID, pseudoElementAdded(f.Nodes, e.PseudoElement)

	case *dom.EventPseudoElementRemoved:
		id, op = e.ParentID, pseudoElementRemoved(f.Nodes, e.PseudoElementID)

	case *dom.EventDistributedNodesUpdated:
		id, op = e.InsertionPointID, distributedNodesUpdated(e.DistributedNodes)

	default:
		h.errorf("unhandled node event %s", reflect.TypeOf(ev))
		return
	}

	// retrieve node
	n, err := h.WaitNode(ctxt, f, id)
	if err != nil {
		s := strings.TrimSuffix(runtime.FuncForPC(reflect.ValueOf(op).Pointer()).Name(), ".func1")
		i := strings.LastIndex(s, ".")
		if i != -1 {
			s = s[i+1:]
		}
		h.errorf("could not perform (%s) operation on node %d (wait node): %v", s, id, err)
		return
	}

	h.Lock()
	defer h.Unlock()

	f.Lock()
	defer f.Unlock()

	op(n)
}

// Listen creates a listener for the specified event types.
func (h *TargetHandler) Listen(eventTypes ...cdp.MethodType) <-chan interface{} {
	h.Lock()
	defer h.Unlock()

	ch := make(chan interface{}, 16)
	for _, evtTyp := range eventTypes {
		if chlist, ok := h.lsnr[evtTyp]; ok {
			chlist = append(chlist, ch)
			if _, etok := h.lsnrchs[ch][evtTyp]; !etok {
				h.lsnrchs[ch][evtTyp] = true
			}
		} else {
			h.lsnr[evtTyp] = []chan interface{}{ch}
			h.lsnrchs[ch] = map[cdp.MethodType]bool{evtTyp: true}
		}
	}
	return ch
}

// Release releases a channel returned from Listen.
func (h *TargetHandler) Release(ch <-chan interface{}) {
	h.Lock()
	defer h.Unlock()

	lsnrchs := h.lsnrchs[ch]
	closed := false
	for evtTyp := range lsnrchs {
		chs := h.lsnr[evtTyp]
		for i := 0; i < len(chs); i++ {
			if ch == chs[i] {
				if !closed {
					close(chs[i])
					chs[i] = nil
					closed = true
				}
				if i == len(chs)-1 {
					chs = chs[:len(chs)-1]
				} else {
					chs = append(chs[:i], chs[i+1:]...)
				}
				h.lsnr[evtTyp] = chs
				break
			}
		}
	}
	delete(h.lsnrchs, ch)
}
