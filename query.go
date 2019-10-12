package chromedp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
)

// QueryAction are element query actions that select node elements from the
// browser's DOM for retrieval or manipulation.
//
// See Query for details on building element query selectors.
type QueryAction Action

// Selector holds information pertaining to an element selection query.
//
// See Query for information on building an element selector and relevant
// options.
type Selector struct {
	sel   interface{}
	exp   int
	by    func(context.Context, *cdp.Node) ([]cdp.NodeID, error)
	wait  func(context.Context, *cdp.Frame, ...cdp.NodeID) ([]*cdp.Node, error)
	after func(context.Context, ...*cdp.Node) error
	raw   bool
}

// Query is a query action that queries the browser for specific element
// node(s) matching the criteria.
//
// Query actions that target a browser DOM element node (or nodes) make use of
// Query, in conjunction with the After option (see below) to retrieve data or
// to modify the element(s) selected by the query.
//
// For example:
//
//     chromedp.Run(ctx, chromedp.SendKeys(`thing`, chromedp.ByID))
//
// The above will perform a "SendKeys" action on the first element matching a
// browser CSS query for "#thing".
//
// Element selection queries work in conjunction with specific actions and form
// the primary way of automating Tasks in the browser. They are typically
// written in the following form:
//
//     Action(selector[, parameter1, ...parameterN][,result][, queryOptions...])
//
// Where:
//
//     Action         - the action to perform
//     selector       - element query selection (typically a string), that any matching node(s) will have the action applied
//     parameter[1-N] - parameter(s) needed for the individual action (if any)
//     result         - pointer to a result (if any)
//     queryOptions   - changes how queries are executed, or how nodes are waited for (see below)
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
// By Options
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
// Node Options
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
func Query(sel interface{}, opts ...QueryOption) QueryAction {
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

// Do executes the selector, only finishing if the selector's by, wait, and
// after funcs succeed, or if the context is cancelled.
func (s *Selector) Do(ctx context.Context) error {
	t := cdp.ExecutorFromContext(ctx).(*Target)
	if t == nil {
		return ErrInvalidTarget
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
				return err
			}
		}
		return nil
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
			errc := make(chan error, 1)
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
			close(errc)
			if first != nil {
				return nil, first
			}
		}
		return nodes, nil
	}
}

// QueryAfter is an element query action that queries the browser for selector
// sel. Waits until the visibility conditions of the query have been met, after
// which executes f.
func QueryAfter(sel interface{}, f func(context.Context, ...*cdp.Node) error, opts ...QueryOption) QueryAction {
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

		// check visibility
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

		// check visibility
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
func WaitReady(sel interface{}, opts ...QueryOption) QueryAction {
	return Query(sel, opts...)
}

// WaitVisible is an element query action that waits until the element matching
// the selector is visible.
func WaitVisible(sel interface{}, opts ...QueryOption) QueryAction {
	return Query(sel, append(opts, NodeVisible)...)
}

// WaitNotVisible is an element query action that waits until the element
// matching the selector is not visible.
func WaitNotVisible(sel interface{}, opts ...QueryOption) QueryAction {
	return Query(sel, append(opts, NodeNotVisible)...)
}

// WaitEnabled is an element query action that waits until the element matching
// the selector is enabled (ie, does not have attribute 'disabled').
func WaitEnabled(sel interface{}, opts ...QueryOption) QueryAction {
	return Query(sel, append(opts, NodeEnabled)...)
}

// WaitSelected is an element query action that waits until the element
// matching the selector is selected (ie, has attribute 'selected').
func WaitSelected(sel interface{}, opts ...QueryOption) QueryAction {
	return Query(sel, append(opts, NodeSelected)...)
}

// WaitNotPresent is an element query action that waits until no elements are
// present matching the selector.
func WaitNotPresent(sel interface{}, opts ...QueryOption) QueryAction {
	return Query(sel, append(opts, NodeNotPresent)...)
}

// Nodes is an element query action that retrieves the document element nodes
// matching the selector.
func Nodes(sel interface{}, nodes *[]*cdp.Node, opts ...QueryOption) QueryAction {
	if nodes == nil {
		panic("nodes cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, n ...*cdp.Node) error {
		*nodes = n
		return nil
	}, opts...)
}

// NodeIDs is an element query action that retrieves the element node IDs matching the
// selector.
func NodeIDs(sel interface{}, ids *[]cdp.NodeID, opts ...QueryOption) QueryAction {
	if ids == nil {
		panic("nodes cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		nodeIDs := make([]cdp.NodeID, len(nodes))
		for i, n := range nodes {
			nodeIDs[i] = n.NodeID
		}

		*ids = nodeIDs

		return nil
	}, opts...)
}

// Focus is an element query action that focuses the first element node matching the
// selector.
func Focus(sel interface{}, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return dom.Focus().WithNodeID(nodes[0].NodeID).Do(ctx)
	}, opts...)
}

