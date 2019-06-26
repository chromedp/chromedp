package chromedp

import (
	"bytes"
	"fmt"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"

	"github.com/chromedp/chromedp/kb"
)

func TestWaitReady(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodeIDs []cdp.NodeID
	if err := Run(ctx, NodeIDs("#input2", &nodeIDs, ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	var value string
	if err := Run(ctx,
		WaitReady("#input2", ByID),
		Value(nodeIDs, &value, ByNodeID),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitVisible(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodeIDs []cdp.NodeID
	if err := Run(ctx, NodeIDs("#input2", &nodeIDs, ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	var value string
	if err := Run(ctx,
		WaitVisible("#input2", ByID),
		Value(nodeIDs, &value, ByNodeID),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitNotVisible(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodeIDs []cdp.NodeID
	if err := Run(ctx, NodeIDs("#input2", &nodeIDs, ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	var value string
	if err := Run(ctx,
		Click("#button2", ByID),
		WaitNotVisible("#input2", ByID),
		Value(nodeIDs, &value, ByNodeID),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitEnabled(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var attr string
	var ok bool
	if err := Run(ctx, AttributeValue("#select1", "disabled", &attr, &ok, ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if !ok {
		t.Fatal("expected element to be disabled")
	}
	if err := Run(ctx,
		Click("#button3", ByID),
		WaitEnabled("#select1", ByID),
		AttributeValue("#select1", "disabled", &attr, &ok, ByID),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if ok {
		t.Fatal("expected element to be enabled")
	}
	var value string
	if err := Run(ctx,
		SetAttributeValue(`//*[@id="select1"]/option[1]`, "selected", "true"),
		Value("#select1", &value, ByID),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if value != "foo" {
		t.Fatalf("expected value to be foo, got: %s", value)
	}
}

func TestWaitSelected(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	if err := Run(ctx,
		Click("#button3", ByID),
		WaitEnabled("#select1", ByID),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}

	var attr string
	ok := false
	if err := Run(ctx, AttributeValue(`//*[@id="select1"]/option[1]`, "selected", &attr, &ok)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if ok {
		t.Fatal("expected element to be not selected")
	}
	if err := Run(ctx,
		SetAttributeValue(`//*[@id="select1"]/option[1]`, "selected", "true"),
		WaitSelected(`//*[@id="select1"]/option[1]`),
		AttributeValue(`//*[@id="select1"]/option[1]`, "selected", &attr, nil),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if attr != "true" {
		t.Fatal("expected element to be selected")
	}
}

func TestWaitNotPresent(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	if err := Run(ctx,
		WaitVisible("#input3", ByID),
		Click("#button4", ByID),
		WaitNotPresent("#input3", ByID),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestAtLeast(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodes []*cdp.Node
	if err := Run(ctx, Nodes("//input", &nodes, AtLeast(3))); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("expected to have at least 3 nodes: got %d", len(nodes))
	}
}

func TestByJSPath(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image2.html")
	defer cancel()

	// check nodes == 1
	var nodes []*cdp.Node
	if err := Run(ctx,
		Nodes(`document.querySelector('#imagething').shadowRoot.querySelector('.container')`, &nodes, ByJSPath),
	); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected nodes to have len 1, got: %d", len(nodes))
	}

	// check class
	class := nodes[0].AttributeValue("class")
	if class != "container" {
		t.Errorf("expected class to be 'container', got: %q", class)
	}
}

func TestNodes(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "table.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
		n   int
	}{
		{`/html/body/table/tbody[1]/tr[2]/td`, BySearch, 3},
		{`body > table > tbody:nth-child(2) > tr:nth-child(2) > td:not(:last-child)`, ByQueryAll, 2},
		{`body > table > tbody:nth-child(2) > tr:nth-child(2) > td`, ByQuery, 1},
		{`#footer`, ByID, 1},
		{`document.querySelector("body > table > tbody:nth-child(2) > tr:nth-child(2) > td:nth-child(1)")`, ByJSPath, 1},
	}

	for i, test := range tests {
		var nodes []*cdp.Node
		if err := Run(ctx, Nodes(test.sel, &nodes, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if len(nodes) != test.n {
			t.Errorf("test %d expected to have %d nodes: got %d", i, test.n, len(nodes))
		}
	}
}

func TestNodeIDs(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "table.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
		n   int
	}{
		{`/html/body/table/tbody[1]/tr[2]/td`, BySearch, 3},
		{`body > table > tbody:nth-child(2) > tr:nth-child(2) > td:not(:last-child)`, ByQueryAll, 2},
		{`body > table > tbody:nth-child(2) > tr:nth-child(2) > td`, ByQuery, 1},
		{`#footer`, ByID, 1},
		{`document.querySelector("body > table > tbody:nth-child(2) > tr:nth-child(2) > td:nth-child(1)")`, ByJSPath, 1},
	}

	for i, test := range tests {
		var ids []cdp.NodeID
		if err := Run(ctx, NodeIDs(test.sel, &ids, test.by)); err != nil {
			t.Fatal(err)
		}

		if len(ids) != test.n {
			t.Errorf("test %d expected to have %d node id's: got %d", i, test.n, len(ids))
		}
	}
}

func TestFocusBlur(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="input1"]`, BySearch},
		{`body > input[type="number"]:nth-child(1)`, ByQueryAll},
		{`body > input[type="number"]:nth-child(1)`, ByQuery},
		{`#input1`, ByID},
		{`document.querySelector("#input1")`, ByJSPath},
	}

	if err := Run(ctx, Click("#input1", ByID)); err != nil {
		t.Fatal(err)
	}

	for i, test := range tests {
		var value string
		if err := Run(ctx,
			Focus(test.sel, test.by),
			Value(test.sel, &value, test.by),
		); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if value != "9999" {
			t.Errorf("test %d expected value is '9999', got: %q", i, value)
		}
		if err := Run(ctx,
			Blur(test.sel, test.by),
			Value(test.sel, &value, test.by),
		); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if value != "0" {
			t.Errorf("test %d expected value is '0', got: %q", i, value)
		}
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []struct {
		sel    string
		by     QueryOption
		width  int64
		height int64
	}{
		{`/html/body/img`, BySearch, 239, 239},
		{`img`, ByQueryAll, 239, 239},
		{`img`, ByQuery, 239, 239},
		{`#icon-github`, ByID, 120, 120},
		{`document.querySelector("#icon-github")`, ByJSPath, 120, 120},
	}

	for i, test := range tests {
		var model *dom.BoxModel
		if err := Run(ctx, Dimensions(test.sel, &model, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if model.Height != test.height || model.Width != test.width {
			t.Errorf("test %d expected %dx%d, got: %dx%d", i, test.width, test.height, model.Height, model.Width)
		}
	}
}

func TestText(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
		exp string
	}{
		{"#foo", ByID, "insert"},
		{"body > form > span", ByQueryAll, "insert"},
		{"body > form > span:nth-child(2)", ByQuery, "keyword"},
		{"/html/body/form/span[2]", BySearch, "keyword"},
		{`document.querySelector("#form > span:nth-child(2)")`, ByJSPath, "keyword"},
		{"#inner-hidden", ByID, "this is"},
		{"#hidden", ByID, ""},
	}

	for i, test := range tests {
		var text string
		if err := Run(ctx, Text(test.sel, &text, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if text != test.exp {
			t.Errorf("test %d expected %q, got: %s", i, test.exp, text)
		}
	}
}

func TestTextContent(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
		exp string
	}{
		{"#inner-hidden", ByID, "this is hidden"},
		{"#hidden", ByID, "hidden"},
	}

	for i, test := range tests {
		var text string
		if err := Run(ctx, TextContent(test.sel, &text, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if text != test.exp {
			t.Errorf("test %d expected %q, got: %s", i, test.exp, text)
		}
	}
}

func TestClear(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		// input fields
		{`//*[@id="form"]/input[1]`, BySearch},
		{`#form > input[type="text"]:nth-child(4)`, ByQuery},
		{`#form > input[type="text"]`, ByQueryAll},
		{`#keyword`, ByID},
		{`document.querySelector("#keyword")`, ByJSPath},

		// textarea fields
		{`//*[@id="bar"]`, BySearch},
		{`#form > textarea`, ByQuery},
		{`#form > textarea`, ByQueryAll},
		{`#bar`, ByID},

		// input + textarea fields
		{`//*[@id="form"]/input`, BySearch},
		{`#form > input[type="text"]`, ByQueryAll},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "form.html")
			defer cancel()

			var val string
			if err := Run(ctx, Value(test.sel, &val, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}
			if val == "" {
				t.Errorf("expected %q to have non empty value", test.sel)
			}
			if err := Run(ctx,
				Clear(test.sel, test.by),
				Value(test.sel, &val, test.by),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}
			if val != "" {
				t.Errorf("expected empty value for %q, got: %s", test.sel, val)
			}
		})
	}
}

func TestReset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel   string
		by    QueryOption
		value string
		exp   string
	}{
		{`//*[@id="keyword"]`, BySearch, "foobar", "chromedp"},
		{`#form > input[type="text"]:nth-child(6)`, ByQuery, "foobar", "foo"},
		{`#form > input[type="text"]`, ByQueryAll, "foobar", "chromedp"},
		{"#bar", ByID, "foobar", "bar"},
		{`document.querySelector("#bar")`, ByJSPath, "foobar", "bar"},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "form.html")
			defer cancel()

			var value string
			if err := Run(ctx,
				SetValue(test.sel, test.value, test.by),
				Reset(test.sel, test.by),
				Value(test.sel, &value, test.by),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if value != test.exp {
				t.Errorf("expected value after reset is %s, got: %q", test.exp, value)
			}
		})
	}
}

func TestValue(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="form"]/input[1]`, BySearch},
		{`#form > input[type="text"]:nth-child(4)`, ByQuery},
		{`#form > input[type="text"]`, ByQueryAll},
		{`#keyword`, ByID},
		{`document.querySelector("#keyword")`, ByJSPath},
	}

	for i, test := range tests {
		var value string
		if err := Run(ctx, Value(test.sel, &value, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if value != "chromedp" {
			t.Errorf("test %d expected `chromedp`, got: %s", i, value)
		}
	}
}

func TestValueUndefined(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	var value string
	err := Run(ctx, Value("foo", &value, ByID))
	want := `could not retrieve attribute "value": encountered an undefined value`
	got := fmt.Sprint(err)
	if !strings.Contains(got, want) {
		t.Fatalf("want error %q, got %q", want, got)
	}
}

func TestSetValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="form"]/input[1]`, BySearch},
		{`#form > input[type="text"]:nth-child(4)`, ByQuery},
		{`#form > input[type="text"]`, ByQueryAll},
		{`#bar`, ByID},
		{`document.querySelector("#bar")`, ByJSPath},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "form.html")
			defer cancel()

			var value string
			if err := Run(ctx,
				SetValue(test.sel, "FOOBAR", test.by),
				Value(test.sel, &value, test.by),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if value != "FOOBAR" {
				t.Errorf("expected `FOOBAR`, got: %s", value)
			}
		})
	}
}

func TestAttributes(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
		exp map[string]string
	}{
		{
			`//*[@id="icon-brankas"]`, BySearch,
			map[string]string{
				"alt": "Brankas - Easy Money Management",
				"id":  "icon-brankas",
				"src": "images/brankas.png",
			},
		},
		{
			`body > img:first-child`, ByQuery,
			map[string]string{
				"alt": "Brankas - Easy Money Management",
				"id":  "icon-brankas",
				"src": "images/brankas.png",
			},
		},
		{
			`body > img:nth-child(2)`, ByQueryAll,
			map[string]string{
				"alt": `How people build software`,
				"id":  "icon-github",
				"src": "images/github.png",
			},
		},
		{
			`#icon-github`, ByID,
			map[string]string{
				"alt": "How people build software",
				"id":  "icon-github",
				"src": "images/github.png",
			},
		},
		{
			`document.querySelector("#icon-github")`, ByJSPath,
			map[string]string{
				"alt": "How people build software",
				"id":  "icon-github",
				"src": "images/github.png",
			},
		},
	}

	for i, test := range tests {
		var attrs map[string]string
		if err := Run(ctx, Attributes(test.sel, &attrs, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if !reflect.DeepEqual(test.exp, attrs) {
			t.Errorf("test %d expected %v, got: %v", i, test.exp, attrs)
		}
	}
}

func TestAttributesAll(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
		exp []map[string]string
	}{
		{
			"img", ByQueryAll,
			[]map[string]string{
				{
					"alt": "Brankas - Easy Money Management",
					"id":  "icon-brankas",
					"src": "images/brankas.png",
				},
				{
					"alt": "How people build software",
					"id":  "icon-github",
					"src": "images/github.png",
				},
			},
		},
	}

	for i, test := range tests {
		var attrs []map[string]string
		if err := Run(ctx, AttributesAll(test.sel, &attrs, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if !reflect.DeepEqual(test.exp, attrs) {
			t.Errorf("test %d expected %v, got: %v", i, test.exp, attrs)
		}
	}
}

func TestSetAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel   string
		by    QueryOption
		attrs map[string]string
		exp   map[string]string
	}{
		{
			`//*[@id="icon-brankas"]`, BySearch,
			map[string]string{"data-url": "brankas"},
			map[string]string{
				"alt":      "Brankas - Easy Money Management",
				"id":       "icon-brankas",
				"src":      "images/brankas.png",
				"data-url": "brankas",
			},
		},
		{
			`body > img:first-child`, ByQuery,
			map[string]string{"data-url": "brankas"},
			map[string]string{
				"alt":      "Brankas - Easy Money Management",
				"id":       "icon-brankas",
				"src":      "images/brankas.png",
				"data-url": "brankas",
			},
		},
		{
			`body > img:nth-child(2)`, ByQueryAll,
			map[string]string{"width": "100", "height": "200"},
			map[string]string{
				"alt":    `How people build software`,
				"id":     "icon-github",
				"src":    "images/github.png",
				"width":  "100",
				"height": "200",
			},
		},
		{
			`#icon-github`, ByID,
			map[string]string{"width": "100", "height": "200"},
			map[string]string{
				"alt":    "How people build software",
				"id":     "icon-github",
				"src":    "images/github.png",
				"width":  "100",
				"height": "200",
			},
		},
		{
			`document.querySelector("#icon-github")`, ByJSPath,
			map[string]string{"width": "100", "height": "200"},
			map[string]string{
				"alt":    "How people build software",
				"id":     "icon-github",
				"src":    "images/github.png",
				"width":  "100",
				"height": "200",
			},
		},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "image.html")
			defer cancel()

			if err := Run(ctx, SetAttributes(test.sel, test.attrs, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}

			// TODO: figure why this test is flaky without this
			time.Sleep(10 * time.Millisecond)

			var attrs map[string]string
			if err := Run(ctx, Attributes(test.sel, &attrs, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if !reflect.DeepEqual(test.exp, attrs) {
				t.Errorf("expected %v, got: %v", test.exp, attrs)
			}
		})
	}
}

func TestAttributeValue(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []struct {
		sel  string
		by   QueryOption
		attr string
		exp  string
	}{
		{`//*[@id="icon-brankas"]`, BySearch, "alt", "Brankas - Easy Money Management"},
		{`body > img:first-child`, ByQuery, "alt", "Brankas - Easy Money Management"},
		{`body > img:nth-child(2)`, ByQueryAll, "alt", "How people build software"},
		{`#icon-github`, ByID, "alt", "How people build software"},
		{`document.querySelector('#icon-github')`, ByJSPath, "alt", "How people build software"},
	}

	for i, test := range tests {
		var value string
		var ok bool
		if err := Run(ctx, AttributeValue(test.sel, test.attr, &value, &ok, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if !ok {
			t.Fatalf("test %d failed to get attribute %s on %s", i, test.attr, test.sel)
		}
		if value != test.exp {
			t.Errorf("test %d expected %s to be %s, got: %s", i, test.attr, test.exp, value)
		}
	}
}

func TestSetAttributeValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel  string
		by   QueryOption
		attr string
		exp  string
	}{
		{`//*[@id="keyword"]`, BySearch, "foo", "bar"},
		{`#form > input[type="text"]:nth-child(6)`, ByQuery, "foo", "bar"},
		{`#form > input[type="text"]`, ByQueryAll, "foo", "bar"},
		{`#bar`, ByID, "foo", "bar"},
		{`document.querySelector('#bar')`, ByJSPath, "foo", "bar"},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "form.html")
			defer cancel()

			if err := Run(ctx, SetAttributeValue(test.sel, test.attr, test.exp, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}

			// TODO: figure why this test is flaky without this
			time.Sleep(10 * time.Millisecond)

			var value string
			var ok bool
			if err := Run(ctx, AttributeValue(test.sel, test.attr, &value, &ok, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}
			if !ok {
				t.Fatalf("failed to get attribute %s on %s", test.attr, test.sel)
			}
			if value != test.exp {
				t.Errorf("expected %s to be %s, got: %s", test.attr, test.exp, value)
			}
		})
	}
}

func TestRemoveAttribute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel  string
		by   QueryOption
		attr string
	}{
		{`/html/body/img`, BySearch, "alt"},
		{`img`, ByQueryAll, "alt"},
		{`img`, ByQuery, "alt"},
		{`#icon-github`, ByID, "alt"},
		{`document.querySelector('#icon-github')`, ByJSPath, "alt"},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "image.html")
			defer cancel()

			if err := Run(ctx, RemoveAttribute(test.sel, test.attr, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}

			// TODO: figure why this test is flaky without this
			time.Sleep(10 * time.Millisecond)

			var value string
			var ok bool
			if err := Run(ctx, AttributeValue(test.sel, test.attr, &value, &ok, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}
			if ok || value != "" {
				t.Fatalf("expected attribute %s removed from element %s", test.attr, test.sel)
			}
		})
	}
}

func TestClick(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="form"]/input[4]`, BySearch},
		{`#form > input[type="submit"]:nth-child(11)`, ByQuery},
		{`#form > input[type="submit"]:nth-child(11)`, ByQueryAll},
		{`#btn2`, ByID},
		{`document.querySelector('#btn2')`, ByJSPath},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "form.html")
			defer cancel()

			var title string
			if err := Run(ctx,
				Click(test.sel, test.by),
				WaitVisible("#icon-brankas", ByID),
				Title(&title),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if title != "this is title" {
				t.Errorf("expected title to be 'chromedp - Google Search', got: %q", title)
			}
		})
	}
}

func TestDoubleClick(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`/html/body/input[2]`, BySearch},
		{`body > input[type="button"]:nth-child(2)`, ByQueryAll},
		{`body > input[type="button"]:nth-child(2)`, ByQuery},
		{`#button1`, ByID},
		{`document.querySelector('#button1')`, ByJSPath},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "js.html")
			defer cancel()

			var value string
			if err := Run(ctx,
				DoubleClick(test.sel, test.by),
				Value("#input1", &value, ByID),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if value != "1" {
				t.Errorf("expected value to be '1', got: %q", value)
			}
		})
	}
}

func TestSendKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel  string
		by   QueryOption
		keys string
		exp  string
	}{
		{`//*[@id="input1"]`, BySearch, "INSERT ", "INSERT some value"}, // 0
		{`#box4 > input:nth-child(1)`, ByQuery, "insert ", "insert some value"},
		{`#box4 > textarea`, ByQueryAll, "prefix " + kb.End + "\b\b SUFFIX\n", "prefix textar SUFFIX\n"},
		{`#textarea1`, ByID, "insert ", "insert textarea"},
		{`#textarea1`, ByID, kb.End + "\b\b\n\naoeu\n\nfoo\n\nbar\n\n", "textar\n\naoeu\n\nfoo\n\nbar\n\n"},
		{`#select1`, ByID, kb.ArrowDown + kb.ArrowDown, "three"}, // 5
		{`document.querySelector('#textarea1')`, ByJSPath, "insert ", "insert textarea"},
	}

	for i, test := range tests {
		i, test := i, test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			if runtime.GOOS == "darwin" && i == 5 {
				t.Skipf("skipping test %d on darwin -- FIXME!", i)
			}

			t.Parallel()

			ctx, cancel := testAllocate(t, "visible.html")
			defer cancel()

			var val string
			if err := Run(ctx,
				SendKeys(test.sel, test.keys, test.by),
				Value(test.sel, &val, test.by),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if val != test.exp {
				t.Errorf("expected value %s, got: %s", test.exp, val)
			}
		})
	}
}

