package chromedp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/mailru/easyjson"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
)

// Target manages a Chrome DevTools Protocol target.
type Target struct {
	browser   *Browser
	SessionID target.SessionID
	TargetID  target.ID

	listenersMu sync.Mutex
	listeners   []cancelableListener

	messageQueue chan *cdproto.Message

	// frameMu protects frames, execContexts, and cur.
	frameMu sync.RWMutex
	// frames is the set of encountered frames.
	frames       map[cdp.FrameID]*cdp.Frame
	execContexts map[cdp.FrameID]runtime.ExecutionContextID
	// cur is the current top level frame.
	cur cdp.FrameID

	// logging funcs
	logf, errf func(string, ...interface{})

	// Indicates if the target is a worker target.
	isWorker bool
}

func (t *Target) enclosingFrame(node *cdp.Node) cdp.FrameID {
	t.frameMu.RLock()
	top := t.frames[t.cur]
	t.frameMu.RUnlock()
	top.RLock()
	defer top.RUnlock()
	for {
		if node == nil {
			// Avoid crashing. This can happen if we're using an old
			// node that has been replaced, for example.
			return ""
		}
		if node.FrameID != "" {
			break
		}
		node = top.Nodes[node.ParentID]
	}
	return node.FrameID
}

// ensureFrame ensures the top frame of this target is loaded and returns the top frame,
// the root node and the ExecutionContextID of this top frame; otherwise, it will return
// false as its last return value.
func (t *Target) ensureFrame() (*cdp.Frame, *cdp.Node, runtime.ExecutionContextID, bool) {
	t.frameMu.RLock()
	frame := t.frames[t.cur]
	execCtx := t.execContexts[t.cur]
	t.frameMu.RUnlock()

	// the frame hasn't loaded yet.
	if frame == nil || execCtx == 0 {
		return nil, nil, 0, false
	}

	frame.RLock()
	root := frame.Root
	frame.RUnlock()

	if root == nil {
		// not root node yet?
		return nil, nil, 0, false
	}
	return frame, root, execCtx, true
}

func (t *Target) run(ctx context.Context) {
	type eventValue struct {
		method cdproto.MethodType
		value  interface{}
	}
	// syncEventQueue is used to handle events synchronously within Target.
	// TODO: If this queue gets full, the goroutine below could get stuck on
	// a send, and response callbacks would never run, resulting in a
	// deadlock. Can we fix this without potentially using lots of memory?
	syncEventQueue := make(chan eventValue, 4096)

	// This goroutine receives events from the browser, calls listeners, and
	// then passes the events onto the main goroutine for the target handler
	// to update itself.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case msg := <-t.messageQueue:
				if msg.ID != 0 {
					t.listenersMu.Lock()
					t.listeners = runListeners(t.listeners, msg)
					t.listenersMu.Unlock()
					continue
				}
				ev, err := cdproto.UnmarshalMessage(msg)
				if err != nil {
					if _, ok := err.(cdp.ErrUnknownCommandOrEvent); ok {
						// This is most likely an event received from an older
						// Chrome which a newer cdproto doesn't have, as it is
						// deprecated. Ignore that error.
						continue
					}
					t.errf("could not unmarshal event: %v", err)
					continue
				}
				t.listenersMu.Lock()
				t.listeners = runListeners(t.listeners, ev)
				t.listenersMu.Unlock()

				switch msg.Method.Domain() {
				case "Runtime", "Page", "DOM":
					select {
					case <-ctx.Done():
						return
					case syncEventQueue <- eventValue{msg.Method, ev}:
					}
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-syncEventQueue:
			switch ev.method.Domain() {
			case "Runtime":
				t.runtimeEvent(ev.value)
			case "Page":
				t.pageEvent(ev.value)
			case "DOM":
				t.domEvent(ctx, ev.value)
			}
		}
	}
}

