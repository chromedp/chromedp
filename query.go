package chromedp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
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

// Selectable is what an element query accepts as the selector. The type of the
// selector chooses the lookup. A plain string is a [Search]. A named string
// type that this package does not define also counts as a [Search].
//
// The set of selector types is closed. A type that you define cannot add a new
// lookup. For a custom lookup, use the option [ByFunc]. It replaces the lookup
// of the selector with a func that receives the [Target] of the tab and the
// [Node] where the query starts, and returns the node IDs of the elements. The
// target is a session for [cdp.Call]. The start node is the root of the
// document, or the node of [FromNode]. The selector is then only a label in
// error messages, so pass "" or any string. For example, this lookup finds the
// elements with a data-testid attribute:
//
//	func byTestID(id string) chromedp.QueryOption {
//		return chromedp.ByFunc(func(ctx context.Context, t *chromedp.Target, n *chromedp.Node) ([]cdp.NodeID, error) {
//			res, err := cdp.Call(ctx, t, dom.QuerySelectorAll, dom.QuerySelectorAllParams{
//				NodeID:   n.NodeID,
//				Selector: fmt.Sprintf("[data-testid=%q]", id),
//			})
//			return res.NodeIDs, err
//		})
//	}
//
//	err := chromedp.Do(ctx, chromedp.Click("", byTestID("submit")))
type Selectable interface {
	~string | ~[]cdp.NodeID
}

// Search is a selector that the browser resolves with DOM.performSearch. It
// matches nodes by plain text, CSS selector or XPath query. It is the lookup of
// a plain string.
type Search string

// CSS is a selector for the first element that matches a CSS selector. It uses
// DOM.querySelector.
type CSS string

// CSSAll is a selector for every element that matches a CSS selector. It uses
// DOM.querySelectorAll.
type CSSAll string

// ID is a selector for the element with this id. A leading "#" is optional. It
// uses DOM.querySelector.
type ID string

// JSPath is a selector for the elements that a JavaScript expression gives. It
// uses Runtime.evaluate. Use it only with trusted values, because chromedp
// passes the expression to the browser without a check.
type JSPath string

// NodeIDs is a selector for the elements with these node IDs.
type NodeIDs []cdp.NodeID

// Selector holds the data of an element selection query.
//
// See [Query] for how to build an element selector and its options.
type Selector struct {
	text          string
	ids           []cdp.NodeID
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
//	chromedp.Do(ctx, chromedp.SendKeys(chromedp.ID("thing"), "hello"))
//
// This runs a [SendKeys] action on the first element that has the id "thing".
//
// Element selection queries work with specific actions. They are the main way
// to automate steps in the browser. They have this form:
//
//	Action(selector[, parameter1, ...parameterN][, queryOptions...])
//
// Where:
//
//   - Action - the action to run
//   - selector - the element query, a [Selectable] value. The action applies to every node that matches it.
//   - parameter[1-N] - the parameters that the action needs (if any)
//   - queryOptions - change how the query runs, or how it waits for nodes
//
// An action that reads a value returns it. For example, [Text] returns the
// text as a string.
//
// # Selectors
//
// The type of the selector chooses the lookup that the browser runs. A plain
// string, or any other string type that this package does not define, is a
// [Search].
//
// The [Search] type (the default) queries elements by plain text, CSS
// selector, or XPath query. It wraps DOM.performSearch.
//
// The [ID] type queries a single element by its CSS ID. It wraps
// DOM.querySelector. ID is like document.querySelector('#' + ID) in the
// browser.
//
// The [CSS] type queries a single element with a CSS selector. It wraps
// DOM.querySelector. CSS is like document.querySelector() in the browser.
//
// The [CSSAll] type queries elements with a CSS selector. It wraps
// DOM.querySelectorAll. CSSAll is like document.querySelectorAll() in the
// browser.
//
// The [JSPath] type queries a single element by its "JS Path" value. It wraps
// Runtime.evaluate. JSPath is like a JavaScript snippet that returns an
// element in the browser. Use it only with trusted element queries. chromedp
// passes the query directly to Runtime.evaluate and does not sanitize it. The
// type is useful for DOM elements that the other types cannot retrieve, such
// as ShadowDOM elements. The expression can give a node, an array or a NodeList
// of nodes, or null or undefined. Null, undefined and an empty list select no
// element, so [WaitNotPresent] succeeds and the other waits keep waiting. Any
// other value is an error, and the query returns it at once.
//
// The [NodeIDs] type selects the elements with the given node IDs. It uses
// DOM.requestChildNodes to retrieve them.
//
// A type outside the [Selectable] set, such as an int or a *[Node], does not
// compile.
//
// # Query Options
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
// The [ByFunc] option replaces the lookup of the selector with a custom func.
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
// all element nodes that match the selector. The nodes must be enabled (that
// is, they have no 'disabled' attribute).
//
// The [NodeSelected] option makes the query wait until the browser has
// returned all element nodes that match the selector. The nodes must be
// selected (that is, they have a 'selected' attribute).
//
// The [NodeNotPresent] option makes the query wait until no element node
// matches the selector.
func Query[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return queryDo(sel, nil, opts...)
}

// QueryAfter is an element query action that queries the browser for selector
// sel. It waits until the node conditions of the query are met, then runs f
// and returns its value.
func QueryAfter[T any, S Selectable](sel S, f func(ctx context.Context, t *Target, nodes []*Node) (T, error), opts ...QueryOption) Action[T] {
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
func queryDo[S Selectable](sel S, f func(ctx context.Context, t *Target, nodes []*Node) error, opts ...QueryOption) Action[Void] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (Void, error) {
		if f == nil {
			return Void{}, nil
		}
		return Void{}, f(ctx, t, nodes)
	}, opts...)
}

