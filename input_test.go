package chromedp

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp/kb"
)

// inViewportJS is a JavaScript snippet that gets the position of the specified
// node relative to the viewport. It returns true if the node is within the
// viewport of the window.
const inViewportJS = `(function(a) {
		var r = a[0].getBoundingClientRect();
		return r.top >= 0 && r.left >= 0 && r.bottom <= window.innerHeight && r.right <= window.innerWidth;
	})($x(%q))`

func TestMouseClickXY(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "input.html")
	defer cancel()

	if err := Do(ctx, WaitVisible(ID(`input1`))); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		x, y float64
	}{
		{100, 100},
		{0, 0},
		{9999, 100},
		{100, 9999},
	}

	for i, test := range tests {
		var xstr, ystr string
		if err := Do(ctx,
			MouseClickXY(test.x, test.y),
			into(&xstr, Value(ID("input1"))),
		); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		x, err := strconv.ParseFloat(xstr, 64)
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if x != test.x {
			t.Fatalf("test %d expected x to be: %f, got: %f", i, test.x, x)
		}
		if err := Do(ctx, into(&ystr, Value(ID("input2")))); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		y, err := strconv.ParseFloat(ystr, 64)
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if y != test.y {
			t.Fatalf("test %d expected y to be: %f, got: %f", i, test.y, y)
		}
	}
}

