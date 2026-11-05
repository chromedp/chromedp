package chromedp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
)

// recorder keeps the events that a listener sees. The listener runs on the
// goroutine of the target, and the test reads from another one.
type recorder struct {
	mu     sync.Mutex
	events []any
}

func (r *recorder) add(ev any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

// snapshot returns the events that the recorder holds now.
func (r *recorder) snapshot() []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

// index returns the position of the first event for which match is true, or -1.
func (r *recorder) index(match func(ev any) bool) int {
	return slices.IndexFunc(r.snapshot(), match)
}

// isConsoleLog returns a match for a console call with the text.
func isConsoleLog(text string) func(ev any) bool {
	return func(ev any) bool {
		e, ok := ev.(*runtime.EventConsoleAPICalled)
		return ok && len(e.Args) > 0 && string(e.Args[0].Value) == `"`+text+`"`
	}
}

func TestListen(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()
	// open the tab, so that Listen does not miss the events of its start
	if err := Do(ctx); err != nil {
		t.Fatal(err)
	}

	var r recorder
	if err := Listen(ctx, r.add); err != nil {
		t.Fatal(err)
	}

	// The console call arrives before the reply to the command that causes it.
	// So the listener has the event when Run returns, with no wait.
	if _, err := Run(ctx, Evaluate[any](`console.log("one"), 1`)); err != nil {
		t.Fatal(err)
	}
	console := r.index(isConsoleLog("one"))
	if console < 0 {
		t.Fatalf("the listener has no console call when the command returns, got %d events", len(r.snapshot()))
	}

	// An event of another type comes after it. The order across the types and
	// across the commands is the order in which the events arrived.
	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err != nil {
		t.Fatal(err)
	}
	navigated := r.index(func(ev any) bool { _, ok := ev.(*page.EventFrameNavigated); return ok })
	if navigated < 0 {
		t.Fatal("the listener has no page.EventFrameNavigated after the navigation")
	}
	if navigated < console {
		t.Fatalf("the frame navigated at %d, before the console call at %d", navigated, console)
	}

	// The listener gets no reply to a command.
	for i, ev := range r.snapshot() {
		if _, ok := ev.(*cdproto.Message); ok {
			t.Fatalf("event %d is a reply to a command: %v", i, ev)
		}
	}
}

func TestListenCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()
	if err := Do(ctx); err != nil {
		t.Fatal(err)
	}

	lctx, lcancel := context.WithCancel(ctx)
	defer lcancel()
	var r recorder
	if err := Listen(lctx, r.add); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(ctx, Evaluate[any](`console.log("before"), 1`)); err != nil {
		t.Fatal(err)
	}
	if r.index(isConsoleLog("before")) < 0 {
		t.Fatal("the listener has no console call before it is canceled")
	}

	// A canceled listener gets no more events. The command that follows has a
	// console call, and the listener must not see it.
	lcancel()
	count := len(r.snapshot())
	if _, err := Run(ctx, Evaluate[any](`console.log("after"), 1`)); err != nil {
		t.Fatal(err)
	}
	if got := len(r.snapshot()); got != count {
		t.Fatalf("the canceled listener saw %d events more", got-count)
	}
}

func TestListenBrowser(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()
	if err := Do(ctx); err != nil {
		t.Fatal(err)
	}

	var r recorder
	if err := ListenBrowser(ctx, r.add); err != nil {
		t.Fatal(err)
	}

	// The browser reports the new tab before the command that opens it
	// returns, so the listener has the event when Do returns.
	tabCtx, tabCancel := NewContext(ctx)
	defer tabCancel()
	if err := Do(tabCtx); err != nil {
		t.Fatal(err)
	}
	id := FromContext(tabCtx).Target.TargetID
	if r.index(func(ev any) bool {
		e, ok := ev.(*target.EventTargetCreated)
		return ok && e.TargetInfo.TargetID == id
	}) < 0 {
		t.Fatalf("the listener has no target.EventTargetCreated for %s", id)
	}
	for i, ev := range r.snapshot() {
		if _, ok := ev.(*cdproto.Message); ok {
			t.Fatalf("event %d is a reply to a command: %v", i, ev)
		}
	}
}

