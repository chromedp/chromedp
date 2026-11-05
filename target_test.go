package chromedp

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/inspector"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
)

// newEventTarget makes a target with one frame that holds the nodes. It needs
// no browser. The returned func gives the messages that the target logged as
// errors.
func newEventTarget(nodes ...*Node) (*Target, func() []string) {
	var mu sync.Mutex
	var logged []string
	f := &Frame{Frame: &cdp.Frame{ID: "frame"}, Nodes: make(map[cdp.NodeID]*Node)}
	for _, n := range nodes {
		f.Nodes[n.NodeID] = n
	}
	t := &Target{
		browser: &Browser{},
		frames:  map[cdp.FrameID]*Frame{f.ID: f},
		cur:     f.ID,
		logf:    func(string, ...any) {},
		errf: func(format string, args ...any) {
			mu.Lock()
			defer mu.Unlock()
			logged = append(logged, strings.TrimSpace(fmt.Sprintf(format, args...)))
		},
	}
	return t, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(logged)
	}
}

// A failed DOM.getDocument must not leave the old root in place. Otherwise the
// next DOM.documentUpdated event closes its channel again and panics.
func TestDocumentUpdatedAfterFailedGetDocument(t *testing.T) {
	t.Parallel()

	tgt, _ := newEventTarget()
	root := &Node{Node: &cdp.Node{NodeID: 1}, Invalidated: make(chan struct{})}
	f := tgt.frames[tgt.cur]
	f.Root = root

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tgt.domEvent(ctx, &dom.EventDocumentUpdated{})
	tgt.domEvent(ctx, &dom.EventDocumentUpdated{})

	select {
	case <-root.Invalidated:
	default:
		t.Fatal("the old root is not invalidated")
	}
	if f.Root != nil {
		t.Fatalf("the frame keeps the invalidated root %v", f.Root)
	}
}

func TestNodeStateEvents(t *testing.T) {
	t.Parallel()

	node := &Node{Node: &cdp.Node{NodeID: 7}}
	tgt, logged := newEventTarget(node)
	ctx := context.Background()

	provenance := &cdp.AdProvenance{FilterlistRule: "rule"}
	sheets := []cdp.StyleSheetID{"sheet1", "sheet2"}
	tgt.domEvent(ctx, &dom.EventScrollableFlagUpdated{NodeID: 7, IsScrollable: true})
	tgt.domEvent(ctx, &dom.EventAdRelatedStateUpdated{NodeID: 7, AdProvenance: provenance})
	tgt.domEvent(ctx, &dom.EventAdoptedStyleSheetsModified{NodeID: 7, AdoptedStyleSheets: sheets})
	tgt.domEvent(ctx, &dom.EventAffectedByStartingStylesFlagUpdated{NodeID: 7, AffectedByStartingStyles: true})
	// An event for a node that is not known changes nothing.
	tgt.domEvent(ctx, &dom.EventScrollableFlagUpdated{NodeID: 8, IsScrollable: false})

	node.RLock()
	if !node.IsScrollable {
		t.Error("IsScrollable was not set")
	}
	if node.AdProvenance != provenance {
		t.Errorf("AdProvenance is %v", node.AdProvenance)
	}
	if !slices.Equal(node.AdoptedStyleSheets, sheets) {
		t.Errorf("AdoptedStyleSheets is %v", node.AdoptedStyleSheets)
	}
	if !node.AffectedByStartingStyles {
		t.Error("AffectedByStartingStyles was not set")
	}
	node.RUnlock()

	// The flags can go back to false.
	tgt.domEvent(ctx, &dom.EventScrollableFlagUpdated{NodeID: 7, IsScrollable: false})
	tgt.domEvent(ctx, &dom.EventAdRelatedStateUpdated{NodeID: 7})
	node.RLock()
	if node.IsScrollable || node.AdProvenance != nil {
		t.Errorf("the state did not reset: %v, %v", node.IsScrollable, node.AdProvenance)
	}
	node.RUnlock()

	if got := logged(); len(got) != 0 {
		t.Errorf("unexpected errors: %v", got)
	}
}

// protocolEventNames returns the names of the event types that the package of
// cdproto with the import path pkg declares. It reads the source files in the
// module cache, so a new version of cdproto in go.mod is checked as well.
func protocolEventNames(t *testing.T, pkg string) []string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("the test needs the go command to find the protocol package: %v", err)
	}
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", pkg).Output()
	if err != nil {
		t.Fatalf("finding the package %s: %v", pkg, err)
	}
	dir := strings.TrimSpace(string(out))
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range pkgs {
		for _, file := range p.Files {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts := spec.(*ast.TypeSpec)
					if _, ok := ts.Type.(*ast.StructType); ok && strings.HasPrefix(ts.Name.Name, "Event") {
						names = append(names, ts.Name.Name)
					}
				}
			}
		}
	}
	slices.Sort(names)
	return names
}