func (t *Target) Execute(ctx context.Context, method string, params easyjson.Marshaler, res easyjson.Unmarshaler) error {
	if method == target.CommandCloseTarget {
		return errors.New("to close the target, cancel its context or use chromedp.Cancel")
	}

	id := atomic.AddInt64(&t.browser.next, 1)
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
	t.listenersMu.Lock()
	t.listeners = append(t.listeners, cancelableListener{lctx, fn})
	t.listenersMu.Unlock()

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
		ID:        id,
		SessionID: t.SessionID,
		Method:    cdproto.MethodType(method),
		Params:    buf,
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case t.browser.cmdQueue <- cmd:
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

// runtimeEvent handles incoming runtime events.
func (t *Target) runtimeEvent(ev interface{}) {
	switch ev := ev.(type) {
	case *runtime.EventExecutionContextCreated:
		var aux struct {
			FrameID cdp.FrameID
		}
		if len(ev.Context.AuxData) == 0 {
			break
		}
		if err := json.Unmarshal(ev.Context.AuxData, &aux); err != nil {
			t.errf("could not decode executionContextCreated auxData %q: %v", ev.Context.AuxData, err)
			break
		}
		if aux.FrameID != "" {
			t.frameMu.Lock()
			t.execContexts[aux.FrameID] = ev.Context.ID
			t.frameMu.Unlock()
		}
	case *runtime.EventExecutionContextDestroyed:
		t.frameMu.Lock()
		for frameID, ctxID := range t.execContexts {
			if ctxID == ev.ExecutionContextID {
				delete(t.execContexts, frameID)
			}
		}
		t.frameMu.Unlock()
	case *runtime.EventExecutionContextsCleared:
		t.frameMu.Lock()
		for frameID := range t.execContexts {
			delete(t.execContexts, frameID)
		}
		t.frameMu.Unlock()
	}
}

// documentUpdated handles the document updated event, retrieving the document
// root for the root frame.
func (t *Target) documentUpdated(ctx context.Context) {
	t.frameMu.RLock()
	f := t.frames[t.cur]
	t.frameMu.RUnlock()
	if f == nil {
		// TODO: This seems to happen on CI, when running the tests
		// under the headless-shell Docker image. Figure out why.
		t.errf("received DOM.documentUpdated when there's no top-level frame")
		return
	}
	f.Lock()
	defer f.Unlock()

	// invalidate nodes
	if f.Root != nil {
		close(f.Root.Invalidated)
	}

	f.Nodes = make(map[cdp.NodeID]*cdp.Node)
	var err error
	f.Root, err = dom.GetDocument().Do(cdp.WithExecutor(ctx, t))
	if err == context.Canceled {
		return // TODO: perhaps not necessary, but useful to keep the tests less noisy
	}
	if err != nil {
		t.errf("could not retrieve document root for %s: %v", f.ID, err)
		return
	}
	f.Root.Invalidated = make(chan struct{})
	walk(f.Nodes, f.Root)
}

// pageEvent handles incoming page events.
func (t *Target) pageEvent(ev interface{}) {
	var id cdp.FrameID
	var op frameOp

	switch e := ev.(type) {
	case *page.EventFrameNavigated:
		t.frameMu.Lock()
		t.frames[e.Frame.ID] = e.Frame
		if e.Frame.ParentID == "" {
			// This frame is only the new top-level frame if it has
			// no parent.
			t.cur = e.Frame.ID
		}
		t.frameMu.Unlock()
		return

	case *page.EventFrameAttached:
		id, op = e.FrameID, frameAttached(e.ParentFrameID)

	case *page.EventFrameDetached:
		id, op = e.FrameID, frameDetached

	case *page.EventFrameStartedLoading:
		id, op = e.FrameID, frameStartedLoading

	case *page.EventFrameStoppedLoading:
		id, op = e.FrameID, frameStoppedLoading

		// ignored events
	case *page.EventCompilationCacheProduced,
		*page.EventDocumentOpened,
		*page.EventDomContentEventFired,
		*page.EventFileChooserOpened,
		*page.EventFrameRequestedNavigation,
		*page.EventFrameResized,
		*page.EventInterstitialHidden,
		*page.EventInterstitialShown,
		*page.EventJavascriptDialogClosed,
		*page.EventJavascriptDialogOpening,
		*page.EventLifecycleEvent,
		*page.EventLoadEventFired,
		*page.EventNavigatedWithinDocument,
		*page.EventScreencastFrame,
		*page.EventScreencastVisibilityChanged,
		*page.EventWindowOpen,
		*page.EventBackForwardCacheNotUsed:
		return

	default:
		t.errf("unhandled page event %T", ev)
		return
	}

	t.frameMu.Lock()
	f := t.frames[id]
	if f == nil {
		// This can happen if a frame is attached or starts loading
		// before it's ever navigated to. We won't have all the frame
		// details just yet, but that's okay.
		f = &cdp.Frame{ID: id}
		t.frames[id] = f
	}
	t.frameMu.Unlock()

	f.Lock()
	op(f)
	f.Unlock()
}

// domEvent handles incoming DOM events.
func (t *Target) domEvent(ctx context.Context, ev interface{}) {
	t.frameMu.RLock()
	f := t.frames[t.cur]
	t.frameMu.RUnlock()

	var id cdp.NodeID
	var op nodeOp

	switch e := ev.(type) {
	case *dom.EventDocumentUpdated:
		t.documentUpdated(ctx)
		return

	case *dom.EventSetChildNodes:
		id, op = e.ParentID, setChildNodes(f.Nodes, e.Nodes)

	case *dom.EventAttributeModified:
		id, op = e.NodeID, attributeModified(e.Name, e.Value)

	case *dom.EventAttributeRemoved:
		id, op = e.NodeID, attributeRemoved(e.Name)

	case *dom.EventInlineStyleInvalidated:
		if len(e.NodeIDs) == 0 {
			return
		}

		id, op = e.NodeIDs[0], inlineStyleInvalidated(e.NodeIDs[1:])

	case *dom.EventCharacterDataModified:
		id, op = e.NodeID, characterDataModified(e.CharacterData)

	case *dom.EventChildNodeCountUpdated:
		id, op = e.NodeID, childNodeCountUpdated(e.ChildNodeCount)

	case *dom.EventChildNodeInserted:
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
		t.errf("unhandled node event %T", ev)
		return
	}

	n, ok := f.Nodes[id]
	if !ok {
		// Node ID has been invalidated. Nothing to do.
		return
	}

	f.Lock()
	op(n)
	f.Unlock()
}
