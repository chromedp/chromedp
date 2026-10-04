package chromedp

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp/kb"
)

// Mouse event types for MouseEvent.
const (
	MousePressed  = input.DispatchMouseEventTypeMousePressed
	MouseReleased = input.DispatchMouseEventTypeMouseReleased
	MouseMoved    = input.DispatchMouseEventTypeMouseMoved
	MouseWheel    = input.DispatchMouseEventTypeMouseWheel
)

// Modifier is a bit field of pressed modifier keys.
type Modifier = kb.Modifier

// Modifier values.
const (
	ModifierNone    = kb.ModifierNone
	ModifierAlt     = kb.ModifierAlt
	ModifierCtrl    = kb.ModifierCtrl
	ModifierMeta    = kb.ModifierMeta
	ModifierShift   = kb.ModifierShift
	ModifierCommand = kb.ModifierCommand
)

// MouseEvent is a mouse event action to dispatch the specified mouse event
// type at coordinates x, y.
func MouseEvent(typ input.DispatchMouseEventType, x, y float64, opts ...MouseOption) Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		p := &input.DispatchMouseEventParams{Type: typ, X: x, Y: y}
		// apply opts
		for _, o := range opts {
			o(p)
		}
		_, err := cdp.Call(ctx, t, input.DispatchMouseEvent, *p)
		return err
	})
}

// MouseClickXY is an action that sends a left mouse button click (that is, a
// mousePressed and a mouseReleased event) to the X, Y location.
func MouseClickXY(x, y float64, opts ...MouseOption) Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		p := &input.DispatchMouseEventParams{
			Type:       MousePressed,
			X:          x,
			Y:          y,
			Button:     input.MouseButtonLeft,
			ClickCount: 1,
		}

		// apply opts
		for _, o := range opts {
			o(p)
		}

		if _, err := cdp.Call(ctx, t, input.DispatchMouseEvent, *p); err != nil {
			return err
		}

		p.Type = MouseReleased
		_, err := cdp.Call(ctx, t, input.DispatchMouseEvent, *p)
		return err
	})
}

// MouseClickNode is an action that dispatches a left mouse button click event
// at the center of a node.
//
// Note: the action scrolls the window if the node is not within the viewport
// of the window.
func MouseClickNode(n *Node, opts ...MouseOption) Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		x, y, err := nodeCenter(ctx, t, n)
		if err != nil {
			return err
		}

		_, err = MouseClickXY(x, y, opts...)(ctx, t)
		return err
	})
}

// nodeCenter scrolls the node into view and returns the center of its first
// content quad.
func nodeCenter(ctx context.Context, t *Target, n *Node) (x, y float64, err error) {
	if _, err := cdp.Call(ctx, t, dom.ScrollIntoViewIfNeeded, dom.ScrollIntoViewIfNeededParams{NodeID: n.NodeID}); err != nil {
		return 0, 0, err
	}
	return quadCenter(ctx, t, n)
}

// quadCenter returns the center of the node in the viewport, as it is now. It
// does not scroll.
func quadCenter(ctx context.Context, t *Target, n *Node) (x, y float64, err error) {
	res, err := cdp.Call(ctx, t, dom.GetContentQuads, dom.GetContentQuadsParams{NodeID: n.NodeID})
	if err != nil {
		return 0, 0, err
	}

	if len(res.Quads) == 0 {
		return 0, 0, ErrInvalidDimensions
	}

	content := res.Quads[0]

	c := len(content)
	if c%2 != 0 || c < 1 {
		return 0, 0, ErrInvalidDimensions
	}

	for i := 0; i < c; i += 2 {
		x += content[i]
		y += content[i+1]
	}
	return x / float64(c/2), y / float64(c/2), nil
}