// first returns the first node of the query result, or an error when the query
// matched no node.
func first[S Selectable](sel S, nodes []*Node) (*Node, error) {
	if len(nodes) < 1 {
		return nil, fmt.Errorf("selector %q did not return any nodes", describe(sel))
	}
	return nodes[0], nil
}

// withOpts returns the options followed by the extra options, without changing
// the array of opts.
func withOpts(opts []QueryOption, extra ...QueryOption) []QueryOption {
	return append(opts[:len(opts):len(opts)], extra...)
}

// newSelector builds the selector of sel and applies the options. The type of
// sel chooses the lookup. The option [ByFunc] can replace it.
func newSelector[S Selectable](sel S, opts []QueryOption) *Selector {
	s := &Selector{
		exp:           1,
		retryInterval: 5 * time.Millisecond,
	}
	s.text, s.ids, s.by = lookup(sel)

	// apply options
	for _, o := range opts {
		o(s)
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
			// instead. Note that util.go stores the nodes of the
			// nested frame in the Nodes map of the root frame.
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
			// A JSPath value of the wrong type does not change by itself.
			if errors.Is(err, errNotNode) {
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
// Note: [Search] and [JSPath] selectors do not support FromNode now. The
// option is mainly useful for [CSS] selectors.
//
// FromNode does not reach into an iframe from another site. Chrome runs such an
// iframe in its own process, as a separate target, and the node of the iframe
// element has no ContentDocument. A query from that node finds nothing and
// waits until its context ends. See [WithTargetID] for how to attach to the
// iframe.
func FromNode(node *Node) QueryOption {
	return func(s *Selector) { s.fromNode = node }
}

// ByFunc is an element query action option that sets the func that selects
// elements. It replaces the lookup that the type of the selector chooses. The
// selector is then only a label in error messages, so pass "" or any string.
func ByFunc(f func(context.Context, *Target, *Node) ([]cdp.NodeID, error)) QueryOption {
	return func(s *Selector) {
		s.by = f
	}
}

// lookupFunc finds the element node IDs of a query. It starts at the node n.
type lookupFunc = func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error)

// lookup returns what a selector of the type S needs. These are the text for
// error messages, the node IDs and the func that selects the elements. A string
// type that this package does not define counts as a [Search].
func lookup[S Selectable](sel S) (text string, ids []cdp.NodeID, by lookupFunc) {
	switch v := any(sel).(type) {
	case CSS:
		return string(v), nil, queryOne(string(v))
	case CSSAll:
		return string(v), nil, queryAll(string(v))
	case ID:
		return string(v), nil, queryOne("#" + strings.TrimPrefix(string(v), "#"))
	case JSPath:
		return string(v), nil, evaluatePath(string(v))
	case NodeIDs:
		return fmt.Sprint([]cdp.NodeID(v)), v, requestNodes(v)
	case Search:
		return string(v), nil, search(string(v))
	case string:
		return v, nil, search(v)
	case []cdp.NodeID:
		return fmt.Sprint(v), v, requestNodes(v)
	}

	// The type is a string type or a node ID slice type that the user
	// defined.
	v := reflect.ValueOf(sel)
	if v.Kind() == reflect.String {
		return v.String(), nil, search(v.String())
	}
	ids = v.Convert(reflect.TypeFor[[]cdp.NodeID]()).Interface().([]cdp.NodeID)
	return fmt.Sprint(ids), ids, requestNodes(ids)
}

// describe returns the text of a selector for error messages.
func describe[S Selectable](sel S) string {
	text, _, _ := lookup(sel)
	return text
}

// queryOne selects a single element by the DOM.querySelector command.
func queryOne(selector string) lookupFunc {
	return func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		res, err := cdp.Call(ctx, t, dom.QuerySelector, dom.QuerySelectorParams{NodeID: n.NodeID, Selector: selector})
		if err != nil {
			return nil, err
		}

		if res.NodeID == EmptyNodeID {
			return []cdp.NodeID{}, nil
		}

		return []cdp.NodeID{res.NodeID}, nil
	}
}

// queryAll selects elements by the DOM.querySelectorAll command.
func queryAll(selector string) lookupFunc {
	return func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		res, err := cdp.Call(ctx, t, dom.QuerySelectorAll, dom.QuerySelectorAllParams{NodeID: n.NodeID, Selector: selector})
		return res.NodeIDs, err
	}
}