func TestScreenshot(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image2.html")
	defer cancel()

	tests := []struct {
		sel  string
		by   QueryOption
		size int
	}{
		{`/html/body/img`, BySearch, 239},
		{`img`, ByQueryAll, 239},
		{`#icon-github`, ByID, 120},
		{`document.querySelector('#imagething').shadowRoot.querySelector('.container')`, ByJSPath, 190},
	}

	// a smaller viewport speeds up this test
	if err := Run(ctx, EmulateViewport(600, 400)); err != nil {
		t.Fatal(err)
	}

	for i, test := range tests {
		var buf []byte
		if err := Run(ctx, Screenshot(test.sel, &buf, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if len(buf) == 0 {
			t.Fatalf("test %d failed to capture screenshot", i)
		}
		img, err := png.Decode(bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		size := img.Bounds().Size()
		if size.X != test.size || size.Y != test.size {
			t.Errorf("expected dimensions to be %d*%d, got %d*%d",
				test.size, test.size, size.X, size.Y)
		}
	}
}

func TestScreenshotHighDPI(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	// Use a weird screen dimension with a 1.5 scale factor, so that
	// cropping the screenshot is forced to use floating point arithmetic
	// and keep the high DPI in mind.
	if err := Run(ctx, EmulateViewport(605, 405, EmulateScale(1.5))); err != nil {
		t.Fatal(err)
	}

	var buf []byte
	if err := Run(ctx, Screenshot("#half-color", &buf, ByID)); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	size := img.Bounds().Size()
	wantSize := 300 // 200px at 1.5 scaling factor
	if size.X != wantSize || size.Y != wantSize {
		t.Fatalf("expected dimensions to be %d*%d, got %d*%d",
			wantSize, wantSize, size.X, size.Y)
	}
	wantColor := func(x, y int, r, g, b, a uint32) {
		color := img.At(x, y)
		r_, g_, b_, a_ := color.RGBA()
		if r_ != r || g_ != g || b_ != b || a_ != a {
			t.Errorf("got 0x%04x%04x%04x%04x at (%d,%d), want 0x%04x%04x%04x%04x",
				r_, g_, b_, a_, x, y, r, g, b, a)
		}
	}
	// The left half is blue.
	wantColor(5, 5, 0x0, 0x0, 0xffff, 0xffff)
	wantColor(5, 295, 0x0, 0x0, 0xffff, 0xffff)
	// The right half is red.
	wantColor(295, 5, 0xffff, 0x0, 0x0, 0xffff)
	wantColor(295, 295, 0xffff, 0x0, 0x0, 0xffff)
}

func TestSubmit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="keyword"]`, BySearch},
		{`#form > input[type="text"]:nth-child(4)`, ByQuery},
		{`#form > input[type="text"]`, ByQueryAll},
		{`#form`, ByID},
		{`document.querySelector('#form')`, ByJSPath},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "form.html")
			defer cancel()

			var title string
			if err := Run(ctx,
				Submit(test.sel, test.by),
				WaitVisible("#icon-brankas", ByID),
				Title(&title),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			if title != "this is title" {
				t.Errorf("expected title to be 'this is title', got: %q", title)
			}
		})
	}
}

func TestComputedStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="input1"]`, BySearch},
		{`body > input[type="number"]:nth-child(1)`, ByQueryAll},
		{`body > input[type="number"]:nth-child(1)`, ByQuery},
		{`#input1`, ByID},
		{`document.querySelector('#input1')`, ByJSPath},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "js.html")
			defer cancel()

			var styles []*css.ComputedProperty
			if err := Run(ctx, ComputedStyle(test.sel, &styles, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}

			for _, style := range styles {
				if style.Name == "background-color" {
					if style.Value != "rgb(255, 0, 0)" {
						t.Logf("expected style 'rgb(255, 0, 0)' got: %s", style.Value)
					}
				}
			}
			if err := Run(ctx,
				Click("#input1", ByID),
				ComputedStyle(test.sel, &styles, test.by),
			); err != nil {
				t.Fatalf("got error: %v", err)
			}

			for _, style := range styles {
				if style.Name == "background-color" {
					if style.Value != "rgb(255, 255, 0)" {
						t.Fatalf("expected style 'rgb(255, 255, 0)' got: %s", style.Value)
					}
				}
			}
		})
	}
}

func TestMatchedStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="input1"]`, BySearch},
		{`body > input[type="number"]:nth-child(1)`, ByQueryAll},
		{`body > input[type="number"]:nth-child(1)`, ByQuery},
		{`#input1`, ByID},
		{`document.querySelector('#input1')`, ByJSPath},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "js.html")
			defer cancel()

			var styles *css.GetMatchedStylesForNodeReturns
			if err := Run(ctx, MatchedStyle(test.sel, &styles, test.by)); err != nil {
				t.Fatalf("got error: %v", err)
			}

			// TODO: Add logic to check if the style returned is true and valid.
		})
	}
}

