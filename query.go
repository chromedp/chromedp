package chromedp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
)

// Selector holds the data of an element selection query.
//
// See [Query] for how to build an element selector and its options.
type Selector struct {
	sel           any
	fromNode      *Node
	retryInterval time.Duration
	exp           int
	by            func(context.Context, *Target, *Node) ([]cdp.NodeID, error)
	wait          func(context.Context, *Target, *Frame, runtime.ExecutionContextID, ...cdp.NodeID) ([]*Node, error)
	after         []func(context.Context, *Target, []*Node) error
}

// Query is an action that queries the browser for element nodes that match the
// criteria, and waits until they meet the node conditions. It returns no
// value. To get the nodes, see [Nodes].
//
// Query actions that target element nodes share one selector. Use the [After]
// option or [QueryAfter] to read data from the selected elements or to change
// them.
//
// For example:
//
//	chromedp.Do(ctx, chromedp.SendKeys("thing", "hello", chromedp.ByID))
//
// This runs a [SendKeys] action on the first element that matches the CSS
// query "#thing".
//
// Element selection queries work with specific actions. They are the main way
// to automate steps in the browser. They have this form:
//
//	Action(selector[, parameter1, ...parameterN][, queryOptions...])
//
// Where:
//
//   - Action - the action to run
//   - selector - the element query (typically a string). The action applies to every node that matches it.
//   - parameter[1-N] - the parameters that the action needs (if any)
//   - queryOptions - change how the query runs, or how it waits for nodes
//
// An action that reads a value returns it. For example, [Text] returns the
// text as a string.
//
// # Query Options
//
// The By* options choose the type of element query that the browser runs.
// Without a By* option, the query uses [BySearch] (a wrapper for
// DOM.performSearch).
//
// The Node* options set node conditions. The query waits until the condition
// is true. Without a Node* option, the query uses the [NodeReady] condition.
//
// The [AtLeast] option sets the minimum number of nodes that the query must
// return. The default is 1.
//
// The [After] option sets a func that runs after the query returns one or more
// elements and the node condition is true.
//
// # By Options
//
// The [BySearch] option (the default) queries elements by plain text, CSS
// selector, or XPath query. It wraps DOM.performSearch.
//
// The [ByID] option queries a single element by its CSS ID. It wraps
// DOM.querySelector. ByID is like document.querySelector('#' + ID) in the
// browser.
//
// The [ByQuery] option queries a single element with a CSS selector. It wraps
// DOM.querySelector. ByQuery is like document.querySelector() in the browser.
//
// The [ByQueryAll] option queries elements with a CSS selector. It wraps
// DOM.querySelectorAll. ByQueryAll is like document.querySelectorAll() in the
// browser.
//
// The [ByJSPath] option queries a single element by its "JS Path" value. It
// wraps Runtime.evaluate. ByJSPath is like a JavaScript snippet that returns
// an element in the browser. Use it only with trusted element queries.
// chromedp passes the query directly to Runtime.evaluate and does not
// sanitize it. The option is useful for DOM elements that the other By* funcs
// cannot retrieve, such as ShadowDOM elements.
//
// # Node Options
//
// The [NodeReady] option (the default) makes the query wait until the browser
// has returned all element nodes that match the selector.
//
// The [NodeVisible] option makes the query wait until the browser has returned
// all element nodes that match the selector, and they are visible.
//
// The [NodeNotVisible] option makes the query wait until the browser has
// returned all element nodes that match the selector, and they are not
// visible.
//
// The [NodeEnabled] option makes the query wait until the browser has returned
// all element nodes that match the selector, and they are enabled (that is,
// they have no 'disabled' attribute).
//
// The [NodeSelected] option makes the query wait until the browser has
// returned all element nodes that match the selector, and they are selected
// (that is, they have a 'selected' attribute).
//
// The [NodeNotPresent] option makes the query wait until no element node
// matches the selector.
func Query(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, nil, opts...)
}

// QueryAfter is an element query action that queries the browser for selector
// sel. It waits until the node conditions of the query are met, then runs f
// and returns its value.
func QueryAfter[T any](sel any, f func(ctx context.Context, t *Target, nodes []*Node) (T, error), opts ...QueryOption) Action[T] {
	s := newSelector(sel, opts)
	return func(ctx context.Context, t *Target) (T, error) {
		var res T
		err := s.run(ctx, t, func(ctx context.Context, nodes []*Node) error {
			var err error
			res, err = f(ctx, t, nodes)
			return err
		})
		return res, err
	}
}

