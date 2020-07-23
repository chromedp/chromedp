package chromedp

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp/kb"
)

// MouseAction are mouse input event actions
type MouseAction Action

// MouseEvent is a mouse event action to dispatch the specified mouse event
// type at coordinates x, y.
func MouseEvent(typ input.MouseType, x, y float64, opts ...MouseOption) MouseAction {
	p := input.DispatchMouseEvent(typ, x, y)
	// apply opts
	for _, o := range opts {
		p = o(p)
	}
	return p
}

// MouseClickXY is an action that sends a left mouse button click (ie,
// mousePressed and mouseReleased event) to the X, Y location.
func MouseClickXY(x, y float64, opts ...MouseOption) MouseAction {
	return ActionFunc(func(ctx context.Context) error {
		p := &input.DispatchMouseEventParams{
			Type:       input.MousePressed,
			X:          x,
			Y:          y,
			Button:     input.Left,
			ClickCount: 1,
		}

		// apply opts
		for _, o := range opts {
			p = o(p)
		}

		if err := p.Do(ctx); err != nil {
			return err
		}

		p.Type = input.MouseReleased
		return p.Do(ctx)
	})
}

// MouseClickNode is an action that dispatches a mouse left button click event
// at the center of a specified node.
//
// Note that the window will be scrolled if the node is not within the window's
// viewport.
func MouseClickNode(n *cdp.Node, opts ...MouseOption) MouseAction {
	return ActionFunc(func(ctx context.Context) error {
		t := cdp.ExecutorFromContext(ctx).(*Target)
		if t == nil {
			return ErrInvalidTarget
		}
		frameID := t.enclosingFrame(n)
		t.frameMu.RLock()
		execCtx := t.execContexts[frameID]
		t.frameMu.RUnlock()

		var pos []float64
		err := evalInCtx(ctx, execCtx, snippet(scrollIntoViewJS, cashX(true), nil, n), &pos)
		if err != nil {
			return err
		}

		boxes, err := dom.GetContentQuads().WithNodeID(n.NodeID).Do(ctx)
		if err != nil {
			return err
		}
		content := boxes[0]

		c := len(content)
		if c%2 != 0 || c < 1 {
			return ErrInvalidDimensions
		}

		var x, y float64
		for i := 0; i < c; i += 2 {
			x += content[i]
			y += content[i+1]
		}
		x /= float64(c / 2)
		y /= float64(c / 2)

		return MouseClickXY(x, y, opts...).Do(ctx)
	})
}

// MouseOption is a mouse action option.
type MouseOption = func(*input.DispatchMouseEventParams) *input.DispatchMouseEventParams

// Button is a mouse action option to set the button to click from a string.
func Button(btn string) MouseOption {
	return ButtonType(input.MouseButton(btn))
}

// ButtonType is a mouse action option to set the button to click.
func ButtonType(button input.MouseButton) MouseOption {
	return func(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
		return p.WithButton(button)
	}
}

// ButtonLeft is a mouse action option to set the button clicked as the left
// mouse button.
func ButtonLeft(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
	return p.WithButton(input.Left)
}

// ButtonMiddle is a mouse action option to set the button clicked as the middle
// mouse button.
func ButtonMiddle(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
	return p.WithButton(input.Middle)
}

// ButtonRight is a mouse action option to set the button clicked as the right
// mouse button.
func ButtonRight(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
	return p.WithButton(input.Right)
}

// ButtonNone is a mouse action option to set the button clicked as none (used
// for mouse movements).
func ButtonNone(p *input.DispatchMouseEventParams) *input.DispatchMouseEventParams {
	return p.WithButton(input.None)
}

// ButtonModifiers is a mouse action option to add additional input modifiers
// for a button click.
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

// KeyAction are keyboard (key) input event actions.
type KeyAction Action

// KeyEvent is a key action that synthesizes a keyDown, char, and keyUp event
// for each rune contained in keys along with any supplied key options.
//
// Only well-known, "printable" characters will have char events synthesized.
//
// See the SendKeys action to synthesize key events for a specific element
// node.
//
// See the chromedp/kb package for implementation details and list of
// well-known keys.
func KeyEvent(keys string, opts ...KeyOption) KeyAction {
	return ActionFunc(func(ctx context.Context) error {
		for _, r := range keys {
			for _, k := range kb.Encode(r) {
				for _, o := range opts {
					o(k)
				}
				if err := k.Do(ctx); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// KeyEventNode is a key action that dispatches a key event on a element node.
func KeyEventNode(n *cdp.Node, keys string, opts ...KeyOption) KeyAction {
	return ActionFunc(func(ctx context.Context) error {
		err := dom.Focus().WithNodeID(n.NodeID).Do(ctx)
		if err != nil {
			return err
		}

		return KeyEvent(keys, opts...).Do(ctx)
	})
}

// KeyOption is a key action option.
type KeyOption = func(*input.DispatchKeyEventParams) *input.DispatchKeyEventParams

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
