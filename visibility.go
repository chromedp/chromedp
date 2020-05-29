package chromedp

import (
	"context"
	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
)

// isCouldNotComputeBoxModelError unwraps err as a MessageError and determines
// if it is a compute box model error.
func isCouldNotComputeBoxModelError(err error) bool {
	e, ok := err.(*cdproto.Error)
	return ok && e.Code == -32000 && e.Message == "Could not compute box model."
}

func isNodeVisible(ctx context.Context, n *cdp.Node, s *Selector) (bool, error) {
	// check box model
	_, err := dom.GetBoxModel().WithNodeID(n.NodeID).Do(ctx)
	if err != nil {
		if isCouldNotComputeBoxModelError(err) {
			return false, nil
		}

		return false, err
	}

	// check visibility
	var res bool
	err = EvaluateAsDevTools(snippet(visibleJS, cashX(true), s, n), &res).Do(ctx)
	if err != nil {
		return false, err
	}
	if !res {
		return false, nil
	}
	return true, nil
}