// TestTargetHandlesEveryEvent makes sure that the target handles every event
// of the DOM and the Page domain, or lists it as an event to ignore. A new
// version of cdproto can add an event. Then the target logs "unhandled
// node event" or "unhandled page event" for every page that sends it. See
// the issue 1530.
//
// If this test fails, add a case for the event to domEvent or pageEvent in
// target.go. Handle the event when it carries node or frame state that chromedp
// keeps. Otherwise add it to the ignored events.
func TestTargetHandlesEveryEvent(t *testing.T) {
	t.Parallel()

	// The canceled context stops Target.documentUpdated before it needs a
	// browser.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, domain := range []struct {
		pkg    string
		domain string
		handle func(*Target, context.Context, any)
	}{
		{"github.com/chromedp/cdproto/dom", "DOM", func(tgt *Target, ctx context.Context, ev any) { tgt.domEvent(ctx, ev) }},
		{"github.com/chromedp/cdproto/page", "Page", func(tgt *Target, _ context.Context, ev any) { tgt.pageEvent(ev) }},
	} {
		names := protocolEventNames(t, domain.pkg)
		if len(names) < 10 {
			t.Fatalf("found only %d events in %s", len(names), domain.pkg)
		}
		for _, name := range names {
			t.Run(domain.domain+"."+name, func(t *testing.T) {
				// The method name is the name of the type without the prefix
				// Event, with a lower case first letter.
				rest := strings.TrimPrefix(name, "Event")
				method := domain.domain + "." + strings.ToLower(rest[:1]) + rest[1:]
				// The protocol always sends the frame of this event, and
				// the handler reads it.
				params := []byte("{}")
				if method == "Page.frameNavigated" {
					params = []byte(`{"frame":{}}`)
				}
				ev, err := cdproto.UnmarshalMessage(&cdproto.Message{
					Method: cdproto.MethodType(method),
					Params: params,
				}, DefaultUnmarshalOptions)
				if err != nil {
					t.Fatalf("decoding an empty %s: %v", method, err)
				}
				tgt, logged := newEventTarget()
				domain.handle(tgt, ctx, ev)
				if got := logged(); len(got) != 0 {
					t.Errorf("target.go does not handle %s: %v", method, got)
				}
			})
		}
	}
}

func TestCallOnDetachedTarget(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()
	// a tab of its own, that the browser closes
	tabCtx, tabCancel := NewContext(ctx)
	defer tabCancel()
	if err := Do(tabCtx); err != nil {
		t.Fatal(err)
	}
	id := FromContext(tabCtx).Target.TargetID

	// The browser sends no reply to a command that is pending when the target
	// goes away. The command must end with an error and not wait for its context.
	go func() {
		time.Sleep(200 * time.Millisecond)
		if _, err := CallBrowser(ctx, target.CloseTarget, target.CloseTargetParams{TargetID: id}); err != nil {
			t.Error(err)
		}
	}()
	rctx, rcancel := context.WithTimeout(tabCtx, 10*time.Second)
	defer rcancel()
	_, err := Call(rctx, runtime.Evaluate, runtime.EvaluateParams{
		Expression:   `new Promise(() => {})`,
		AwaitPromise: ptr(true),
	})
	if !errors.Is(err, ErrTargetDetached) {
		t.Fatalf("want ErrTargetDetached, got %v", err)
	}

	// A later command fails at once.
	if _, err := Call(rctx, runtime.Evaluate, runtime.EvaluateParams{Expression: `1`}); !errors.Is(err, ErrTargetDetached) {
		t.Fatalf("want ErrTargetDetached for a later command, got %v", err)
	}
}

func TestDetached(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "")
	defer cancel()
	// a tab of its own, that the browser closes
	tabCtx, tabCancel := NewContext(ctx)
	defer tabCancel()
	if err := Do(tabCtx); err != nil {
		t.Fatal(err)
	}
	tgt := FromContext(tabCtx).Target

	ch, err := Detached(tabCtx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
		t.Fatal("the channel is closed before the tab closes")
	default:
	}

	// The listener and the subscription get the same events from the same
	// loop. The page logs all the time, so events arrive until the tab closes.
	// Chrome sends inspector.EventDetached just before the detach. The
	// listener is slow on it, so that a channel that closes too early shows.
	var r recorder
	if err := Listen(tabCtx, func(ev any) {
		switch ev.(type) {
		case *runtime.EventConsoleAPICalled:
			r.add(ev)
		case *inspector.EventDetached:
			time.Sleep(100 * time.Millisecond)
			r.add(ev)
		}
	}); err != nil {
		t.Fatal(err)
	}
	events, stop := tgt.events.subscribeMany(cdproto.EventRuntimeConsoleAPICalled, cdproto.EventInspectorDetached)
	defer stop()
	if _, err := Run(tabCtx, Evaluate[any](`setInterval(() => console.log("tick"), 0), 1`)); err != nil {
		t.Fatal(err)
	}
	if _, err := CallBrowser(ctx, target.CloseTarget, target.CloseTargetParams{TargetID: tgt.TargetID}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("the channel did not close after the tab closed")
	}

	// When the channel closes, the listener saw every event that the target
	// got. The subscription delivers them later, so it has no more.
	seen := len(r.snapshot())
	if seen == 0 {
		t.Fatal("the listener saw no event")
	}
	for i := range seen {
		select {
		case <-events:
		case <-time.After(10 * time.Second):
			t.Fatalf("the subscription has %d events, and the listener saw %d", i, seen)
		}
	}
	select {
	case <-events:
		t.Fatalf("the subscription has more events than the %d that the listener saw before the channel closed", seen)
	case <-time.After(200 * time.Millisecond):
	}

	// A call after the detach gets a closed channel.
	later, err := Detached(tabCtx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-later:
	default:
		t.Fatal("the channel of a call after the detach is not closed")
	}
}

func TestDetachedInvalidContext(t *testing.T) {
	t.Parallel()

	if _, err := Detached(context.Background()); !errors.Is(err, ErrInvalidContext) {
		t.Fatalf("want ErrInvalidContext, got %v", err)
	}
}
