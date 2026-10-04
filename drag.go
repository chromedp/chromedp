package chromedp

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
)

const (
	// defaultDragSteps is the number of mouse moves of a drag when the caller
	// gives none.
	defaultDragSteps = 10

	// dragInterceptWait is how long a drag waits after the last mouse move for
	// the browser to report a native drag. The report can arrive after the
	// answer to the mouse move.
	dragInterceptWait = 100 * time.Millisecond

	// dragDraggableWait is how long a drag waits for the report of a native drag
	// when the element at the start point can start one, such as an element
	// with draggable="true". A slow machine needs more than dragInterceptWait.
	// The wait ends as soon as the report arrives.
	dragDraggableWait = 2 * time.Second
)

// DragAndDrop is an element query action that drags the first element node that
// matches from, and drops it on the first element node that matches to. It
// presses the left mouse button at the center of the first node, moves the
// mouse to the center of the second node in several steps, and releases the
// button. See [DragAndDropXY] for the details.
//
// The two selectors can have different types. The query options apply to both
// queries. Both nodes must be in the viewport at the same time, because the
// action does not scroll while it drags. When they do not fit, the action
// returns [ErrDragOutsideViewport] and does not drag.
//
// For example, to move a handle to the end of a slider:
//
//	err := chromedp.Do(ctx, chromedp.DragAndDrop(chromedp.CSS("#handle"), chromedp.CSS("#end")))
func DragAndDrop[S, T Selectable](from S, to T, opts ...QueryOption) Action[Void] {
	return func(ctx context.Context, t *Target) (Void, error) {
		// Scroll each node into view. Scrolling the second node can move the
		// first node, so read the centers only after both scrolls.
		fromNode, err := queryScrolled(from, opts)(ctx, t)
		if err != nil {
			return Void{}, err
		}
		toNode, err := queryScrolled(to, opts)(ctx, t)
		if err != nil {
			return Void{}, err
		}
		fx, fy, err := quadCenter(ctx, t, fromNode)
		if err != nil {
			return Void{}, err
		}
		tx, ty, err := quadCenter(ctx, t, toNode)
		if err != nil {
			return Void{}, err
		}
		if err := checkInViewport(ctx, t, [2]float64{fx, fy}, [2]float64{tx, ty}); err != nil {
			return Void{}, err
		}
		return DragAndDropXY(fx, fy, tx, ty)(ctx, t)
	}
}

// ErrDragOutsideViewport is the error of [DragAndDrop] when the two nodes do
// not fit in the viewport at the same time. The action does not scroll while
// it drags, so a drag between two such nodes does nothing.
const ErrDragOutsideViewport Error = "the nodes of the drag are not in the viewport at the same time"

// checkInViewport returns ErrDragOutsideViewport when a point is outside the
// visual viewport.
func checkInViewport(ctx context.Context, t *Target, points ...[2]float64) error {
	m, err := cdp.Call(ctx, t, page.GetLayoutMetrics, cdp.Empty{})
	if err != nil {
		return err
	}
	vp := m.CSSVisualViewport
	if vp == nil {
		return nil
	}
	for _, p := range points {
		if p[0] < 0 || p[1] < 0 || p[0] > vp.ClientWidth || p[1] > vp.ClientHeight {
			return ErrDragOutsideViewport
		}
	}
	return nil
}

// queryScrolled is an element query action that scrolls the first element node
// that matches the selector into view, and returns the node.
func queryScrolled[S Selectable](sel S, opts []QueryOption) Action[*Node] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (*Node, error) {
		n, err := first(sel, nodes)
		if err != nil {
			return nil, err
		}
		if _, err := cdp.Call(ctx, t, dom.ScrollIntoViewIfNeeded, dom.ScrollIntoViewIfNeededParams{NodeID: n.NodeID}); err != nil {
			return nil, err
		}
		return n, nil
	}, withOpts(opts, NodeVisible)...)
}

// draggableAt reports whether the element at the point can start a native
// drag: an element inside a node with draggable="true", a link or an image. It
// returns false when it cannot tell.
func draggableAt(ctx context.Context, t *Target, x, y float64) bool {
	expr := fmt.Sprintf(`(function(x, y) {
		var e = document.elementFromPoint(x, y);
		return !!(e && e.closest('[draggable="true"], a[href], img'));
	})(%v, %v)`, x, y)
	res, err := cdp.Call(ctx, t, runtime.Evaluate, runtime.EvaluateParams{Expression: expr, ReturnByValue: new(true)})
	if err != nil || res.ExceptionDetails != nil || res.Result == nil {
		return false
	}
	return string(res.Result.Value) == "true"
}

