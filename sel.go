package chromedp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
)

// Selector holds information pertaining to an element selection query action.
//
// Selectors are constructed using Query. See below for information on building
// an element selector.
type Selector struct {
	sel   interface{}
	exp   int
	by    func(context.Context, *cdp.Node) ([]cdp.NodeID, error)
	wait  func(context.Context, *cdp.Frame, ...cdp.NodeID) ([]*cdp.Node, error)
	after func(context.Context, ...*cdp.Node) error
	raw   bool
}

// Query builds a element selector query action action, that queries for
// specific element nodes matching sel.
//
// Actions that target a browser DOM element node (or nodes) make use of Query,
// in conjunction with the After option (see below) to retrieve data or to
// modify the element(s) selected by the query.
//
// For example:
//
// 	chromedp.Run(ctx, chromedp.SendKeys(`thing`, chromedp.ByID))
//
// In the above will perform a "SendKeys" action on the first element matching a
// browser CSS query for "#thing".
//
// Element selection queries work in conjunction with specific actions and form
// the primary way of automating Tasks in the browser. They are typically
// written in the following form:
//
// 	Action(selector[, parameter1, ...parameterN][,result][, queryOptions...])
//
// Where:
//
// 	Action         - the action to perform
// 	selector       - element query selection (typically a string), that any matching node(s) will have the action applied
// 	parameter[1-N] - parameter(s) needed for the individual action (if any)
// 	result         - pointer to a result (if any)
// 	queryOptions   - changes how queries are executed, or how nodes are waited for (see below)
//
// Query Options
//
// By* options specify the type of element query used By the browser to perform
// the selection query. When not specified, element queries will use BySearch
// (a wrapper for DOM.performSearch).
//
// Node* options specify node conditions that cause the query to wait until the
// specified condition is true. When not specified, queries will use the
// NodeReady wait condition.
//
// The AtLeast option alters the minimum number of nodes that must be returned
// by the element query. If not specified, the default value is 1.
//
// The After option is used to specify a func that will be executed when
// element query has returned one or more elements, and after the node condition is
// true.
//
// By* Options
//
// The BySearch (default) option enables querying for elements with a CSS or
// XPath selector, wrapping DOM.performSearch.
//
// The ByID option enables querying for a single element with the matching CSS
// ID, wrapping DOM.querySelector. ByID is similar to calling
// document.querySelector('#' + ID) from within the browser.
//
// The ByQuery option enables querying for a single element using a CSS
// selector, wrapping DOM.querySelector. ByQuery is similar to calling
// document.querySelector() from within the browser.
//
// The ByQueryAll option enables querying for elements using a CSS selector,
// wrapping DOM.querySelectorAll. ByQueryAll is similar to calling
// document.querySelectorAll() from within the browser.
//
// The ByJSPath option enables querying for a single element using its "JS
// Path" value, wrapping Runtime.evaluate. ByJSPath is similar to executing a
// Javascript snippet that returns a element from within the browser. ByJSPath
// should be used only with trusted element queries, as it is passed directly
// to Runtime.evaluate, and no attempt is made to sanitize the query. Useful
// for querying DOM elements that cannot be retrieved using other By* funcs,
// such as ShadowDOM elements.
//
// Node* Options
//
// The NodeReady (default) option causes the query to wait until all element
// nodes matching the selector have been retrieved from the browser.
//
// The NodeVisible option causes the query to wait until all element nodes
// matching the selector have been retrieved from the browser, and are visible.
//
// The NodeNotVisible option causes the query to wait until all element nodes
// matching the selector have been retrieved from the browser, and are not
// visible.
//
// The NodeEnabled option causes the query to wait until all element nodes
// matching the selector have been retrieved from the browser, and are enabled
// (ie, do not have a 'disabled' attribute).
//
// The NodeSelected option causes the query to wait until all element nodes
// matching the selector have been retrieved from the browser, and are are
// selected (ie, has a 'selected' attribute).
//
// The NodeNotPresent option causes the query to wait until there are no
// element nodes matching the selector.
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