// Blur is an element query action that unfocuses (blurs) the first element node
// matching the selector.
func Blur(sel interface{}, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		var res bool
		err := EvaluateAsDevTools(snippet(blurJS, cashX(true), sel, nodes[0]), &res).Do(ctx)
		if err != nil {
			return err
		}

		if !res {
			return fmt.Errorf("could not blur node %d", nodes[0].NodeID)
		}

		return nil
	}, opts...)
}

// Dimensions is an element query action that retrieves the box model dimensions for the
// first element node matching the selector.
func Dimensions(sel interface{}, model **dom.BoxModel, opts ...QueryOption) QueryAction {
	if model == nil {
		panic("model cannot be nil")
	}
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}
		var err error
		*model, err = dom.GetBoxModel().WithNodeID(nodes[0].NodeID).Do(ctx)
		return err
	}, opts...)
}

// Text is an element query action that retrieves the visible text of the first element
// node matching the selector.
func Text(sel interface{}, text *string, opts ...QueryOption) QueryAction {
	if text == nil {
		panic("text cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return EvaluateAsDevTools(snippet(textJS, cashX(false), sel, nodes[0]), text).Do(ctx)
	}, opts...)
}

// TextContent is an element query action that retrieves the text content of the first element
// node matching the selector.
func TextContent(sel interface{}, text *string, opts ...QueryOption) QueryAction {
	if text == nil {
		panic("text cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return EvaluateAsDevTools(snippet(textContentJS, cashX(false), sel, nodes[0]), text).Do(ctx)
	}, opts...)
}

// Clear is an element query action that clears the values of any input/textarea element
// nodes matching the selector.
func Clear(sel interface{}, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		for _, n := range nodes {
			if n.NodeType != cdp.NodeTypeElement || (n.NodeName != "INPUT" && n.NodeName != "TEXTAREA") {
				return fmt.Errorf("selector %q matched node %d with name %s", sel, n.NodeID, strings.ToLower(n.NodeName))
			}
		}

		errs := make([]error, len(nodes))
		var wg sync.WaitGroup
		for i, n := range nodes {
			wg.Add(1)
			go func(i int, n *cdp.Node) {
				defer wg.Done()

				var a Action
				if n.NodeName == "INPUT" {
					a = dom.SetAttributeValue(n.NodeID, "value", "")
				} else {
					// find textarea's child #text node
					var textID cdp.NodeID
					var found bool
					for _, c := range n.Children {
						if c.NodeType == cdp.NodeTypeText {
							textID = c.NodeID
							found = true
							break
						}
					}

					if !found {
						errs[i] = fmt.Errorf("textarea node %d does not have child #text node", n.NodeID)
						return
					}

					a = dom.SetNodeValue(textID, "")
				}
				errs[i] = a.Do(ctx)
			}(i, n)
		}
		wg.Wait()

		for _, err := range errs {
			if err != nil {
				return err
			}
		}

		return nil
	}, opts...)
}

// Value is an element query action that retrieves the Javascript value field of the
// first element node matching the selector.
//
// Useful for retrieving an element's Javascript value, namely form, input,
// textarea, select, or any other element with a '.value' field.
func Value(sel interface{}, value *string, opts ...QueryOption) QueryAction {
	if value == nil {
		panic("value cannot be nil")
	}

	return JavascriptAttribute(sel, "value", value, opts...)
}

// SetValue is an element query action that sets the Javascript value of the first
// element node matching the selector.
//
// Useful for setting an element's Javascript value, namely form, input,
// textarea, select, or other element with a '.value' field.
func SetValue(sel interface{}, value string, opts ...QueryOption) QueryAction {
	return SetJavascriptAttribute(sel, "value", value, opts...)
}

// Attributes is an element query action that retrieves the element attributes for the
// first element node matching the selector.
func Attributes(sel interface{}, attributes *map[string]string, opts ...QueryOption) QueryAction {
	if attributes == nil {
		panic("attributes cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		nodes[0].RLock()
		defer nodes[0].RUnlock()

		m := make(map[string]string)
		attrs := nodes[0].Attributes
		for i := 0; i < len(attrs); i += 2 {
			m[attrs[i]] = attrs[i+1]
		}

		*attributes = m

		return nil
	}, opts...)
}

// AttributesAll is an element query action that retrieves the element attributes for
// all element nodes matching the selector.
//
// Note: this should be used with the ByQueryAll query option.
func AttributesAll(sel interface{}, attributes *[]map[string]string, opts ...QueryOption) QueryAction {
	if attributes == nil {
		panic("attributes cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		for _, node := range nodes {
			node.RLock()
			m := make(map[string]string)
			attrs := node.Attributes
			for i := 0; i < len(attrs); i += 2 {
				m[attrs[i]] = attrs[i+1]
			}
			*attributes = append(*attributes, m)
			node.RUnlock()
		}
		return nil
	}, opts...)
}