// TapXY is an action that sends a touch tap to the X, Y location. It sends a
// touchStart event and then a touchEnd event with one touch point. The browser
// turns the touch into a click event, if the page runs with touch emulation.
// See [EmulateTouch].
func TapXY(x, y float64) Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		p := input.DispatchTouchEventParams{
			Type: input.DispatchTouchEventTypeTouchStart,
			TouchPoints: []*input.TouchPoint{{
				X:       x,
				Y:       y,
				RadiusX: new(1.0),
				RadiusY: new(1.0),
				Force:   new(1.0),
				ID:      new(0.0),
			}},
		}
		if _, err := cdp.Call(ctx, t, input.DispatchTouchEvent, p); err != nil {
			return err
		}

		// A touchEnd event has no touch points.
		p.Type = input.DispatchTouchEventTypeTouchEnd
		p.TouchPoints = []*input.TouchPoint{}
		_, err := cdp.Call(ctx, t, input.DispatchTouchEvent, p)
		return err
	})
}

// MouseOption is a mouse action option.
type MouseOption = func(*input.DispatchMouseEventParams)

// Button is a mouse action option to set the button to click from a string.
func Button(btn string) MouseOption {
	return ButtonType(input.MouseButton(btn))
}

// ButtonType is a mouse action option to set the button to click.
func ButtonType(button input.MouseButton) MouseOption {
	return func(p *input.DispatchMouseEventParams) {
		p.Button = button
	}
}

// ButtonLeft is a mouse action option to set the button clicked as the left
// mouse button.
func ButtonLeft(p *input.DispatchMouseEventParams) {
	p.Button = input.MouseButtonLeft
}

// ButtonMiddle is a mouse action option to set the button clicked as the middle
// mouse button.
func ButtonMiddle(p *input.DispatchMouseEventParams) {
	p.Button = input.MouseButtonMiddle
}

// ButtonRight is a mouse action option to set the button clicked as the right
// mouse button.
func ButtonRight(p *input.DispatchMouseEventParams) {
	p.Button = input.MouseButtonRight
}

// ButtonNone is a mouse action option to set the button clicked as none (used
// for mouse movements).
func ButtonNone(p *input.DispatchMouseEventParams) {
	p.Button = input.MouseButtonNone
}

// ButtonModifiers is a mouse action option to add input modifiers for a button
// click.
func ButtonModifiers(modifiers ...Modifier) MouseOption {
	return func(p *input.DispatchMouseEventParams) {
		for _, m := range modifiers {
			p.Modifiers |= int64(m)
		}
	}
}

// ClickCount is a mouse action option to set the click count.
func ClickCount(n int) MouseOption {
	return func(p *input.DispatchMouseEventParams) {
		p.ClickCount = int64(n)
	}
}

// KeyEvent is a key action that synthesizes a keyDown, char, and keyUp event
// for each rune in keys, with any key options.
//
// Only well-known, "printable" characters get char events. A key with the
// modifier Ctrl, Alt or Meta gets no char event, because the char event types
// the character on top of the shortcut. For example, Ctrl+A selects the
// text and types no "a". A key with the modifier Shift keeps its char event.
//
// See the [SendKeys] action to synthesize key events for a specific element
// node.
//
// See the [kb] package for implementation details and a list of well-known
// keys.
func KeyEvent(keys string, opts ...KeyOption) Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		for _, r := range keys {
			for _, k := range kb.Encode(r) {
				for _, o := range opts {
					o(k)
				}
				if k.Type == kb.KeyChar && k.Modifiers&^int64(ModifierShift) != 0 {
					continue
				}
				if _, err := cdp.Call(ctx, t, input.DispatchKeyEvent, *k); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// KeyEventNode is a key action that dispatches a key event on an element node.
func KeyEventNode(n *Node, keys string, opts ...KeyOption) Action[Void] {
	return Func(func(ctx context.Context, t *Target) error {
		if _, err := cdp.Call(ctx, t, dom.Focus, dom.FocusParams{NodeID: n.NodeID}); err != nil {
			return err
		}

		_, err := KeyEvent(keys, opts...)(ctx, t)
		return err
	})
}

// KeyOption is a key action option.
type KeyOption = func(*input.DispatchKeyEventParams)

// KeyModifiers is a key action option to add modifiers to the key press.
func KeyModifiers(modifiers ...Modifier) KeyOption {
	return func(p *input.DispatchKeyEventParams) {
		for _, m := range modifiers {
			p.Modifiers |= int64(m)
		}
	}
}
