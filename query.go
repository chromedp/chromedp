package chromedp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/dom"
	"github.com/knq/chromedp/cdp/input"
	"github.com/knq/chromedp/cdp/page"
	rundom "github.com/knq/chromedp/cdp/runtime"
)

var (
	// ErrInvalidBoxModel is the error returned when the retrieved box model is
	// invalid.
	ErrInvalidBoxModel = errors.New("invalid box model")
)

// Focus focuses the first element returned by the selector.
func Focus(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		return dom.Focus(nodes[0].NodeID).Do(ctxt, h)
	}, opts...)
}

// Clear clears input and textarea fields of their values.
func Clear(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		for _, n := range nodes {
			if n.NodeType != cdp.NodeTypeElement || (n.NodeName != "INPUT" && n.NodeName != "TEXTAREA") {
				return fmt.Errorf("selector `%s` matched node %d with name %s", sel, n.NodeID, strings.ToLower(n.NodeName))
			}
		}

		errs := make([]error, len(nodes))
		wg := new(sync.WaitGroup)
		for i, n := range nodes {
			wg.Add(1)
			go func(i int, n *cdp.Node) {
				defer wg.Done()

				var a Action
				if n.NodeName == "INPUT" {
					a = dom.SetAttributeValue(n.NodeID, "value", "")
				} else {
					a = dom.SetNodeValue(n.NodeID, "")
				}
				errs[i] = a.Do(ctxt, h)
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

// Nodes retrieves the DOM nodes matching the selector.
func Nodes(sel interface{}, nodes *[]*cdp.Node, opts ...QueryOption) Action {
	if nodes == nil {
		panic("nodes cannot be nil")
	}

	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, n ...*cdp.Node) error {
		*nodes = n
		return nil
	}, opts...)
}

// NodeIDs returns the node IDs of the matching selector.
func NodeIDs(sel interface{}, ids *[]cdp.NodeID, opts ...QueryOption) Action {
	if ids == nil {
		panic("nodes cannot be nil")
	}

	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		nodeIDs := make([]cdp.NodeID, len(nodes))
		for i, n := range nodes {
			nodeIDs[i] = n.NodeID
		}

		*ids = nodeIDs

		return nil
	}, opts...)
}

// Dimensions retrieves the box model dimensions for the first node matching
// the specified selector.
func Dimensions(sel interface{}, model **dom.BoxModel, opts ...QueryOption) Action {
	if model == nil {
		panic("model cannot be nil")
	}
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}
		var err error
		*model, err = dom.GetBoxModel(nodes[0].NodeID).Do(ctxt, h)
		return err
	}, opts...)
}

// Value retrieves the value of an element.
func Value(sel interface{}, value *string, opts ...QueryOption) Action {
	if value == nil {
		panic("value cannot be nil")
	}
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		p := rundom.Evaluate(fmt.Sprintf(valueJS, nodes[0].FullXPath()))
		p.IncludeCommandLineAPI = true
		p.ObjectGroup = "console"

		res, exp, err := p.Do(ctxt, h)
		if err != nil {
			return err
		}
		if exp != nil {
			return fmt.Errorf("got exception evaluating script: %#v", exp)
		}
		if res.Type != rundom.TypeString || len(res.Value) < 2 {
			return fmt.Errorf("expected string of at least length 2, got %s length %d", res.Subtype, len(res.Value))
		}

		*value = string(res.Value[1 : len(res.Value)-1])

		return nil
	}, opts...)
}

// SetValue sets the value of an element.
func SetValue(sel interface{}, value string, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		p := rundom.Evaluate(fmt.Sprintf(setValueJS, nodes[0].FullXPath(), value))
		p.IncludeCommandLineAPI = true
		p.ObjectGroup = "console"

		res, exp, err := p.Do(ctxt, h)
		if err != nil {
			return err
		}
		if exp != nil {
			return fmt.Errorf("got exception evaluating script: %#v", exp)
		}
		if res.Type != rundom.TypeString || len(res.Value) < 2 {
			return fmt.Errorf("expected string of at least length 2, got %s length %d", res.Subtype, len(res.Value))
		}

		return nil
	}, opts...)
}

// Text retrieves the text of the first element matching the selector.
func Text(sel interface{}, text *string, opts ...QueryOption) Action {
	if text == nil {
		panic("text cannot be nil")
	}
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		p := rundom.Evaluate(fmt.Sprintf(textJS, nodes[0].FullXPath()))
		p.IncludeCommandLineAPI = true
		p.ObjectGroup = "console"

		res, exp, err := p.Do(ctxt, h)
		if err != nil {
			return err
		}
		if exp != nil {
			return fmt.Errorf("got exception evaluating script: %#v", exp)
		}
		if res.Type != rundom.TypeString || len(res.Value) < 2 {
			return fmt.Errorf("expected string of at least length 2, got %s length %d", res.Subtype, len(res.Value))
		}

		*text = string(res.Value[1 : len(res.Value)-1])

		return nil
	}, opts...)
}

// Attributes retrieves the attributes for the specified element.
func Attributes(sel interface{}, attributes *map[string]string, opts ...QueryOption) Action {
	if attributes == nil {
		panic("attributes cannot be nil")
	}

	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
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

// SetAttributes sets the attributes for the specified element.
func SetAttributes(sel interface{}, attributes map[string]string, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return errors.New("expected at least one element")
		}

		return nil
	}, opts...)
}

