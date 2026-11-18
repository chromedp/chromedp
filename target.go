package chromedp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	jsonv2 "github.com/chromedp/cdproto/cdp/jsonv2"
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

	// detached closes after the target has handled every message that the
	// browser routed to it before the detach.
	detached chan struct{}

	// events holds the subscriptions made with Subscribe.
	events subscribers

	// exposedMu protects exposed, the names of the funcs that ExposeFunc made
	// available to the page, and exposedSeq, the number of the last binding.
	exposedMu  sync.Mutex
	exposed    map[string]struct{}
	exposedSeq int

	// frameMu protects frames, execContexts, cur and worldGen.
	frameMu sync.RWMutex
	// frames is the set of encountered frames.
	frames       map[cdp.FrameID]*Frame
	execContexts map[cdp.FrameID]runtime.ExecutionContextID
	// execUniqueIDs holds the unique id of each execution context in
	// execContexts, because the destroyed event names a context by it.
	execUniqueIDs map[cdp.FrameID]string
	// cur is the current top level frame.
	cur cdp.FrameID
	// worldGen counts the calls of dropWorlds, so that execContext does not
	// keep a world that it created for an old document.
	worldGen uint64

	// noRuntime is true when the target did not get Runtime.enable. Then
	// execContexts holds the isolated worlds that execContext creates.
	// attachTarget sets it before run starts. See WithRuntimeDisabled.
	noRuntime bool

	// logging funcs
	logf, errf func(string, ...any)

	// isWorker is true when the target is a worker target.
	isWorker bool
}

func (t *Target) enclosingFrame(node *Node) cdp.FrameID {
	t.frameMu.RLock()
	top := t.frames[t.cur]
	t.frameMu.RUnlock()
	top.RLock()
	defer top.RUnlock()
	for {
		if node == nil {
			// Avoid crashing. This can happen if we are using an old
			// node that the browser replaced, for example.
			return ""
		}
		if node.FrameID != "" {
			break
		}
		node = top.Nodes[node.ParentID]
	}
	return node.FrameID
}

// ensureFrame makes sure that the top frame of this target is loaded. It
// returns the top frame, the root node, and the ExecutionContextID of the top
// frame. Otherwise it returns false as its last return value.
//
// Without the Runtime domain, it can send a command. So it must not run on the
// goroutines of Target.run.
func (t *Target) ensureFrame(ctx context.Context) (*Frame, *Node, runtime.ExecutionContextID, bool) {
	t.frameMu.RLock()
	cur := t.cur
	frame := t.frames[cur]
	t.frameMu.RUnlock()

	// the frame has not loaded yet.
	if frame == nil {
		return nil, nil, 0, false
	}

	frame.RLock()
	root := frame.Root
	frame.RUnlock()

	if root == nil {
		// not root node yet?
		return nil, nil, 0, false
	}

	execCtx, err := t.execContext(ctx, cur)
	if err != nil || execCtx == 0 {
		return nil, nil, 0, false
	}
	return frame, root, execCtx, true
}

// worldName is the name of the isolated world that a target without the
// Runtime domain creates in a frame. The page cannot see an isolated world.
const worldName = "chromedp"

// execContext returns the execution context of the frame with the id, where
// the queries and Poll run their scripts.
//
// With the Runtime domain, it is the main world of the frame, as the Runtime
// events reported it, or 0 when there is none yet. Without the Runtime domain,
// it is an isolated world. execContext creates it with Page.createIsolatedWorld
// on the first call for the document of the frame. dropWorlds forgets it when
// the frame navigates or detaches, or when the document is updated.
//
// Without the Runtime domain, it sends a command. So it must not run on the
// goroutines of Target.run.
func (t *Target) execContext(ctx context.Context, id cdp.FrameID) (runtime.ExecutionContextID, error) {
	t.frameMu.RLock()
	execCtx, gen := t.execContexts[id], t.worldGen
	t.frameMu.RUnlock()
	if execCtx != 0 || !t.noRuntime || id == "" {
		return execCtx, nil
	}
	res, err := cdp.Call(ctx, t, page.CreateIsolatedWorld, page.CreateIsolatedWorldParams{
		FrameID:   id,
		WorldName: worldName,
	})
	if err != nil {
		return 0, fmt.Errorf("creating an isolated world in frame %s: %w", id, err)
	}
	// When the frame navigated between the command and the reply, the world
	// can belong to the old document. Use it for this call, and do not keep it.
	t.frameMu.Lock()
	if t.worldGen == gen {
		t.execContexts[id] = res.ExecutionContextID
	}
	t.frameMu.Unlock()
	return res.ExecutionContextID, nil
}