// SetAttributes is an element query action that sets the element attributes for the
// first element node matching the selector.
func SetAttributes(sel interface{}, attributes map[string]string, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return errors.New("expected at least one element")
		}

		i, attrs := 0, make([]string, len(attributes))
		for k, v := range attributes {
			attrs[i] = fmt.Sprintf(`%s=%s`, k, strconv.Quote(v))
			i++
		}

		return dom.SetAttributesAsText(nodes[0].NodeID, strings.Join(attrs, " ")).Do(ctx)
	}, opts...)
}

// AttributeValue is an element query action that retrieves the element attribute value
// for the first element node matching the selector.
func AttributeValue(sel interface{}, name string, value *string, ok *bool, opts ...QueryOption) QueryAction {
	if value == nil {
		panic("value cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return errors.New("expected at least one element")
		}

		nodes[0].RLock()
		defer nodes[0].RUnlock()

		attrs := nodes[0].Attributes
		for i := 0; i < len(attrs); i += 2 {
			if attrs[i] == name {
				*value = attrs[i+1]
				if ok != nil {
					*ok = true
				}
				return nil
			}
		}

		if ok != nil {
			*ok = false
		}

		return nil
	}, opts...)
}

// SetAttributeValue is an element query action that sets the element attribute with
// name to value for the first element node matching the selector.
func SetAttributeValue(sel interface{}, name, value string, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return dom.SetAttributeValue(nodes[0].NodeID, name, value).Do(ctx)
	}, opts...)
}

// RemoveAttribute is an element query action that removes the element attribute with
// name from the first element node matching the selector.
func RemoveAttribute(sel interface{}, name string, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return dom.RemoveAttribute(nodes[0].NodeID, name).Do(ctx)
	}, opts...)
}

// JavascriptAttribute is an element query action that retrieves the Javascript
// attribute for the first element node matching the selector.
func JavascriptAttribute(sel interface{}, name string, res interface{}, opts ...QueryOption) QueryAction {
	if res == nil {
		panic("res cannot be nil")
	}
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		if err := EvaluateAsDevTools(
			snippet(attributeJS, cashX(true), sel, nodes[0], name), res,
		).Do(ctx); err != nil {
			return fmt.Errorf("could not retrieve attribute %q: %v", name, err)
		}
		return nil
	}, opts...)
}

// SetJavascriptAttribute is an element query action that sets the Javascript attribute
// for the first element node matching the selector.
func SetJavascriptAttribute(sel interface{}, name, value string, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		var res string
		err := EvaluateAsDevTools(snippet(setAttributeJS, cashX(true), sel, nodes[0], name, value), &res).Do(ctx)
		if err != nil {
			return err
		}
		if res != value {
			return fmt.Errorf("could not set value on node %d", nodes[0].NodeID)
		}

		return nil
	}, opts...)
}

// OuterHTML is an element query action that retrieves the outer html of the first
// element node matching the selector.
func OuterHTML(sel interface{}, html *string, opts ...QueryOption) QueryAction {
	if html == nil {
		panic("html cannot be nil")
	}
	return JavascriptAttribute(sel, "outerHTML", html, opts...)
}

// InnerHTML is an element query action that retrieves the inner html of the first
// element node matching the selector.
func InnerHTML(sel interface{}, html *string, opts ...QueryOption) QueryAction {
	if html == nil {
		panic("html cannot be nil")
	}
	return JavascriptAttribute(sel, "innerHTML", html, opts...)
}

// Click is an element query action that sends a mouse click event to the first element
// node matching the selector.
func Click(sel interface{}, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return MouseClickNode(nodes[0]).Do(ctx)
	}, append(opts, NodeVisible)...)
}

// DoubleClick is an element query action that sends a mouse double click event to the
// first element node matching the selector.
func DoubleClick(sel interface{}, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return MouseClickNode(nodes[0], ClickCount(2)).Do(ctx)
	}, append(opts, NodeVisible)...)
}