// AttributeValue retrieves the name'd attribute value for the specified
// element.
func AttributeValue(sel interface{}, name string, value *string, ok *bool, opts ...QueryOption) Action {
	if value == nil {
		panic("value cannot be nil")
	}

	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
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

// SetAttributeValue sets an element's attribute with name to value.
func SetAttributeValue(sel interface{}, name, value string, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		return dom.SetAttributeValue(nodes[0].NodeID, name, value).Do(ctxt, h)
	}, opts...)
}

// RemoveAttribute removes an element's attribute with name.
func RemoveAttribute(sel interface{}, name string, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		return dom.RemoveAttribute(nodes[0].NodeID, name).Do(ctxt, h)
	}, opts...)
}

// Click sends a click to the first element returned by the selector.
func Click(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		return MouseActionNode(nodes[0], Button(input.ButtonLeft), ClickCount(1)).Do(ctxt, h)
	}, append(opts, ElementVisible)...)
}

// DoubleClick does a double click on the first element returned by selector.
func DoubleClick(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		return MouseActionNode(nodes[0], Button(input.ButtonLeft), ClickCount(2)).Do(ctxt, h)
	}, append(opts, ElementVisible)...)
}

// Hover hovers (moves) the mouse over the first element returned by the
// selector.
func Hover(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		return MouseActionNode(nodes[0], Button(input.ButtonNone)).Do(ctxt, h)
	}, append(opts, ElementVisible)...)
}

// SendKeys sends keys to the first element returned by selector.
func SendKeys(sel interface{}, v string, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}
		return KeyActionNode(nodes[0], v).Do(ctxt, h)
	}, append(opts, ElementVisible)...)
}

// Screenshot takes a screenshot of the first element matching the selector.
func Screenshot(sel interface{}, picbuf *[]byte, opts ...QueryOption) Action {
	if picbuf == nil {
		panic("picbuf cannot be nil")
	}

	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		var err error

		// get box model
		box, err := dom.GetBoxModel(nodes[0].NodeID).Do(ctxt, h)
		if err != nil {
			return err
		}

		// check box
		if len(box.Margin) != 8 {
			return ErrInvalidBoxModel
		}

		// evaluate scroll script
		res, exp, err := rundom.Evaluate(fmt.Sprintf(scrollJS, int64(box.Margin[0]), int64(box.Margin[1]))).Do(ctxt, h)
		if err != nil {
			return err
		}
		if exp != nil {
			return fmt.Errorf("got exception evaluating script: %#v", exp)
		}
		if res.Type != rundom.TypeString || len(res.Value) < 2 {
			return fmt.Errorf("expected string of at least length 2, got %s length %d", res.Subtype, len(res.Value))
		}

		// parse response
		var scroll []int
		err = json.Unmarshal([]byte(res.Value[1:len(res.Value)-1]), &scroll)
		if err != nil {
			return err
		}

		// take page screenshot
		buf, err := page.CaptureScreenshot().Do(ctxt, h)
		if err != nil {
			return err
		}

		// load image
		img, err := png.Decode(bytes.NewReader(buf))
		if err != nil {
			return err
		}

		// crop to box model contents.
		cropped := imaging.Crop(img, image.Rect(
			int(box.Margin[0])-scroll[0], int(box.Margin[1])-scroll[1],
			int(box.Margin[4])-scroll[0], int(box.Margin[5])-scroll[1],
		))

		// encode
		var croppedBuf bytes.Buffer
		err = png.Encode(&croppedBuf, cropped)
		if err != nil {
			return err
		}

		*picbuf = croppedBuf.Bytes()

		return nil
	}, append(opts, ElementVisible)...)
}

// Submit is an action that submits whatever form the first element belongs to.
func Submit(sel interface{}, opts ...QueryOption) Action {
	return QueryAfter(sel, func(ctxt context.Context, h cdp.FrameHandler, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector `%s` did not return any nodes", sel)
		}

		p := rundom.Evaluate(fmt.Sprintf(submitJS, nodes[0].FullXPath()))
		p.IncludeCommandLineAPI = true
		p.ObjectGroup = "console"

		res, exp, err := p.Do(ctxt, h)
		if err != nil {
			return err
		}
		if exp != nil {
			return fmt.Errorf("got exception evaluating script: %#v", exp)
		}
		if res.Type != rundom.TypeString || len(res.Value) < 2 {
			return fmt.Errorf("expected string of at least length 2, got %s length %d", res.Subtype, len(res.Value))
		}

		return nil
	}, opts...)
}

const (
	textJS = `(function(a) {
		var s = '';
		for (var i = 0; i < a.length; i++) {
			if (a[i].offsetParent !== null) {
				s += a[i].textContent;
			}
		}
		return s;
	})($x("%s/node()"))`

	scrollJS = `(function(x, y) {
		window.scrollTo(x, y);
		return '['+window.scrollX+','+window.scrollY+']';
	})(%d, %d)`

	submitJS = `(function(a) {
		if (a[0].form !== null) {
			return "" + a[0].form.submit();
		}
		return 'false';
	})($x('%s'))`

	valueJS = `(function(a) {
		return a[0].value;
	})($x('%s'))`

	setValueJS = `(function(a, val) {
		return a[0].value = val;
	})($x('%s'), '%s')`
)

/*

ScrollTo

Title
SetTitle
OuterHTML
SetOuterHTML

NodeName -- ?

Style(Matched)
Style(Computed)
SetStyle
GetStyle(Inline)

*/