// run executes the selector, restarting if returned nodes are invalidated
// prior to finishing the selector's by, wait, and after funcs.
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

		if cur == nil {
			// the frame hasn't loaded yet.
			continue
		}

		cur.RLock()
		root := cur.Root
		cur.RUnlock()

		if root == nil {
			// not root node yet?
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

// QueryAfter is an element  query action that queries the browser for selector
// sel. Waits until the visibility conditions of the query have been met, after
// which executes f.
func QueryAfter(sel interface{}, f func(context.Context, ...*cdp.Node) error, opts ...QueryOption) Action {
	return Query(sel, append(opts, After(f))...)
}

// QueryOption is an element query action option.
type QueryOption = func(*Selector)

// ByFunc is an element query action option to set the func used to select elements.
func ByFunc(f func(context.Context, *cdp.Node) ([]cdp.NodeID, error)) QueryOption {
	return func(s *Selector) {
		s.by = f
	}
}

// ByQuery is an element query action option to select a single element by the
// DOM.querySelector command.
//
// Similar to calling document.querySelector() in the browser.
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

// ByQueryAll is an element query action option to select elements by the
// DOM.querySelectorAll command.
//
// Similar to calling document.querySelectorAll() in the browser.
func ByQueryAll(s *Selector) {
	ByFunc(func(ctx context.Context, n *cdp.Node) ([]cdp.NodeID, error) {
		return dom.QuerySelectorAll(n.NodeID, s.selAsString()).Do(ctx)
	})(s)
}

// ByID is an element query option to select a single element by its CSS #id.
//
// Similar to calling document.querySelector('#' + ID) in the browser.
func ByID(s *Selector) {
	s.sel = "#" + strings.TrimPrefix(s.selAsString(), "#")
	ByQuery(s)
}

// BySearch is an element query option to select elements by the DOM.performSearch
// command. Works with both CSS and XPath queries.
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

// ByJSPath is an element query option to select elements by the "JS Path"
// value (as shown in the Chrome DevTools UI).
//
// Allows for the direct querying of DOM elements that otherwise cannot be
// retrieved using the other By* funcs, such as ShadowDOM elements.
//
// Note: Do not use with an untrusted selector value, as any defined selector
// will be passed to runtime.Evaluate.
func ByJSPath(s *Selector) {
	s.raw = true
	ByFunc(func(ctx context.Context, n *cdp.Node) ([]cdp.NodeID, error) {
		// set up eval command
		p := runtime.Evaluate(s.selAsString()).
			WithAwaitPromise(true).
			WithObjectGroup("console").
			WithIncludeCommandLineAPI(true)

		// execute
		v, exp, err := p.Do(ctx)
		if err != nil {
			return nil, err
		}
		if exp != nil {
			return nil, exp
		}

		// use the ObjectID from the evaluation to get the nodeID
		nodeID, err := dom.RequestNode(v.ObjectID).Do(ctx)
		if err != nil {
			return nil, err
		}

		if nodeID == cdp.EmptyNodeID {
			return []cdp.NodeID{}, nil
		}

		return []cdp.NodeID{nodeID}, nil
	})(s)
}

// ByNodeID is an element query option to select elements by their node IDs.
//
// Uses DOM.requestChildNodes to retrieve elements with specific node IDs.
//
// Note: must be used with []cdp.NodeID.
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

// WaitFunc is an element query option to set a custom node condition wait.
func WaitFunc(wait func(context.Context, *cdp.Frame, ...cdp.NodeID) ([]*cdp.Node, error)) QueryOption {
	return func(s *Selector) {
		s.wait = wait
	}
}

// NodeReady is an element query option to wait until all queried element nodes
// have been sent by the browser.
func NodeReady(s *Selector) {
	WaitFunc(s.waitReady(nil))(s)
}

// NodeVisible is an element query option to wait until all queried element
// nodes have been sent by the browser and are visible.
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
		err = EvaluateAsDevTools(snippet(visibleJS, cashX(true), s, n), &res).Do(ctx)
		if err != nil {
			return err
		}
		if !res {
			return ErrNotVisible
		}
		return nil
	}))(s)
}

// NodeNotVisible is an element query option to wait until all queried element
// nodes have been sent by the browser and are not visible.
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
		err = EvaluateAsDevTools(snippet(visibleJS, cashX(true), s, n), &res).Do(ctx)
		if err != nil {
			return err
		}
		if res {
			return ErrVisible
		}
		return nil
	}))(s)
}

// NodeEnabled is an element query option to wait until all queried element
// nodes have been sent by the browser and are enabled (ie, do not have a
// 'disabled' attribute).
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

// NodeSelected is an element query option to wait until all queried element
// nodes have been sent by the browser and are selected (ie, has 'selected'
// attribute).
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

// NodeNotPresent is an element query option to wait until no elements are
// present that match the query.
//
// Note: forces the expected number of element nodes to be 0.
func NodeNotPresent(s *Selector) {
	s.exp = 0
	WaitFunc(func(ctx context.Context, cur *cdp.Frame, ids ...cdp.NodeID) ([]*cdp.Node, error) {
		if len(ids) != 0 {
			return nil, ErrHasResults
		}
		return []*cdp.Node{}, nil
	})(s)
}

// AtLeast is an element query option to set a minimum number of elements that
// must be returned by the query.
//
// By default, a query will have a value of 1.
func AtLeast(n int) QueryOption {
	return func(s *Selector) {
		s.exp = n
	}
}

// After is an element query option that sets a func to execute after the
// matched nodes have been returned by the browser, and after the node
// condition is true.
func After(f func(context.Context, ...*cdp.Node) error) QueryOption {
	return func(s *Selector) {
		s.after = f
	}
}

// WaitReady is an element query action that waits until the element matching
// the selector is ready (ie, has been "loaded").
func WaitReady(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, opts...)
}

// WaitVisible is an element query action that waits until the element matching
// the selector is visible.
func WaitVisible(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeVisible)...)
}

// WaitNotVisible is an element query action that waits until the element
// matching the selector is not visible.
func WaitNotVisible(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeNotVisible)...)
}

// WaitEnabled is an element query action that waits until the element matching
// the selector is enabled (ie, does not have attribute 'disabled').
func WaitEnabled(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeEnabled)...)
}

// WaitSelected is an element query action that waits until the element
// matching the selector is selected (ie, has attribute 'selected').
func WaitSelected(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeSelected)...)
}

// WaitNotPresent is an action that waits until no elements are present
// matching the selector.
func WaitNotPresent(sel interface{}, opts ...QueryOption) Action {
	return Query(sel, append(opts, NodeNotPresent)...)
}
