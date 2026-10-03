package chromedp

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
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
