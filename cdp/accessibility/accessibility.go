// Package accessibility provides the Chrome Debugging Protocol
// commands, types, and events for the Accessibility domain.
//
// Generated by the chromedp-gen command.
package accessibility

// Code generated by chromedp-gen. DO NOT EDIT.

import (
	"context"

	cdp "github.com/igsky/chromedp/cdp"
)

// GetPartialAXTreeParams fetches the accessibility node and partial
// accessibility tree for this DOM node, if it exists.
type GetPartialAXTreeParams struct {
	NodeID         cdp.NodeID `json:"nodeId"`                   // ID of node to get the partial accessibility tree for.
	FetchRelatives bool       `json:"fetchRelatives,omitempty"` // Whether to fetch this nodes ancestors, siblings and children. Defaults to true.
}

// GetPartialAXTree fetches the accessibility node and partial accessibility
// tree for this DOM node, if it exists.
//
// parameters:
//   nodeID - ID of node to get the partial accessibility tree for.
func GetPartialAXTree(nodeID cdp.NodeID) *GetPartialAXTreeParams {
	return &GetPartialAXTreeParams{
		NodeID: nodeID,
	}
}

// WithFetchRelatives whether to fetch this nodes ancestors, siblings and
// children. Defaults to true.
func (p GetPartialAXTreeParams) WithFetchRelatives(fetchRelatives bool) *GetPartialAXTreeParams {
	p.FetchRelatives = fetchRelatives
	return &p
}

// GetPartialAXTreeReturns return values.
type GetPartialAXTreeReturns struct {
	Nodes []*AXNode `json:"nodes,omitempty"` // The Accessibility.AXNode for this DOM node, if it exists, plus its ancestors, siblings and children, if requested.
}

// Do executes Accessibility.getPartialAXTree against the provided context and
// target handler.
//
// returns:
//   nodes - The Accessibility.AXNode for this DOM node, if it exists, plus its ancestors, siblings and children, if requested.
func (p *GetPartialAXTreeParams) Do(ctxt context.Context, h cdp.Handler) (nodes []*AXNode, err error) {
	// execute
	var res GetPartialAXTreeReturns
	err = h.Execute(ctxt, cdp.CommandAccessibilityGetPartialAXTree, p, &res)
	if err != nil {
		return nil, err
	}

	return res.Nodes, nil
}
