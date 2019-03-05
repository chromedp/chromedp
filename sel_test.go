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
	err := Run(ctx, NodeIDs("#input2", &nodeIDs, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	if err := Run(ctx, WaitReady("#input2", ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}

	var value string
	if err := Run(ctx, Value(nodeIDs, &value, ByNodeID)); err != nil {
		t.Fatalf("got error: %v", err)
	}

}

func TestWaitVisible(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodeIDs []cdp.NodeID
	err := Run(ctx, NodeIDs("#input2", &nodeIDs, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	if err := Run(ctx, WaitVisible("#input2", ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}

	var value string
	if err := Run(ctx, Value(nodeIDs, &value, ByNodeID)); err != nil {
		t.Fatalf("got error: %v", err)
	}

}

func TestWaitNotVisible(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodeIDs []cdp.NodeID
	err := Run(ctx, NodeIDs("#input2", &nodeIDs, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}
	if err := Run(ctx, Click("#button2", ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if err := Run(ctx, WaitNotVisible("#input2", ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}

	var value string
	if err := Run(ctx, Value(nodeIDs, &value, ByNodeID)); err != nil {
		t.Fatalf("got error: %v", err)
	}

}

func TestWaitEnabled(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var attr string
	var ok bool
	err := Run(ctx, AttributeValue("#select1", "disabled", &attr, &ok, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if !ok {
		t.Fatal("expected element to be disabled")
	}
	if err := Run(ctx, Click("#button3", ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if err := Run(ctx, WaitEnabled("#select1", ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if err := Run(ctx, AttributeValue("#select1", "disabled", &attr, &ok, ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if ok {
		t.Fatal("expected element to be enabled")
	}
	if err := Run(ctx, SetAttributeValue(`//*[@id="select1"]/option[1]`, "selected", "true")); err != nil {
		t.Fatalf("got error: %v", err)
	}

	var value string
	if err := Run(ctx, Value("#select1", &value, ByID)); err != nil {
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

	err := Run(ctx, Click("#button3", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if err := Run(ctx, WaitEnabled("#select1", ByID)); err != nil {
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
	if err := Run(ctx, SetAttributeValue(`//*[@id="select1"]/option[1]`, "selected", "true")); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if err := Run(ctx, WaitSelected(`//*[@id="select1"]/option[1]`)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if err := Run(ctx, AttributeValue(`//*[@id="select1"]/option[1]`, "selected", &attr, nil)); err != nil {
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

	err := Run(ctx, WaitVisible("#input3", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if err := Run(ctx, Click("#button4", ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}
	if err := Run(ctx, WaitNotPresent("#input3", ByID)); err != nil {
		t.Fatalf("got error: %v", err)
	}

}

func TestAtLeast(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "js.html")
	defer cancel()

	var nodes []*cdp.Node
	err := Run(ctx, Nodes("//input", &nodes, AtLeast(3)))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("expected to have at least 3 nodes: got %d", len(nodes))
	}
}
