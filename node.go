package chromedp

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/cdp"
)

// Node is a DOM node in the node tree of a frame. It embeds the protocol node
// and adds the tree links and the state that chromedp keeps up to date from
// the browser events.
//
// Hold the lock of a Node before you read a field of the tree.
type Node struct {
	*cdp.Node

	// Parent is the parent node.
	Parent *Node
	// Children shadows the Children field of the protocol node.
	Children []*Node
	// ContentDocument shadows the ContentDocument field of the protocol node.
	ContentDocument *Node
	// ShadowRoots shadows the ShadowRoots field of the protocol node.
	ShadowRoots []*Node
	// TemplateContent shadows the TemplateContent field of the protocol node.
	TemplateContent *Node
	// PseudoElements shadows the PseudoElements field of the protocol node.
	PseudoElements []*Node

	// Invalidated is closed when the node is no longer valid.
	Invalidated chan struct{}
	// State is the state of the node.
	State NodeState

	sync.RWMutex
}

// newNode converts a protocol node and its descendants into a Node tree. It
// moves the child links from the protocol nodes to the new nodes.
func newNode(n *cdp.Node) *Node {
	if n == nil {
		return nil
	}
	nn := &Node{
		Node:            n,
		Children:        newNodes(n.Children),
		ContentDocument: newNode(n.ContentDocument),
		ShadowRoots:     newNodes(n.ShadowRoots),
		TemplateContent: newNode(n.TemplateContent),
		PseudoElements:  newNodes(n.PseudoElements),
	}
	n.Children, n.ContentDocument, n.ShadowRoots = nil, nil, nil
	n.TemplateContent, n.PseudoElements = nil, nil
	return nn
}

// newNodes converts a list of protocol nodes with newNode.
func newNodes(nodes []*cdp.Node) []*Node {
	if len(nodes) == 0 {
		return nil
	}
	res := make([]*Node, len(nodes))
	for i, n := range nodes {
		res[i] = newNode(n)
	}
	return res
}

// AttributeValue returns the named attribute for the node.
func (n *Node) AttributeValue(name string) string {
	value, _ := n.Attribute(name)
	return value
}

// Attribute returns the named attribute for the node and if it exists.
func (n *Node) Attribute(name string) (string, bool) {
	n.RLock()
	defer n.RUnlock()

	for i := 0; i+1 < len(n.Attributes); i += 2 {
		if n.Attributes[i] == name {
			return n.Attributes[i+1], true
		}
	}
	return "", false
}

// xpath builds the xpath string.
func (n *Node) xpath(stopAtDocument, stopAtID bool) string {
	n.RLock()
	defer n.RUnlock()

	p, pos := "", ""
	id := n.attributeValueLocked("id")
	switch {
	case n.Parent == nil:
		return n.LocalName

	case stopAtDocument && n.NodeType == NodeTypeDocument:
		return ""

	case stopAtID && id != "":
		p = "/"
		pos = `[@id='` + id + `']`

	case n.Parent != nil:
		var i int
		var found bool

		n.Parent.RLock()
		for j := 0; j < len(n.Parent.Children); j++ {
			if n.Parent.Children[j].LocalName == n.LocalName {
				i++
			}
			if n.Parent.Children[j].NodeID == n.NodeID {
				found = true
				break
			}
		}
		n.Parent.RUnlock()

		if found {
			pos = "[" + strconv.Itoa(i) + "]"
		}

		p = n.Parent.xpath(stopAtDocument, stopAtID)
	}

	localName := n.LocalName
	if n.IsSVG {
		localName = `*[local-name()='` + localName + `']`
	}
	return p + "/" + localName + pos
}

// attributeValueLocked returns the named attribute. The caller must hold the
// lock of the node.
func (n *Node) attributeValueLocked(name string) string {
	for i := 0; i+1 < len(n.Attributes); i += 2 {
		if n.Attributes[i] == name {
			return n.Attributes[i+1]
		}
	}
	return ""
}

// PartialXPathByID returns the partial XPath for the node. It stops at the
// first parent with an id attribute or at the nearest parent document node.
func (n *Node) PartialXPathByID() string {
	return n.xpath(true, true)
}

// PartialXPath returns the partial XPath for the node. It stops at the nearest
// parent document node.
func (n *Node) PartialXPath() string {
	return n.xpath(true, false)
}

// FullXPathByID returns the full XPath for the node. It stops at the top most
// document root or at the closest parent node with an id attribute.
func (n *Node) FullXPathByID() string {
	return n.xpath(false, true)
}

// FullXPath returns the full XPath for the node. It stops only at the top most
// document root.
func (n *Node) FullXPath() string {
	return n.xpath(false, false)
}

