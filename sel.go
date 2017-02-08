package chromedp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/css"
	"github.com/knq/chromedp/cdp/dom"
)

/*

TODO: selector 'by' type, as below:
classname
linktext
name
partiallinktext
tagname

*/

// Error types.
var (
	ErrNoResults   = errors.New("no results")
	ErrNotVisible  = errors.New("not visible")
	ErrVisible     = errors.New("visible")
	ErrDisabled    = errors.New("disabled")
	ErrNotSelected = errors.New("not selected")
)

// Selector holds information pertaining to an element query select action.
type Selector struct {
	sel   interface{}
	exp   int
	by    func(context.Context, cdp.FrameHandler, *cdp.Node) ([]cdp.NodeID, error)
	wait  func(context.Context, cdp.FrameHandler, *cdp.Node, ...cdp.NodeID) ([]*cdp.Node, error)
	after func(context.Context, cdp.FrameHandler, ...*cdp.Node) error
}

// Query is an action to query for document nodes match the specified sel and
// the supplied query options.
func Query(sel interface{}, opts ...QueryOption) Action {
	s := &Selector{
		sel: sel,
		exp: 1,
	}

	// apply options
	for _, o := range opts {
		o(s)
	}

	if s.by == nil {
		BySearch(s)
	}

	if s.wait == nil {
		ElementReady(s)
	}

	return s
}

// Do satisfies the Action interface.
func (s *Selector) Do(ctxt context.Context, h cdp.FrameHandler) error {
	// TODO: fix this
	ctxt, cancel := context.WithTimeout(ctxt, 100*time.Second)
	defer cancel()

	var err error
	select {
	case err = <-s.run(ctxt, h):
	case <-ctxt.Done():
		err = ctxt.Err()
	}

	return err
}

// run runs the selector action, starting over if the original returned nodes
// are invalidated prior to finishing the selector's by, wait, check, and after
// funcs.
func (s *Selector) run(ctxt context.Context, h cdp.FrameHandler) chan error {
	ch := make(chan error)

	go func() {
		defer close(ch)

		for {
			root, err := h.GetRoot(ctxt)
			if err != nil {
				continue
			}

			select {
			default:
				ids, err := s.by(ctxt, h, root)
				if err == nil && len(ids) >= s.exp {
					nodes, err := s.wait(ctxt, h, root, ids...)
					if err == nil {
						if s.after == nil {
							return
						}

						err = s.after(ctxt, h, nodes...)
						if err != nil {
							ch <- err
						}
						return
					}
				}

				time.Sleep(DefaultCheckDuration)

			case <-root.Invalidated:
				continue

			case <-ctxt.Done():
				ch <- ctxt.Err()
				return
			}
		}
	}()

	return ch
}

// selAsString forces sel into a string.
func (s *Selector) selAsString() string {
	if sel, ok := s.sel.(string); ok {
		return sel
	}

	return fmt.Sprintf("%s", s.sel)
}

// selAsInt forces sel into a int.
func (s *Selector) selAsInt() int {
	sel, ok := s.sel.(int)
	if !ok {
		panic("selector must be int")
	}

	return sel
}

// QueryAfter is an action that will match the specified sel using the supplied
// query options, and after the visibility conditions of the query have been
// met, will execute f.
func QueryAfter(sel interface{}, f func(context.Context, cdp.FrameHandler, ...*cdp.Node) error, opts ...QueryOption) Action {
	return Query(sel, append(opts, After(f))...)
}

// QueryOption is a element query selector option.
type QueryOption func(*Selector)

// ByFunc is a query option to set the func used to select elements.
func ByFunc(f func(context.Context, cdp.FrameHandler, *cdp.Node) ([]cdp.NodeID, error)) QueryOption {
	return func(s *Selector) {
		s.by = f
	}
}

// ByQuery is a query option to select a single element using
// DOM.querySelector.
func ByQuery(s *Selector) {
	ByFunc(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) ([]cdp.NodeID, error) {
		nodeID, err := dom.QuerySelector(n.NodeID, s.selAsString()).Do(ctxt, h)
		if err != nil {
			return nil, err
		}

		if nodeID == emptyNodeID {
			return []cdp.NodeID{}, nil
		}

		return []cdp.NodeID{nodeID}, nil
	})(s)
}

// ByQueryAll is a query option to select elements by DOM.querySelectorAll.
func ByQueryAll(s *Selector) {
	ByFunc(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) ([]cdp.NodeID, error) {
		return dom.QuerySelectorAll(n.NodeID, s.selAsString()).Do(ctxt, h)
	})(s)
}