// queryDo is like QueryAfter for a func that returns no value. A nil f only
// waits for the nodes.
func queryDo(sel any, f func(ctx context.Context, t *Target, nodes []*Node) error, opts ...QueryOption) Action[Void] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (Void, error) {
		if f == nil {
			return Void{}, nil
		}
		return Void{}, f(ctx, t, nodes)
	}, opts...)
}

// first returns the first node of the query result, or an error when the query
// matched no node.
func first(sel any, nodes []*Node) (*Node, error) {
	if len(nodes) < 1 {
		return nil, fmt.Errorf("selector %q did not return any nodes", sel)
	}
	return nodes[0], nil
}

// withOpts returns the options followed by the extra options, without changing
// the array of opts.
func withOpts(opts []QueryOption, extra ...QueryOption) []QueryOption {
	return append(opts[:len(opts):len(opts)], extra...)
}

func newSelector(sel any, opts []QueryOption) *Selector {
	s := &Selector{
		sel:           sel,
		exp:           1,
		retryInterval: 5 * time.Millisecond,
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

// run executes the selector. It finishes only when the by, wait, after, and
// last funcs succeed, or when the context is canceled. The last func can be
// nil.
func (s *Selector) run(ctx context.Context, t *Target, last func(context.Context, []*Node) error) error {
	return retryWithSleep(ctx, s.retryInterval, func(ctx context.Context) (bool, error) {
		frame, root, execCtx, ok := t.ensureFrame()
		if !ok {
			return false, nil
		}

		fromNode := s.fromNode
		if fromNode == nil {
			fromNode = root
		} else {
			frameID := t.enclosingFrame(fromNode)
			t.frameMu.RLock()
			execCtx = t.execContexts[frameID]
			t.frameMu.RUnlock()

			// TODO: we probably want to use the nested frame
			// instead, but note that util.go stores the nested
			// frame's nodes in the root frame's Nodes map.
			// frame = t.frames[fromNode.FrameID]
			// if frame == nil {
			// 	return fmt.Errorf("FromNode provided does not belong to any active frame")
			// }
		}

		// If this is an iframe node, we want to run the query
		// on its "content document" node instead. Otherwise,
		// queries will return no results.
		if doc := fromNode.ContentDocument; doc != nil {
			fromNode = doc
		}

		ids, err := s.by(ctx, t, fromNode)
		if err != nil {
			var e *cdproto.Error
			// When the selector is invalid (for example, "#a:b" or "#3"), the
			// browser always fails with "DOM Error while querying". It makes no
			// sense to retry in this case.
			// "DOM Error while querying" can also mean other errors. But the
			// response has nothing else that tells them apart. So we have to go
			// with it.
			if errors.As(err, &e) && e.Message == "DOM Error while querying" {
				return true, err
			}
			return false, nil
		}
		if len(ids) < s.exp {
			return false, nil
		}
		nodes, err := s.wait(ctx, t, frame, execCtx, ids...)
		// if nodes==nil, we are not yet ready
		if nodes == nil || err != nil {
			return false, nil
		}
		for _, f := range s.after {
			if err := f(ctx, t, nodes); err != nil {
				return true, err
			}
		}
		if last != nil {
			if err := last(ctx, nodes); err != nil {
				return true, err
			}
		}
		return true, nil
	})
}

// selAsString forces sel into a string.
func (s *Selector) selAsString() string {
	if sel, ok := s.sel.(string); ok {
		return sel
	}
	return fmt.Sprintf("%s", s.sel)
}

// waitReady waits for the specified nodes to be ready.
func (s *Selector) waitReady(check func(context.Context, *Target, runtime.ExecutionContextID, *Node) error) func(context.Context, *Target, *Frame, runtime.ExecutionContextID, ...cdp.NodeID) ([]*Node, error) {
	return func(ctx context.Context, t *Target, cur *Frame, execCtx runtime.ExecutionContextID, ids ...cdp.NodeID) ([]*Node, error) {
		nodes := make([]*Node, len(ids))
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
				go func(n *Node) {
					select {
					case <-ctx.Done():
						errc <- ctx.Err()
					case errc <- check(ctx, t, execCtx, n):
					}
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

// QueryOption is an element query action option.
type QueryOption = func(*Selector)

// FromNode is an element query action option that sets the node where the
// query runs. That is, the query looks only at the element sub-tree of the
// node. By default, or when you pass nil, the query uses the root element of
// the document.
//
// Note: BySearch and ByJSPath do not support FromNode now. The option is
// mainly useful for ByQuery selectors.
func FromNode(node *Node) QueryOption {
	return func(s *Selector) { s.fromNode = node }
}

// ByFunc is an element query action option that sets the func that selects elements.
func ByFunc(f func(context.Context, *Target, *Node) ([]cdp.NodeID, error)) QueryOption {
	return func(s *Selector) {
		s.by = f
	}
}

// ByQuery is an element query action option to select a single element by the
// DOM.querySelector command.
//
// Similar to calling document.querySelector() in the browser.
func ByQuery(s *Selector) {
	ByFunc(func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		res, err := cdp.Call(ctx, t, dom.QuerySelector, dom.QuerySelectorParams{NodeID: n.NodeID, Selector: s.selAsString()})
		if err != nil {
			return nil, err
		}

		if res.NodeID == EmptyNodeID {
			return []cdp.NodeID{}, nil
		}

		return []cdp.NodeID{res.NodeID}, nil
	})(s)
}

// ByQueryAll is an element query action option to select elements by the
// DOM.querySelectorAll command.
//
// Similar to calling document.querySelectorAll() in the browser.
func ByQueryAll(s *Selector) {
	ByFunc(func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		res, err := cdp.Call(ctx, t, dom.QuerySelectorAll, dom.QuerySelectorAllParams{NodeID: n.NodeID, Selector: s.selAsString()})
		return res.NodeIDs, err
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
// command. It matches nodes by plain text, CSS selector or XPath query.
func BySearch(s *Selector) {
	ByFunc(func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		search, err := cdp.Call(ctx, t, dom.PerformSearch, dom.PerformSearchParams{Query: s.selAsString()})
		if err != nil {
			return nil, err
		}

		defer func() {
			_, _ = cdp.Call(ctx, t, dom.DiscardSearchResults, dom.DiscardSearchResultsParams{SearchID: search.SearchID})
		}()

		if search.ResultCount < 1 {
			return []cdp.NodeID{}, nil
		}

		res, err := cdp.Call(ctx, t, dom.GetSearchResults, dom.GetSearchResultsParams{
			SearchID: search.SearchID,
			ToIndex:  search.ResultCount,
		})
		if err != nil {
			return nil, err
		}

		return res.NodeIDs, nil
	})(s)
}

// ByJSPath is an element query option that selects elements by the "JS Path"
// value (as the Chrome DevTools UI shows it).
//
// It queries DOM elements that the other By* funcs cannot retrieve, such as
// ShadowDOM elements.
//
// Note: do not use it with an untrusted selector value, because chromedp
// passes any selector to runtime.Evaluate.
func ByJSPath(s *Selector) {
	ByFunc(func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		// set up eval command
		// execute
		v, err := cdp.Call(ctx, t, runtime.Evaluate, runtime.EvaluateParams{
			Expression:            s.selAsString(),
			AwaitPromise:          new(true),
			ObjectGroup:           "console",
			IncludeCommandLineAPI: new(true),
		})
		if err != nil {
			return nil, err
		}
		if v.ExceptionDetails != nil {
			return nil, &ExceptionError{v.ExceptionDetails}
		}

		// use the ObjectID from the evaluation to get the nodeID
		res, err := cdp.Call(ctx, t, dom.RequestNode, dom.RequestNodeParams{ObjectID: v.Result.ObjectID})
		if err != nil {
			return nil, err
		}

		if res.NodeID == EmptyNodeID {
			return []cdp.NodeID{}, nil
		}

		return []cdp.NodeID{res.NodeID}, nil
	})(s)
}

// ByNodeID is an element query option that selects elements by their node IDs.
//
// It uses DOM.requestChildNodes to retrieve elements with the given node IDs.
//
// Note: use it with []cdp.NodeID.
func ByNodeID(s *Selector) {
	ids, ok := s.sel.([]cdp.NodeID)
	if !ok {
		panic("ByNodeID can only work on []cdp.NodeID")
	}

	ByFunc(func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		for _, id := range ids {
			_, err := cdp.Call(ctx, t, dom.RequestChildNodes, dom.RequestChildNodesParams{NodeID: id, Pierce: new(true)})
			if err != nil {
				return nil, err
			}
		}

		return ids, nil
	})(s)
}

// WaitFunc is an element query option to set a custom node condition wait.
func WaitFunc(wait func(context.Context, *Target, *Frame, runtime.ExecutionContextID, ...cdp.NodeID) ([]*Node, error)) QueryOption {
	return func(s *Selector) {
		s.wait = wait
	}
}

// NodeReady is an element query option that waits until the browser has sent
// all queried element nodes.
func NodeReady(s *Selector) {
	WaitFunc(s.waitReady(nil))(s)
}

// callFunctionOnNode calls the function with the node as its "this" value, and
// returns the decoded result.
func callFunctionOnNode[T any](ctx context.Context, t *Target, node *Node, function string, args ...any) (T, error) {
	var zero T
	r, err := cdp.Call(ctx, t, dom.ResolveNode, dom.ResolveNodeParams{NodeID: node.NodeID})
	if err != nil {
		return zero, err
	}
	res, _, err := callFunctionOn[T](ctx, t, function,
		func(p *runtime.CallFunctionOnParams) {
			p.ObjectID = r.Object.ObjectID
		},
		args...,
	)
	if err != nil {
		return zero, err
	}

	// Try to release the remote object.
	// It fails if the page navigated or closed,
	// and we can ignore the error in this case.
	_, _ = cdp.Call(ctx, t, runtime.ReleaseObject, runtime.ReleaseObjectParams{ObjectID: r.Object.ObjectID})

	return res, nil
}

// NodeVisible is an element query option that waits until the browser has sent
// all queried element nodes and they are visible.
func NodeVisible(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, t *Target, execCtx runtime.ExecutionContextID, n *Node) error {
		// check box model
		_, err := cdp.Call(ctx, t, dom.GetBoxModel, dom.GetBoxModelParams{NodeID: n.NodeID})
		if err != nil {
			if isCouldNotComputeBoxModelError(err) {
				return ErrNotVisible
			}

			return err
		}

		// check visibility
		res, err := callFunctionOnNode[bool](ctx, t, n, visibleJS)
		if err != nil {
			return err
		}
		if !res {
			return ErrNotVisible
		}
		return nil
	}))(s)
}