// WriteTo writes a readable representation of the node and all its children to w.
func (n *Node) WriteTo(w io.Writer, prefix, indent string, nodeIDs bool) (int, error) {
	if n == nil {
		return w.Write([]byte(prefix + "<nil>"))
	}

	n.RLock()
	defer n.RUnlock()

	var err error
	var nn, c int

	// prefix
	if c, err = w.Write([]byte(prefix)); err != nil {
		return nn + c, err
	}
	nn += c

	// node name
	name := n.NodeName
	if n.LocalName != "" {
		name = n.LocalName
	}
	if c, err = w.Write([]byte(name)); err != nil {
		return nn + c, err
	}
	nn += c

	// add #id
	var hasID int
	for i := 0; i+1 < len(n.Attributes); i += 2 {
		if strings.ToLower(n.Attributes[i]) == "id" {
			if c, err = w.Write([]byte("#" + n.Attributes[i+1])); err != nil {
				return nn + c, err
			}
			nn += c
			hasID = 2
			break
		}
	}

	// node type
	if n.NodeType != NodeTypeElement && n.NodeType != NodeTypeText {
		if c, err = fmt.Fprintf(w, " <%s>", NodeType(n.NodeType)); err != nil {
			return nn + c, err
		}
		nn += c
	}

	// node value
	if n.NodeType == NodeTypeText {
		v := n.NodeValue
		if len(v) > 15 {
			v = v[:15] + "..."
		}
		if c, err = fmt.Fprintf(w, " %q", v); err != nil {
			return nn + c, err
		}
		nn += c
	}

	// attributes
	if n.NodeType == NodeTypeElement && len(n.Attributes) > hasID {
		if c, err = w.Write([]byte(" [")); err != nil {
			return nn + c, err
		}
		nn += c
		for i, space := 0, ""; i+1 < len(n.Attributes); i += 2 {
			if strings.ToLower(n.Attributes[i]) == "id" {
				continue
			}
			if c, err = fmt.Fprintf(w, "%s%s=%q", space, n.Attributes[i], n.Attributes[i+1]); err != nil {
				return nn + c, err
			}
			nn += c
			if space == "" {
				space = " "
			}
		}
		if c, err = w.Write([]byte{']'}); err != nil {
			return nn + c, err
		}
		nn += c
	}

	// node id
	if nodeIDs {
		if c, err = fmt.Fprintf(w, " (%d)", n.NodeID); err != nil {
			return nn + c, err
		}
		nn += c
	}

	// children
	for _, child := range n.Children {
		if c, err = fmt.Fprintln(w); err != nil {
			return nn + c, err
		}
		nn += c
		if c, err = child.WriteTo(w, prefix+indent, indent, nodeIDs); err != nil {
			return nn + c, err
		}
		nn += c
	}
	return nn, nil
}

// Dump builds a printable string representation of the node and its children.
func (n *Node) Dump(prefix, indent string, nodeIDs bool) string {
	var buf bytes.Buffer
	_, _ = n.WriteTo(&buf, prefix, indent, nodeIDs)
	return buf.String()
}

// NodeType is the type of a DOM node. It is the value of the NodeType field of
// the protocol node.
type NodeType int64

// NodeType values.
const (
	NodeTypeElement               = 1
	NodeTypeAttribute             = 2
	NodeTypeText                  = 3
	NodeTypeCDATA                 = 4
	NodeTypeEntityReference       = 5
	NodeTypeEntity                = 6
	NodeTypeProcessingInstruction = 7
	NodeTypeComment               = 8
	NodeTypeDocument              = 9
	NodeTypeDocumentType          = 10
	NodeTypeDocumentFragment      = 11
	NodeTypeNotation              = 12
)

// nodeTypeNames are the names of the node types.
var nodeTypeNames = map[NodeType]string{
	NodeTypeElement:               "Element",
	NodeTypeAttribute:             "Attribute",
	NodeTypeText:                  "Text",
	NodeTypeCDATA:                 "CDATA",
	NodeTypeEntityReference:       "EntityReference",
	NodeTypeEntity:                "Entity",
	NodeTypeProcessingInstruction: "ProcessingInstruction",
	NodeTypeComment:               "Comment",
	NodeTypeDocument:              "Document",
	NodeTypeDocumentType:          "DocumentType",
	NodeTypeDocumentFragment:      "DocumentFragment",
	NodeTypeNotation:              "Notation",
}

// String returns the name of the node type.
func (t NodeType) String() string {
	if s, ok := nodeTypeNames[t]; ok {
		return s
	}
	return "NodeType(" + strconv.FormatInt(int64(t), 10) + ")"
}

// NodeState is the state of a DOM node.
type NodeState uint8

// NodeState enum values.
const (
	NodeStateReady NodeState = 1 << (7 - iota)
	NodeStateVisible
	NodeStateHighlighted
)

// nodeStateNames are the names of the node states.
var nodeStateNames = map[NodeState]string{
	NodeStateReady:       "Ready",
	NodeStateVisible:     "Visible",
	NodeStateHighlighted: "Highlighted",
}

// String satisfies stringer interface.
func (ns NodeState) String() string {
	var s []string
	for k, v := range nodeStateNames {
		if ns&k != 0 {
			s = append(s, v)
		}
	}
	return "[" + strings.Join(s, " ") + "]"
}

// EmptyNodeID is the "non-existent" node id.
const EmptyNodeID = cdp.NodeID(0)
