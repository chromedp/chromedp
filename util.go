package chromedp

import (
	. "github.com/knq/chromedp/cdp"
	"github.com/knq/chromedp/cdp/util"
)

const (
	// emptyFrameID is the "non-existent" (ie current) frame.
	emptyFrameID FrameID = FrameID("")

	// emptyNodeID is the "non-existent" node id.
	emptyNodeID NodeID = NodeID(0)
)

// UnmarshalMessage unmarshals the message result or params.
func UnmarshalMessage(msg *Message) (interface{}, error) {
	return util.UnmarshalMessage(msg)
}

// FrameOp is a frame manipulation operation.
type FrameOp func(*Frame)

/*func domContentEventFired(f *Frame) {
}

func loadEventFired(f *Frame) {
}*/

func frameAttached(id FrameID) FrameOp {
	return func(f *Frame) {
		f.ParentID = id
		setFrameState(f, FrameAttached)
	}
}

/*func frameNavigated(f *Frame) {
	setFrameState(f, FrameNavigated)
}*/

func frameDetached(f *Frame) {
	f.ParentID = emptyFrameID
	clearFrameState(f, FrameAttached)
}

func frameStartedLoading(f *Frame) {
	setFrameState(f, FrameLoading)
}

func frameStoppedLoading(f *Frame) {
	clearFrameState(f, FrameLoading)
}

func frameScheduledNavigation(f *Frame) {
	setFrameState(f, FrameScheduledNavigation)
}

func frameClearedScheduledNavigation(f *Frame) {
	clearFrameState(f, FrameScheduledNavigation)
}

/*func frameResized(f *Frame) {
	// TODO
}*/

// setFrameState sets the frame state via bitwise or (|).
func setFrameState(f *Frame, fs FrameState) {
	f.State |= fs
}

// clearFrameState clears the frame state via bit clear (&^).
func clearFrameState(f *Frame, fs FrameState) {
	f.State &^= fs
}

// NodeOp is a node manipulation operation.
type NodeOp func(*Node)

func walk(m map[NodeID]*Node, n *Node) {
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

	for _, c := range []*Node{n.ContentDocument, n.TemplateContent, n.ImportedDocument} {
		if c == nil {
			continue
		}

		c.Parent = n
		c.Invalidated = n.Invalidated
		walk(m, c)
	}
}

func setChildNodes(m map[NodeID]*Node, nodes []*Node) NodeOp {
	return func(n *Node) {
		n.Children = nodes
		walk(m, n)
	}
}

func attributeModified(name, value string) NodeOp {
	return func(n *Node) {
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
	return func(n *Node) {
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

func inlineStyleInvalidated(ids []NodeID) NodeOp {
	return func(n *Node) {
	}
}

func characterDataModified(characterData string) NodeOp {
	return func(n *Node) {
		n.Value = characterData
	}
}

func childNodeCountUpdated(count int64) NodeOp {
	return func(n *Node) {
		n.ChildNodeCount = count
	}
}

func childNodeInserted(m map[NodeID]*Node, prevID NodeID, c *Node) NodeOp {
	return func(n *Node) {
		n.Children = insertNode(n.Children, prevID, c)
		walk(m, n)
	}
}

func childNodeRemoved(m map[NodeID]*Node, id NodeID) NodeOp {
	return func(n *Node) {
		n.Children = removeNode(n.Children, id)
		//delete(m, id)
	}
}

func shadowRootPushed(m map[NodeID]*Node, c *Node) NodeOp {
	return func(n *Node) {
		n.ShadowRoots = append(n.ShadowRoots, c)
		walk(m, n)
	}
}

func shadowRootPopped(m map[NodeID]*Node, id NodeID) NodeOp {
	return func(n *Node) {
		n.ShadowRoots = removeNode(n.ShadowRoots, id)
		//delete(m, id)
	}
}

func pseudoElementAdded(m map[NodeID]*Node, c *Node) NodeOp {
	return func(n *Node) {
		n.PseudoElements = append(n.PseudoElements, c)
		walk(m, n)
	}
}

func pseudoElementRemoved(m map[NodeID]*Node, id NodeID) NodeOp {
	return func(n *Node) {
		n.PseudoElements = removeNode(n.PseudoElements, id)
		//delete(m, id)
	}
}

func distributedNodesUpdated(nodes []*BackendNode) NodeOp {
	return func(n *Node) {
		n.DistributedNodes = nodes
	}
}

func nodeHighlightRequested(n *Node) {
	// TODO
}

func insertNode(n []*Node, prevID NodeID, c *Node) []*Node {
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

func removeNode(n []*Node, id NodeID) []*Node {
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
