package chromedp

import (
	"testing"

	"github.com/chromedp/cdproto/cdp"
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
