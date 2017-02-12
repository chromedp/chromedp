package chromedp

import (
	"time"

	"github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/util"
)

const (
	// DefaultNewTargetTimeout is the default time to wait for a new target to
	// be started.
	DefaultNewTargetTimeout = 3 * time.Second

	// DefaultCheckDuration is the default time to sleep between a check.
	DefaultCheckDuration = 50 * time.Millisecond

	// EmptyFrameID is the "non-existent" (ie current) frame.
	EmptyFrameID cdp.FrameID = cdp.FrameID("")

	// EmptyNodeID is the "non-existent" node id.
	EmptyNodeID cdp.NodeID = cdp.NodeID(0)
)

// UnmarshalMessage unmarshals the message result or params.
func UnmarshalMessage(msg *cdp.Message) (interface{}, error) {
	return util.UnmarshalMessage(msg)
}

// FrameOp is a frame manipulation operation.
type FrameOp func(*cdp.Frame)

/*func domContentEventFired(f *cdp.Frame) {
}

func loadEventFired(f *cdp.Frame) {
}*/

func frameAttached(id cdp.FrameID) FrameOp {
	return func(f *cdp.Frame) {
		f.ParentID = id
		setFrameState(f, cdp.FrameAttached)
	}
}

/*func frameNavigated(f *cdp.Frame) {
	setFrameState(f, cdp.FrameNavigated)
}*/

func frameDetached(f *cdp.Frame) {
	f.ParentID = EmptyFrameID
	clearFrameState(f, cdp.FrameAttached)
}

func frameStartedLoading(f *cdp.Frame) {
	setFrameState(f, cdp.FrameLoading)
}

func frameStoppedLoading(f *cdp.Frame) {
	clearFrameState(f, cdp.FrameLoading)
}

func frameScheduledNavigation(f *cdp.Frame) {
	setFrameState(f, cdp.FrameScheduledNavigation)
}

func frameClearedScheduledNavigation(f *cdp.Frame) {
	clearFrameState(f, cdp.FrameScheduledNavigation)
}

/*func frameResized(f *cdp.Frame) {
	// TODO
}*/

// setFrameState sets the frame state via bitwise or (|).
func setFrameState(f *cdp.Frame, fs cdp.FrameState) {
	f.State |= fs
}

// clearFrameState clears the frame state via bit clear (&^).
func clearFrameState(f *cdp.Frame, fs cdp.FrameState) {
	f.State &^= fs
}

// NodeOp is a node manipulation operation.
type NodeOp func(*cdp.Node)

func walk(m map[cdp.NodeID]*cdp.Node, n *cdp.Node) {
	m[n.NodeID] = n

	for _, c := range n.Children {
		c.Parent = n
		c.Invalidated = n.Invalidated
		walk(m, c)
	}

	for _, c := range n.ShadowRoots {
		c.Parent = n
		c.Invalidated = n.Invalidated
		walk(m, c)
	}

	for _, c := range n.PseudoElements {
		c.Parent = n
		c.Invalidated = n.Invalidated
		walk(m, c)
	}

	for _, c := range []*cdp.Node{n.ContentDocument, n.TemplateContent, n.ImportedDocument} {
		if c == nil {
			continue
		}

		c.Parent = n
		c.Invalidated = n.Invalidated
		walk(m, c)
	}
}

func setChildNodes(m map[cdp.NodeID]*cdp.Node, nodes []*cdp.Node) NodeOp {
	return func(n *cdp.Node) {
		n.Children = nodes
		walk(m, n)
	}
}

func attributeModified(name, value string) NodeOp {
	return func(n *cdp.Node) {
		var found bool

		i := 0
		for ; i < len(n.Attributes); i += 2 {
			if n.Attributes[i] == name {
				found = true
				break
			}
		}

		if found {
			n.Attributes[i] = name
			n.Attributes[i+1] = value
		} else {
			n.Attributes = append(n.Attributes, name, value)
		}
	}
}

func attributeRemoved(name string) NodeOp {
	return func(n *cdp.Node) {
		var a []string
		for i := 0; i < len(n.Attributes); i += 2 {
			if n.Attributes[i] == name {
				continue
			}
			a = append(a, n.Attributes[i], n.Attributes[i+1])
		}
		n.Attributes = a
	}
}

func inlineStyleInvalidated(ids []cdp.NodeID) NodeOp {
	return func(n *cdp.Node) {
	}
}

func characterDataModified(characterData string) NodeOp {
	return func(n *cdp.Node) {
		n.Value = characterData
	}
}

func childNodeCountUpdated(count int64) NodeOp {
	return func(n *cdp.Node) {
		n.ChildNodeCount = count
	}
}

func childNodeInserted(m map[cdp.NodeID]*cdp.Node, prevID cdp.NodeID, c *cdp.Node) NodeOp {
	return func(n *cdp.Node) {
		n.Children = insertNode(n.Children, prevID, c)
		walk(m, n)
	}
}

func childNodeRemoved(m map[cdp.NodeID]*cdp.Node, id cdp.NodeID) NodeOp {
	return func(n *cdp.Node) {
		n.Children = removeNode(n.Children, id)
		//delete(m, id)
	}
}

func shadowRootPushed(m map[cdp.NodeID]*cdp.Node, c *cdp.Node) NodeOp {
	return func(n *cdp.Node) {
		n.ShadowRoots = append(n.ShadowRoots, c)
		walk(m, n)
	}
}

func shadowRootPopped(m map[cdp.NodeID]*cdp.Node, id cdp.NodeID) NodeOp {
	return func(n *cdp.Node) {
		n.ShadowRoots = removeNode(n.ShadowRoots, id)
		//delete(m, id)
	}
}

func pseudoElementAdded(m map[cdp.NodeID]*cdp.Node, c *cdp.Node) NodeOp {
	return func(n *cdp.Node) {
		n.PseudoElements = append(n.PseudoElements, c)
		walk(m, n)
	}
}

func pseudoElementRemoved(m map[cdp.NodeID]*cdp.Node, id cdp.NodeID) NodeOp {
	return func(n *cdp.Node) {
		n.PseudoElements = removeNode(n.PseudoElements, id)
		//delete(m, id)
	}
}

func distributedNodesUpdated(nodes []*cdp.BackendNode) NodeOp {
	return func(n *cdp.Node) {
		n.DistributedNodes = nodes
	}
}

func nodeHighlightRequested(n *cdp.Node) {
	// TODO
}

func insertNode(n []*cdp.Node, prevID cdp.NodeID, c *cdp.Node) []*cdp.Node {
	i := 0
	found := false
	for ; i < len(n); i++ {
		if n[i].NodeID == prevID {
			found = true
			break
		}
	}

	if !found {
		return append(n, c)
	}

	i++
	n = append(n, nil)
	copy(n[i+1:], n[i:])
	n[i] = c

	return n
}

func removeNode(n []*cdp.Node, id cdp.NodeID) []*cdp.Node {
	if len(n) == 0 {
		return n
	}

	var found bool
	i := 0
	for ; i < len(n); i++ {
		if n[i].NodeID == id {
			found = true
			break
		}
	}

	if !found {
		return n
	}

	return append(n[:i], n[i+1:]...)
}

// isCouldNotComputeBoxModelError unwraps the err as a MessagError and
// determines if it is a compute box model error.
func isCouldNotComputeBoxModelError(err error) bool {
	e, ok := err.(*cdp.MessageError)
	return ok && e.Code == -32000 && e.Message == "Could not compute box model."
}
