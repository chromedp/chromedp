package chromedp

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp/kb"
)

// inViewportJS is a JavaScript snippet that will get the specified node
// position relative to the viewport and returns true if the specified node
// is within the window's viewport.
const inViewportJS = `(function(a) {
		var r = a[0].getBoundingClientRect();
		return r.top >= 0 && r.left >= 0 && r.bottom <= window.innerHeight && r.right <= window.innerWidth;
	})($x(%q))`

func TestMouseClickXY(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "input.html")
	defer cancel()

	if err := Do(ctx, WaitVisible(`#input1`, ByID)); err != nil {
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
			into(&xstr, Value("#input1", ByID)),
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
		if err := Do(ctx, into(&ystr, Value("#input2", ByID))); err != nil {
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

	tests := []struct {
		sel, exp string
		opt      MouseOption
		by       QueryOption
	}{
		{`button2`, "foo", ButtonType(input.MouseButtonNone), ByID},
		{`button2`, "bar", ButtonType(input.MouseButtonLeft), ByID},
		{`button2`, "bar-middle", ButtonType(input.MouseButtonMiddle), ByID},
		{`input3`, "foo", ButtonModifiers(ModifierNone), ByID},
		{`input3`, "bar-right", ButtonType(input.MouseButtonRight), ByID},
		{`input3`, "bar-right", Button("right"), ByID},
		{`document.querySelector('#input3')`, "bar-right", ButtonType(input.MouseButtonRight), ByJSPath},
		{`link`, "clicked", ButtonType(input.MouseButtonLeft), ByID},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "input.html")
			defer cancel()

			var nodes []*Node
			if err := Do(ctx, into(&nodes, Nodes(test.sel, test.by))); err != nil {
				t.Fatalf("got error: %v", err)
			}
			if len(nodes) != 1 {
				t.Fatalf("expected nodes to have exactly 1 element, got: %d", len(nodes))
			}
			var value string
			if err := Do(ctx,
				MouseClickNode(nodes[0], test.opt),
				into(&value, Value("#input3", ByID)),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if value != test.exp {
				t.Fatalf("expected to have value %s, got: %s", test.exp, value)
			}
		})
	}
}

func TestMouseClickOffscreenNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		exp int
		by  QueryOption
	}{
		{`#button3`, 0, ByID},
		{`#button3`, 2, ByID},
		{`#button3`, 10, ByID},
		{`document.querySelector('#button3')`, 10, ByJSPath},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "input.html")
			defer cancel()

			var nodes []*Node
			if err := Do(ctx, into(&nodes, Nodes(test.sel, test.by))); err != nil {
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

			for i := test.exp; i > 0; i-- {
				if err := Do(ctx, MouseClickNode(nodes[0])); err != nil {
					t.Fatalf("got error: %v", err)
				}
			}

			var value int
			if err := Do(ctx, into(&value, Evaluate[int]("window.document.test_i"))); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if value != test.exp {
				t.Fatalf("expected to have value %d, got: %d", test.exp, value)
			}
		})
	}
}

func TestKeyEvent(t *testing.T) {
	if os.Getenv("HEADLESS_SHELL") != "" {
		t.Skip(`Skip in headless-shell due to "Check failed: IsSupportedClipboardBuffer(buffer)"`)
	}

	t.Parallel()

	tests := []struct {
		sel, exp string
		by       QueryOption
	}{
		{`#input4`, "foo", ByID},
		{`#input4`, "foo and bar", ByID},
		{`#input4`, "1234567890", ByID},
		{`#input4`, "~!@#$%^&*()_+=[];'", ByID},
		{`#input4`, "你", ByID},
		{`#input4`, "\n\nfoo\n\nbar\n\n", ByID},
		{`document.querySelector('#input4')`, "\n\ntest\n\n", ByJSPath},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "input.html")
			defer cancel()

			var nodes []*Node
			if err := Do(ctx, into(&nodes, Nodes(test.sel, test.by))); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if len(nodes) != 1 {
				t.Fatalf("expected nodes to have exactly 1 element, got: %d", len(nodes))
			}
			if err := Do(ctx,
				Focus(test.sel, test.by),
				KeyEvent(kb.Home),
				// "KeyEvent(kb.End, KeyModifiers(ModifierShift))" crash headless-shell with this error:
				// [...:FATAL:headless_clipboard.cc(296)] Check failed: IsSupportedClipboardBuffer(buffer)
				KeyEvent(kb.End, KeyModifiers(ModifierShift)),
				KeyEvent(test.exp),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			var value string
			if err := Do(ctx, into(&value, Value(test.sel, test.by))); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if value != test.exp {
				t.Fatalf("expected to have value %s, got: %s", test.exp, value)
			}
		})
	}
}

func TestKeyEventNode(t *testing.T) {
	if os.Getenv("HEADLESS_SHELL") != "" {
		t.Skip(`Skip in headless-shell due to "Check failed: IsSupportedClipboardBuffer(buffer)"`)
	}

	t.Parallel()

	tests := []struct {
		sel, exp string
		by       QueryOption
	}{
		{`#input4`, "foo", ByID},
		{`#input4`, "foo and bar", ByID},
		{`#input4`, "1234567890", ByID},
		{`#input4`, "~!@#$%^&*()_+=[];'", ByID},
		{`#input4`, "你", ByID},
		{`#input4`, "\n\nfoo\n\nbar\n\n", ByID},
		{`document.querySelector('#input4')`, "\n\ntest\n\n", ByJSPath},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "input.html")
			defer cancel()

			var nodes []*Node
			if err := Do(ctx, into(&nodes, Nodes(test.sel, test.by))); err != nil {
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
				KeyEventNode(nodes[0], test.exp),
				into(&value, Value(test.sel, test.by)),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if value != test.exp {
				t.Fatalf("expected to have value %s, got: %s", test.exp, value)
			}
		})
	}
}
