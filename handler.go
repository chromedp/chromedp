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
	"github.com/knq/chromedp/cdp/css"
	"github.com/knq/chromedp/cdp/dom"
	"github.com/knq/chromedp/cdp/inspector"
	logdom "github.com/knq/chromedp/cdp/log"
	"github.com/knq/chromedp/cdp/page"
	rundom "github.com/knq/chromedp/cdp/runtime"
	"github.com/knq/chromedp/client"
)

// TargetHandler manages a Chrome Debugging Protocol target.
type TargetHandler struct {
	conn client.Transport

	// frames is the set of encountered frames.
	frames map[cdp.FrameID]*cdp.Frame

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

	pageWaitGroup, domWaitGroup *sync.WaitGroup

	// last is the last sent message identifier.
	last  int64
	lastm sync.Mutex

	// res is the id->result channel map.
	res   map[int64]chan interface{}
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
	h.qcmd = make(chan *cdp.Message)
	h.qres = make(chan *cdp.Message)
	h.qevents = make(chan *cdp.Message)
	h.res = make(map[int64]chan interface{})
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
		//network.Enable(),
		inspector.Enable(),
		page.Enable(),
		dom.Enable(),
		css.Enable(),
	} {
		err = a.Do(ctxt, h)
		if err != nil {
			return fmt.Errorf("unable to execute %s, got: %v", reflect.TypeOf(a), err)
		}
	}

	h.Lock()

	// get page resources
	tree, err := page.GetResourceTree().Do(ctxt, h)
	if err != nil {
		return fmt.Errorf("unable to get resource tree, got: %v", err)
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
					h.qevents <- msg

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

	var err error

	// process queues
	for {
		select {
		case ev := <-h.qevents:
			err = h.processEvent(ctxt, ev)
			if err != nil {
				h.errorf("could not process event, got: %v", err)
			}

		case res := <-h.qres:
			err = h.processResult(res)
			if err != nil {
				h.errorf("could not process command result, got: %v", err)
			}

		case cmd := <-h.qcmd:
			err = h.processCommand(cmd)
			if err != nil {
				h.errorf("could not process command, got: %v", err)
			}

		case <-ctxt.Done():
			return
		}
	}
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

// processEvent processes an incoming event.
func (h *TargetHandler) processEvent(ctxt context.Context, msg *cdp.Message) error {
	if msg == nil {
		return cdp.ErrChannelClosed
	}

	// unmarshal
	ev, err := UnmarshalMessage(msg)
	if err != nil {
		return err
	}

	switch e := ev.(type) {
	case *inspector.EventDetached:
		h.Lock()
		defer h.Unlock()
		h.detached <- e
		return nil

	case *dom.EventDocumentUpdated:
		h.domWaitGroup.Wait()
		go h.documentUpdated(ctxt)
		return nil
	}

	d := msg.Method.Domain()
	if d != "Page" && d != "DOM" {
		return nil
	}

	switch d {
	case "Page":
		h.pageWaitGroup.Add(1)
		go h.pageEvent(ctxt, ev)

	case "DOM":
		h.domWaitGroup.Add(1)
		go h.domEvent(ctxt, ev)
	}

	return nil
}

// documentUpdated handles the document updated event, retrieving the document
// root for the root frame.
func (h *TargetHandler) documentUpdated(ctxt context.Context) {
	f, err := h.WaitFrame(ctxt, EmptyFrameID)
	if err != nil {
		h.errorf("could not get current frame, got: %v", err)
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
		h.errorf("could not retrieve document root for %s, got: %v", f.ID, err)
		return
	}
	f.Root.Invalidated = make(chan struct{})
	walk(f.Nodes, f.Root)
}

// processResult processes an incoming command result.
func (h *TargetHandler) processResult(msg *cdp.Message) error {
	h.resrw.Lock()
	defer h.resrw.Unlock()

	res, ok := h.res[msg.ID]
	if !ok {
		err := fmt.Errorf("expected result to be present for message id %d", msg.ID)
		h.errorf(err.Error())
		return err
	}

	if msg.Error != nil {
		res <- msg.Error
	} else {
		res <- msg.Result
	}

	delete(h.res, msg.ID)

	return nil
}

// processCommand writes a command to the client connection.
func (h *TargetHandler) processCommand(cmd *cdp.Message) error {
	// FIXME: there are two possible error conditions here, check and
	// do some kind of logging ...
	buf, err := easyjson.Marshal(cmd)
	if err != nil {
		return err
	}

	h.debugf("<- %s", string(buf))

	// write
	return h.conn.Write(buf)
}

// Execute executes commandType against the endpoint passed to Run, using the
// provided context and the raw JSON encoded params.
//
// Returns a result channel that will receive AT MOST ONE result. A result is
// either the command's result value (as a raw JSON encoded value), or any
// error encountered during operation. After the result (or an error) is passed
// to the returned channel, the channel will be closed.
//
// Note: the returned channel will be closed after the result is read. If the
// passed context finishes prior to receiving the command result, then
// ctxt.Err() will be sent to the channel.
func (h *TargetHandler) Execute(ctxt context.Context, commandType cdp.MethodType, params easyjson.RawMessage) <-chan interface{} {
	ch := make(chan interface{}, 1)

	go func() {
		defer close(ch)

		res := make(chan interface{}, 1)
		defer close(res)

		// get next id
		h.lastm.Lock()
		h.last++
		id := h.last
		h.lastm.Unlock()

		// save channel
		h.resrw.Lock()
		h.res[id] = res
		h.resrw.Unlock()

		h.qcmd <- &cdp.Message{
			ID:     id,
			Method: commandType,
			Params: params,
		}

		select {
		case v := <-res:
			if v != nil {
				ch <- v
			} else {
				ch <- cdp.ErrChannelClosed
			}

		case <-ctxt.Done():
			ch <- ctxt.Err()
		}
	}()

	return ch
}

// GetRoot returns the current top level frame's root document node.
func (h *TargetHandler) GetRoot(ctxt context.Context) (*cdp.Node, error) {
	// TODO: fix this
	ctxt, cancel := context.WithTimeout(ctxt, 10*time.Second)
	defer cancel()

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
			if id == EmptyFrameID {
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
		h.Unlock()
		return

	case *page.EventFrameAttached:
		id, op = e.FrameID, frameAttached(e.ParentFrameID)

	case *page.EventFrameDetached:
		id, op = e.FrameID, frameDetached

	case *page.EventFrameStartedLoading:
		id, op = e.FrameID, frameStartedLoading

	case *page.EventFrameStoppedLoading:
		id, op = e.FrameID, frameStoppedLoading

	case *page.EventFrameScheduledNavigation:
		id, op = e.FrameID, frameScheduledNavigation

	case *page.EventFrameClearedScheduledNavigation:
		id, op = e.FrameID, frameClearedScheduledNavigation

	case *page.EventDomContentEventFired:
		return
	case *page.EventLoadEventFired:
		return
	case *page.EventFrameResized:
		return

	default:
		h.errorf("unhandled page event %s", reflect.TypeOf(ev))
		return
	}

	f, err := h.WaitFrame(ctxt, id)
	if err != nil {
		h.errorf("could not get frame %s, got: %v", id, err)
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
	f, err := h.WaitFrame(ctxt, EmptyFrameID)
	if err != nil {
		h.errorf("error processing DOM event %s: error waiting for frame, got: %v", reflect.TypeOf(ev), err)
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
		id, op = e.NodeIds[0], inlineStyleInvalidated(e.NodeIds[1:])

	case *dom.EventCharacterDataModified:
		id, op = e.NodeID, characterDataModified(e.CharacterData)

	case *dom.EventChildNodeCountUpdated:
		id, op = e.NodeID, childNodeCountUpdated(e.ChildNodeCount)

	case *dom.EventChildNodeInserted:
		if e.PreviousNodeID != EmptyNodeID {
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

	case *dom.EventNodeHighlightRequested:
		id, op = e.NodeID, nodeHighlightRequested

	case *dom.EventInspectNodeRequested:
		return

	default:
		h.errorf("unhandled node event %s", reflect.TypeOf(ev))
		return
	}

	s := strings.TrimPrefix(strings.TrimSuffix(runtime.FuncForPC(reflect.ValueOf(op).Pointer()).Name(), ".func1"), "github.com/knq/chromedp.")

	// retrieve node
	n, err := h.WaitNode(ctxt, f, id)
	if err != nil {
		h.errorf("error could not perform (%s) operation on node %d (wait node error), got: %v", s, id, err)
		return
	}

	h.Lock()
	defer h.Unlock()

	f.Lock()
	defer f.Unlock()

	n.Lock()
	defer n.Unlock()

	op(n)
}

// Listen creates a listener for the specified event types.
func (h *TargetHandler) Listen(eventTypes ...cdp.MethodType) <-chan interface{} {
	return nil
}

// Release releases a channel returned from Listen.
func (h *TargetHandler) Release(ch <-chan interface{}) {

}
