package chromedp

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"

	"github.com/chromedp/chromedp/kb"
)

func TestNodes(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "table.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
		len int
	}{
		{"/html/body/table/tbody[1]/tr[2]/td", BySearch, 3},
		{"body > table > tbody:nth-child(2) > tr:nth-child(2) > td:not(:last-child)", ByQueryAll, 2},
		{"body > table > tbody:nth-child(2) > tr:nth-child(2) > td", ByQuery, 1},
		{"#footer", ByID, 1},
	}

	var err error
	for i, test := range tests {
		var nodes []*cdp.Node
		err = c.Run(defaultContext, Nodes(test.sel, &nodes, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if len(nodes) != test.len {
			t.Errorf("test %d expected to have %d nodes: got %d", i, test.len, len(nodes))
		}
	}
}

func TestNodeIDs(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "table.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
		len int
	}{
		{"/html/body/table/tbody[1]/tr[2]/td", BySearch, 3},
		{"body > table > tbody:nth-child(2) > tr:nth-child(2) > td:not(:last-child)", ByQueryAll, 2},
		{"body > table > tbody:nth-child(2) > tr:nth-child(2) > td", ByQuery, 1},
		{"#footer", ByID, 1},
	}

	var err error
	for i, test := range tests {
		var ids []cdp.NodeID
		err = c.Run(defaultContext, NodeIDs(test.sel, &ids, test.by))
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) != test.len {
			t.Errorf("test %d expected to have %d node id's: got %d", i, test.len, len(ids))
		}
	}
}

