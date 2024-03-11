package chromedp

import (
	"context"
	"fmt"
	"math"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
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
func Screenshot(sel interface{}, picbuf *[]byte, opts ...QueryOption) QueryAction {
	return ScreenshotScale(sel, 1, picbuf, opts...)
}

// ScreenshotScale is like [Screenshot] but accepts a scale parameter that
// specifies the page scale factor.
func ScreenshotScale(sel interface{}, scale float64, picbuf *[]byte, opts ...QueryOption) QueryAction {
	if picbuf == nil {
		panic("picbuf cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, execCtx runtime.ExecutionContextID, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}
		return ScreenshotNodes(nodes, scale, picbuf).Do(ctx)
	}, append(opts, NodeVisible)...)
}

// ScreenshotNodes is an action that captures/takes a screenshot of the
// specified nodes, by calculating the extents of the top most left node and
// bottom most right node.
func ScreenshotNodes(nodes []*cdp.Node, scale float64, picbuf *[]byte) Action {
	if len(nodes) == 0 {
		panic("nodes must be non-empty")
	}
	if picbuf == nil {
		panic("picbuf cannot be nil")
	}

	return ActionFunc(func(ctx context.Context) error {
		var clip page.Viewport

		// get box model of first node
		if err := callFunctionOnNode(ctx, nodes[0], getClientRectJS, &clip); err != nil {
			return err
		}

		// remainder
		for _, node := range nodes[1:] {
			var v page.Viewport
			// get box model of first node
			if err := callFunctionOnNode(ctx, node, getClientRectJS, &v); err != nil {
				return err
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
		buf, err := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatPng).
			WithCaptureBeyondViewport(true).
			WithFromSurface(true).
			WithClip(&clip).
			Do(ctx)
		if err != nil {
			return err
		}

		*picbuf = buf
		return nil
	})
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
func CaptureScreenshot(res *[]byte) Action {
	if res == nil {
		panic("res cannot be nil")
	}

	return ActionFunc(func(ctx context.Context) error {
		var err error
		*res, err = page.CaptureScreenshot().
			WithFromSurface(true).
			Do(ctx)
		return err
	})
}

// FullScreenshot takes a full screenshot with the specified image quality of
// the entire browser viewport.
//
// It's supposed to act the same as the command "Capture full size screenshot"
// in Chrome. See the behavior notes of Screenshot for more information.
//
// The valid range of the compression quality is [0..100]. When this value is
// 100, the image format is png; otherwise, the image format is jpeg.
func FullScreenshot(res *[]byte, quality int) EmulateAction {
	if res == nil {
		panic("res cannot be nil")
	}
	return ActionFunc(func(ctx context.Context) error {
		format := page.CaptureScreenshotFormatPng
		if quality != 100 {
			format = page.CaptureScreenshotFormatJpeg
		}

		// capture screenshot
		var err error
		*res, err = page.CaptureScreenshot().
			WithCaptureBeyondViewport(true).
			WithFromSurface(true).
			WithFormat(format).
			WithQuality(int64(quality)).
			Do(ctx)
		if err != nil {
			return err
		}
		return nil
	})
}

func extents(m, n, o, p float64) (float64, float64) {
	a := min(m, o)
	b := max(m+n, o+p)
	return a, b - a
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
