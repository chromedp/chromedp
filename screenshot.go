package chromedp

import (
	"context"
	"math"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
)

// Screenshot is an element query action that takes a screenshot of the first element
// node matching the selector.
//
// It's supposed to act the same as the command "Capture node screenshot" in Chrome.
//
// Behavior notes: the Protocol Monitor shows that the command sends the following
// CDP commands too:
//   - Emulation.clearDeviceMetricsOverride
//   - Network.setUserAgentOverride with {"userAgent": ""}
//   - Overlay.setShowViewportSizeOnResize with {"show": false}
//
// These CDP commands are not sent by chromedp. If it does not work as expected,
// you can try to send those commands yourself.
//
// See [CaptureScreenshot] for capturing a screenshot of the browser viewport.
//
// See [screenshot] for an example of taking a screenshot of the entire page.
//
// [screenshot]: https://github.com/chromedp/examples/tree/master/screenshot
func Screenshot(sel any, opts ...QueryOption) Action[[]byte] {
	return ScreenshotScale(sel, 1, opts...)
}

// ScreenshotScale is like [Screenshot] but accepts a scale parameter that
// specifies the page scale factor.
func ScreenshotScale(sel any, scale float64, opts ...QueryOption) Action[[]byte] {
	return QueryAfter(sel, func(ctx context.Context, t *Target, nodes []*Node) ([]byte, error) {
		if _, err := first(sel, nodes); err != nil {
			return nil, err
		}
		return ScreenshotNodes(nodes, scale)(ctx, t)
	}, withOpts(opts, NodeVisible)...)
}

// ScreenshotNodes is an action that captures/takes a screenshot of the
// specified nodes, by calculating the extents of the top most left node and
// bottom most right node.
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

		// The "Capture node screenshot" command does not handle fractional dimensions properly.
		// Let's align with puppeteer:
		// https://github.com/puppeteer/puppeteer/blob/bba3f41286908ced8f03faf98242d4c3359a5efc/src/common/Page.ts#L2002-L2011
		x, y := math.Round(clip.X), math.Round(clip.Y)
		clip.Width, clip.Height = math.Round(clip.Width+clip.X-x), math.Round(clip.Height+clip.Y-y)
		clip.X, clip.Y = x, y

		clip.Scale = scale

		// take screenshot of the box
		res, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{
			Format:                page.CaptureScreenshotFormatPng,
			CaptureBeyondViewport: new(true),
			FromSurface:           new(true),
			Clip:                  &clip,
		})
		if err != nil {
			return nil, err
		}

		return res.Data, nil
	}
}

// CaptureScreenshot is an action that captures/takes a screenshot of the
// current browser viewport.
//
// It's supposed to act the same as the command "Capture screenshot" in
// Chrome. See the behavior notes of Screenshot for more information.
//
// See the [Screenshot] action to take a screenshot of a specific element.
//
// See [screenshot] for an example of taking a screenshot of the entire page.
//
// [screenshot]: https://github.com/chromedp/examples/tree/master/screenshot
func CaptureScreenshot() Action[[]byte] {
	return func(ctx context.Context, t *Target) ([]byte, error) {
		r, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{FromSurface: new(true)})
		if err != nil {
			return nil, err
		}
		return r.Data, nil
	}
}

// FullScreenshot takes a full screenshot with the specified image quality of
// the entire browser viewport.
//
// It's supposed to act the same as the command "Capture full size screenshot"
// in Chrome. See the behavior notes of Screenshot for more information.
//
// The valid range of the compression quality is [0..100]. When this value is
// 100, the image format is png; otherwise, the image format is jpeg.
func FullScreenshot(quality int) Action[[]byte] {
	return func(ctx context.Context, t *Target) ([]byte, error) {
		format := page.CaptureScreenshotFormatPng
		if quality != 100 {
			format = page.CaptureScreenshotFormatJpeg
		}

		// capture screenshot
		r, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{
			CaptureBeyondViewport: new(true),
			FromSurface:           new(true),
			Format:                format,
			Quality:               int64(quality),
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