// search selects elements by the DOM.performSearch command. It matches nodes by
// plain text, CSS selector or XPath query.
func search(query string) lookupFunc {
	return func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		found, err := cdp.Call(ctx, t, dom.PerformSearch, dom.PerformSearchParams{Query: query})
		if err != nil {
			return nil, err
		}

		defer func() {
			_, _ = cdp.Call(ctx, t, dom.DiscardSearchResults, dom.DiscardSearchResultsParams{SearchID: found.SearchID})
		}()

		if found.ResultCount < 1 {
			return []cdp.NodeID{}, nil
		}

		res, err := cdp.Call(ctx, t, dom.GetSearchResults, dom.GetSearchResultsParams{
			SearchID: found.SearchID,
			ToIndex:  found.ResultCount,
		})
		if err != nil {
			return nil, err
		}

		return res.NodeIDs, nil
	}
}

// evaluatePath selects the elements that a JavaScript expression gives, by the
// Runtime.evaluate command. The expression can give a node, an array or a
// NodeList of nodes, or null or undefined. The last one and an empty list select
// no element, so that [NodeNotPresent] succeeds and the other conditions wait.
// Any other value is an error that [Selector.run] does not retry.
func evaluatePath(expression string) lookupFunc {
	return func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		v, err := cdp.Call(ctx, t, runtime.Evaluate, runtime.EvaluateParams{
			Expression:            expression,
			AwaitPromise:          ptr(true),
			ObjectGroup:           "console",
			IncludeCommandLineAPI: ptr(true),
		})
		if err != nil {
			return nil, err
		}
		if v.ExceptionDetails != nil {
			return nil, &ExceptionError{v.ExceptionDetails}
		}

		obj := v.Result
		switch {
		case obj.Type == runtime.RemoteObjectTypeUndefined, obj.Subtype == runtime.RemoteObjectSubtypeNull:
			return []cdp.NodeID{}, nil
		case obj.Subtype == runtime.RemoteObjectSubtypeNode && obj.ObjectID != "":
			return requestNode(ctx, t, obj.ObjectID)
		case obj.Subtype != runtime.RemoteObjectSubtypeArray || obj.ObjectID == "":
			return nil, fmt.Errorf("JSPath %q gave a %s and not a node: %w", expression, valueKind(obj), errNotNode)
		}

		// An array or a NodeList: its indexes are its own properties.
		props, err := cdp.Call(ctx, t, runtime.GetProperties, runtime.GetPropertiesParams{
			ObjectID:      obj.ObjectID,
			OwnProperties: ptr(true),
		})
		if err != nil {
			return nil, err
		}
		ids := []cdp.NodeID{}
		for _, p := range props.Result {
			if _, err := strconv.ParseUint(p.Name, 10, 32); err != nil || p.Value == nil {
				continue
			}
			if p.Value.Subtype != runtime.RemoteObjectSubtypeNode {
				return nil, fmt.Errorf("JSPath %q gave a list with a value that is not a node: %w", expression, errNotNode)
			}
			nodeIDs, err := requestNode(ctx, t, p.Value.ObjectID)
			if err != nil {
				return nil, err
			}
			ids = append(ids, nodeIDs...)
		}
		return ids, nil
	}
}