func TestFileUpload(t *testing.T) {
	t.Parallel()

	// create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(res, "%s", uploadHTML)
	})
	mux.HandleFunc("/upload", func(res http.ResponseWriter, req *http.Request) {
		f, _, err := req.FormFile("upload")
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()

		buf, err := ioutil.ReadAll(f)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Fprintf(res, resultHTML, len(buf))
	})
	s := httptest.NewServer(mux)
	defer s.Close()

	// create temporary file on disk
	tmpfile, err := ioutil.TempFile("", "chromedp-upload-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()
	if _, err := tmpfile.WriteString(uploadHTML); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		a Action
	}{
		{SendKeys(`input[name="upload"]`, tmpfile.Name(), NodeVisible)},
		{SetUploadFiles(`input[name="upload"]`, []string{tmpfile.Name()}, NodeVisible)},
	}

	// Don't run these tests in parallel. The only way to do so would be to
	// fire a separate httptest server and tmpfile for each. There's no way
	// to share these resources easily among parallel subtests, as the
	// parent must finish for the children to run, made impossible by the
	// defers above.
	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			var result string
			if err := Run(ctx,
				Navigate(s.URL),
				test.a,
				Click(`input[name="submit"]`),
				Text(`#result`, &result, ByID, NodeVisible),
			); err != nil {
				t.Fatalf("test %d expected no error, got: %v", i, err)
			}

			if result != fmt.Sprintf("%d", len(uploadHTML)) {
				t.Errorf("test %d expected result to be %d, got: %s", i, len(uploadHTML), result)
			}
		})
	}
}