// NodeNotVisible is an element query option that waits until the browser has
// sent all queried element nodes and they are not visible.
func NodeNotVisible(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, t *Target, execCtx runtime.ExecutionContextID, n *Node) error {
		// check box model
		_, err := cdp.Call(ctx, t, dom.GetBoxModel, dom.GetBoxModelParams{NodeID: n.NodeID})
		if err != nil {
			if isCouldNotComputeBoxModelError(err) {
				return nil
			}

			return err
		}

		// check visibility
		res, err := callFunctionOnNode[bool](ctx, t, n, visibleJS)
		if err != nil {
			return err
		}
		if res {
			return ErrVisible
		}
		return nil
	}))(s)
}

// NodeEnabled is an element query option that waits until the browser has sent
// all queried element nodes and they are enabled (that is, they have no
// 'disabled' attribute).
func NodeEnabled(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, t *Target, execCtx runtime.ExecutionContextID, n *Node) error {
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

// NodeSelected is an element query option that waits until the browser has
// sent all queried element nodes and they are selected (that is, they have a
// 'selected' attribute).
func NodeSelected(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, t *Target, execCtx runtime.ExecutionContextID, n *Node) error {
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

// NodeNotPresent is an element query option that waits until no element
// matches the query.
//
// Note: it sets the expected number of element nodes to 0.
func NodeNotPresent(s *Selector) {
	s.exp = 0
	WaitFunc(func(ctx context.Context, t *Target, cur *Frame, execCtx runtime.ExecutionContextID, ids ...cdp.NodeID) ([]*Node, error) {
		if len(ids) != 0 {
			return nil, ErrHasResults
		}
		return []*Node{}, nil
	})(s)
}

// AtLeast is an element query option that sets the minimum number of elements
// that the query must return.
//
// By default, a query needs 1.
func AtLeast(n int) QueryOption {
	return func(s *Selector) {
		s.exp = n
	}
}

// RetryInterval is an element query action option that sets how often the
// query retries when it fails to select the target elements.
//
// The default is 5ms.
func RetryInterval(interval time.Duration) QueryOption {
	return func(s *Selector) {
		s.retryInterval = interval
	}
}

// After is an element query option that sets a func to run after the browser
// has returned the matched nodes and the node condition is true.
func After(f func(ctx context.Context, t *Target, nodes []*Node) error) QueryOption {
	return func(s *Selector) {
		s.after = append(s.after, f)
	}
}

// Populate is an element query option that retrieves the queried nodes for
// later use. Use a depth of -1 to retrieve all child nodes. When pierce is
// true, it also pierces child containers (for example iframes).
//
// NOTE: this can use a lot of resources. Use it only when necessary.
func Populate(depth int64, pierce bool, opts ...PopulateOption) QueryOption {
	return After(func(ctx context.Context, t *Target, nodes []*Node) error {
		var d time.Duration
		for _, o := range opts {
			o(&d)
		}
		for _, n := range nodes {
			_, err := cdp.Call(ctx, t, dom.RequestChildNodes, dom.RequestChildNodesParams{
				NodeID: n.NodeID,
				Depth:  depth,
				Pierce: &pierce,
			})
			if err != nil {
				return err
			}
		}
		if d != 0 {
			<-time.After(d)
		}
		return nil
	})
}

// PopulateOption is an element populate action option.
type PopulateOption = func(*time.Duration)

// PopulateWait is a populate option that sets a wait interval after the
// request for child nodes.
func PopulateWait(wait time.Duration) PopulateOption {
	return func(d *time.Duration) {
		*d = wait
	}
}

// WaitReady is an element query action that waits until the element that
// matches the selector is ready (that is, "loaded").
func WaitReady(sel any, opts ...QueryOption) Action[Void] {
	return Query(sel, opts...)
}

// WaitVisible is an element query action that waits until the element matching
// the selector is visible.
func WaitVisible(sel any, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeVisible)...)
}

