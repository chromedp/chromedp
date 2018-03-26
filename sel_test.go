package chromedp

import (
	"testing"

	"github.com/chromedp/cdproto/cdp"
)

func TestWaitReady(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "js.html")
	defer c.Release()

	var nodeIDs []cdp.NodeID
	err := c.Run(defaultContext, NodeIDs("#input2", &nodeIDs, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}

	err = c.Run(defaultContext, WaitReady("#input2", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	var value string
	err = c.Run(defaultContext, Value(nodeIDs, &value, ByNodeID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitVisible(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "js.html")
	defer c.Release()

	var nodeIDs []cdp.NodeID
	err := c.Run(defaultContext, NodeIDs("#input2", &nodeIDs, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}

	err = c.Run(defaultContext, WaitVisible("#input2", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	var value string
	err = c.Run(defaultContext, Value(nodeIDs, &value, ByNodeID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitNotVisible(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "js.html")
	defer c.Release()

	var nodeIDs []cdp.NodeID
	err := c.Run(defaultContext, NodeIDs("#input2", &nodeIDs, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodeIDs) != 1 {
		t.Errorf("expected to have exactly 1 node id: got %d", len(nodeIDs))
	}

	err = c.Run(defaultContext, Click("#button2", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	err = c.Run(defaultContext, WaitNotVisible("#input2", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	var value string
	err = c.Run(defaultContext, Value(nodeIDs, &value, ByNodeID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestWaitEnabled(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "js.html")
	defer c.Release()

	var attr string
	var ok bool
	err := c.Run(defaultContext, AttributeValue("#select1", "disabled", &attr, &ok, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if !ok {
		t.Fatal("expected element to be disabled")
	}

	err = c.Run(defaultContext, Click("#button3", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	err = c.Run(defaultContext, WaitEnabled("#select1", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	err = c.Run(defaultContext, AttributeValue("#select1", "disabled", &attr, &ok, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if ok {
		t.Fatal("expected element to be enabled")
	}

	err = c.Run(defaultContext, SetAttributeValue(`//*[@id="select1"]/option[1]`, "selected", "true"))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	var value string
	err = c.Run(defaultContext, Value("#select1", &value, ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if value != "foo" {
		t.Fatalf("expected value to be foo, got: %s", value)
	}
}

func TestWaitSelected(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "js.html")
	defer c.Release()

	err := c.Run(defaultContext, Click("#button3", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	err = c.Run(defaultContext, WaitEnabled("#select1", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	var attr string
	ok := false
	err = c.Run(defaultContext, AttributeValue(`//*[@id="select1"]/option[1]`, "selected", &attr, &ok))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if ok {
		t.Fatal("expected element to be not selected")
	}

	err = c.Run(defaultContext, SetAttributeValue(`//*[@id="select1"]/option[1]`, "selected", "true"))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	err = c.Run(defaultContext, WaitSelected(`//*[@id="select1"]/option[1]`))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	err = c.Run(defaultContext, AttributeValue(`//*[@id="select1"]/option[1]`, "selected", &attr, nil))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if attr != "true" {
		t.Fatal("expected element to be selected")
	}
}

func TestWaitNotPresent(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "js.html")
	defer c.Release()

	err := c.Run(defaultContext, WaitVisible("#input3", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	err = c.Run(defaultContext, Click("#button4", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	err = c.Run(defaultContext, WaitNotPresent("#input3", ByID))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
}

func TestAtLeast(t *testing.T) {
	t.Parallel()

	c := testAllocate(t, "js.html")
	defer c.Release()

	var nodes []*cdp.Node
	err := c.Run(defaultContext, Nodes("//input", &nodes, AtLeast(3)))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("expected to have at least 3 nodes: got %d", len(nodes))
	}
}
