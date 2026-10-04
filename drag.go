package chromedp

import (
	"context"
	"fmt"
	"time"

	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
)

const (
	// defaultDragSteps is the number of mouse moves of a drag when the caller
	// gives none.
	defaultDragSteps = 10

	// dragInterceptWait is how long a drag waits after the last mouse move for
	// the browser to report a native drag. The report can arrive after the
	// answer to the mouse move.
	dragInterceptWait = 100 * time.Millisecond
)

// DragAndDrop is an element query action that drags the first element node that
// matches from, and drops it on the first element node that matches to. It
// presses the left mouse button at the center of the first node, moves the
// mouse to the center of the second node in several steps, and releases the
// button. See [DragAndDropXY] for the details.
//
// The two selectors can have different types. The query options apply to both
// queries. Both nodes must be in the viewport at the same time, because the
// action scrolls each node into view only once and does not scroll while it
// drags.
//
// For example, to move a handle to the end of a slider:
//
//	err := chromedp.Do(ctx, chromedp.DragAndDrop(chromedp.CSS("#handle"), chromedp.CSS("#end")))
func DragAndDrop[S, T Selectable](from S, to T, opts ...QueryOption) Action[Void] {
	return func(ctx context.Context, t *Target) (Void, error) {
		fx, fy, err := queryCenter(from, opts)(ctx, t)
		if err != nil {
			return Void{}, err
		}
		tx, ty, err := queryCenter(to, opts)(ctx, t)
		if err != nil {
			return Void{}, err
		}
		return DragAndDropXY(fx, fy, tx, ty)(ctx, t)
	}
}

// center is a point in the viewport.
type center struct{ x, y float64 }

// queryCenter is an element query action that scrolls the first element node
// that matches the selector into view, and returns its center.
func queryCenter[S Selectable](sel S, opts []QueryOption) func(context.Context, *Target) (x, y float64, err error) {
	a := QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (center, error) {
		n, err := first(sel, nodes)
		if err != nil {
			return center{}, err
		}
		x, y, err := nodeCenter(ctx, t, n)
		return center{x, y}, err
	}, withOpts(opts, NodeVisible)...)
	return func(ctx context.Context, t *Target) (float64, float64, error) {
		c, err := a(ctx, t)
		return c.x, c.y, err
	}
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
// The action turns the interception off when it returns. A drag that a page
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

		mouse := func(typ input.DispatchMouseEventType, x, y float64) error {
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
				wait = dragInterceptWait
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
