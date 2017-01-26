package chromedp

import (
	"context"
	"time"

	"github.com/knq/chromedp/cdp"
)

// Action is a single atomic action.
type Action interface {
	Do(context.Context, cdp.FrameHandler) error
}

// ActionFunc is a single action func.
type ActionFunc func(context.Context, cdp.FrameHandler) error

// Do executes the action using the provided context.
func (f ActionFunc) Do(ctxt context.Context, h cdp.FrameHandler) error {
	return f(ctxt, h)
}

// Tasks is a list of Actions that can be used as a single Action.
type Tasks []Action

// Do executes the list of Tasks using the provided context.
func (t Tasks) Do(ctxt context.Context, h cdp.FrameHandler) error {
	var err error

	// TODO: put individual task timeouts from context here
	for _, a := range t {
		// ctxt, cancel = context.WithTimeout(ctxt, timeout)
		// defer cancel()
		err = a.Do(ctxt, h)
		if err != nil {
			return err
		}
	}

	return nil
}

// Sleep is an empty action that calls time.Sleep with the specified duration.
func Sleep(d time.Duration) Action {
	return ActionFunc(func(context.Context, cdp.FrameHandler) error {
		time.Sleep(d)
		return nil
	})
}