// valueKind names the kind of a value for an error message.
func valueKind(obj *runtime.RemoteObject) string {
	if obj.ClassName != "" {
		return obj.ClassName
	}
	return string(obj.Type)
}

// errNotNode marks the error of a JSPath expression that gave a value that is
// not a node, an array or NodeList of nodes, null or undefined. A retry gives
// the same error, so [Selector.run] returns it at once.
var errNotNode = errors.New("not a node")

// requestNode returns the ID of the node that the object is.
func requestNode(ctx context.Context, t *Target, id runtime.RemoteObjectID) ([]cdp.NodeID, error) {
	res, err := cdp.Call(ctx, t, dom.RequestNode, dom.RequestNodeParams{ObjectID: id})
	if err != nil {
		return nil, err
	}
	if res.NodeID == EmptyNodeID {
		return []cdp.NodeID{}, nil
	}
	return []cdp.NodeID{res.NodeID}, nil
}

// requestNodes selects the elements with the given node IDs. It uses the
// DOM.requestChildNodes command.
func requestNodes(ids []cdp.NodeID) lookupFunc {
	return func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		for _, id := range ids {
			_, err := cdp.Call(ctx, t, dom.RequestChildNodes, dom.RequestChildNodesParams{NodeID: id, Pierce: ptr(true)})
			if err != nil {
				return nil, err
			}
		}

		return ids, nil
	}
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

	// Try to release the remote object, also when the call failed.
	// It fails if the page navigated or closed,
	// and we can ignore the error in this case.
	_, _ = cdp.Call(ctx, t, runtime.ReleaseObject, runtime.ReleaseObjectParams{ObjectID: r.Object.ObjectID})

	if err != nil {
		return zero, err
	}
	return res, nil
}

// NodeVisible is an element query option that waits until the browser has sent
// all queried element nodes and they are visible.
func NodeVisible(s *Selector) {
	WaitFunc(s.waitReady(func(ctx context.Context, t *Target, execCtx runtime.ExecutionContextID, n *Node) error {
		// get the box model
		_, err := cdp.Call(ctx, t, dom.GetBoxModel, dom.GetBoxModelParams{NodeID: n.NodeID})
		if err != nil {
			if isCouldNotComputeBoxModelError(err) {
				return ErrNotVisible
			}

			return err
		}

		// make sure that the visibility is as expected
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
		// get the box model
		_, err := cdp.Call(ctx, t, dom.GetBoxModel, dom.GetBoxModelParams{NodeID: n.NodeID})
		if err != nil {
			if isCouldNotComputeBoxModelError(err) {
				return nil
			}

			return err
		}

		// make sure that the visibility is as expected
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
// all queried element nodes. The nodes must be enabled (that is, they have no
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
// sent all queried element nodes. The nodes must be selected (that is, they
// have a 'selected' attribute).
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

// After is an element query option that sets a func to run when the browser
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
				Depth:  &depth,
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
func WaitReady[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return Query(sel, opts...)
}

// WaitVisible is an element query action that waits until the element matching
// the selector is visible.
func WaitVisible[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeVisible)...)
}

// WaitNotVisible is an element query action that waits until the element
// matching the selector is not visible.
func WaitNotVisible[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeNotVisible)...)
}

// WaitEnabled is an element query action that waits until the element that
// matches the selector is enabled (that is, it has no attribute 'disabled').
func WaitEnabled[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeEnabled)...)
}

// WaitSelected is an element query action that waits until the element that
// matches the selector is selected (that is, it has the attribute 'selected').
func WaitSelected[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeSelected)...)
}

// WaitNotPresent is an element query action that waits until no element
// that matches the selector is present.
func WaitNotPresent[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return Query(sel, withOpts(opts, NodeNotPresent)...)
}