// ByID is a query option to select a single element by their CSS #id.
func ByID(s *Selector) {
	s.sel = "#" + strings.TrimPrefix(s.selAsString(), "#")
	ByQuery(s)
}

// BySearch is a query option via DOM.performSearch (works with both CSS and
// XPath queries).
func BySearch(s *Selector) {
	ByFunc(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) ([]cdp.NodeID, error) {
		id, count, err := dom.PerformSearch(s.selAsString()).Do(ctxt, h)
		if err != nil {
			return nil, err
		}

		if count < 1 {
			return []cdp.NodeID{}, nil
		}

		nodes, err := dom.GetSearchResults(id, 0, count).Do(ctxt, h)
		if err != nil {
			return nil, err
		}

		return nodes, nil
	})(s)
}

// ByNodeID is a query option to select elements by their NodeIDs.
func ByNodeID(s *Selector) {
	ids, ok := s.sel.([]cdp.NodeID)
	if !ok {
		panic("ByNodeID can only work on []cdp.NodeID")
	}

	ByFunc(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) ([]cdp.NodeID, error) {
		var err error
		for _, id := range ids {
			err = dom.RequestChildNodes(id).WithPierce(true).Do(ctxt, h)
			if err != nil {
				return nil, err
			}
		}

		return ids, nil
	})(s)
}

// waitReady waits for the specified nodes to be ready.
func (s *Selector) waitReady(check func(context.Context, cdp.FrameHandler, *cdp.Node) error) func(context.Context, cdp.FrameHandler, *cdp.Node, ...cdp.NodeID) ([]*cdp.Node, error) {
	return func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node, ids ...cdp.NodeID) ([]*cdp.Node, error) {
		f, err := h.WaitFrame(ctxt, emptyFrameID)
		if err != nil {
			return nil, err
		}

		wg := new(sync.WaitGroup)
		nodes := make([]*cdp.Node, len(ids))
		errs := make([]error, len(ids))
		for i, id := range ids {
			wg.Add(1)
			go func(i int, id cdp.NodeID) {
				defer wg.Done()
				nodes[i], errs[i] = h.WaitNode(ctxt, f, id)
			}(i, id)
		}
		wg.Wait()

		for _, err := range errs {
			if err != nil {
				return nil, err
			}
		}

		if check != nil {
			errs := make([]error, len(nodes))
			for i, n := range nodes {
				wg.Add(1)
				go func(i int, n *cdp.Node) {
					defer wg.Done()
					errs[i] = check(ctxt, h, n)
				}(i, n)
			}
			wg.Wait()

			for _, err := range errs {
				if err != nil {
					return nil, err
				}
			}
		}

		return nodes, nil
	}
}

// WaitFunc is a query option to set a custom wait func.
func WaitFunc(wait func(context.Context, cdp.FrameHandler, *cdp.Node, ...cdp.NodeID) ([]*cdp.Node, error)) QueryOption {
	return func(s *Selector) {
		s.wait = wait
	}
}

// ElementReady is a query option to wait until the element is ready.
func ElementReady(s *Selector) {
	WaitFunc(s.waitReady(nil))(s)
}

// ElementVisible is a query option to wait until the element is visible.
func ElementVisible(s *Selector) {
	WaitFunc(s.waitReady(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) error {
		var err error

		// check box model
		_, err = dom.GetBoxModel(n.NodeID).Do(ctxt, h)
		if err != nil {
			if isCouldNotComputeBoxModelError(err) {
				return ErrNotVisible
			}

			return err
		}

		// check offsetParent
		var res bool
		err = EvaluateAsDevTools(fmt.Sprintf(visibleJS, n.FullXPath()), &res).Do(ctxt, h)
		if err != nil {
			return err
		}
		if !res {
			return ErrNotVisible
		}
		return nil
	}))(s)
}

// ElementNotVisible is a query option to wait until the element is not visible.
func ElementNotVisible(s *Selector) {
	WaitFunc(s.waitReady(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) error {
		var err error

		// check box model
		_, err = dom.GetBoxModel(n.NodeID).Do(ctxt, h)
		if err != nil {
			if isCouldNotComputeBoxModelError(err) {
				return nil
			}

			return err
		}

		// check offsetParent
		var res bool
		err = EvaluateAsDevTools(fmt.Sprintf(visibleJS, n.FullXPath()), &res).Do(ctxt, h)
		if err != nil {
			return err
		}
		if res {
			return ErrVisible
		}
		return nil
	}))(s)
}

