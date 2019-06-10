package chromedp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
)

/*

TODO: selector 'by' type, as below:
classname
linktext
name
partiallinktext
tagname

*/

// Selector holds information pertaining to an element query select action.
type Selector struct {
	sel   interface{}
	exp   int
	by    func(context.Context, *cdp.Node) ([]cdp.NodeID, error)
	wait  func(context.Context, *cdp.Frame, ...cdp.NodeID) ([]*cdp.Node, error)
	after func(context.Context, ...*cdp.Node) error
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
		NodeReady(s)
	}

	return s
}

// Do satisfies the Action interface.
func (s *Selector) Do(ctx context.Context) error {
	t := cdp.ExecutorFromContext(ctx).(*Target)
	if t == nil {
		return ErrInvalidTarget
	}
	ch := make(chan error, 1)
	go s.run(ctx, t, ch)
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-ch:
	}
	return err
}

// run runs the selector action, starting over if the original returned nodes
// are invalidated prior to finishing the selector's by, wait, check, and after
// funcs.
func (s *Selector) run(ctx context.Context, t *Target, ch chan error) {
	for {
		select {
		case <-ctx.Done():
			ch <- ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
		t.curMu.RLock()
		cur := t.cur
		t.curMu.RUnlock()

		cur.RLock()
		root := cur.Root
		cur.RUnlock()

		if root == nil {
			// not ready?
			continue
		}

		ids, err := s.by(ctx, root)
		if err != nil || len(ids) < s.exp {
			continue
		}
		nodes, err := s.wait(ctx, cur, ids...)
		// if nodes==nil, we're not yet ready
		if nodes == nil || err != nil {
			continue
		}
		if s.after != nil {
			if err := s.after(ctx, nodes...); err != nil {
				ch <- err
			}
		}
		close(ch)
		break
	}
}

// selAsString forces sel into a string.
func (s *Selector) selAsString() string {
	if sel, ok := s.sel.(string); ok {
		return sel
	}

	return fmt.Sprintf("%s", s.sel)
}

// QueryAfter is an action that will match the specified sel using the supplied
// query options, and after the visibility conditions of the query have been
// met, will execute f.
func QueryAfter(sel interface{}, f func(context.Context, ...*cdp.Node) error, opts ...QueryOption) Action {
	return Query(sel, append(opts, After(f))...)
}

// QueryOption is a element query selector option.
type QueryOption func(*Selector)

// ByFunc is a query option to set the func used to select elements.
func ByFunc(f func(context.Context, *cdp.Node) ([]cdp.NodeID, error)) QueryOption {
	return func(s *Selector) {
		s.by = f
	}
}

// ByQuery is a query option to select a single element using
// DOM.querySelector.
func ByQuery(s *Selector) {
	ByFunc(func(ctx context.Context, n *cdp.Node) ([]cdp.NodeID, error) {
		nodeID, err := dom.QuerySelector(n.NodeID, s.selAsString()).Do(ctx)
		if err != nil {
			return nil, err
		}

		if nodeID == cdp.EmptyNodeID {
			return []cdp.NodeID{}, nil
		}

		return []cdp.NodeID{nodeID}, nil
	})(s)
}

// ByQueryAll is a query option to select elements by DOM.querySelectorAll.
func ByQueryAll(s *Selector) {
	ByFunc(func(ctx context.Context, n *cdp.Node) ([]cdp.NodeID, error) {
		return dom.QuerySelectorAll(n.NodeID, s.selAsString()).Do(ctx)
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
	ByFunc(func(ctx context.Context, n *cdp.Node) ([]cdp.NodeID, error) {
		id, count, err := dom.PerformSearch(s.selAsString()).Do(ctx)
		if err != nil {
			return nil, err
		}

		if count < 1 {
			return []cdp.NodeID{}, nil
		}

		nodes, err := dom.GetSearchResults(id, 0, count).Do(ctx)
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

	ByFunc(func(ctx context.Context, n *cdp.Node) ([]cdp.NodeID, error) {
		for _, id := range ids {
			err := dom.RequestChildNodes(id).WithPierce(true).Do(ctx)
			if err != nil {
				return nil, err
			}
		}

		return ids, nil
	})(s)
}

// waitReady waits for the specified nodes to be ready.
func (s *Selector) waitReady(check func(context.Context, *cdp.Node) error) func(context.Context, *cdp.Frame, ...cdp.NodeID) ([]*cdp.Node, error) {
	errc := make(chan error, 1)
	return func(ctx context.Context, cur *cdp.Frame, ids ...cdp.NodeID) ([]*cdp.Node, error) {
		nodes := make([]*cdp.Node, len(ids))
		cur.RLock()
		for i, id := range ids {
			nodes[i] = cur.Nodes[id]
			if nodes[i] == nil {
				cur.RUnlock()
				// not yet ready
				return nil, nil
			}
		}
		cur.RUnlock()

		if check != nil {
			for _, n := range nodes {
				go func(n *cdp.Node) {
					errc <- check(ctx, n)
				}(n)
			}

			var first error
			for range nodes {
				if err := <-errc; first == nil {
					first = err
				}
			}
			if first != nil {
				return nil, first
			}
		}

		close(errc)
		return nodes, nil
	}
}

// WaitFunc is a query option to set a custom wait func.
func WaitFunc(wait func(context.Context, *cdp.Frame, ...cdp.NodeID) ([]*cdp.Node, error)) QueryOption {
	return func(s *Selector) {
		s.wait = wait
	}
}

// NodeReady is a query option to wait until the element is ready.
func NodeReady(s *Selector) {
	WaitFunc(s.waitReady(nil))(s)
}

// NodeVisible is a query option to wait until the element is visible.
func NodeVisible(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, n *cdp.Node) error {
		// check box model
		_, err := dom.GetBoxModel().WithNodeID(n.NodeID).Do(ctx)
		if err != nil {
			if isCouldNotComputeBoxModelError(err) {
				return ErrNotVisible
			}

			return err
		}

		// check offsetParent
		var res bool
		err = EvaluateAsDevTools(fmt.Sprintf(visibleJS, n.FullXPath()), &res).Do(ctx)
		if err != nil {
			return err
		}
		if !res {
			return ErrNotVisible
		}
		return nil
	}))(s)
}

// NodeNotVisible is a query option to wait until the element is not visible.
func NodeNotVisible(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, n *cdp.Node) error {
		// check box model
		_, err := dom.GetBoxModel().WithNodeID(n.NodeID).Do(ctx)
		if err != nil {
			if isCouldNotComputeBoxModelError(err) {
				return nil
			}

			return err
		}

		// check offsetParent
		var res bool
		err = EvaluateAsDevTools(fmt.Sprintf(visibleJS, n.FullXPath()), &res).Do(ctx)
		if err != nil {
			return err
		}
		if res {
			return ErrVisible
		}
		return nil
	}))(s)
}

