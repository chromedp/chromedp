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
// See CaptureScreenshot for capturing a screenshot of the browser viewport.
//
// See the 'screenshot' example in the https://github.com/chromedp/examples
// project for an example of taking a screenshot of the entire page.
func Screenshot(sel interface{}, picbuf *[]byte, opts ...QueryOption) QueryAction {
	if picbuf == nil {
		panic("picbuf cannot be nil")
	}

	return QueryAfter(sel, func(ctx context.Context, execCtx runtime.ExecutionContextID, nodes ...*cdp.Node) error {
		if len(nodes) < 1 {
			return fmt.Errorf("selector %q did not return any nodes", sel)
		}

		// get box model
		var clip page.Viewport
		if err := callFunctionOnNode(ctx, nodes[0], getClientRectJS, &clip); err != nil {
			return err
		}

		// The "Capture node screenshot" command does not handle fractional dimensions properly.
		// Let's align with puppeteer:
		// https://github.com/puppeteer/puppeteer/blob/bba3f41286908ced8f03faf98242d4c3359a5efc/src/common/Page.ts#L2002-L2011
		x, y := math.Round(clip.X), math.Round(clip.Y)
		clip.Width, clip.Height = math.Round(clip.Width+clip.X-x), math.Round(clip.Height+clip.Y-y)
		clip.X, clip.Y = x, y

		// The next comment is copied from the original code.
		// This seems to be necessary? Seems to do the right thing regardless of DPI.
		clip.Scale = 1

		// take screenshot of the box
		buf, err := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatPng).
			WithCaptureBeyondViewport(true).
			WithClip(&clip).
			Do(ctx)
		if err != nil {
			return err
		}

		*picbuf = buf
		return nil
	}, append(opts, NodeVisible)...)
}

// CaptureScreenshot is an action that captures/takes a screenshot of the
// current browser viewport.
//
// It's supposed to act the same as the command "Capture screenshot" in
// Chrome. See the behavior notes of Screenshot for more information.
//
// See the Screenshot action to take a screenshot of a specific element.
//
// See the 'screenshot' example in the https://github.com/chromedp/examples
// project for an example of taking a screenshot of the entire page.
func CaptureScreenshot(res *[]byte) Action {
	if res == nil {
		panic("res cannot be nil")
	}

	return ActionFunc(func(ctx context.Context) error {
		var err error
		*res, err = page.CaptureScreenshot().
			WithCaptureBeyondViewport(true).
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
		// get layout metrics
		_, _, contentSize, _, _, cssContentSize, err := page.GetLayoutMetrics().Do(ctx)
		if err != nil {
			return err
		}
		// protocol v90 changed the return parameter name (contentSize -> cssContentSize)
		if cssContentSize != nil {
			contentSize = cssContentSize
		}

		format := page.CaptureScreenshotFormatPng
		if quality != 100 {
			format = page.CaptureScreenshotFormatJpeg
		}

		// capture screenshot
		*res, err = page.CaptureScreenshot().
			WithCaptureBeyondViewport(true).
			WithFormat(format).
			WithQuality(int64(quality)).
			WithClip(&page.Viewport{
				X:      0,
				Y:      0,
				Width:  contentSize.Width,
				Height: contentSize.Height,
				Scale:  1,
			}).Do(ctx)
		if err != nil {
			return err
		}
		return nil
	})
}