// WaitNotVisible is an element query action that waits until the element
// matching the selector is not visible.
func WaitNotVisible(sel any, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeNotVisible)...)
}

// WaitEnabled is an element query action that waits until the element that
// matches the selector is enabled (that is, it has no attribute 'disabled').
func WaitEnabled(sel any, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeEnabled)...)
}

// WaitSelected is an element query action that waits until the element that
// matches the selector is selected (that is, it has the attribute 'selected').
func WaitSelected(sel any, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeSelected)...)
}

// WaitNotPresent is an element query action that waits until no element
// that matches the selector is present.
func WaitNotPresent(sel any, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeNotPresent)...)
}

// Nodes is an element query action that retrieves the document element nodes
// matching the selector.
func Nodes(sel any, opts ...QueryOption) Action[[]*Node] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) ([]*Node, error) {
		return nodes, nil
	}, opts...)
}

// NodeIDs is an element query action that retrieves the element node IDs matching the
// selector.
func NodeIDs(sel any, opts ...QueryOption) Action[[]cdp.NodeID] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) ([]cdp.NodeID, error) {
		nodeIDs := make([]cdp.NodeID, len(nodes))
		for i, n := range nodes {
			nodeIDs[i] = n.NodeID
		}

		return nodeIDs, nil
	}, opts...)
}