// DragAndDropXY is an action that drags from the point fromX, fromY and drops
// at the point toX, toY. The points are in CSS pixels, relative to the
// viewport. The optional steps value is the number of mouse moves between the
// two points. The default is 10, and a value below 1 means 1.
//
// The action works for two kinds of page. It handles both with the same calls,
// and the page decides which one it is:
//
//   - A page that listens for mouse events, such as a slider, a drag handle or
//     a sortable list, gets a mousePressed event, the mouseMoved events and a
//     mouseReleased event.
//   - A page that uses HTML5 drag and drop, with draggable elements and the
//     events dragstart, dragover and drop, gets the same mouse events up to
//     the start of the drag. Then the browser starts a native drag, which a
//     headless browser does not always finish with mouse events. So the action turns
//     on [input.SetInterceptDrags], reads the data of the drag from the event
//     [input.DragIntercepted], and sends the events dragEnter, dragOver and drop
//     with [input.DispatchDragEvent]. The page sees them as the real events,
//     with a DataTransfer that holds the data of the drag.
//
// The action turns the interception off when it returns. When an error ends the
// drag after the mouse button went down, for example when the context ends, the
// action still releases the button at the last point of the mouse. It uses a new
// context with a limit of 5 seconds for the release, and ignores an error of the
// release. A drag that a page
// starts with a distance that is shorter than the drag threshold of the
// browser, a few pixels, does not start. Use points that are at least 10
// pixels apart.
func DragAndDropXY(fromX, fromY, toX, toY float64, steps ...int) Action[Void] {
	n := defaultDragSteps
	if len(steps) > 0 {
		n = max(steps[0], 1)
	}
	return Func(func(ctx context.Context, t *Target) (err error) {
		if _, err := cdp.Call(ctx, t, input.SetInterceptDrags, input.SetInterceptDragsParams{Enabled: true}); err != nil {
			return err
		}
		defer func() {
			// Turn the interception off, also when the context ended.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if _, off := cdp.Call(ctx, t, input.SetInterceptDrags, input.SetInterceptDragsParams{Enabled: false}); off != nil && err == nil {
				err = off
			}
		}()

		// The subscription holds each event from now on.
		intercepted, cancel := t.Subscribe(input.DragIntercepted.Method)
		defer cancel()

		// Track the button, so that an error between the press and the release
		// does not leave the button down in the browser.
		var pressed bool
		var lastX, lastY float64
		defer func() {
			if !pressed {
				return
			}
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			p := input.DispatchMouseEventParams{Type: MouseReleased, X: lastX, Y: lastY, Button: input.MouseButtonLeft, ClickCount: 1}
			_, _ = cdp.Call(ctx, t, input.DispatchMouseEvent, p)
		}()

		mouse := func(typ input.DispatchMouseEventType, x, y float64) error {
			switch typ {
			case MousePressed:
				pressed = true
			case MouseReleased:
				// Do not send a second release, also when this one fails.
				pressed = false
			}
			lastX, lastY = x, y
			p := input.DispatchMouseEventParams{Type: typ, X: x, Y: y, Button: input.MouseButtonLeft}
			switch typ {
			case MousePressed:
				p.Buttons, p.ClickCount = 1, 1
			case MouseMoved:
				p.Buttons = 1
			case MouseReleased:
				p.ClickCount = 1
			}
			_, err := cdp.Call(ctx, t, input.DispatchMouseEvent, p)
			return err
		}

		// intercept returns the data of a native drag, when the browser
		// reported one within the wait.
		intercept := func(wait time.Duration) (*input.DragData, error) {
			var raw jsontext.Value
			if wait <= 0 {
				select {
				case raw = <-intercepted:
				default:
					return nil, nil
				}
			} else {
				timer := time.NewTimer(wait)
				defer timer.Stop()
				select {
				case raw = <-intercepted:
				case <-timer.C:
					return nil, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			var ev input.EventDragIntercepted
			if err := jsonv2.Unmarshal(raw, &ev, DefaultUnmarshalOptions); err != nil {
				return nil, fmt.Errorf("decoding %s: %w", input.DragIntercepted.Method, err)
			}
			return ev.Data, nil
		}

		// An element that can start a native drag gets a longer wait for the
		// report of the drag. Other elements keep the short wait.
		lastWait := dragInterceptWait
		if draggableAt(ctx, t, fromX, fromY) {
			lastWait = dragDraggableWait
		}

		if err := mouse(MousePressed, fromX, fromY); err != nil {
			return err
		}
		for i := 1; i <= n; i++ {
			x := fromX + (toX-fromX)*float64(i)/float64(n)
			y := fromY + (toY-fromY)*float64(i)/float64(n)
			if err := mouse(MouseMoved, x, y); err != nil {
				return err
			}
			wait := time.Duration(0)
			if i == n {
				wait = lastWait
			}
			data, err := intercept(wait)
			if err != nil {
				return err
			}
			if data != nil {
				return dragNative(ctx, t, data, mouse, [2]float64{x, y}, [2]float64{toX, toY}, n-i)
			}
		}
		return mouse(MouseReleased, toX, toY)
	})
}

// dragNative continues a drag that the browser intercepted. The mouse is at
// the point from, and the drag ends at the point to after the given number of
// steps.
func dragNative(ctx context.Context, t *Target, data *input.DragData, mouse func(input.DispatchMouseEventType, float64, float64) error, from, to [2]float64, steps int) error {
	send := func(typ input.DispatchDragEventType, x, y float64) error {
		_, err := cdp.Call(ctx, t, input.DispatchDragEvent, input.DispatchDragEventParams{Type: typ, X: x, Y: y, Data: data})
		return err
	}
	if err := send(input.DispatchDragEventTypeDragEnter, from[0], from[1]); err != nil {
		return err
	}
	for i := 1; i <= steps; i++ {
		x := from[0] + (to[0]-from[0])*float64(i)/float64(steps)
		y := from[1] + (to[1]-from[1])*float64(i)/float64(steps)
		if err := send(input.DispatchDragEventTypeDragOver, x, y); err != nil {
			return err
		}
	}
	if err := send(input.DispatchDragEventTypeDragOver, to[0], to[1]); err != nil {
		return err
	}
	if err := send(input.DispatchDragEventTypeDrop, to[0], to[1]); err != nil {
		return err
	}
	return mouse(MouseReleased, to[0], to[1])
}
