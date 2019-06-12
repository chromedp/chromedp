package chromedp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/page"
)

// Nodes retrieves the document nodes matching the selector.
func Nodes(sel interface{}, nodes *[]*cdp.Node, opts ...QueryOption) Action {
	if nodes == nil {
		panic("nodes cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, n ...*cdp.Node) error {
		*nodes = n
		return nil
	}, opts...)
}

// NodeIDs retrieves the node IDs matching the selector.
func NodeIDs(sel interface{}, ids *[]cdp.NodeID, opts ...QueryOption) Action {
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

// Focus focuses the first node matching the selector.
func Focus(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return dom.Focus().WithNodeID(nodes[0].NodeID).Do(ctx)
	}, opts...)
}

// Blur unfocuses (blurs) the first node matching the selector.
func Blur(sel interface{}, opts ...QueryOption) Action {
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

// Dimensions retrieves the box model dimensions for the first node matching
// the selector.
func Dimensions(sel interface{}, model **dom.BoxModel, opts ...QueryOption) Action {
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

// Text retrieves the visible text of the first node matching the selector.
func Text(sel interface{}, text *string, opts ...QueryOption) Action {
	if text == nil {
		panic("text cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return EvaluateAsDevTools(snippet(textJS, cashXNode(false), sel, nodes[0]), text).Do(ctx)
	}, opts...)
}

// Clear clears the values of any input/textarea nodes matching the selector.
func Clear(sel interface{}, opts ...QueryOption) Action {
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

// Value retrieves the value of the first node matching the selector.
func Value(sel interface{}, value *string, opts ...QueryOption) Action {
	if value == nil {
		panic("value cannot be nil")
	}

	return JavascriptAttribute(sel, "value", value, opts...)
}

// SetValue sets the value of an element.
func SetValue(sel interface{}, value string, opts ...QueryOption) Action {
	return SetJavascriptAttribute(sel, "value", value, opts...)
}

// Attributes retrieves the element attributes for the first node matching the
// selector.
func Attributes(sel interface{}, attributes *map[string]string, opts ...QueryOption) Action {
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

// AttributesAll retrieves the element attributes for all nodes matching the
// selector.
//
// Note: this should be used with the ByQueryAll selector option.
func AttributesAll(sel interface{}, attributes *[]map[string]string, opts ...QueryOption) Action {
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

// SetAttributes sets the element attributes for the first node matching the
// selector.
func SetAttributes(sel interface{}, attributes map[string]string, opts ...QueryOption) Action {
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

// AttributeValue retrieves the element attribute value for the first node
// matching the selector.
func AttributeValue(sel interface{}, name string, value *string, ok *bool, opts ...QueryOption) Action {
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

// SetAttributeValue sets the element attribute with name to value for the
// first node matching the selector.
func SetAttributeValue(sel interface{}, name, value string, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return dom.SetAttributeValue(nodes[0].NodeID, name, value).Do(ctx)
	}, opts...)
}

// RemoveAttribute removes the element attribute with name from the first node
// matching the selector.
func RemoveAttribute(sel interface{}, name string, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return dom.RemoveAttribute(nodes[0].NodeID, name).Do(ctx)
	}, opts...)
}

// JavascriptAttribute retrieves the Javascript attribute for the first node
// matching the selector.
func JavascriptAttribute(sel interface{}, name string, res interface{}, opts ...QueryOption) Action {
	if res == nil {
		panic("res cannot be nil")
	}
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return EvaluateAsDevTools(snippet(attributeJS, cashX(true), sel, nodes[0], name), res).Do(ctx)
	}, opts...)
}

// SetJavascriptAttribute sets the javascript attribute for the first node
// matching the selector.
func SetJavascriptAttribute(sel interface{}, name, value string, opts ...QueryOption) Action {
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

// OuterHTML retrieves the outer html of the first node matching the selector.
func OuterHTML(sel interface{}, html *string, opts ...QueryOption) Action {
	if html == nil {
		panic("html cannot be nil")
	}
	return JavascriptAttribute(sel, "outerHTML", html, opts...)
}

// InnerHTML retrieves the inner html of the first node matching the selector.
func InnerHTML(sel interface{}, html *string, opts ...QueryOption) Action {
	if html == nil {
		panic("html cannot be nil")
	}
	return JavascriptAttribute(sel, "innerHTML", html, opts...)
}

// Click sends a mouse click event to the first node matching the selector.
func Click(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return MouseClickNode(nodes[0]).Do(ctx)
	}, append(opts, NodeVisible)...)
}

// DoubleClick sends a mouse double click event to the first node matching the
// selector.
func DoubleClick(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return MouseClickNode(nodes[0], ClickCount(2)).Do(ctx)
	}, append(opts, NodeVisible)...)
}

// SendKeys synthesizes the key up, char, and down events as needed for the
// runes in v, sending them to the first node matching the selector.
//
// Note: when selector matches a input[type="file"] node, then dom.SetFileInputFiles
// is used to set the upload path of the input node to v.
func SendKeys(sel interface{}, v string, opts ...QueryOption) Action {
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

		return KeyActionNode(n, v).Do(ctx)
	}, append(opts, NodeVisible)...)
}

// SetUploadFiles sets the files to upload (ie, for a input[type="file"] node)
// for the first node matching the selector.
func SetUploadFiles(sel interface{}, files []string, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctx context.Context, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		return dom.SetFileInputFiles(files).WithNodeID(nodes[0].NodeID).Do(ctx)
	}, opts...)
}

// Screenshot takes a screenshot of the first node matching the selector.
func Screenshot(sel interface{}, picbuf *[]byte, opts ...QueryOption) Action {
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

// Submit is an action that submits the form of the first node matching the
// selector belongs to.
func Submit(sel interface{}, opts ...QueryOption) Action {
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

// Reset is an action that resets the form of the first node matching the
// selector belongs to.
func Reset(sel interface{}, opts ...QueryOption) Action {
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

// ComputedStyle retrieves the computed style of the first node matching the selector.
func ComputedStyle(sel interface{}, style *[]*css.ComputedProperty, opts ...QueryOption) Action {
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

// MatchedStyle retrieves the matched style information for the first node
// matching the selector.
func MatchedStyle(sel interface{}, style **css.GetMatchedStylesForNodeReturns, opts ...QueryOption) Action {
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

// ScrollIntoView scrolls the window to the first node matching the selector.
func ScrollIntoView(sel interface{}, opts ...QueryOption) Action {
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