// ElementVisibleOld is a query option to wait until the element is visible.
//
// This is the old, complicated, implementation (deprecated).
func ElementVisibleOld(s *Selector) {
	WaitFunc(s.waitReady(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) error {
		var err error

		// check node has box model
		_, err = dom.GetBoxModel(n.NodeID).Do(ctxt, h)
		if err != nil {
			return err
		}

		// check if any of the parents are not visible ...
		var hidden bool
		for ; n.Parent != nil; n = n.Parent {
			// get style
			style, err := css.GetComputedStyleForNode(n.NodeID).Do(ctxt, h)
			if err != nil {
				return err
			}

			// check if hidden
			for _, c := range style {
				switch c.Name {
				case "display":
					//log.Printf("%d >>>> %s=%s", n.NodeID, c.Name, c.Value)
					hidden = c.Value == "none"

				case "visibility":
					//log.Printf("%d >>>> %s=%s", n.NodeID, c.Name, c.Value)
					hidden = c.Value != "visible"

				case "hidden":
					//log.Printf("%d >>>> %s=%s", n.NodeID, c.Name, c.Value)
					hidden = true
				}

				if hidden {
					return ErrNotVisible
				}
			}
		}

		return nil
	}))(s)
}

// ElementNotVisibleOld is a query option to wait until the element is not
// visible.
//
// This is the old, complicated, implementation (deprecated).
func ElementNotVisibleOld(s *Selector) {
	WaitFunc(s.waitReady(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) error {
		var err error

		// check node has box model
		_, err = dom.GetBoxModel(n.NodeID).Do(ctxt, h)
		if err != nil {
			return nil
		}

		// check if any of the parents are not visible ...
		var hidden bool
		for ; n.Parent != nil; n = n.Parent {
			// get style
			style, err := css.GetComputedStyleForNode(n.NodeID).Do(ctxt, h)
			if err != nil {
				return err
			}

			// check if hidden
			for _, c := range style {
				switch c.Name {
				case "display":
					//log.Printf("%d >>>> %s=%s", n.NodeID, c.Name, c.Value)
					hidden = c.Value == "none"

				case "visibility":
					//log.Printf("%d >>>> %s=%s", n.NodeID, c.Name, c.Value)
					hidden = c.Value != "visible"

				case "hidden":
					//log.Printf("%d >>>> %s=%s", n.NodeID, c.Name, c.Value)
					hidden = true
				}

				if hidden {
					return nil
				}
			}
		}

		return ErrVisible
	}))(s)
}

// ElementEnabled is a query option to wait until the element is enabled.
func ElementEnabled(s *Selector) {
	WaitFunc(s.waitReady(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) error {
		n.RLock()
		defer n.RUnlock()

		for i := 0; i < len(n.Attributes); i += 2 {
			if n.Attributes[i] == "disabled" {
				return ErrDisabled
			}
		}

		return nil
	}))(s)
}

// ElementSelected is a query option to wait until the element is selected.
func ElementSelected(s *Selector) {
	WaitFunc(s.waitReady(func(ctxt context.Context, h cdp.FrameHandler, n *cdp.Node) error {
		n.RLock()
		defer n.RUnlock()

		for i := 0; i < len(n.Attributes); i += 2 {
			if n.Attributes[i] == "selected" {
				return nil
			}
		}

		return ErrNotSelected
	}))(s)
}

// AtLeast is a query option to wait until at least n elements are returned
// from the query selector.
func AtLeast(n int) QueryOption {
	return func(s *Selector) {
		s.exp = n
	}
}

// After is a query option to set a func that will be executed after the wait
// has succeeded.
func After(f func(context.Context, cdp.FrameHandler, ...*cdp.Node) error) QueryOption {
	return func(s *Selector) {
		s.after = f
	}
}

// WaitReady waits until the element is ready (ie, loaded by chromedp).
func WaitReady(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, opts...)
}

// WaitVisible waits until the selected element is visible.
func WaitVisible(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, ElementVisible)...)
}

// WaitNotVisible waits until the selected element is not visible.
func WaitNotVisible(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, ElementNotVisible)...)
}

// WaitEnabled waits until the selected element is enabled (does not have
// attribute 'disabled').
func WaitEnabled(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, ElementEnabled)...)
}

// WaitSelected waits until the element is selected (has attribute 'selected').
func WaitSelected(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, ElementSelected)...)
}

const (
	// visibleJS is a javascript snippet that returns true or false depending
	// on if the specified node's offsetParent is not null.
	visibleJS = `(function(a) {
		return a[0].offsetParent !== null
	})($x('%s'))`
)