// Focus is an element query action that focuses the first element node matching the
// selector.
func Focus(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = cdp.Call(ctx, t, dom.Focus, dom.FocusParams{NodeID: n.NodeID})
		return err
	}, opts...)
}

// Blur is an element query action that unfocuses (blurs) the first element node
// matching the selector.
func Blur(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		res, err := callFunctionOnNode[bool](ctx, t, n, blurJS)
		if err != nil {
			return err
		}

		if !res {
			return fmt.Errorf("could not blur node %d", n.NodeID)
		}

		return nil
	}, opts...)
}

// Dimensions is an element query action that retrieves the box model dimensions for the
// first element node matching the selector.
func Dimensions(sel any, opts ...QueryOption) Action[*dom.BoxModel] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (*dom.BoxModel, error) {
		n, err := first(sel, nodes)
		if err != nil {
			return nil, err
		}
		res, err := cdp.Call(ctx, t, dom.GetBoxModel, dom.GetBoxModelParams{NodeID: n.NodeID})
		if err != nil {
			return nil, err
		}
		return res.Model, nil
	}, opts...)
}

// Text is an element query action that retrieves the visible text of the first element
// node matching the selector.
func Text(sel any, opts ...QueryOption) Action[string] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (string, error) {
		n, err := first(sel, nodes)
		if err != nil {
			return "", err
		}

		return callFunctionOnNode[string](ctx, t, n, textJS)
	}, opts...)
}