// NodeEnabled is a query option to wait until the element is enabled.
func NodeEnabled(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, n *cdp.Node) error {
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

// NodeSelected is a query option to wait until the element is selected.
func NodeSelected(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, n *cdp.Node) error {
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

// NodeNotPresent is a query option to wait until no elements are present
// matching the selector.
func NodeNotPresent(s *Selector) {
	s.exp = 0
	WaitFunc(func(ctx context.Context, cur *cdp.Frame, ids ...cdp.NodeID) ([]*cdp.Node, error) {
		if len(ids) != 0 {
			return nil, ErrHasResults
		}
		return []*cdp.Node{}, nil
	})(s)
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
func After(f func(context.Context, ...*cdp.Node) error) QueryOption {
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
	return Query(sel, append(opts, NodeVisible)...)
}

// WaitNotVisible waits until the selected element is not visible.
func WaitNotVisible(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeNotVisible)...)
}

// WaitEnabled waits until the selected element is enabled (does not have
// attribute 'disabled').
func WaitEnabled(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeEnabled)...)
}

// WaitSelected waits until the element is selected (has attribute 'selected').
func WaitSelected(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeSelected)...)
}

// WaitNotPresent waits until no elements match the specified selector.
func WaitNotPresent(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeNotPresent)...)
}
