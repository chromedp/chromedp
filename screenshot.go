package chromedp

import (
	"context"
	"math"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
)

// Screenshot is an element query action that takes a screenshot of the first
// element node that matches the selector.
//
// It acts like the command "Capture node screenshot" in Chrome.
//
// Behavior notes: the Protocol Monitor shows that the command also sends these
// protocol commands:
//   - Emulation.clearDeviceMetricsOverride
//   - Network.setUserAgentOverride with {"userAgent": ""}
//   - Overlay.setShowViewportSizeOnResize with {"show": false}
//
// chromedp does not send these protocol commands. If the result is not what
// you expect, send them yourself.
//
// To capture the browser viewport, see [CaptureScreenshot].
//
// For an example that takes a screenshot of the entire page, see [screenshot].
//
// [screenshot]: https://github.com/chromedp/examples/tree/main/screenshot
func Screenshot[S Selectable](sel S, opts ...QueryOption) Action[[]byte] {
	return ScreenshotScale(sel, 1, opts...)
}

// ScreenshotScale is like [Screenshot] but takes a scale parameter, which is
// the page scale factor.
func ScreenshotScale[S Selectable](sel S, scale float64, opts ...QueryOption) Action[[]byte] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) ([]byte, error) {
		if _, err := first(sel, nodes); err != nil {
			return nil, err
		}
		return ScreenshotNodes(nodes, scale)(ctx, t)
	}, withOpts(opts, NodeVisible)...)
}

// ScreenshotNodes is an action that takes a screenshot of the specified nodes.
// It calculates the extents from the top most left node and the bottom most
// right node.
func ScreenshotNodes(nodes []*Node, scale float64) Action[[]byte] {
	if len(nodes) == 0 {
		panic("nodes must be non-empty")
	}

	return func(ctx context.Context, t *Target) ([]byte, error) {
		// get box model of first node
		clip, err := callFunctionOnNode[page.Viewport](ctx, t, nodes[0], getClientRectJS)
		if err != nil {
			return nil, err
		}

		// remainder
		for _, node := range nodes[1:] {
			// get box model of the node
			v, err := callFunctionOnNode[page.Viewport](ctx, t, node, getClientRectJS)
			if err != nil {
				return nil, err
			}
			clip.X, clip.Width = extents(clip.X, clip.Width, v.X, v.Width)
			clip.Y, clip.Height = extents(clip.Y, clip.Height, v.Y, v.Height)
		}

		// The "Capture node screenshot" command does not handle fractional dimensions correctly.
		// Do the same as puppeteer:
		// https://github.com/puppeteer/puppeteer/blob/bba3f41286908ced8f03faf98242d4c3359a5efc/src/common/Page.ts#L2002-L2011
		x, y := math.Round(clip.X), math.Round(clip.Y)
		clip.Width, clip.Height = math.Round(clip.Width+clip.X-x), math.Round(clip.Height+clip.Y-y)
		clip.X, clip.Y = x, y

		clip.Scale = scale

		// take screenshot of the box
		res, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{
			Format:                page.CaptureScreenshotFormatPng,
			CaptureBeyondViewport: ptr(true),
			FromSurface:           ptr(true),
			Clip:                  &clip,
		})
		if err != nil {
			return nil, err
		}

		return res.Data, nil
	}
}

// CaptureScreenshot is an action that takes a screenshot of the current browser
// viewport.
//
// It acts like the command "Capture screenshot" in Chrome. See the behavior
// notes of Screenshot for more information.
//
// To take a screenshot of one element, use the [Screenshot] action.
//
// For an example that takes a screenshot of the entire page, see [screenshot].
//
// [screenshot]: https://github.com/chromedp/examples/tree/main/screenshot
func CaptureScreenshot() Action[[]byte] {
	return func(ctx context.Context, t *Target) ([]byte, error) {
		r, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{FromSurface: ptr(true)})
		if err != nil {
			return nil, err
		}
		return r.Data, nil
	}
}

// FullScreenshot takes a full screenshot of the entire browser viewport, with
// the given image quality.
//
// It acts like the command "Capture full size screenshot" in Chrome. See the
// behavior notes of Screenshot for more information.
//
// The valid range of the compression quality is [0..100]. When the quality is
// 100, the image format is png. Otherwise the image format is jpeg.
func FullScreenshot(quality int) Action[[]byte] {
	return func(ctx context.Context, t *Target) ([]byte, error) {
		format := page.CaptureScreenshotFormatPng
		if quality != 100 {
			format = page.CaptureScreenshotFormatJpeg
		}

		// capture screenshot
		q := int64(quality)
		r, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{
			CaptureBeyondViewport: ptr(true),
			FromSurface:           ptr(true),
			Format:                format,
			Quality:               &q,
		})
		if err != nil {
			return nil, err
		}
		return r.Data, nil
	}
}

func extents(m, n, o, p float64) (float64, float64) {
	a := min(m, o)
	b := max(m+n, o+p)
	return a, b - a
}