// Nodes is an element query action that retrieves the document element nodes
// matching the selector.
func Nodes[S Selectable](sel S, opts ...QueryOption) Action[[]*Node] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) ([]*Node, error) {
		return nodes, nil
	}, opts...)
}

// QueryNodeIDs is an element query action that retrieves the element node IDs
// matching the selector.
func QueryNodeIDs[S Selectable](sel S, opts ...QueryOption) Action[[]cdp.NodeID] {
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
func Focus[S Selectable](sel S, opts ...QueryOption) Action[Void] {
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
func Blur[S Selectable](sel S, opts ...QueryOption) Action[Void] {
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
func Dimensions[S Selectable](sel S, opts ...QueryOption) Action[*dom.BoxModel] {
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
func Text[S Selectable](sel S, opts ...QueryOption) Action[string] {
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
func TextContent[S Selectable](sel S, opts ...QueryOption) Action[string] {
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
func Clear[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		if _, err := first(sel, nodes); err != nil {
			return err
		}

		for _, n := range nodes {
			if n.NodeType != NodeTypeElement || (n.NodeName != "INPUT" && n.NodeName != "TEXTAREA") {
				return fmt.Errorf("selector %q matched node %d with name %s", describe(sel), n.NodeID, strings.ToLower(n.NodeName))
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
//
// Value follows [JavascriptAttribute]. An element without a '.value' field
// gives an error that wraps [ErrJSUndefined].
func Value[S Selectable](sel S, opts ...QueryOption) Action[string] {
	return JavascriptAttribute[string](sel, "value", opts...)
}

// SetValue is an element query action that sets the JavaScript value of the first
// element node matching the selector.
//
// Use it to set the JavaScript value of a form, input, textarea, select, or
// other element with a '.value' field.
func SetValue[S Selectable](sel S, value string, opts ...QueryOption) Action[Void] {
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
func Attributes[S Selectable](sel S, opts ...QueryOption) Action[map[string]string] {
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
// Note: use it with a [CSSAll] selector.
func AttributesAll[S Selectable](sel S, opts ...QueryOption) Action[[]map[string]string] {
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
func SetAttributes[S Selectable](sel S, attributes map[string]string, opts ...QueryOption) Action[Void] {
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
func AttributeValue[S Selectable](sel S, name string, opts ...QueryOption) Action[AttributeResult] {
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
func SetAttributeValue[S Selectable](sel S, name, value string, opts ...QueryOption) Action[Void] {
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
func RemoveAttribute[S Selectable](sel S, name string, opts ...QueryOption) Action[Void] {
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
// attribute into the type T, as [Evaluate] does, with one difference. When the
// attribute is null, for example "onclick" with no handler, JavascriptAttribute
// returns the zero value of T and no error, and it does not return [ErrJSNull].
// An attribute that the element does not have is undefined, and the action
// returns an error that wraps [ErrJSUndefined] for a type that cannot be nil.
func JavascriptAttribute[T any, S Selectable](sel S, name string, opts ...QueryOption) Action[T] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) (T, error) {
		var zero T
		n, err := first(sel, nodes)
		if err != nil {
			return zero, err
		}

		res, err := callFunctionOnNode[T](ctx, t, n, attributeJS, name)
		if errors.Is(err, ErrJSNull) {
			return zero, nil
		}
		if err != nil {
			return zero, fmt.Errorf("could not retrieve attribute %q: %w", name, err)
		}

		return res, nil
	}, opts...)
}

// SetJavascriptAttribute is an element query action that sets the JavaScript attribute
// for the first element node matching the selector.
func SetJavascriptAttribute[S Selectable](sel S, name, value string, opts ...QueryOption) Action[Void] {
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
func OuterHTML[S Selectable](sel S, opts ...QueryOption) Action[string] {
	return JavascriptAttribute[string](sel, "outerHTML", opts...)
}

// InnerHTML is an element query action that retrieves the inner html of the first
// element node matching the selector.
func InnerHTML[S Selectable](sel S, opts ...QueryOption) Action[string] {
	return JavascriptAttribute[string](sel, "innerHTML", opts...)
}

// Click is an element query action that sends a mouse click event to the first element
// node matching the selector.
func Click[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = MouseClickNode(n)(ctx, t)
		return err
	}, withOpts(opts, NodeVisible)...)
}

// Tap is an element query action that sends a touch tap to the center of the
// first element node matching the selector. The action scrolls the node into
// view, and then sends a touchStart event and a touchEnd event. See [TapXY].
//
// The browser sends the touch events and the click event to the page only when
// touch emulation is on. Use [EmulateViewport] with [EmulateTouch] for it.
func Tap[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		x, y, err := nodeCenter(ctx, t, n)
		if err != nil {
			return err
		}

		_, err = TapXY(x, y)(ctx, t)
		return err
	}, withOpts(opts, NodeVisible)...)
}

// DoubleClick is an element query action that sends a mouse double click event to the
// first element node matching the selector.
func DoubleClick[S Selectable](sel S, opts ...QueryOption) Action[Void] {
	return queryDo(sel, func(ctx context.Context, t *Target, nodes []*Node) error {
		n, err := first(sel, nodes)
		if err != nil {
			return err
		}

		_, err = MouseClickNode(n, ClickCount(2))(ctx, t)
		return err
	}, withOpts(opts, NodeVisible)...)
}

// SendKeys is an element query action that sends key events to the first
// element node that matches the selector. It synthesizes the key down, char,
// and key up events that the runes in v need.
//
// See [keys] for a complete example of how to use SendKeys.
//
// Note: when the element query matches an input[type="file"] node, SendKeys
// uses dom.SetFileInputFiles to set the upload path of the input node to v.
//
// A "\n" in v is the Enter key. SendKeys sends it as "\r", which is the same as
// [kb.Enter]: a keyDown event, a char event with the text "\r", and a keyUp
// event. An editor that handles Enter on keydown and cancels the default, such
// as Lexical, can then insert two line breaks, because the char event inserts
// one more. To send the Enter key with no char event, send a rawKeyDown event
// and a keyUp event with the command [input.DispatchKeyEvent]:
//
//	enter := chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
//		for _, typ := range []input.DispatchKeyEventType{
//			input.DispatchKeyEventTypeRawKeyDown, input.DispatchKeyEventTypeKeyUp,
//		} {
//			_, err := cdp.Call(ctx, t, input.DispatchKeyEvent, input.DispatchKeyEventParams{
//				Type: typ, Key: "Enter", Code: "Enter",
//				WindowsVirtualKeyCode: 13, NativeVirtualKeyCode: 13,
//			})
//			if err != nil {
//				return err
//			}
//		}
//		return nil
//	})
//	err := chromedp.Do(ctx,
//		chromedp.SendKeys(sel, "Hello"),
//		enter,
//		chromedp.SendKeys(sel, "World"),
//	)
//
// [keys]: https://github.com/chromedp/examples/tree/main/keys
func SendKeys[S Selectable](sel S, v string, opts ...QueryOption) Action[Void] {
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

// SetUploadFiles is an element query action that sets the files to upload for
// the first element node matching the selector. The node must be an
// input[type="file"] node.
func SetUploadFiles[S Selectable](sel S, files []string, opts ...QueryOption) Action[Void] {
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
func Submit[S Selectable](sel S, opts ...QueryOption) Action[Void] {
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
func Reset[S Selectable](sel S, opts ...QueryOption) Action[Void] {
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
func ComputedStyle[S Selectable](sel S, opts ...QueryOption) Action[[]*css.ComputedStyleProperty] {
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
func MatchedStyle[S Selectable](sel S, opts ...QueryOption) Action[*css.GetMatchedStylesForNodeResult] {
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
func ScrollIntoView[S Selectable](sel S, opts ...QueryOption) Action[Void] {
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
// element node matching the selector and its children. The tree ends at the
// specified depth.
//
// See [Dump] for a simpler interface.
func DumpTo[S Selectable](sel S, w io.Writer, prefix, indent string, nodeIDs bool, depth int64, pierce bool, wait time.Duration, opts ...QueryOption) Action[Void] {
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
// element node matching the selector and its children. The tree ends at the
// specified depth.
//
// See [DumpTo] for more options, which include the sleep wait timeout.
func Dump[S Selectable](sel S, w io.Writer, opts ...QueryOption) Action[Void] {
	return DumpTo(sel, w, "", "  ", false, -1, true, 80*time.Millisecond, opts...)
}