// dropWorlds forgets the isolated world of the frame with the id, or of every
// frame when the id is empty, because a new document has none of the old
// worlds. With the Runtime domain, it does nothing, because the Runtime events
// keep execContexts.
func (t *Target) dropWorlds(id cdp.FrameID) {
	if !t.noRuntime {
		return
	}
	t.frameMu.Lock()
	defer t.frameMu.Unlock()
	if id == "" {
		clear(t.execContexts)
	} else {
		delete(t.execContexts, id)
	}
	t.worldGen++
}

func (t *Target) run(ctx context.Context) {
	type eventValue struct {
		method cdproto.MethodType
		value  any
	}
	// syncEventQueue is used to handle events synchronously within Target.
	// TODO: If this queue gets full, the goroutine below can get stuck on a
	// send. Then the response callbacks never run, and this causes a deadlock.
	// Can we fix this without potentially using lots of memory?
	syncEventQueue := make(chan eventValue, 4096)

	// This goroutine receives events from the browser, calls listeners, and
	// then passes the events onto the main goroutine for the target handler
	// to update itself.
	go func() {
		defer t.events.close()
		for {
			select {
			case <-ctx.Done():
				return

			case msg := <-t.messageQueue:
				if msg == detachedMessage {
					close(t.detached)
					continue
				}
				if msg.ID != 0 {
					t.listenersMu.Lock()
					t.listeners = runListeners(t.listeners, msg)
					t.listenersMu.Unlock()
					continue
				}
				t.events.publish(string(msg.Method), msg.Params)
				ev, err := cdproto.UnmarshalMessage(msg, DefaultUnmarshalOptions)
				if err != nil {
					if _, ok := err.(cdp.ErrUnknownCommandOrEvent); ok {
						// This is most likely an event from an older Chrome
						// that a newer cdproto does not have, because it is
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

// detachedMessage is the message that the browser puts in the queue of a target
// after the last message that it routes to the target.
var detachedMessage = new(cdproto.Message)

// Call sends the command to the target, waits for the response, and decodes
// the result into res. It satisfies [cdp.Session].
//
// Call returns a browser error as a [*cdproto.Error]. It returns an error that
// wraps [ErrTargetDetached] when the browser detaches the target before it
// replies, for example when the page closes. It returns an error when
// the connection to the browser is lost, also when ctx never ends. The error
// wraps the reason of the loss and [context.Canceled].
func (t *Target) Call(ctx context.Context, method string, params, res any) error {
	if method == target.CommandCloseTarget {
		return errors.New("to close the target, cancel its context or use chromedp.Cancel")
	}

	id := atomic.AddInt64(&t.browser.next, 1)
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
	t.listenersMu.Lock()
	t.listeners = append(t.listeners, cancelableListener{lctx, fn})
	t.listenersMu.Unlock()

	// send command
	var buf []byte
	if params != nil {
		var err error
		if buf, err = jsonv2.Marshal(params, DefaultMarshalOptions); err != nil {
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
	case <-t.browser.LostConnection:
		return t.browser.lostError(ctx)
	case <-t.detached:
		return t.detachedError()
	case t.browser.cmdQueue <- cmd:
	}

	// wait for result
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.browser.LostConnection:
		// The reply can arrive just before the loss is known.
		select {
		case msg := <-ch:
			if msg != nil {
				return decodeReply(msg, res)
			}
		default:
		}
		return t.browser.lostError(ctx)
	case msg := <-ch:
		return decodeReply(msg, res)
	case <-t.detached:
		// The target handled every earlier message before it closed detached,
		// so a reply that exists is in the channel by now.
		select {
		case msg := <-ch:
			return decodeReply(msg, res)
		default:
		}
		return t.detachedError()
	}
}

// detachedError returns the error of a command on a detached target.
func (t *Target) detachedError() error {
	return fmt.Errorf("target %s: %w: inspected target navigated or closed", t.TargetID, ErrTargetDetached)
}

// Subscribe starts to buffer the target events with the method, and returns
// the channel that the raw event parameters arrive on. It satisfies
// [cdp.Session].
func (t *Target) Subscribe(method string) (<-chan jsonv2.Value, func()) {
	return t.events.subscribe(method)
}

// runtimeEvent handles incoming runtime events.
func (t *Target) runtimeEvent(ev any) {
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
			t.execUniqueIDs[aux.FrameID] = ev.Context.UniqueID
			t.frameMu.Unlock()
		}
	case *runtime.EventExecutionContextDestroyed:
		t.frameMu.Lock()
		for frameID, uniqueID := range t.execUniqueIDs {
			if uniqueID == ev.ExecutionContextUniqueID {
				delete(t.execContexts, frameID)
				delete(t.execUniqueIDs, frameID)
			}
		}
		t.frameMu.Unlock()
	case *runtime.EventExecutionContextsCleared:
		t.frameMu.Lock()
		clear(t.execContexts)
		clear(t.execUniqueIDs)
		t.frameMu.Unlock()
	}
}

// documentUpdated handles the document updated event. It retrieves the
// document root for the root frame.
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
		f.Root = nil
	}

	f.Nodes = make(map[cdp.NodeID]*Node)
	res, err := cdp.Call(ctx, t, dom.GetDocument, dom.GetDocumentParams{})
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrTargetDetached) {
		return // TODO: perhaps not necessary, but useful to keep the tests less noisy
	}
	if err != nil {
		t.errf("could not retrieve document root for %s: %v", f.ID, err)
		return
	}
	f.Root = newNode(res.Root)
	f.Root.Invalidated = make(chan struct{})
	walk(f.Nodes, f.Root)
}

// pageEvent handles incoming page events.
func (t *Target) pageEvent(ev any) {
	var id cdp.FrameID
	var op frameOp

	switch e := ev.(type) {
	case *page.EventFrameNavigated:
		t.dropWorlds(e.Frame.ID)
		t.frameMu.Lock()
		t.frames[e.Frame.ID] = &Frame{Frame: e.Frame}
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
		t.dropWorlds(e.FrameID)
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
		*page.EventBackForwardCacheNotUsed,
		*page.EventFrameSubtreeWillBeDetached,
		*page.EventFrameStartedNavigating:
		return

	default:
		t.errf("unhandled page event %T", ev)
		return
	}

	t.frameMu.Lock()
	f := t.frames[id]
	if f == nil {
		// This can happen if a frame is attached or starts loading
		// before it is ever navigated to. We do not have all the frame
		// details yet, but that is fine.
		f = &Frame{Frame: &cdp.Frame{ID: id}}
		t.frames[id] = f
	}
	t.frameMu.Unlock()

	f.Lock()
	op(f)
	f.Unlock()
}

// domEvent handles incoming DOM events.
func (t *Target) domEvent(ctx context.Context, ev any) {
	t.frameMu.RLock()
	f := t.frames[t.cur]
	t.frameMu.RUnlock()

	var id cdp.NodeID
	var op nodeOp

	switch e := ev.(type) {
	case *dom.EventDocumentUpdated:
		t.dropWorlds("")
		t.documentUpdated(ctx)
		return

	case *dom.EventSetChildNodes:
		id, op = e.ParentID, setChildNodes(f.Nodes, newNodes(e.Nodes))

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
		id, op = e.ParentNodeID, childNodeInserted(f.Nodes, e.PreviousNodeID, newNode(e.Node))

	case *dom.EventChildNodeRemoved:
		id, op = e.ParentNodeID, childNodeRemoved(f.Nodes, e.NodeID)

	case *dom.EventShadowRootPushed:
		id, op = e.HostID, shadowRootPushed(f.Nodes, newNode(e.Root))

	case *dom.EventShadowRootPopped:
		id, op = e.HostID, shadowRootPopped(f.Nodes, e.RootID)

	case *dom.EventPseudoElementAdded:
		id, op = e.ParentID, pseudoElementAdded(f.Nodes, newNode(e.PseudoElement))

	case *dom.EventPseudoElementRemoved:
		id, op = e.ParentID, pseudoElementRemoved(f.Nodes, e.PseudoElementID)

	case *dom.EventDistributedNodesUpdated:
		id, op = e.InsertionPointID, distributedNodesUpdated(e.DistributedNodes)

	case *dom.EventScrollableFlagUpdated:
		id, op = e.NodeID, scrollableFlagUpdated(e.IsScrollable)

	case *dom.EventAdRelatedStateUpdated:
		id, op = e.NodeID, adRelatedStateUpdated(e.AdProvenance)

	case *dom.EventAdoptedStyleSheetsModified:
		id, op = e.NodeID, adoptedStyleSheetsModified(e.AdoptedStyleSheets)

	case *dom.EventAffectedByStartingStylesFlagUpdated:
		id, op = e.NodeID, affectedByStartingStylesFlagUpdated(e.AffectedByStartingStyles)

		// ignored events
	case *dom.EventTopLayerElementsUpdated:
		// The event has no parameters and names no node. The top layer
		// holds the elements that a page shows above all others, and
		// chromedp does not keep it.
		return

	default:
		t.errf("unhandled node event %T", ev)
		return
	}

	n, ok := f.Nodes[id]
	if !ok {
		// The node ID is no longer valid. Nothing to do.
		return
	}

	f.Lock()
	op(n)
	f.Unlock()
}