func TestListenInvalidContext(t *testing.T) {
	t.Parallel()

	fn := func(any) { t.Error("the listener was called") }
	if err := Listen(context.Background(), fn); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("want ErrInvalidContext from Listen, got %v", err)
	}
	if err := ListenBrowser(context.Background(), fn); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("want ErrInvalidContext from ListenBrowser, got %v", err)
	}
}

func TestEventsOnly(t *testing.T) {
	t.Parallel()

	var got []any
	fn := eventsOnly(func(ev any) { got = append(got, ev) })
	event := &page.EventLoadEventFired{}
	fn(&cdproto.Message{ID: 1})
	fn(event)
	fn(&cdproto.Message{ID: 2})
	if len(got) != 1 || got[0] != any(event) {
		t.Fatalf("want only the event, got %v", got)
	}
}

// defaultContextFrames returns the frames that have a default JavaScript
// context in the events.
func defaultContextFrames(t *testing.T, events []any) map[cdp.FrameID]bool {
	t.Helper()
	frames := make(map[cdp.FrameID]bool)
	for _, ev := range events {
		e, ok := ev.(*runtime.EventExecutionContextCreated)
		if !ok || len(e.Context.AuxData) == 0 {
			continue
		}
		var aux struct {
			IsDefault bool        `json:"isDefault"`
			FrameID   cdp.FrameID `json:"frameId"`
		}
		if err := json.Unmarshal(e.Context.AuxData, &aux); err != nil {
			t.Fatal(err)
		}
		if aux.IsDefault {
			frames[aux.FrameID] = true
		}
	}
	return frames
}

// frameIDs returns the id of each frame in the tree.
func frameIDs(tree *page.FrameTree) []cdp.FrameID {
	ids := []cdp.FrameID{tree.Frame.ID}
	for _, child := range tree.ChildFrames {
		ids = append(ids, frameIDs(child)...)
	}
	return ids
}

func TestWithListenerNewTab(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()
	// Two funcs, so that the option works more than once.
	var r1, r2 recorder
	tabCtx, tabCancel := NewContext(ctx, WithListener(r1.add), WithListener(r2.add))
	defer tabCancel()
	if err := Do(tabCtx); err != nil {
		t.Fatal(err)
	}

	// Runtime.enable announces the context of the blank page, during the
	// first Do. A func of Listen after Do does not see it.
	id := cdp.FrameID(FromContext(tabCtx).Target.TargetID)
	for i, r := range []*recorder{&r1, &r2} {
		events := r.snapshot()
		if !defaultContextFrames(t, events)[id] {
			t.Fatalf("func %d has no runtime.EventExecutionContextCreated for the frame %s, got %d events", i+1, id, len(events))
		}
		for _, ev := range events {
			if _, ok := ev.(*cdproto.Message); ok {
				t.Fatalf("func %d got a reply to a command: %v", i+1, ev)
			}
		}
	}
}

func TestWithListenerTargetID(t *testing.T) {
	t.Parallel()

	// A tab with an iframe, which has its own JavaScript context.
	ctx, cancel := testAllocate(t, "iframe.html")
	defer cancel()
	id := FromContext(ctx).Target.TargetID

	// Attach a second context to the tab that runs already. Leave the tab
	// open, because the first context closes it.
	var r recorder
	frameCtx, frameCancel := NewContext(ctx, WithTargetID(id), WithDetachOnCancel(), WithListener(r.add))
	defer frameCancel()
	tree, err := Run(frameCtx, func(ctx context.Context, t *Target) (*page.FrameTree, error) {
		res, err := cdp.Call(ctx, t, page.GetFrameTree, cdp.Empty{})
		return res.FrameTree, err
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := frameIDs(tree)
	if len(ids) != 2 {
		t.Fatalf("want the page and the iframe, got the frames %v", ids)
	}

	// Runtime.enable announces the contexts that exist, and the func sees
	// the context of each frame.
	frames := defaultContextFrames(t, r.snapshot())
	for _, id := range ids {
		if !frames[id] {
			t.Errorf("no runtime.EventExecutionContextCreated for the frame %s, got the frames %v", id, frames)
		}
	}
}