// TextContent is an element query action that retrieves the text content of the first element
// node matching the selector.
func TextContent(sel any, opts ...QueryOption) Action[string] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (string, error) {
		n, err := first(sel, nodes)
		if err != nil {
			return "", err
		}

		return callFunctionOnNode[string](ctx, t, n, textContentJS)
	}, opts...)
}

// Clear is an element query action that clears the values of any input/textarea element
// nodes matching the selector.
func Clear(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		if _, err := first(sel, nodes); err != nil {
			return err
		}

		for _, n := range nodes {
			if n.NodeType != NodeTypeElement || (n.NodeName != "INPUT" && n.NodeName != "TEXTAREA") {
				return fmt.Errorf("selector %q matched node %d with name %s", sel, n.NodeID, strings.ToLower(n.NodeName))
			}
		}

		errs := make([]error, len(nodes))
		var wg sync.WaitGroup
		for i, n := range nodes {
			wg.Add(1)
			go func(i int, n *Node) {
				defer wg.Done()

				if n.NodeName == "INPUT" {
					_, errs[i] = cdp.Call(ctx, t, dom.SetAttributeValue, dom.SetAttributeValueParams{NodeID: n.NodeID, Name: "value"})
				} else {
					// find textarea's child #text node
					var textID cdp.NodeID
					var found bool
					for _, c := range n.Children {
						if c.NodeType == NodeTypeText {
							textID = c.NodeID
							found = true
							break
						}
					}

					if !found {
						errs[i] = fmt.Errorf("textarea node %d does not have child #text node", n.NodeID)
						return
					}

					_, errs[i] = cdp.Call(ctx, t, dom.SetNodeValue, dom.SetNodeValueParams{NodeID: textID})
				}
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

// Value is an element query action that retrieves the JavaScript value field of the
// first element node matching the selector.
//
// Use it to read the JavaScript value of a form, input, textarea, select, or
// other element with a '.value' field.
func Value(sel any, opts ...QueryOption) Action[string] {
	return JavascriptAttribute[string](sel, "value", opts...)
}

// SetValue is an element query action that sets the JavaScript value of the first
// element node matching the selector.
//
// Use it to set the JavaScript value of a form, input, textarea, select, or
// other element with a '.value' field.
func SetValue(sel any, value string, opts ...QueryOption) Action[Void] {
	return SetJavascriptAttribute(sel, "value", value, opts...)
}

// attributeMap returns the attributes of the node as a map.
func attributeMap(n *Node) map[string]string {
	n.RLock()
	defer n.RUnlock()

	m := make(map[string]string)
	attrs := n.Attributes
	for i := 0; i < len(attrs); i += 2 {
		m[attrs[i]] = attrs[i+1]
	}
	return m
}

// Attributes is an element query action that retrieves the element attributes for the
// first element node matching the selector.
func Attributes(sel any, opts ...QueryOption) Action[map[string]string] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (map[string]string, error) {
		n, err := first(sel, nodes)
		if err != nil {
			return nil, err
		}
		return attributeMap(n), nil
	}, opts...)
}

// AttributesAll is an element query action that retrieves the element attributes for
// all element nodes matching the selector.
// Note: use it with the ByQueryAll query option.
func AttributesAll(sel any, opts ...QueryOption) Action[[]map[string]string] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) ([]map[string]string, error) {
		if _, err := first(sel, nodes); err != nil {
			return nil, err
		}

		all := make([]map[string]string, 0, len(nodes))
		for _, n := range nodes {
			all = append(all, attributeMap(n))
		}
		return all, nil
	}, opts...)
}