func TestInnerHTML(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "table.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`/html/body/table/thead`, BySearch},
		{`thead`, ByQueryAll},
		{`thead`, ByQuery},
		{`document.querySelector("#footer > td:nth-child(2)")`, ByJSPath},
	}
	for i, test := range tests {
		var html string
		if err := Run(ctx, InnerHTML(test.sel, &html, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if html == "" {
			t.Fatalf("test %d: InnerHTML is empty", i)
		}
	}
}

func TestOuterHTML(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "table.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`/html/body/table/thead/tr`, BySearch},
		{`thead tr`, ByQueryAll},
		{`thead tr`, ByQuery},
		{`document.querySelector("#footer > td:nth-child(2)")`, ByJSPath},
	}
	for i, test := range tests {
		var html string
		if err := Run(ctx, OuterHTML(test.sel, &html, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if html == "" {
			t.Fatalf("test %d: OuterHTML is empty", i)
		}
	}
}

func TestScrollIntoView(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`/html/body/img`, BySearch},
		{`img`, ByQueryAll},
		{`img`, ByQuery},
		{`#icon-github`, ByID},
		{`document.querySelector('#icon-github')`, ByJSPath},
	}
	for i, test := range tests {
		if err := Run(ctx, ScrollIntoView(test.sel, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		// TODO test scroll event
	}
}

func TestSVGFullXPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`#brankas`, ByQuery},
		{`text`, ByQueryAll},
		{`brankas`, ByID},
		{`//*[local-name()='text']`, BySearch},
		{`document.querySelector('#brankas')`, ByJSPath},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "svg.html")
			defer cancel()

			var nodes []*cdp.Node
			if err := Run(ctx, Nodes(test.sel, &nodes, test.by)); err != nil {
				t.Fatal(err)
			}

			if len(nodes) < 1 {
				t.Fatalf("expected at least 1 node, got: %d", len(nodes))
			}

			const exp = `/html[1]/body[1]/*[local-name()='svg'][1]/*[local-name()='text'][1]`
			xpath := nodes[0].FullXPath()
			if exp != xpath {
				t.Errorf("expected %q, got: %q", exp, xpath)
			}

			var text1, text2 string
			if err := Run(ctx, TextContent(test.sel, &text1, test.by)); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(text1) != "Brankas" {
				t.Errorf("expected %q, got: %q", "Brankas", text1)
			}
			if err := Run(ctx, TextContent(xpath, &text2)); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(text2) != "Brankas" {
				t.Errorf("expected %q, got: %q", "Brankas", text2)
			}
		})
	}
}

const (
	uploadHTML = `<!doctype html>
<html>
<body>
	<form method="POST" action="/upload" enctype="multipart/form-data">
		<input name="upload" type="file"/>
		<input name="submit" type="submit"/>
	</form>
</body>
</html>`

	resultHTML = `<!doctype html>
<html>
<body>
	<div id="result">%d</div>
</body>
</html>`
)

func TestWaitReadyReuseAction(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	// Reusing a single WaitReady action used to panic.
	action := WaitReady("#input2", ByID)
	for i := 0; i < 3; i++ {
		if err := Run(ctx, action); err != nil {
			t.Fatalf("got error: %v", err)
		}
	}
}