func TestMouseClickNode(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		mouseClickNodeTest(ID(`button2`), "foo", ButtonType(input.MouseButtonNone)),
		mouseClickNodeTest(ID(`button2`), "bar", ButtonType(input.MouseButtonLeft)),
		mouseClickNodeTest(ID(`button2`), "bar-middle", ButtonType(input.MouseButtonMiddle)),
		mouseClickNodeTest(ID(`input3`), "foo", ButtonModifiers(ModifierNone)),
		mouseClickNodeTest(ID(`input3`), "bar-right", ButtonType(input.MouseButtonRight)),
		mouseClickNodeTest(ID(`input3`), "bar-right", Button("right")),
		mouseClickNodeTest(JSPath(`document.querySelector('#input3')`), "bar-right", ButtonType(input.MouseButtonRight)),
		mouseClickNodeTest(ID(`link`), "clicked", ButtonType(input.MouseButtonLeft)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// mouseClickNodeTest returns a test that clicks the node that sel selects.
func mouseClickNodeTest[S Selectable](sel S, exp string, opt MouseOption) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "input.html")
		defer cancel()

		var nodes []*Node
		if err := Do(ctx, into(&nodes, Nodes(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}
		if len(nodes) != 1 {
			t.Fatalf("expected nodes to have exactly 1 element, got: %d", len(nodes))
		}
		var value string
		if err := Do(ctx,
			MouseClickNode(nodes[0], opt),
			into(&value, Value(ID("input3"))),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != exp {
			t.Fatalf("expected to have value %s, got: %s", exp, value)
		}
	}
}

func TestMouseClickOffscreenNode(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		mouseClickOffscreenNodeTest(ID(`button3`), 0),
		mouseClickOffscreenNodeTest(ID(`button3`), 2),
		mouseClickOffscreenNodeTest(ID(`button3`), 10),
		mouseClickOffscreenNodeTest(JSPath(`document.querySelector('#button3')`), 10),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// mouseClickOffscreenNodeTest returns a test that clicks the offscreen node that
// sel selects exp times.
func mouseClickOffscreenNodeTest[S Selectable](sel S, exp int) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "input.html")
		defer cancel()

		var nodes []*Node
		if err := Do(ctx, into(&nodes, Nodes(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if len(nodes) != 1 {
			t.Fatalf("expected nodes to have exactly 1 element, got: %d", len(nodes))
		}

		var ok bool
		if err := Do(ctx, into(&ok, EvaluateAsDevTools[bool](fmt.Sprintf(inViewportJS, nodes[0].FullXPath())))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if ok {
			t.Fatal("expected node to be offscreen")
		}

		for i := exp; i > 0; i-- {
			if err := Do(ctx, MouseClickNode(nodes[0])); err != nil {
				t.Fatalf("got error: %v", err)
			}
		}

		var value int
		if err := Do(ctx, into(&value, Evaluate[int]("window.document.test_i"))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != exp {
			t.Fatalf("expected to have value %d, got: %d", exp, value)
		}
	}
}

func TestKeyEvent(t *testing.T) {
	if os.Getenv("HEADLESS_SHELL") != "" {
		t.Skip(`Skip in headless-shell due to "Check failed: IsSupportedClipboardBuffer(buffer)"`)
	}

	t.Parallel()

	tests := []func(t *testing.T){
		keyEventTest(ID(`input4`), "foo"),
		keyEventTest(ID(`input4`), "foo and bar"),
		keyEventTest(ID(`input4`), "1234567890"),
		keyEventTest(ID(`input4`), "~!@#$%^&*()_+=[];'"),
		keyEventTest(ID(`input4`), "你"),
		keyEventTest(ID(`input4`), "\n\nfoo\n\nbar\n\n"),
		keyEventTest(JSPath(`document.querySelector('#input4')`), "\n\ntest\n\n"),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// keyEventTest returns a test that sends the keys of exp to the node that sel
// selects.
func keyEventTest[S Selectable](sel S, exp string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "input.html")
		defer cancel()

		var nodes []*Node
		if err := Do(ctx, into(&nodes, Nodes(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if len(nodes) != 1 {
			t.Fatalf("expected nodes to have exactly 1 element, got: %d", len(nodes))
		}
		if err := Do(ctx,
			Focus(sel),
			KeyEvent(kb.Home),
			// "KeyEvent(kb.End, KeyModifiers(ModifierShift))" crash headless-shell with this error:
			// [...:FATAL:headless_clipboard.cc(296)] Check failed: IsSupportedClipboardBuffer(buffer)
			KeyEvent(kb.End, KeyModifiers(ModifierShift)),
			KeyEvent(exp),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		var value string
		if err := Do(ctx, into(&value, Value(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != exp {
			t.Fatalf("expected to have value %s, got: %s", exp, value)
		}
	}
}

func TestKeyEventNode(t *testing.T) {
	if os.Getenv("HEADLESS_SHELL") != "" {
		t.Skip(`Skip in headless-shell due to "Check failed: IsSupportedClipboardBuffer(buffer)"`)
	}

	t.Parallel()

	tests := []func(t *testing.T){
		keyEventNodeTest(ID(`input4`), "foo"),
		keyEventNodeTest(ID(`input4`), "foo and bar"),
		keyEventNodeTest(ID(`input4`), "1234567890"),
		keyEventNodeTest(ID(`input4`), "~!@#$%^&*()_+=[];'"),
		keyEventNodeTest(ID(`input4`), "你"),
		keyEventNodeTest(ID(`input4`), "\n\nfoo\n\nbar\n\n"),
		keyEventNodeTest(JSPath(`document.querySelector('#input4')`), "\n\ntest\n\n"),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// keyEventNodeTest returns a test that sends the keys of exp to the node that
// sel selects.
func keyEventNodeTest[S Selectable](sel S, exp string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "input.html")
		defer cancel()

		var nodes []*Node
		if err := Do(ctx, into(&nodes, Nodes(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if len(nodes) != 1 {
			t.Fatalf("expected nodes to have exactly 1 element, got: %d", len(nodes))
		}
		var value string
		if err := Do(ctx,
			KeyEventNode(nodes[0], kb.Home),
			// "KeyEventNode(nodes[0], kb.End, KeyModifiers(ModifierShift))" crash headless-shell with this error:
			// [...:FATAL:headless_clipboard.cc(296)] Check failed: IsSupportedClipboardBuffer(buffer)
			KeyEventNode(nodes[0], kb.End, KeyModifiers(ModifierShift)),
			KeyEventNode(nodes[0], exp),
			into(&value, Value(sel)),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != exp {
			t.Fatalf("expected to have value %s, got: %s", exp, value)
		}
	}
}
