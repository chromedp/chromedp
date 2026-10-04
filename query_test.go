package chromedp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp/kb"
)

func TestWaitReady(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodeIDs []cdp.NodeID
	if err := Do(ctx, into(&nodeIDs, QueryNodeIDs(ID("input2")))); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	var value string
	if err := Do(ctx,
		WaitReady(ID("input2")),
		into(&value, Value(NodeIDs(nodeIDs))),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitVisible(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodeIDs []cdp.NodeID
	if err := Do(ctx, into(&nodeIDs, QueryNodeIDs(ID("input2")))); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	var value string
	if err := Do(ctx,
		WaitVisible(ID("input2")),
		into(&value, Value(NodeIDs(nodeIDs))),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitNotVisible(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodeIDs []cdp.NodeID
	if err := Do(ctx, into(&nodeIDs, QueryNodeIDs(ID("input2")))); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	var value string
	if err := Do(ctx,
		Click(ID("button2")),
		WaitNotVisible(ID("input2")),
		into(&value, Value(NodeIDs(nodeIDs))),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitEnabled(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	attr, err := Run(ctx, AttributeValue(ID("select1"), "disabled"))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if !attr.Exists {
		t.Fatal("expected element to be disabled")
	}
	if err := Do(ctx,
		Click(ID("button3")),
		WaitEnabled(ID("select1")),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
	attr, err = Run(ctx, AttributeValue(ID("select1"), "disabled"))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if attr.Exists {
		t.Fatal("expected element to be enabled")
	}
	var value string
	if err := Do(ctx,
		SetAttributeValue(`//*[@id="select1"]/option[1]`, "selected", "true"),
		into(&value, Value(ID("select1"))),
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

	if err := Do(ctx,
		Click(ID("button3")),
		WaitEnabled(ID("select1")),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}

	attr, err := Run(ctx, AttributeValue(`//*[@id="select1"]/option[1]`, "selected"))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if attr.Exists {
		t.Fatal("expected element to be not selected")
	}
	if err := Do(ctx,
		SetAttributeValue(`//*[@id="select1"]/option[1]`, "selected", "true"),
		WaitSelected(`//*[@id="select1"]/option[1]`),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
	attr, err = Run(ctx, AttributeValue(`//*[@id="select1"]/option[1]`, "selected"))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	if attr.Value != "true" {
		t.Fatal("expected element to be selected")
	}
}

func TestWaitNotPresent(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	if err := Do(ctx,
		WaitVisible(ID("input3")),
		Click(ID("button4")),
		WaitNotPresent(ID("input3")),
	); err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestAtLeast(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodes []*Node
	if err := Do(ctx, into(&nodes, Nodes("//input", AtLeast(3)))); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("expected to have at least 3 nodes: got %d", len(nodes))
	}
}

func TestRetryInterval(t *testing.T) {
	// Do not run in parallel. The test counts the retries in a time window. The
	// window is long and the lower limits are low, so that a slow machine, such
	// as a CI runner that stalls for 100 ms, does not fail the test. The upper
	// limits check that the interval is not shorter than it must be.
	const window = 500 * time.Millisecond

	tests := []struct {
		name         string
		opts         []QueryOption
		wantCountMin int
		wantCountMax int
	}{
		{
			name: "default",
			opts: []QueryOption{},
			// in 500ms
			wantCountMin: 5,
			wantCountMax: 100,
		},
		{
			name: "large interval",
			opts: []QueryOption{RetryInterval(60 * time.Millisecond)},
			// in 500ms
			wantCountMin: 2,
			wantCountMax: 9,
		},
	}

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			retryCount := 0

			// count is a wait function that makes the query always fail and
			// counts the number of retries. Note that the wait func is called
			// only after the number of result nodes >= s.exp .
			count := WaitFunc(
				func(ctx context.Context, t *Target, f *Frame, eci cdpruntime.ExecutionContextID, ni ...cdp.NodeID) ([]*Node, error) {
					retryCount++
					return nil, ErrInvalidTarget
				},
			)

			ctx, cancel := context.WithTimeout(ctx, window)
			defer cancel()

			opts := append(tc.opts, count)
			err := Do(ctx, Query("//input", opts...))

			if err == nil || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("want error context.DeadlineExceeded, got: %v", err)
			}
			if retryCount < tc.wantCountMin {
				t.Fatalf("want retry count > %d, got: %d", tc.wantCountMin, retryCount)
			}
			if retryCount > tc.wantCountMax {
				t.Fatalf("want retry count < %d, got: %d", tc.wantCountMax, retryCount)
			}
		})
	}
}

func TestNoRetryForInvalidSelector(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "table.html")
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tests := []struct {
		name    string
		sel     string
		wantErr string
	}{
		{`pseudo class`, `#a:b`, "DOM Error while querying (-32000)"},
		{`leading number`, `#3`, "DOM Error while querying (-32000)"},
		{`empty selector`, ``, "DOM Error while querying (-32000)"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var nodes []*Node
			if err := Do(ctx, into(&nodes, Nodes(CSS(test.sel)))); err.Error() != test.wantErr {
				t.Fatalf("want error %v, got error: %v", test.wantErr, err)
			}
		})
	}
}

func TestByJSPath(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image2.html")
	defer cancel()

	// make sure that nodes == 1
	var nodes []*Node
	if err := Do(ctx,
		into(&nodes, Nodes(JSPath(`document.querySelector('#imagething').shadowRoot.querySelector('.container')`))),
	); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected nodes to have len 1, got: %d", len(nodes))
	}

	// make sure that the class is right
	class := nodes[0].AttributeValue("class")
	if class != "container" {
		t.Errorf("expected class to be 'container', got: %q", class)
	}
}

func TestNodes(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "table.html")
	defer cancel()

	tests := []func(t *testing.T){
		nodesTest(ctx, Search(`/html/body/table/tbody[1]/tr[2]/td`), 3),
		nodesTest(ctx, CSSAll(`body > table > tbody:nth-child(2) > tr:nth-child(2) > td:not(:last-child)`), 2),
		nodesTest(ctx, CSS(`body > table > tbody:nth-child(2) > tr:nth-child(2) > td`), 1),
		nodesTest(ctx, ID(`#footer`), 1),
		nodesTest(ctx, JSPath(`document.querySelector("body > table > tbody:nth-child(2) > tr:nth-child(2) > td:nth-child(1)")`), 1),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// nodesTest returns a test for TestNodes that selects the element with sel.
func nodesTest[S Selectable](ctx context.Context, sel S, n int) func(t *testing.T) {
	return func(t *testing.T) {
		var nodes []*Node
		if err := Do(ctx, into(&nodes, Nodes(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if len(nodes) != n {
			t.Errorf("expected to have %d nodes: got %d", n, len(nodes))
		}
	}
}

func TestNodeIDs(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "table.html")
	defer cancel()

	tests := []func(t *testing.T){
		nodeIDsTest(ctx, Search(`/html/body/table/tbody[1]/tr[2]/td`), 3),
		nodeIDsTest(ctx, CSSAll(`body > table > tbody:nth-child(2) > tr:nth-child(2) > td:not(:last-child)`), 2),
		nodeIDsTest(ctx, CSS(`body > table > tbody:nth-child(2) > tr:nth-child(2) > td`), 1),
		nodeIDsTest(ctx, ID(`#footer`), 1),
		nodeIDsTest(ctx, JSPath(`document.querySelector("body > table > tbody:nth-child(2) > tr:nth-child(2) > td:nth-child(1)")`), 1),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// nodeIDsTest returns a test for TestNodeIDs that selects the element with sel.
func nodeIDsTest[S Selectable](ctx context.Context, sel S, n int) func(t *testing.T) {
	return func(t *testing.T) {
		var ids []cdp.NodeID
		if err := Do(ctx, into(&ids, QueryNodeIDs(sel))); err != nil {
			t.Fatal(err)
		}

		if len(ids) != n {
			t.Errorf("expected to have %d node id's: got %d", n, len(ids))
		}
	}
}

func TestFocusBlur(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	tests := []func(t *testing.T){
		focusBlurTest(ctx, Search(`//*[@id="input1"]`)),
		focusBlurTest(ctx, CSSAll(`body > input[type="number"]:nth-child(1)`)),
		focusBlurTest(ctx, CSS(`body > input[type="number"]:nth-child(1)`)),
		focusBlurTest(ctx, ID(`#input1`)),
		focusBlurTest(ctx, JSPath(`document.querySelector("#input1")`)),
	}

	if err := Do(ctx, Click(ID("input1"))); err != nil {
		t.Fatal(err)
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// focusBlurTest returns a test for TestFocusBlur that selects the element with sel.
func focusBlurTest[S Selectable](ctx context.Context, sel S) func(t *testing.T) {
	return func(t *testing.T) {
		var value string
		if err := Do(ctx,
			Focus(sel),
			into(&value, Value(sel)),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != "9999" {
			t.Errorf("expected value is '9999', got: %q", value)
		}
		if err := Do(ctx,
			Blur(sel),
			into(&value, Value(sel)),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != "0" {
			t.Errorf("expected value is '0', got: %q", value)
		}
	}
}

func TestDimensions(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []func(t *testing.T){
		dimensionsTest(ctx, Search(`/html/body/img`), 239, 239),
		dimensionsTest(ctx, CSSAll(`img`), 239, 239),
		dimensionsTest(ctx, CSS(`img`), 239, 239),
		dimensionsTest(ctx, ID(`#icon-github`), 120, 120),
		dimensionsTest(ctx, JSPath(`document.querySelector("#icon-github")`), 120, 120),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// dimensionsTest returns a test for TestDimensions that selects the element with sel.
func dimensionsTest[S Selectable](ctx context.Context, sel S, width int64, height int64) func(t *testing.T) {
	return func(t *testing.T) {
		var model *dom.BoxModel
		if err := Do(ctx, into(&model, Dimensions(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if model.Height != height || model.Width != width {
			t.Errorf("expected %dx%d, got: %dx%d", width, height, model.Height, model.Width)
		}
	}
}

func TestText(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	tests := []func(t *testing.T){
		textTest(ctx, ID("#foo"), "insert"),
		textTest(ctx, CSSAll("body > form > span"), "insert"),
		textTest(ctx, CSS("body > form > span:nth-child(2)"), "keyword"),
		textTest(ctx, Search("/html/body/form/span[2]"), "keyword"),
		textTest(ctx, JSPath(`document.querySelector("#form > span:nth-child(2)")`), "keyword"),
		textTest(ctx, ID("#inner-hidden"), "this is"),
		textTest(ctx, ID("#hidden"), ""),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// textTest returns a test for TestText that selects the element with sel.
func textTest[S Selectable](ctx context.Context, sel S, exp string) func(t *testing.T) {
	return func(t *testing.T) {
		var text string
		if err := Do(ctx, into(&text, Text(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if text != exp {
			t.Errorf("expected %q, got: %s", exp, text)
		}
	}
}

func TestTextContent(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	tests := []func(t *testing.T){
		textContentTest(ctx, ID("#inner-hidden"), "this is hidden"),
		textContentTest(ctx, ID("#hidden"), "hidden"),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// textContentTest returns a test for TestTextContent that selects the element with sel.
func textContentTest[S Selectable](ctx context.Context, sel S, exp string) func(t *testing.T) {
	return func(t *testing.T) {
		var text string
		if err := Do(ctx, into(&text, TextContent(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if text != exp {
			t.Errorf("expected %q, got: %s", exp, text)
		}
	}
}

func TestClear(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		// input fields
		clearTest(Search(`//*[@id="form"]/input[1]`)),
		clearTest(CSS(`#form > input[type="text"]:nth-child(4)`)),
		clearTest(CSSAll(`#form > input[type="text"]`)),
		clearTest(ID(`#keyword`)),
		clearTest(JSPath(`document.querySelector("#keyword")`)),

		// textarea fields
		clearTest(Search(`//*[@id="bar"]`)),
		clearTest(CSS(`#form > textarea`)),
		clearTest(CSSAll(`#form > textarea`)),
		clearTest(ID(`#bar`)),

		// input + textarea fields
		clearTest(Search(`//*[@id="form"]/input`)),
		clearTest(CSSAll(`#form > input[type="text"]`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// clearTest returns a test for TestClear that selects the element with sel.
func clearTest[S Selectable](sel S) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "form.html")
		defer cancel()

		var val string
		if err := Do(ctx, into(&val, Value(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}
		if val == "" {
			t.Errorf("expected %v to have non empty value", sel)
		}
		if err := Do(ctx,
			Clear(sel),
			into(&val, Value(sel)),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}
		if val != "" {
			t.Errorf("expected empty value for %v, got: %s", sel, val)
		}
	}
}

func TestReset(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		resetTest(Search(`//*[@id="keyword"]`), "foobar", "chromedp"),
		resetTest(CSS(`#form > input[type="text"]:nth-child(6)`), "foobar", "foo"),
		resetTest(CSSAll(`#form > input[type="text"]`), "foobar", "chromedp"),
		resetTest(ID("#bar"), "foobar", "bar"),
		resetTest(JSPath(`document.querySelector("#bar")`), "foobar", "bar"),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// resetTest returns a test for TestReset that selects the element with sel.
func resetTest[S Selectable](sel S, set string, exp string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "form.html")
		defer cancel()

		var value string
		if err := Do(ctx,
			SetValue(sel, set),
			Reset(sel),
			into(&value, Value(sel)),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != exp {
			t.Errorf("expected value after reset is %s, got: %q", exp, value)
		}
	}
}

func TestValue(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	tests := []func(t *testing.T){
		valueTest(ctx, Search(`//*[@id="form"]/input[1]`)),
		valueTest(ctx, CSS(`#form > input[type="text"]:nth-child(4)`)),
		valueTest(ctx, CSSAll(`#form > input[type="text"]`)),
		valueTest(ctx, ID(`#keyword`)),
		valueTest(ctx, JSPath(`document.querySelector("#keyword")`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// valueTest returns a test for TestValue that selects the element with sel.
func valueTest[S Selectable](ctx context.Context, sel S) func(t *testing.T) {
	return func(t *testing.T) {
		var value string
		if err := Do(ctx, into(&value, Value(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != "chromedp" {
			t.Errorf("expected `chromedp`, got: %s", value)
		}
	}
}

func TestValueUndefined(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "form.html")
	defer cancel()

	var value string
	err := Do(ctx, into(&value, Value(ID("foo"))))
	want := `could not retrieve attribute "value": encountered an undefined value`
	got := fmt.Sprint(err)
	if !strings.Contains(got, want) {
		t.Fatalf("want error %q, got %q", want, got)
	}
}

func TestSetValue(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		setValueTest(Search(`//*[@id="form"]/input[1]`)),
		setValueTest(CSS(`#form > input[type="text"]:nth-child(4)`)),
		setValueTest(CSSAll(`#form > input[type="text"]`)),
		setValueTest(ID(`#bar`)),
		setValueTest(JSPath(`document.querySelector("#bar")`)),
		setValueTest(CSS(`#select`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// setValueTest returns a test for TestSetValue that selects the element with sel.
func setValueTest[S Selectable](sel S) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "form.html")
		defer cancel()

		var value string
		if err := Do(ctx,
			SetValue(sel, "FOOBAR"),
			into(&value, Value(sel)),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != "FOOBAR" {
			t.Errorf("expected `FOOBAR`, got: %s", value)
		}

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := Do(ctx,
			WaitVisible(CSS("#event-input")),
			WaitVisible(CSS("#event-change")),
		); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("input and/or change events not fired")
			} else {
				t.Fatalf("got error: %v", err)
			}
		}
	}
}

func TestAttributes(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []func(t *testing.T){
		attributesTest(ctx, Search(`//*[@id="icon-brankas"]`), map[string]string{
			"alt": "Brankas - Easy Money Management",
			"id":  "icon-brankas",
			"src": "images/brankas.png",
		},
		),
		attributesTest(ctx, CSS(`body > img:first-child`), map[string]string{
			"alt": "Brankas - Easy Money Management",
			"id":  "icon-brankas",
			"src": "images/brankas.png",
		},
		),
		attributesTest(ctx, CSSAll(`body > img:nth-child(2)`), map[string]string{
			"alt": `How people build software`,
			"id":  "icon-github",
			"src": "images/github.png",
		},
		),
		attributesTest(ctx, ID(`#icon-github`), map[string]string{
			"alt": "How people build software",
			"id":  "icon-github",
			"src": "images/github.png",
		},
		),
		attributesTest(ctx, JSPath(`document.querySelector("#icon-github")`), map[string]string{
			"alt": "How people build software",
			"id":  "icon-github",
			"src": "images/github.png",
		},
		),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// attributesTest returns a test for TestAttributes that selects the element with sel.
func attributesTest[S Selectable](ctx context.Context, sel S, exp map[string]string) func(t *testing.T) {
	return func(t *testing.T) {
		var attrs map[string]string
		if err := Do(ctx, into(&attrs, Attributes(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if !reflect.DeepEqual(exp, attrs) {
			t.Errorf("expected %v, got: %v", exp, attrs)
		}
	}
}

func TestAttributesAll(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []func(t *testing.T){
		attributesAllTest(ctx, CSSAll("img"), []map[string]string{
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
		),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// attributesAllTest returns a test for TestAttributesAll that selects the element with sel.
func attributesAllTest[S Selectable](ctx context.Context, sel S, exp []map[string]string) func(t *testing.T) {
	return func(t *testing.T) {
		var attrs []map[string]string
		if err := Do(ctx, into(&attrs, AttributesAll(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if !reflect.DeepEqual(exp, attrs) {
			t.Errorf("expected %v, got: %v", exp, attrs)
		}
	}
}

func TestSetAttributes(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		setAttributesTest(Search(`//*[@id="icon-brankas"]`), map[string]string{"data-url": "brankas"},
			map[string]string{
				"alt":      "Brankas - Easy Money Management",
				"id":       "icon-brankas",
				"src":      "images/brankas.png",
				"data-url": "brankas",
			},
		),
		setAttributesTest(CSS(`body > img:first-child`), map[string]string{"data-url": "brankas"},
			map[string]string{
				"alt":      "Brankas - Easy Money Management",
				"id":       "icon-brankas",
				"src":      "images/brankas.png",
				"data-url": "brankas",
			},
		),
		setAttributesTest(CSSAll(`body > img:nth-child(2)`), map[string]string{"width": "100", "height": "200"},
			map[string]string{
				"alt":    `How people build software`,
				"id":     "icon-github",
				"src":    "images/github.png",
				"width":  "100",
				"height": "200",
			},
		),
		setAttributesTest(ID(`#icon-github`), map[string]string{"width": "100", "height": "200"},
			map[string]string{
				"alt":    "How people build software",
				"id":     "icon-github",
				"src":    "images/github.png",
				"width":  "100",
				"height": "200",
			},
		),
		setAttributesTest(JSPath(`document.querySelector("#icon-github")`), map[string]string{"width": "100", "height": "200"},
			map[string]string{
				"alt":    "How people build software",
				"id":     "icon-github",
				"src":    "images/github.png",
				"width":  "100",
				"height": "200",
			},
		),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// setAttributesTest returns a test for TestSetAttributes that selects the element with sel.
func setAttributesTest[S Selectable](sel S, attrs map[string]string, exp map[string]string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "image.html")
		defer cancel()

		if err := Do(ctx, SetAttributes(sel, attrs)); err != nil {
			t.Fatalf("got error: %v", err)
		}

		// TODO: find out why this test is flaky without this
		time.Sleep(10 * time.Millisecond)

		var attrs map[string]string
		if err := Do(ctx, into(&attrs, Attributes(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if !reflect.DeepEqual(exp, attrs) {
			t.Errorf("expected %v, got: %v", exp, attrs)
		}
	}
}

func TestAttributeValue(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []func(t *testing.T){
		attributeValueTest(ctx, Search(`//*[@id="icon-brankas"]`), "alt", "Brankas - Easy Money Management"),
		attributeValueTest(ctx, CSS(`body > img:first-child`), "alt", "Brankas - Easy Money Management"),
		attributeValueTest(ctx, CSSAll(`body > img:nth-child(2)`), "alt", "How people build software"),
		attributeValueTest(ctx, ID(`#icon-github`), "alt", "How people build software"),
		attributeValueTest(ctx, JSPath(`document.querySelector('#icon-github')`), "alt", "How people build software"),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// attributeValueTest returns a test for TestAttributeValue that selects the element with sel.
func attributeValueTest[S Selectable](ctx context.Context, sel S, name string, exp string) func(t *testing.T) {
	return func(t *testing.T) {
		attr, err := Run(ctx, AttributeValue(sel, name))
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if !attr.Exists {
			t.Fatalf("failed to get attribute %s on %v", name, sel)
		}
		if attr.Value != exp {
			t.Errorf("expected %s to be %s, got: %s", name, exp, attr.Value)
		}
	}
}

func TestSetAttributeValue(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		setAttributeValueTest(Search(`//*[@id="keyword"]`), "foo", "bar"),
		setAttributeValueTest(CSS(`#form > input[type="text"]:nth-child(6)`), "foo", "bar"),
		setAttributeValueTest(CSSAll(`#form > input[type="text"]`), "foo", "bar"),
		setAttributeValueTest(ID(`#bar`), "foo", "bar"),
		setAttributeValueTest(JSPath(`document.querySelector('#bar')`), "foo", "bar"),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// setAttributeValueTest returns a test for TestSetAttributeValue that selects the element with sel.
func setAttributeValueTest[S Selectable](sel S, name string, exp string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "form.html")
		defer cancel()

		if err := Do(ctx, SetAttributeValue(sel, name, exp)); err != nil {
			t.Fatalf("got error: %v", err)
		}

		// TODO: find out why this test is flaky without this
		time.Sleep(10 * time.Millisecond)

		attr, err := Run(ctx, AttributeValue(sel, name))
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if !attr.Exists {
			t.Fatalf("failed to get attribute %s on %v", name, sel)
		}
		if attr.Value != exp {
			t.Errorf("expected %s to be %s, got: %s", name, exp, attr.Value)
		}
	}
}

func TestRemoveAttribute(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		removeAttributeTest(Search(`/html/body/img`), "alt"),
		removeAttributeTest(CSSAll(`img`), "alt"),
		removeAttributeTest(CSS(`img`), "alt"),
		removeAttributeTest(ID(`#icon-github`), "alt"),
		removeAttributeTest(JSPath(`document.querySelector('#icon-github')`), "alt"),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// removeAttributeTest returns a test for TestRemoveAttribute that selects the element with sel.
func removeAttributeTest[S Selectable](sel S, name string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "image.html")
		defer cancel()

		if err := Do(ctx, RemoveAttribute(sel, name)); err != nil {
			t.Fatalf("got error: %v", err)
		}

		// TODO: find out why this test is flaky without this
		time.Sleep(10 * time.Millisecond)

		attr, err := Run(ctx, AttributeValue(sel, name))
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		if attr.Exists || attr.Value != "" {
			t.Fatalf("expected attribute %s removed from element %v", name, sel)
		}
	}
}

func TestClick(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		clickTest(Search(`//*[@id="form"]/input[4]`)),
		clickTest(CSS(`#form > input[type="submit"]:nth-child(11)`)),
		clickTest(CSSAll(`#form > input[type="submit"]:nth-child(11)`)),
		clickTest(ID(`#btn2`)),
		clickTest(JSPath(`document.querySelector('#btn2')`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// clickTest returns a test for TestClick that selects the element with sel.
func clickTest[S Selectable](sel S) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "form.html")
		defer cancel()

		var title string
		if err := Do(ctx,
			Click(sel),
			WaitVisible(ID("icon-brankas")),
			into(&title, Title()),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if title != "this is title" {
			t.Errorf("expected title to be 'chromedp - Google Search', got: %q", title)
		}
	}
}

func TestDoubleClick(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		doubleClickTest(Search(`/html/body/input[2]`)),
		doubleClickTest(CSSAll(`body > input[type="button"]:nth-child(2)`)),
		doubleClickTest(CSS(`body > input[type="button"]:nth-child(2)`)),
		doubleClickTest(ID(`#button1`)),
		doubleClickTest(JSPath(`document.querySelector('#button1')`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// doubleClickTest returns a test for TestDoubleClick that selects the element with sel.
func doubleClickTest[S Selectable](sel S) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "js.html")
		defer cancel()

		var value string
		if err := Do(ctx,
			DoubleClick(sel),
			into(&value, Value(ID("input1"))),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if value != "1" {
			t.Errorf("expected value to be '1', got: %q", value)
		}
	}
}

func TestSendKeys(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		sendKeysTest(Search(`//*[@id="input1"]`), "INSERT ", "INSERT some value"), // 0
		sendKeysTest(CSS(`#box4 > input:nth-child(1)`), "insert ", "insert some value"),
		sendKeysTest(CSSAll(`#box4 > textarea`), "prefix "+kb.End+"\b\b SUFFIX\n", "prefix textar SUFFIX\n"),
		sendKeysTest(ID(`#textarea1`), "insert ", "insert textarea"),
		sendKeysTest(ID(`#textarea1`), kb.End+"\b\b\n\naoeu\n\nfoo\n\nbar\n\n", "textar\n\naoeu\n\nfoo\n\nbar\n\n"),
		sendKeysTest(ID(`#select1`), kb.ArrowDown+kb.ArrowDown, "three"), // 5
		sendKeysTest(JSPath(`document.querySelector('#textarea1')`), "insert ", "insert textarea"),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			if runtime.GOOS == "darwin" && i == 5 {
				t.Skipf("skipping test %d on darwin -- FIXME!", i)
			}
			test(t)
		})
	}
}

// sendKeysTest returns a test for TestSendKeys that selects the element with sel.
func sendKeysTest[S Selectable](sel S, keys string, exp string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "visible.html")
		defer cancel()

		var val string
		if err := Do(ctx,
			SendKeys(sel, keys),
			into(&val, Value(sel)),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if val != exp {
			t.Errorf("expected value %s, got: %s", exp, val)
		}
	}
}

func TestSubmit(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		submitTest(Search(`//*[@id="keyword"]`)),
		submitTest(CSS(`#form > input[type="text"]:nth-child(4)`)),
		submitTest(CSSAll(`#form > input[type="text"]`)),
		submitTest(ID(`#form`)),
		submitTest(JSPath(`document.querySelector('#form')`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// submitTest returns a test for TestSubmit that selects the element with sel.
func submitTest[S Selectable](sel S) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "form.html")
		defer cancel()

		var title string
		if err := Do(ctx,
			Submit(sel),
			WaitVisible(ID("icon-brankas")),
			into(&title, Title()),
		); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if title != "this is title" {
			t.Errorf("expected title to be 'this is title', got: %q", title)
		}
	}
}

func TestComputedStyle(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		computedStyleTest(Search(`//*[@id="input1"]`)),
		computedStyleTest(CSSAll(`body > input[type="number"]:nth-child(1)`)),
		computedStyleTest(CSS(`body > input[type="number"]:nth-child(1)`)),
		computedStyleTest(ID(`#input1`)),
		computedStyleTest(JSPath(`document.querySelector('#input1')`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// computedStyleTest returns a test for TestComputedStyle that selects the element with sel.
func computedStyleTest[S Selectable](sel S) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "js.html")
		defer cancel()

		var styles []*css.ComputedStyleProperty
		if err := Do(ctx, into(&styles, ComputedStyle(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		for _, style := range styles {
			if style.Name == "background-color" {
				if style.Value != "rgb(255, 0, 0)" {
					t.Logf("expected style 'rgb(255, 0, 0)' got: %s", style.Value)
				}
			}
		}
		if err := Do(ctx,
			Click(ID("input1")),
			into(&styles, ComputedStyle(sel)),
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
	}
}

func TestMatchedStyle(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		matchedStyleTest(Search(`//*[@id="input1"]`)),
		matchedStyleTest(CSSAll(`body > input[type="number"]:nth-child(1)`)),
		matchedStyleTest(CSS(`body > input[type="number"]:nth-child(1)`)),
		matchedStyleTest(ID(`#input1`)),
		matchedStyleTest(JSPath(`document.querySelector('#input1')`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// matchedStyleTest returns a test for TestMatchedStyle that selects the element with sel.
func matchedStyleTest[S Selectable](sel S) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "js.html")
		defer cancel()

		var styles *css.GetMatchedStylesForNodeResult
		if err := Do(ctx, into(&styles, MatchedStyle(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		// TODO: Add logic to make sure that the returned style is true and valid.
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

		buf, err := io.ReadAll(f)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Fprintf(res, resultHTML, len(buf))
	})
	s := httptest.NewServer(mux)
	defer s.Close()

	uploadFile := filepath.Join(t.TempDir(), "chromedp-upload-test")
	if err := os.WriteFile(uploadFile, []byte(uploadHTML), 0o666); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		a Action[Void]
	}{
		{SendKeys(`input[name="upload"]`, uploadFile, NodeVisible)},
		{SetUploadFiles(`input[name="upload"]`, []string{uploadFile}, NodeVisible)},
	}

	// Do not run these tests in parallel. The only way is to fire a separate
	// httptest server and tmpfile for each. We cannot share these resources
	// among parallel subtests, because the parent must finish for the children
	// to run, and the defers above prevent that.
	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			ctx, cancel := testAllocate(t, "")
			defer cancel()

			var result string
			if err := Do(ctx,
				Navigate(s.URL),
				test.a,
				Click(`input[name="submit"]`),
				into(&result, Text(ID(`result`), NodeVisible)),
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

	tests := []func(t *testing.T){
		innerHTMLTest(ctx, Search(`/html/body/table/thead`)),
		innerHTMLTest(ctx, CSSAll(`thead`)),
		innerHTMLTest(ctx, CSS(`thead`)),
		innerHTMLTest(ctx, JSPath(`document.querySelector("#footer > td:nth-child(2)")`)),
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// innerHTMLTest returns a test for TestInnerHTML that selects the element with sel.
func innerHTMLTest[S Selectable](ctx context.Context, sel S) func(t *testing.T) {
	return func(t *testing.T) {
		var html string
		if err := Do(ctx, into(&html, InnerHTML(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if html == "" {
			t.Fatal("InnerHTML is empty")
		}
	}
}

func TestOuterHTML(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "table.html")
	defer cancel()

	tests := []func(t *testing.T){
		outerHTMLTest(ctx, Search(`/html/body/table/thead/tr`)),
		outerHTMLTest(ctx, CSSAll(`thead tr`)),
		outerHTMLTest(ctx, CSS(`thead tr`)),
		outerHTMLTest(ctx, JSPath(`document.querySelector("#footer > td:nth-child(2)")`)),
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// outerHTMLTest returns a test for TestOuterHTML that selects the element with sel.
func outerHTMLTest[S Selectable](ctx context.Context, sel S) func(t *testing.T) {
	return func(t *testing.T) {
		var html string
		if err := Do(ctx, into(&html, OuterHTML(sel))); err != nil {
			t.Fatalf("got error: %v", err)
		}

		if html == "" {
			t.Fatal("OuterHTML is empty")
		}
	}
}

func TestScrollIntoView(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	tests := []func(t *testing.T){
		scrollIntoViewTest(ctx, Search(`/html/body/img`)),
		scrollIntoViewTest(ctx, CSSAll(`img`)),
		scrollIntoViewTest(ctx, CSS(`img`)),
		scrollIntoViewTest(ctx, ID(`#icon-github`)),
		scrollIntoViewTest(ctx, JSPath(`document.querySelector('#icon-github')`)),
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// scrollIntoViewTest returns a test for TestScrollIntoView that selects the element with sel.
func scrollIntoViewTest[S Selectable](ctx context.Context, sel S) func(t *testing.T) {
	return func(t *testing.T) {
		if err := Do(ctx, ScrollIntoView(sel)); err != nil {
			t.Fatalf("got error: %v", err)
		}

		// TODO test scroll event
	}
}

func TestSVGFullXPath(t *testing.T) {
	t.Parallel()

	tests := []func(t *testing.T){
		sVGFullXPathTest(CSS(`#brankas`)),
		sVGFullXPathTest(CSSAll(`text`)),
		sVGFullXPathTest(ID(`brankas`)),
		sVGFullXPathTest(Search(`//*[local-name()='text']`)),
		sVGFullXPathTest(JSPath(`document.querySelector('#brankas')`)),
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%02d", i), test)
	}
}

// sVGFullXPathTest returns a test for TestSVGFullXPath that selects the element with sel.
func sVGFullXPathTest[S Selectable](sel S) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		ctx, cancel := testAllocate(t, "svg.html")
		defer cancel()

		var nodes []*Node
		if err := Do(ctx, into(&nodes, Nodes(sel))); err != nil {
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
		if err := Do(ctx, into(&text1, TextContent(sel))); err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(text1) != "Brankas" {
			t.Errorf("expected %q, got: %q", "Brankas", text1)
		}
		if err := Do(ctx, into(&text2, TextContent(xpath))); err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(text2) != "Brankas" {
			t.Errorf("expected %q, got: %q", "Brankas", text2)
		}
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

// TestWaitReadyAfterFreshNavigation runs WaitReady right after a navigation,
// before the target has any reason to have the nodes of the new document. The
// issue 1593 says that this always times out. It does not, because the query
// polls until the DOM events fill the node tree of the frame.
func TestWaitReadyAfterFreshNavigation(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		url  string
		wait Action[Void]
	}{
		{"data url html", "data:text/html,<html><body>Hello</body></html>", WaitReady(CSS("html"))},
		{"data url empty html", "data:text/html,<html></html>", WaitReady(CSS("html"))},
		{"data url body", "data:text/html,<html><body>Hello</body></html>", WaitReady(CSS("body"))},
		{"search", "data:text/html,<html><body><h1 id=x>Hi</h1></body></html>", WaitReady(Search("//h1"))},
		{"file", testdataDir + "/form.html", WaitReady(ID("form"))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testAllocate(t, "")
			defer cancel()
			// The first run attaches the tab, so it runs on the context of the
			// tab, and not on a context with a timeout. The new tab has
			// loaded nothing yet.
			if err := Do(ctx, Navigate(tt.url), tt.wait); err != nil {
				t.Fatalf("first navigation: %v", err)
			}
			// More navigations on the same tab. Each one replaces the node
			// tree of the old document.
			for i := range 4 {
				tctx, tcancel := context.WithTimeout(ctx, 10*time.Second)
				err := Do(tctx, Navigate(tt.url), tt.wait)
				tcancel()
				if err != nil {
					t.Fatalf("navigation %d: %v", i+2, err)
				}
			}
		})
	}
}

func TestWaitReadyReuseAction(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	// Reusing a single WaitReady action used to panic.
	action := WaitReady(ID("input2"))
	for range 3 {
		if err := Do(ctx, action); err != nil {
			t.Fatalf("got error: %v", err)
		}
	}
}

func TestFromNode(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "nested.html")
	defer cancel()

	tests := []struct {
		name       string
		fromQuery  string
		nodesQuery string
		nodesCount int
	}{
		{"DefaultRoot", "", ".content", 4},
		{"Body", "body", ".content", 4},
		{"Empty", "#empty", ".content", 0},
		{"Parent1", "#parent1", ".content", 1},
		{"Parent2", "#parent2", ".content", 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var from *Node
			if test.fromQuery != "" {
				var nodes []*Node
				if err := Do(ctx,
					into(&nodes, Nodes(CSS(test.fromQuery), AtLeast(0))),
				); err != nil {
					t.Fatal(err)
				}
				if len(nodes) != 1 {
					t.Fatalf("expected to have 1 node, got %d", len(nodes))
				}
				from = nodes[0]
			}
			var nodes []*Node
			if err := Do(ctx,
				into(&nodes, Nodes(CSSAll(test.nodesQuery), AtLeast(0), FromNode(from))),
			); err != nil {
				t.Fatal(err)
			}
			if len(nodes) != test.nodesCount {
				t.Fatalf("expected to have %d node, got %d", test.nodesCount, len(nodes))
			}
		})
	}
}

// selectedIDs runs Nodes with the selector and returns the id attribute of each
// node that it selects.
func selectedIDs[S Selectable](ctx context.Context, tb testing.TB, sel S, opts ...QueryOption) []string {
	tb.Helper()

	nodes, err := Run(ctx, Nodes(sel, opts...))
	if err != nil {
		tb.Fatalf("got error: %v", err)
	}
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.AttributeValue("id")
	}
	return ids
}

// A string myString is a string type that the package does not define. It
// counts as a Search.
type myString string

// myNodeIDs is a slice type that the package does not define. It counts as
// NodeIDs.
type myNodeIDs []cdp.NodeID

func TestSelectorSearch(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// Search matches an XPath query, a CSS selector and plain text, as the
	// old Search lookup did. A CSS selector gives every match, not the first.
	if got, want := selectedIDs(ctx, t, Search(`//p`)), []string{"one", "two", "three"}; !reflect.DeepEqual(got, want) {
		t.Errorf("XPath: want %v, got %v", want, got)
	}
	if got, want := selectedIDs(ctx, t, Search(`.item`)), []string{"one", "two"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CSS selector: want %v, got %v", want, got)
	}
	nodes, err := Run(ctx, Nodes(Search(`second`)))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("plain text: want 1 node, got %d", len(nodes))
	}
	if nodes[0].NodeType != NodeTypeText || nodes[0].NodeValue != "second" {
		t.Errorf("plain text: want the text node %q, got %q", "second", nodes[0].NodeValue)
	}
}

func TestSelectorPlainString(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// A string constant, a string variable and a named string type are all a
	// Search.
	want := []string{"one", "two", "three"}
	variable := `//p`
	if got := selectedIDs(ctx, t, `//p`); !reflect.DeepEqual(got, want) {
		t.Errorf("constant: want %v, got %v", want, got)
	}
	if got := selectedIDs(ctx, t, variable); !reflect.DeepEqual(got, want) {
		t.Errorf("variable: want %v, got %v", want, got)
	}
	if got := selectedIDs(ctx, t, myString(variable)); !reflect.DeepEqual(got, want) {
		t.Errorf("named string type: want %v, got %v", want, got)
	}

	var text string
	if err := Do(ctx, into(&text, Text("#three"))); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if text != "third" {
		t.Errorf("want text %q, got %q", "third", text)
	}
}

func TestSelectorCSS(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// CSS gives only the first match, as the old lookup of a single CSS query did.
	if got, want := selectedIDs(ctx, t, CSS(`.item`)), []string{"one"}; !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}

	// CSS does not take an XPath query. DOM.querySelector fails.
	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := Do(ctx, Query(CSS(`//p`)))
	if err == nil || !strings.Contains(err.Error(), "DOM Error while querying") {
		t.Errorf("want a DOM error for an XPath query, got: %v", err)
	}
}

func TestSelectorCSSAll(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// CSSAll gives every match, as the old lookup of every CSS match did.
	if got, want := selectedIDs(ctx, t, CSSAll(`.item`)), []string{"one", "two"}; !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
	if got, want := selectedIDs(ctx, t, CSSAll(`p`)), []string{"one", "two", "three"}; !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestSelectorID(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// ID selects the element with this id, with or without a leading "#", as
	// the old lookup by id did.
	for _, id := range []ID{"two", "#two"} {
		if got, want := selectedIDs(ctx, t, id), []string{"two"}; !reflect.DeepEqual(got, want) {
			t.Errorf("%q: want %v, got %v", id, want, got)
		}
	}

	// A class name is not an id.
	if got := selectedIDs(ctx, t, ID(`item`), AtLeast(0)); len(got) != 0 {
		t.Errorf("want no node for a class name, got %v", got)
	}
}

func TestSelectorJSPath(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// JSPath runs the expression and selects the node that it gives, as the
	// old lookup by JavaScript path did. Here the node is in a shadow tree.
	const path = `document.getElementById('host').shadowRoot.querySelector('.inner')`
	nodes, err := Run(ctx, Nodes(JSPath(path)))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodes) != 1 || nodes[0].AttributeValue("class") != "inner" {
		t.Errorf("want the one node with the class inner, got %d nodes", len(nodes))
	}

	// CSSAll does not reach into the shadow tree.
	if got := selectedIDs(ctx, t, CSSAll(`.inner`), AtLeast(0)); len(got) != 0 {
		t.Errorf("want no node outside of JSPath, got %v", got)
	}
}

func TestSelectorNodeIDs(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// NodeIDs selects the nodes with these ids, as the old lookup by
	// node ids did.
	ids, err := Run(ctx, QueryNodeIDs(CSSAll(`.item`)))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("want 2 node ids, got %d", len(ids))
	}
	want := []string{"one", "two"}
	if got := selectedIDs(ctx, t, NodeIDs(ids)); !reflect.DeepEqual(got, want) {
		t.Errorf("NodeIDs: want %v, got %v", want, got)
	}
	if got := selectedIDs(ctx, t, ids); !reflect.DeepEqual(got, want) {
		t.Errorf("[]cdp.NodeID: want %v, got %v", want, got)
	}
	if got := selectedIDs(ctx, t, myNodeIDs(ids)); !reflect.DeepEqual(got, want) {
		t.Errorf("named slice type: want %v, got %v", want, got)
	}

	// An action reads from the node of the first id.
	text, err := Run(ctx, Text(NodeIDs(ids[:1])))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if text != "first" {
		t.Errorf("want text %q, got %q", "first", text)
	}
}

func TestSelectorByFunc(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// ByFunc replaces the lookup of the selector. The selector is only a
	// label for error messages.
	ids, err := Run(ctx, QueryNodeIDs(CSS(`#three`)))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	text, err := Run(ctx, Text("label", ByFunc(func(ctx context.Context, t *Target, n *Node) ([]cdp.NodeID, error) {
		return ids, nil
	})))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if text != "third" {
		t.Errorf("want text %q, got %q", "third", text)
	}
}

func TestSelectorNoMatchError(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "selectors.html")
	defer cancel()

	// The error text names the selector, whatever its type.
	tests := []struct {
		name string
		act  Action[string]
		want string
	}{
		{"CSS", Text(CSS(`#missing`), AtLeast(0)), `selector "#missing" did not return any nodes`},
		{"Search", Text(`//missing`, AtLeast(0)), `selector "//missing" did not return any nodes`},
		{"ID", Text(ID(`missing`), AtLeast(0)), `selector "missing" did not return any nodes`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Run(ctx, test.act)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want error %q, got: %v", test.want, err)
			}
		})
	}
}

// TestWaitNotPresentJSPath checks that WaitNotPresent succeeds when the JSPath
// expression gives null, undefined or an empty NodeList. These did not select a
// node, and the lookup retried until the timeout. See the issue 1600.
func TestWaitNotPresentJSPath(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "shadow.html")
	defer cancel()
	// A failure must not hang the test.
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	const (
		one      = `document.getElementById('host').shadowRoot.querySelector('.mask')`
		list     = `document.getElementById('host').shadowRoot.querySelectorAll('.item')`
		optional = `document.querySelector('#none')?.shadowRoot?.querySelectorAll('.item')`
		missing  = `window.nothingIsHere`
	)

	// The nodes are present.
	if err := Do(ctx, WaitVisible(JSPath(one))); err != nil {
		t.Fatalf("WaitVisible of one node: %v", err)
	}
	nodes, err := Run(ctx, Nodes(JSPath(list)))
	if err != nil {
		t.Fatalf("Nodes of a NodeList: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes from the NodeList, got %d", len(nodes))
	}
	// A wait for a node that is present must not return for NodeNotPresent
	// while the node exists.
	short, shortCancel := context.WithTimeout(ctx, 300*time.Millisecond)
	err = Do(short, WaitNotPresent(JSPath(one)))
	shortCancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want the deadline while the node is present, got %v", err)
	}

	// Remove the nodes. The expressions give null and an empty NodeList.
	if _, err := Run(ctx, Evaluate[any](`removeMask(); removeItems();`)); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"null":      one,
		"empty":     list,
		"undefined": optional,
		"missing":   missing,
	} {
		if err := Do(ctx, WaitNotPresent(JSPath(path))); err != nil {
			t.Errorf("WaitNotPresent of %s: %v", name, err)
		}
	}

	// The other waits keep waiting for a node. They end with the error of the
	// context and not with an error of the lookup.
	short, shortCancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer shortCancel()
	for name, path := range map[string]string{"null": one, "empty": list} {
		err := Do(short, WaitVisible(JSPath(path)))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("WaitVisible of %s: want the deadline, got %v", name, err)
		}
	}
}

// TestJSPathWrongValue checks that a JSPath expression that gives a value that
// is not a node returns an error at once, and does not retry until the timeout.
func TestJSPathWrongValue(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "shadow.html")
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for name, path := range map[string]string{
		"number":     `1 + 2`,
		"string":     `'text'`,
		"object":     `({})`,
		"list value": `[1, 2]`,
	} {
		for _, wait := range []Action[Void]{WaitVisible(JSPath(path)), WaitNotPresent(JSPath(path))} {
			err := Do(ctx, wait)
			if !errors.Is(err, errNotNode) {
				t.Errorf("%s: want an error for a value that is not a node, got %v", name, err)
			}
		}
	}
}