// SetAttributes is an element query action that sets the element attributes for the
// first element node matching the selector.
func SetAttributes(sel any, attributes map[string]string, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		if len(nodes) < 1 {
			return errors.New("expected at least one element")
		}

		i, attrs := 0, make([]string, len(attributes))
		for k, v := range attributes {
			attrs[i] = fmt.Sprintf(`%s=%s`, k, strconv.Quote(v))
			i++
		}

		_, err := cdp.Call(ctx, t, dom.SetAttributesAsText, dom.SetAttributesAsTextParams{
			NodeID: nodes[0].NodeID,
			Text:   strings.Join(attrs, " "),
		})
		return err
	}, opts...)
}

// AttributeResult is the result of the [AttributeValue] action.
type AttributeResult struct {
	// Value is the value of the attribute. It is empty when the attribute
	// does not exist.
	Value string

	// Exists is true when the element has the attribute.
	Exists bool
}

// AttributeValue is an element query action that retrieves the element attribute value
// for the first element node matching the selector. The result tells whether the
// attribute exists.
func AttributeValue(sel any, name string, opts ...QueryOption) Action[AttributeResult] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (AttributeResult, error) {
		if len(nodes) < 1 {
			return AttributeResult{}, errors.New("expected at least one element")
		}

		nodes[0].RLock()
		defer nodes[0].RUnlock()

		attrs := nodes[0].Attributes
		for i := 0; i < len(attrs); i += 2 {
			if attrs[i] == name {
				return AttributeResult{Value: attrs[i+1], Exists: true}, nil
			}
		}

		return AttributeResult{}, nil
	}, opts...)
}

// SetAttributeValue is an element query action that sets the element attribute with
// name to value for the first element node matching the selector.
func SetAttributeValue(sel any, name, value string, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = cdp.Call(ctx, t, dom.SetAttributeValue, dom.SetAttributeValueParams{NodeID: n.NodeID, Name: name, Value: value})
		return err
	}, opts...)
}

// RemoveAttribute is an element query action that removes the element attribute with
// name from the first element node matching the selector.
func RemoveAttribute(sel any, name string, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = cdp.Call(ctx, t, dom.RemoveAttribute, dom.RemoveAttributeParams{NodeID: n.NodeID, Name: name})
		return err
	}, opts...)
}

// JavascriptAttribute is an element query action that retrieves the JavaScript
// attribute for the first element node matching the selector. It decodes the
// attribute into the type T, as [Evaluate] does.
func JavascriptAttribute[T any](sel any, name string, opts ...QueryOption) Action[T] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (T, error) {
		var zero T
		n, err := first(sel, nodes)
		if err != nil {
			return zero, err
		}

		res, err := callFunctionOnNode[T](ctx, t, n, attributeJS, name)
		if err != nil {
			return zero, fmt.Errorf("could not retrieve attribute %q: %w", name, err)
		}

		return res, nil
	}, opts...)
}

// SetJavascriptAttribute is an element query action that sets the JavaScript attribute
// for the first element node matching the selector.
func SetJavascriptAttribute(sel any, name, value string, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		res, err := callFunctionOnNode[string](ctx, t, n, setAttributeJS, name, value)
		if err != nil {
			return err
		}
		if res != value {
			return fmt.Errorf("could not set value on node %d", n.NodeID)
		}

		return nil
	}, opts...)
}

// OuterHTML is an element query action that retrieves the outer html of the first
// element node matching the selector.
func OuterHTML(sel any, opts ...QueryOption) Action[string] {
	return JavascriptAttribute[string](sel, "outerHTML", opts...)
}

// InnerHTML is an element query action that retrieves the inner html of the first
// element node matching the selector.
func InnerHTML(sel any, opts ...QueryOption) Action[string] {
	return JavascriptAttribute[string](sel, "innerHTML", opts...)
}

// Click is an element query action that sends a mouse click event to the first element
// node matching the selector.
func Click(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = MouseClickNode(n)(ctx, t)
		return err
	}, withOpts(opts, NodeVisible)...)
}

// DoubleClick is an element query action that sends a mouse double click event to the
// first element node matching the selector.
func DoubleClick(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = MouseClickNode(n, ClickCount(2))(ctx, t)
		return err
	}, withOpts(opts, NodeVisible)...)
}

