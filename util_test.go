package chromedp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
)

// dumpJS is a javascript snippet that dumps the passed element node in the
// same text format as cdproto/cdp/Node.Dump.
//
// Used to verify that the DOM tree built by chromedp is the same as what
// chrome itself sees.
//
// Note: written to be "line-by-line equivalent" with Node.Dump's
// implementation.
const dumpJS = `(function dump(n, prefix, indent, nodeIDs) {
	if (n === null || typeof n !== 'object') {
		return prefix + '<nil>';
	}

	var s = '';
	if (typeof n.localName !== 'undefined') {
		s = n.localName;
	}
	if (s === '' && typeof n.nodeName !== 'undefined') {
		s = n.nodeName;
	}

	if (s === '') {
		throw 'invalid node element';
	}

	if (typeof n.attributes !== 'undefined') {
		for (var i = 0; i < n.attributes.length; i++) {
			if (n.attributes[i].name.toLowerCase() === 'id') {
				s += '#' + n.attributes[i].value;
				break;
			}
		}
	}

	if (n.nodeType !== 1 && n.nodeType !== 3) {
		s += ' <' + [
			'Element',
			'Attribute',
			'Text',
			'CDATA',
			'EntityReference',
			'Entity',
			'ProcessingInstruction',
			'Comment',
			'Document',
			'DocumentType',
			'DocumentFragment',
			'Notation'
		][n.nodeType - 1] + '>';
	}

	if (n.nodeType === 3) {
		var v = n.nodeValue;
		if (v.length > 15) {
			v = v.substring(0, 15) + '...';
		}
		s += ' ' + JSON.stringify(v);
	}

	if (n.nodeType === 1 && typeof n.attributes !== 'undefined' && n.attributes.length > 0) {
		var attrs = '';
		for (var i = 0; i <  n.attributes.length; i++) {
			if (n.attributes[i].name.toLowerCase() === 'id') {
				continue;
			}
			if (attrs !== '') {
				attrs += ' ';
			}
			attrs += n.attributes[i].name + '=' + JSON.stringify(n.attributes[i].value);
		}
		if (attrs != '') {
			s += ' [' + attrs + ']';
		}
	}

	if (nodeIDs) {
		throw 'cannot read element node ID from scripts';
	}

	if (typeof n.childNodes !== 'undefined' && n.childNodes.length > 0) {
		for (var i = 0; i < n.childNodes.length; i++)	{
			// skip empty #text nodes
			if (n.childNodes[i].nodeType === 3 && n.childNodes[i].nodeValue.trim() === '') {
				continue;
			}

			s += '\n' + dump(n.childNodes[i], prefix+indent, indent, nodeIDs);
		}
	}

	return prefix + s;
})(%s, %q, %q, %t);`

const insertJS = `(function(n, typ, id, text) {
	var el = document.createElement(typ);
	el.id = id;
	el.innerText = text;
	%s;
})(document.querySelector(%q), %q, %q, %q)`

// test:
//   insertBefore
//   removeChild
//   appendChild
//   replaceChild
//   prepend
//   append
//   insertAdjacentElement
func TestNodeOp(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(res, `<!doctype html>
<html>
  <head>
    <title>empty test page</title>
  </head>
  <body>
    <div id="div1">div1 content</div>
  </body>
<html>`)
	}))
	defer s.Close()

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// get document root
	var nodes []*cdp.Node
	if err := Run(ctx,
		Navigate(s.URL),
		WaitVisible(`#div1`),
		Nodes(`//*`, &nodes),
		Nodes(`document`, &nodes, ByJSPath),
	); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		q, expr string
	}{
		{`body`, `n.insertBefore(el, n.childNodes[0])`},
		{`body`, `n.insertBefore(el, n.childNodes[1])`},
		{`#div3`, `n.prepend(el)`},
		{`#div2`, `n.append(el)`},
		{`#div2`, `n.appendChild(el)`},
		{`#div3`, `n.removeChild(n.childNodes[0])`},
		{`#div2`, `n.insertAdjacentElement('afterend', el)`},
		{`body`, `n.insertBefore(el, n.childNodes[0])`},
		{`body`, `n.insertBefore(el, n.childNodes[1])`},
		{`#div2`, `n.replaceChild(el, n.childNodes[1])`},
	}

	prev := nodes[0].Dump("", "  ", false)
	for i, test := range tests {
		// modify tree
		if err := Run(ctx,
			ActionFunc(func(ctx context.Context) error {
				id := strconv.Itoa(i + 2)
				expr := fmt.Sprintf(insertJS, test.expr, test.q, `div`, `div`+id, `div`+id+` content`)
				_, exp, err := runtime.Evaluate(expr).Do(ctx)
				if err != nil {
					return err
				}
				if exp != nil {
					return exp
				}
				return nil
			}),
		); err != nil {
			t.Fatal(err)
		}

		// wait for events to propagate
		time.Sleep(5 * time.Millisecond)

		tree := nodes[0].Dump("", "  ", false)
		if prev == tree {
			t.Fatalf("test %d expected tree to change (prev == tree)\n-- PREV:\n%s\n-- TREE:\n%s\n--\n", i, prev, tree)
		}

		// retrieve browser's tree view
		var exp string
		if err := Run(ctx,
			EvaluateAsDevTools(fmt.Sprintf(dumpJS, `document`, "", "  ", false), &exp),
		); err != nil {
			t.Fatal(err)
		}

		if exp != tree {
			t.Errorf("test %d expected tree and node tree do not match:\n-- EXPECTED:\n%s\n-- GOT:\n%s\n--\n", i, exp, tree)
		}

		prev = tree

		// t.Logf("test %d:\n%s\n--\n", i, tree)
	}
}
