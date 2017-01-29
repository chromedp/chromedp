package chromedp

import (
	"context"
	"errors"
	"time"

	"github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/dom"
	"github.com/knq/chromedp/cdp/input"
	"github.com/knq/chromedp/kb"
)

// Error types.
var (
	ErrInvalidDimensions = errors.New("invalid box dimensions")
)

// MouseAction is a mouse action.
func MouseAction(typ input.MouseType, x, y int64, opts ...MouseOption) Action {
	f := input.DispatchMouseEvent(typ, x, y)
	for _, o := range opts {
		f = o(f)
	}

	return f
}

// MouseClickXY sends a left mouse button click at the X, Y location.
func MouseClickXY(x, y int64, opts ...MouseOption) Action {
	return Tasks{
		MouseAction(input.MousePressed, x, y, append(opts, Button(input.ButtonLeft), ClickCount(1))...),
		MouseAction(input.MouseReleased, x, y, append(opts, Button(input.ButtonLeft), ClickCount(1))...),
	}
}

// MouseActionNode dispatches a mouse event at the center of a specified node.
func MouseActionNode(n *cdp.Node, opts ...MouseOption) Action {
	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		box, err := dom.GetBoxModel(n.NodeID).Do(ctxt, h)
		if err != nil {
			return err
		}

		c := len(box.Content)
		if c%2 != 0 {
			return ErrInvalidDimensions
		}

		var x, y int64
		for i := 0; i < c; i += 2 {
			x += int64(box.Content[i])
			y += int64(box.Content[i+1])
		}

		return MouseClickXY(x/int64(c/2), y/int64(c/2), opts...).Do(ctxt, h)
	})
}

// MouseOption is a mouse action option.
type MouseOption func(*input.DispatchMouseEventParams) *input.DispatchMouseEventParams

// Button is a mouse action option to set the button to click.
func Button(button input.ButtonType) MouseOption {
	return func(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
		return p.WithButton(button)
	}
}

// ButtonString is a mouse action option to set the button to click as a
// string.
func ButtonString(btn string) MouseOption {
	return Button(input.ButtonType(btn))
}

// ButtonModifiers is a mouse action option to add additional modifiers for the
// button.
func ButtonModifiers(modifiers ...input.Modifier) MouseOption {
	return func(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
		for _, m := range modifiers {
			p.Modifiers |= m
		}
		return p
	}
}

// ClickCount is a mouse action option to set the click count.
func ClickCount(n int) MouseOption {
	return func(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
		return p.WithClickCount(int64(n))
	}
}

// KeyAction will synthesize a keyDown, char, and keyUp event for each rune
// contained in keys along with any supplied key options. Well known KeyCode
// runes will not synthesize the char event.
//
// Note: KeyCodeCR and KeyCodeLF are exceptions to the above, and a char of
// KeyCodeCR ('\r') will be synthesized for both.
func KeyAction(keys string, opts ...KeyOption) Action {
	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		var err error

		for _, r := range keys {
			for _, k := range kb.Encode(r) {
				err = k.Do(ctxt, h)
				if err != nil {
					return err
				}
			}

			// TODO: move to context
			time.Sleep(5 * time.Millisecond)
		}

		return nil
	})
}

// KeyActionNode dispatches a key event on a node.
func KeyActionNode(n *cdp.Node, keys string, opts ...KeyOption) Action {
	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		err := dom.Focus(n.NodeID).Do(ctxt, h)
		if err != nil {
			return err
		}

		return KeyAction(keys, opts...).Do(ctxt, h)
	})
}

// KeyOption is a key action option.
type KeyOption func(*input.DispatchKeyEventParams) *input.DispatchKeyEventParams

// KeyModifiers is a key action option to add additional modifiers on the key
// press.
func KeyModifiers(modifiers ...input.Modifier) KeyOption {
	return func(p *input.DispatchKeyEventParams) *input.DispatchKeyEventParams {
		for _, m := range modifiers {
			p.Modifiers |= m
		}
		return p
	}
}