func TestFocusBlur(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "js.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="input1"]`, BySearch},
		{`body > input[type="number"]:nth-child(1)`, ByQueryAll},
		{`body > input[type="number"]:nth-child(1)`, ByQuery},
		{"#input1", ByID},
	}

	err := c.Run(defaultContext, Click("#input1", ByID))
	if err != nil {
		t.Fatal(err)
	}

	for i, test := range tests {
		err = c.Run(defaultContext, Focus(test.sel, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		var value string
		err = c.Run(defaultContext, Value(test.sel, &value, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if value != "9999" {
			t.Errorf("test %d expected value is '9999', got: '%s'", i, value)
		}

		err = c.Run(defaultContext, Blur(test.sel, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		err = c.Run(defaultContext, Value(test.sel, &value, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if value != "0" {
			t.Errorf("test %d expected value is '0', got: '%s'", i, value)
		}
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "image.html")
	defer c.Release()

	tests := []struct {
		sel    string
		by     QueryOption
		width  int64
		height int64
	}{
		{"/html/body/img", BySearch, 239, 239},
		{"img", ByQueryAll, 239, 239},
		{"img", ByQuery, 239, 239},
		{"#icon-github", ByID, 120, 120},
	}

	var err error
	for i, test := range tests {
		var model *dom.BoxModel
		err = c.Run(defaultContext, Dimensions(test.sel, &model))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if model.Height != test.height || model.Width != test.width {
			t.Errorf("test %d expected %dx%d, got: %dx%d", i, test.width, test.height, model.Height, model.Width)
		}
	}
}

func TestText(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "form.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
		exp string
	}{
		{"#foo", ByID, "insert"},
		{"body > form > span", ByQueryAll, "insert"},
		{"body > form > span:nth-child(2)", ByQuery, "keyword"},
		{"/html/body/form/span[2]", BySearch, "keyword"},
	}

	var err error
	for i, test := range tests {
		var text string
		err = c.Run(defaultContext, Text(test.sel, &text, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if text != test.exp {
			t.Errorf("test %d expected `%s`, got: %s", i, test.exp, text)
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
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "form.html")
			defer c.Release()

			var val string
			err := c.Run(defaultContext, Value(test.sel, &val, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if val == "" {
				t.Errorf("expected `%s` to have non empty value", test.sel)
			}

			err = c.Run(defaultContext, Clear(test.sel, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			err = c.Run(defaultContext, Value(test.sel, &val, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if val != "" {
				t.Errorf("expected empty value for `%s`, got: %s", test.sel, val)
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
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "form.html")
			defer c.Release()

			err := c.Run(defaultContext, SetValue(test.sel, test.value, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			err = c.Run(defaultContext, Reset(test.sel, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			var value string
			err = c.Run(defaultContext, Value(test.sel, &value, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if value != test.exp {
				t.Errorf("expected value after reset is %s, got: '%s'", test.exp, value)
			}
		})
	}
}

func TestValue(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "form.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{`//*[@id="form"]/input[1]`, BySearch},
		{`#form > input[type="text"]:nth-child(4)`, ByQuery},
		{`#form > input[type="text"]`, ByQueryAll},
		{`#keyword`, ByID},
	}

	var err error
	for i, test := range tests {
		var value string
		err = c.Run(defaultContext, Value(test.sel, &value, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if value != "chromedp" {
			t.Errorf("test %d expected `chromedp`, got: %s", i, value)
		}
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
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "form.html")
			defer c.Release()

			err := c.Run(defaultContext, SetValue(test.sel, "FOOBAR", test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			var value string
			err = c.Run(defaultContext, Value(test.sel, &value, test.by))
			if err != nil {
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

	c := testAllocate(t, "image.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
		exp map[string]string
	}{
		{`//*[@id="icon-brankas"]`, BySearch,
			map[string]string{
				"alt": "Brankas - Easy Money Management",
				"id":  "icon-brankas",
				"src": "images/brankas.png",
			}},
		{"body > img:first-child", ByQuery,
			map[string]string{
				"alt": "Brankas - Easy Money Management",
				"id":  "icon-brankas",
				"src": "images/brankas.png",
			}},
		{"body > img:nth-child(2)", ByQueryAll,
			map[string]string{
				"alt": `How people build software`,
				"id":  "icon-github",
				"src": "images/github.png",
			}},
		{"#icon-github", ByID,
			map[string]string{
				"alt": "How people build software",
				"id":  "icon-github",
				"src": "images/github.png",
			}},
	}

	var err error
	for i, test := range tests {
		var attrs map[string]string
		err = c.Run(defaultContext, Attributes(test.sel, &attrs, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if !reflect.DeepEqual(test.exp, attrs) {
			t.Errorf("test %d expected %v, got: %v", i, test.exp, attrs)
		}
	}
}

func TestAttributesAll(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "image.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
		exp []map[string]string
	}{
		{"img", ByQueryAll,
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

	var err error
	for i, test := range tests {
		var attrs []map[string]string
		err = c.Run(defaultContext, AttributesAll(test.sel, &attrs, test.by))
		if err != nil {
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
		{`//*[@id="icon-brankas"]`, BySearch,
			map[string]string{"data-url": "brankas"},
			map[string]string{
				"alt":      "Brankas - Easy Money Management",
				"id":       "icon-brankas",
				"src":      "images/brankas.png",
				"data-url": "brankas"}},
		{"body > img:first-child", ByQuery,
			map[string]string{"data-url": "brankas"},
			map[string]string{
				"alt":      "Brankas - Easy Money Management",
				"id":       "icon-brankas",
				"src":      "images/brankas.png",
				"data-url": "brankas",
			}},
		{"body > img:nth-child(2)", ByQueryAll,
			map[string]string{"width": "100", "height": "200"},
			map[string]string{
				"alt":    `How people build software`,
				"id":     "icon-github",
				"src":    "images/github.png",
				"width":  "100",
				"height": "200",
			}},
		{"#icon-github", ByID,
			map[string]string{"width": "100", "height": "200"},
			map[string]string{
				"alt":    "How people build software",
				"id":     "icon-github",
				"src":    "images/github.png",
				"width":  "100",
				"height": "200",
			}},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "image.html")
			defer c.Release()

			err := c.Run(defaultContext, SetAttributes(test.sel, test.attrs, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			var attrs map[string]string
			err = c.Run(defaultContext, Attributes(test.sel, &attrs, test.by))
			if err != nil {
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

	c := testAllocate(t, "image.html")
	defer c.Release()

	tests := []struct {
		sel  string
		by   QueryOption
		attr string
		exp  string
	}{
		{`//*[@id="icon-brankas"]`, BySearch, "alt", "Brankas - Easy Money Management"},
		{"body > img:first-child", ByQuery, "alt", "Brankas - Easy Money Management"},
		{"body > img:nth-child(2)", ByQueryAll, "alt", "How people build software"},
		{"#icon-github", ByID, "alt", "How people build software"},
	}

	var err error
	for i, test := range tests {
		var value string
		var ok bool

		err = c.Run(defaultContext, AttributeValue(test.sel, test.attr, &value, &ok, test.by))
		if err != nil {
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
		{"#bar", ByID, "foo", "bar"},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "form.html")
			defer c.Release()

			err := c.Run(defaultContext, SetAttributeValue(test.sel, test.attr, test.exp, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			var value string
			var ok bool
			err = c.Run(defaultContext, AttributeValue(test.sel, test.attr, &value, &ok, test.by))
			if err != nil {
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
		{"/html/body/img", BySearch, "alt"},
		{"img", ByQueryAll, "alt"},
		{"img", ByQuery, "alt"},
		{"#icon-github", ByID, "alt"},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "image.html")
			defer c.Release()

			err := c.Run(defaultContext, RemoveAttribute(test.sel, test.attr))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			var value string
			var ok bool
			err = c.Run(defaultContext, AttributeValue(test.sel, test.attr, &value, &ok, test.by))
			if err != nil {
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
		{"#btn2", ByID},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "form.html")
			defer c.Release()

			err := c.Run(defaultContext, Click(test.sel, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			err = c.Run(defaultContext, WaitVisible("#icon-brankas", ByID))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			var title string
			err = c.Run(defaultContext, Title(&title))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if title != "this is title" {
				t.Errorf("expected title to be 'chromedp - Google Search', got: '%s'", title)
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
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "js.html")
			defer c.Release()

			err := c.Run(defaultContext, DoubleClick(test.sel, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			time.Sleep(50 * time.Millisecond)

			var value string
			err = c.Run(defaultContext, Value("#input1", &value, ByID))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if value != "1" {
				t.Errorf("expected value to be '1', got: '%s'", value)
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
		{`//*[@id="input1"]`, BySearch, "INSERT ", "INSERT some value"},
		{`#box4 > input:nth-child(1)`, ByQuery, "insert ", "insert some value"},
		{`#box4 > textarea`, ByQueryAll, "prefix " + kb.End + "\b\b SUFFIX\n", "prefix textar SUFFIX\n"},
		{"#textarea1", ByID, "insert ", "insert textarea"},
		{"#textarea1", ByID, kb.End + "\b\b\n\naoeu\n\nfoo\n\nbar\n\n", "textar\n\naoeu\n\nfoo\n\nbar\n\n"},
		{"#select1", ByID, kb.ArrowDown + kb.ArrowDown, "three"},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "visible.html")
			defer c.Release()

			err := c.Run(defaultContext, SendKeys(test.sel, test.keys, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			var val string
			err = c.Run(defaultContext, Value(test.sel, &val, test.by))
			if err != nil {
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

	c := testAllocate(t, "image.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{"/html/body/img", BySearch},
		{"img", ByQueryAll},
		{"img", ByQuery},
		{"#icon-github", ByID},
	}

	var err error
	for i, test := range tests {
		var buf []byte
		err = c.Run(defaultContext, Screenshot(test.sel, &buf))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if len(buf) == 0 {
			t.Fatalf("test %d failed to capture screenshot", i)
		}
		//TODO: test image
	}
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
		{"#form", ByID},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "form.html")
			defer c.Release()

			err := c.Run(defaultContext, Submit(test.sel, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			err = c.Run(defaultContext, WaitVisible("#icon-brankas", ByID))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			var title string
			err = c.Run(defaultContext, Title(&title))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if title != "this is title" {
				t.Errorf("expected title to be 'this is title', got: '%s'", title)
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
		{"#input1", ByID},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "js.html")
			defer c.Release()

			time.Sleep(50 * time.Millisecond)

			var styles []*css.ComputedProperty
			err := c.Run(defaultContext, ComputedStyle(test.sel, &styles, test.by))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			for _, style := range styles {
				if style.Name == "background-color" {
					if style.Value != "rgb(255, 0, 0)" {
						t.Logf("expected style 'rgb(255, 0, 0)' got: %s", style.Value)
					}
				}
			}

			err = c.Run(defaultContext, Click("#input1", ByID))
			if err != nil {
				t.Fatalf("got error: %v", err)
			}

			time.Sleep(50 * time.Millisecond)

			err = c.Run(defaultContext, ComputedStyle(test.sel, &styles, test.by))
			if err != nil {
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
		{"#input1", ByID},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			t.Parallel()

			c := testAllocate(t, "js.html")
			defer c.Release()

			time.Sleep(50 * time.Millisecond)

			var styles *css.GetMatchedStylesForNodeReturns
			err := c.Run(defaultContext, MatchedStyle(test.sel, &styles, test.by))
			if err != nil {
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
		fmt.Fprintf(res, uploadHTML)
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
	if _, err = tmpfile.WriteString(uploadHTML); err != nil {
		t.Fatal(err)
	}
	if err = tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		a Action
	}{
		{SendKeys(`input[name="upload"]`, tmpfile.Name(), NodeVisible)},
		{SetUploadFiles(`input[name="upload"]`, []string{tmpfile.Name()}, NodeVisible)},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			// TODO: refactor the test so the subtests can run in
			// parallel
			//t.Parallel()

			c := testAllocate(t, "")
			defer c.Release()

			var result string
			err = c.Run(defaultContext, Tasks{
				Navigate(s.URL),
				test.a,
				Click(`input[name="submit"]`),
				Text(`#result`, &result, ByID, NodeVisible),
			})
			if err != nil {
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

	c := testAllocate(t, "table.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{"/html/body/table/thead", BySearch},
		{"thead", ByQueryAll},
		{"thead", ByQuery},
	}
	var err error
	for i, test := range tests {
		var html string
		err = c.Run(defaultContext, InnerHTML(test.sel, &html))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if html == "" {
			t.Fatalf("test %d: InnerHTML is empty", i)
		}
	}
}

func TestOuterHTML(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "table.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{"/html/body/table/thead/tr", BySearch},
		{"thead tr", ByQueryAll},
		{"thead tr", ByQuery},
	}
	var err error
	for i, test := range tests {
		var html string
		err = c.Run(defaultContext, OuterHTML(test.sel, &html))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		if html == "" {
			t.Fatalf("test %d: OuterHTML is empty", i)
		}
	}
}

func TestScrollIntoView(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "image.html")
	defer c.Release()

	tests := []struct {
		sel string
		by  QueryOption
	}{
		{"/html/body/img", BySearch},
		{"img", ByQueryAll},
		{"img", ByQuery},
		{"#icon-github", ByID},
	}
	var err error
	for i, test := range tests {
		err = c.Run(defaultContext, ScrollIntoView(test.sel, test.by))
		if err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}
		// TODO test scroll event
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