// SendKeys is an element query action that synthesizes the key up, char, and
// down events that the runes in v need, and sends them to the first element
// node that matches the selector.
//
// See [keys] for a complete example of how to use SendKeys.
//
// Note: when the element query matches an input[type="file"] node, SendKeys
// uses dom.SetFileInputFiles to set the upload path of the input node to v.
//
// [keys]: https://github.com/chromedp/examples/tree/master/keys
func SendKeys(sel any, v string, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

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
			_, err := cdp.Call(ctx, t, dom.SetFileInputFiles, dom.SetFileInputFilesParams{Files: []string{v}, NodeID: n.NodeID})
			return err
		}

		_, err = KeyEventNode(n, v)(ctx, t)
		return err
	}, withOpts(opts, NodeVisible)...)
}

// SetUploadFiles is an element query action that sets the files to upload (that is, for an
// input[type="file"] node) for the first element node matching the selector.
func SetUploadFiles(sel any, files []string, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = cdp.Call(ctx, t, dom.SetFileInputFiles, dom.SetFileInputFilesParams{Files: files, NodeID: n.NodeID})
		return err
	}, opts...)
}

// Submit is an element query action that submits the parent form of the first element
// node matching the selector.
func Submit(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		res, err := callFunctionOnNode[bool](ctx, t, n, submitJS)
		if err != nil {
			return err
		}

		if !res {
			return fmt.Errorf("could not call submit on node %d", n.NodeID)
		}

		return nil
	}, opts...)
}

// Reset is an element query action that resets the parent form of the first element
// node matching the selector.
func Reset(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		res, err := callFunctionOnNode[bool](ctx, t, n, resetJS)
		if err != nil {
			return err
		}

		if !res {
			return fmt.Errorf("could not call reset on node %d", n.NodeID)
		}

		return nil
	}, opts...)
}

// ComputedStyle is an element query action that retrieves the computed style of the
// first element node matching the selector.
func ComputedStyle(sel any, opts ...QueryOption) Action[[]*css.ComputedStyleProperty] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) ([]*css.ComputedStyleProperty, error) {
		n, err := first(sel, nodes)
		if err != nil {
			return nil, err
		}

		res, err := cdp.Call(ctx, t, css.GetComputedStyleForNode, css.GetComputedStyleForNodeParams{NodeID: n.NodeID})
		if err != nil {
			return nil, err
		}

		return res.ComputedStyle, nil
	}, opts...)
}

// MatchedStyle is an element query action that retrieves the matched style information
// for the first element node matching the selector.
func MatchedStyle(sel any, opts ...QueryOption) Action[*css.GetMatchedStylesForNodeResult] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (*css.GetMatchedStylesForNodeResult, error) {
		n, err := first(sel, nodes)
		if err != nil {
			return nil, err
		}

		res, err := cdp.Call(ctx, t, css.GetMatchedStylesForNode, css.GetMatchedStylesForNodeParams{NodeID: n.NodeID})
		if err != nil {
			return nil, err
		}

		return &res, nil
	}, opts...)
}

// ScrollIntoView is an element query action that scrolls the window to the
// first element node matching the selector.
func ScrollIntoView(sel any, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = cdp.Call(ctx, t, dom.ScrollIntoViewIfNeeded, dom.ScrollIntoViewIfNeededParams{NodeID: n.NodeID})
		return err
	}, opts...)
}

// DumpTo is an element query action that writes a readable tree of the first
// element node matching the selector and its children, up to the specified
// depth.
//
// See [Dump] for a simpler interface.
func DumpTo(sel any, w io.Writer, prefix, indent string, nodeIDs bool, depth int64, pierce bool, wait time.Duration, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts,
		Populate(depth, pierce, PopulateWait(wait)),
		After(func(ctx context.Context, t *Target, nodes []*Node) error {
			var n *Node
			if len(nodes) > 0 {
				n = nodes[0]
			}
			_, err := n.WriteTo(w, prefix, indent, nodeIDs)
			return err
		}),
	)...)
}

// Dump is an element query action that writes a readable tree of the first
// element node matching the selector and its children, up to the specified
// depth.
//
// See [DumpTo] for more options, which include the sleep wait timeout.
func Dump(sel any, w io.Writer, opts ...QueryOption) Action[Void] {
	return DumpTo(sel, w, "", "  ", false, -1, true, 80*time.Millisecond, opts...)
}