// SendKeys is an element query action that synthesizes the key up, char, and down
// events as needed for the runes in v, sending them to the first element node
// matching the selector.
//
// For a complete example on how to use SendKeys, see
// https://github.com/chromedp/examples/tree/master/keys.
//
// Note: when the element query matches a input[type="file"] node, then
// dom.SetFileInputFiles is used to set the upload path of the input node to v.
func SendKeys(sel interface{}, v string, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		n := nodes[0]

		// grab type attribute from node
		typ, attrs := "", n.Attributes
		n.RLock()
		for i := 0; i < len(attrs); i += 2 {
			if attrs[i] == "type" {
				typ = attrs[i+1]
			}
		}
		n.RUnlock()

		// when working with input[type="file"], call dom.SetFileInputFiles
		if n.NodeName == "INPUT" && typ == "file" {
			return dom.SetFileInputFiles([]string{v}).WithNodeID(n.NodeID).Do(ctx)
		}

		return KeyEventNode(n, v).Do(ctx)
	}, append(opts, NodeVisible)...)
}

// SetUploadFiles is an element query action that sets the files to upload (ie, for a
// input[type="file"] node) for the first element node matching the selector.
func SetUploadFiles(sel interface{}, files []string, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return dom.SetFileInputFiles(files).WithNodeID(nodes[0].NodeID).Do(ctx)
	}, opts...)
}

// Screenshot is an element query action that takes a screenshot of the first element
// node matching the selector.
//
// See CaptureScreenshot for capturing a screenshot of the browser viewport.
//
// See the 'screenshot' example in the https://github.com/chromedp/examples
// project for an example of taking a screenshot of the entire page.
func Screenshot(sel interface{}, picbuf *[]byte, opts ...QueryOption) QueryAction {
	if picbuf == nil {
		panic("picbuf cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		// get box model
		box, err := dom.GetBoxModel().WithNodeID(nodes[0].NodeID).Do(ctx)
		if err != nil {
			return err
		}
		if len(box.Margin) != 8 {
			return ErrInvalidBoxModel
		}

		// take screenshot of the box
		buf, err := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatPng).
			WithClip(&page.Viewport{
				// Round the dimensions, as otherwise we might
				// lose one pixel in either dimension.
				X:      math.Round(box.Margin[0]),
				Y:      math.Round(box.Margin[1]),
				Width:  math.Round(box.Margin[4] - box.Margin[0]),
				Height: math.Round(box.Margin[5] - box.Margin[1]),
				// This seems to be necessary? Seems to do the
				// right thing regardless of DPI.
				Scale: 1.0,
			}).Do(ctx)
		if err != nil {
			return err
		}

		*picbuf = buf
		return nil
	}, append(opts, NodeVisible)...)
}

// Submit is an element query action that submits the parent form of the first element
// node matching the selector.
func Submit(sel interface{}, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		var res bool
		err := EvaluateAsDevTools(snippet(submitJS, cashX(true), sel, nodes[0]), &res).Do(ctx)
		if err != nil {
			return err
		}

		if !res {
			return fmt.Errorf("could not call submit on node %d", nodes[0].NodeID)
		}

		return nil
	}, opts...)
}

// Reset is an element query action that resets the parent form of the first element
// node matching the selector.
func Reset(sel interface{}, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		var res bool
		err := EvaluateAsDevTools(snippet(resetJS, cashX(true), sel, nodes[0]), &res).Do(ctx)
		if err != nil {
			return err
		}

		if !res {
			return fmt.Errorf("could not call reset on node %d", nodes[0].NodeID)
		}

		return nil
	}, opts...)
}

// ComputedStyle is an element query action that retrieves the computed style of the
// first element node matching the selector.
func ComputedStyle(sel interface{}, style *[]*css.ComputedProperty, opts ...QueryOption) QueryAction {
	if style == nil {
		panic("style cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		computed, err := css.GetComputedStyleForNode(nodes[0].NodeID).Do(ctx)
		if err != nil {
			return err
		}

		*style = computed

		return nil
	}, opts...)
}

// MatchedStyle is an element query action that retrieves the matched style information
// for the first element node matching the selector.
func MatchedStyle(sel interface{}, style **css.GetMatchedStylesForNodeReturns, opts ...QueryOption) QueryAction {
	if style == nil {
		panic("style cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		var err error
		ret := &css.GetMatchedStylesForNodeReturns{}
		ret.InlineStyle, ret.AttributesStyle, ret.MatchedCSSRules,
			ret.PseudoElements, ret.Inherited, ret.CSSKeyframesRules,
			err = css.GetMatchedStylesForNode(nodes[0].NodeID).Do(ctx)
		if err != nil {
			return err
		}

		*style = ret

		return nil
	}, opts...)
}

// ScrollIntoView is an element query action that scrolls the window to the
// first element node matching the selector.
func ScrollIntoView(sel interface{}, opts ...QueryOption) QueryAction {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		var pos []int
		err := EvaluateAsDevTools(snippet(scrollIntoViewJS, cashX(true), sel, nodes[0]), &pos).Do(ctx)
		if err != nil {
			return err
		}

		if pos == nil {
			return fmt.Errorf("could not scroll into node %d", nodes[0].NodeID)
		}

		return nil
	}, opts...)
}
