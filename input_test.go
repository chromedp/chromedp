package chromedp

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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

// TestKeyEventModifier checks that a letter with Ctrl, Alt or Meta fires the
// key events with the modifier, and that it types no character. The old code
// sent a char event with the text of the letter too, so Ctrl+A selected the
// text, and then the char event replaced it with "a". See the issue 1384.
func TestKeyEventModifier(t *testing.T) {
	if os.Getenv("HEADLESS_SHELL") != "" {
		t.Skip(`Skip in headless-shell due to "Check failed: IsSupportedClipboardBuffer(buffer)"`)
	}

	t.Parallel()

	const seed = "admin123"
	const probe = `(function() {
		const el = document.getElementById("input4");
		window.probe = {inputs: 0, keydowns: []};
		el.addEventListener("input", () => window.probe.inputs++);
		el.addEventListener("keydown", e => window.probe.keydowns.push(
			[e.key, e.ctrlKey, e.altKey, e.metaKey, e.shiftKey].join(",")));
	})()`
	type state struct {
		Value    string `json:"value"`
		Start    int    `json:"start"`
		End      int    `json:"end"`
		Inputs   int    `json:"inputs"`
		Keydowns string `json:"keydowns"`
	}
	read := func(t *testing.T, ctx context.Context) state {
		t.Helper()
		st, err := Run(ctx, Evaluate[state](`(function() {
			const el = document.getElementById("input4");
			return {value: el.value, start: el.selectionStart, end: el.selectionEnd,
				inputs: window.probe.inputs, keydowns: window.probe.keydowns.join(";")};
		})()`))
		if err != nil {
			t.Fatal(err)
		}
		return st
	}

	for _, tt := range []struct {
		name string
		key  string
		mod  Modifier
		// want is the part of the Keydowns text that the page must see.
		want string
	}{
		{"ctrl+a", "a", ModifierCtrl, "a,true,false,false,false"},
		{"alt+x", "x", ModifierAlt, "x,false,true,false,false"},
		{"meta+x", "x", ModifierMeta, "x,false,false,true,false"},
		{"ctrl+shift+x", "x", ModifierCtrl | ModifierShift, "x,true,false,false,true"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "input.html")
			defer cancel()
			if err := Do(ctx,
				Focus(ID("input4")),
				KeyEvent(kb.Home),
				KeyEvent(kb.End, KeyModifiers(ModifierShift)),
				KeyEvent(seed),
			); err != nil {
				t.Fatal(err)
			}
			if _, err := Run(ctx, Evaluate[any](probe)); err != nil {
				t.Fatal(err)
			}
			if err := Do(ctx, KeyEvent(tt.key, KeyModifiers(tt.mod))); err != nil {
				t.Fatal(err)
			}
			st := read(t, ctx)
			if st.Value != seed || st.Inputs != 0 {
				t.Errorf("the key typed text: value %q after %d input events", st.Value, st.Inputs)
			}
			if !strings.Contains(strings.ToLower(st.Keydowns), strings.ToLower(tt.want)) {
				t.Errorf("want a keydown with %q, got %q", tt.want, st.Keydowns)
			}
			if tt.mod == ModifierCtrl && (st.Start != 0 || st.End != len(seed)) {
				t.Errorf("Ctrl+A must select all of the text, got the selection %d to %d", st.Start, st.End)
			}
		})
	}

	// Shift alone does not stop the char event. The text of the event is the
	// letter that the caller gave.
	t.Run("shift+e", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "input.html")
		defer cancel()
		if err := Do(ctx,
			Focus(ID("input4")),
			KeyEvent(kb.Home),
			KeyEvent(kb.End, KeyModifiers(ModifierShift)),
			KeyEvent("e", KeyModifiers(ModifierShift)),
		); err != nil {
			t.Fatal(err)
		}
		got, err := Run(ctx, Value(ID("input4")))
		if err != nil {
			t.Fatal(err)
		}
		if got != "e" {
			t.Fatalf("want e, got %q", got)
		}
	})
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
