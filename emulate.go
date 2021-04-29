package chromedp

import (
	"context"
	"math"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp/device"
)

// EmulateAction are actions that change the emulation settings for the
// browser.
type EmulateAction Action

// EmulateViewport is an action to change the browser viewport.
//
// Wraps calls to emulation.SetDeviceMetricsOverride and emulation.SetTouchEmulationEnabled.
//
// Note: this has the effect of setting/forcing the screen orientation to
// landscape, and will disable mobile and touch emulation by default. If this
// is not the desired behavior, use the emulate viewport options
// EmulateOrientation (or EmulateLandscape/EmulatePortrait), EmulateMobile, and
// EmulateTouch, respectively.
func EmulateViewport(width, height int64, opts ...EmulateViewportOption) EmulateAction {
	p1 := emulation.SetDeviceMetricsOverride(width, height, 1.0, false)
	p2 := emulation.SetTouchEmulationEnabled(false)
	for _, o := range opts {
		o(p1, p2)
	}
	return Tasks{p1, p2}
}

// EmulateViewportOption is the type for emulate viewport options.
type EmulateViewportOption = func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams)

// EmulateScale is an emulate viewport option to set the device viewport scaling
// factor.
func EmulateScale(scale float64) EmulateViewportOption {
	return func(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
		p1.DeviceScaleFactor = scale
	}
}

// EmulateOrientation is an emulate viewport option to set the device viewport
// screen orientation.
func EmulateOrientation(orientation emulation.OrientationType, angle int64) EmulateViewportOption {
	return func(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
		p1.ScreenOrientation = &emulation.ScreenOrientation{
			Type:  orientation,
			Angle: angle,
		}
	}
}

// EmulateLandscape is an emulate viewport option to set the device viewport
// screen orientation in landscape primary mode and an angle of 90.
func EmulateLandscape(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	EmulateOrientation(emulation.OrientationTypeLandscapePrimary, 90)(p1, p2)
}

// EmulatePortrait is an emulate viewport option to set the device viewport
// screen orentation in portrait primary mode and an angle of 0.
func EmulatePortrait(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	EmulateOrientation(emulation.OrientationTypePortraitPrimary, 0)(p1, p2)
}

// EmulateMobile is an emulate viewport option to toggle the device viewport to
// display as a mobile device.
func EmulateMobile(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	p1.Mobile = true
}

// EmulateTouch is an emulate viewport option to enable touch emulation.
func EmulateTouch(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	p2.Enabled = true
}

// ResetViewport is an action to reset the browser viewport to the default
// values the browser was started with.
//
// Note: does not modify / change the browser's emulated User-Agent, if any.
func ResetViewport() EmulateAction {
	return EmulateViewport(0, 0, EmulatePortrait)
}

// Device is the shared interface for known device types.
//
// See: github.com/chromedp/chromedp/device for a set of off-the-shelf devices
// and modes.
type Device interface {
	// Device returns the device info.
	Device() device.Info
}

// Emulate is an action to emulate a specific device.
//
// See: github.com/chromedp/chromedp/device for a set of off-the-shelf devices
// and modes.
func Emulate(device Device) EmulateAction {
	d := device.Device()

	var angle int64
	orientation := emulation.OrientationTypePortraitPrimary
	if d.Landscape {
		orientation, angle = emulation.OrientationTypeLandscapePrimary, 90
	}

	return Tasks{
		emulation.SetUserAgentOverride(d.UserAgent),
		emulation.SetDeviceMetricsOverride(d.Width, d.Height, d.Scale, d.Mobile).
			WithScreenOrientation(&emulation.ScreenOrientation{
				Type:  orientation,
				Angle: angle,
			}),
		emulation.SetTouchEmulationEnabled(d.Touch),
	}
}

// EmulateReset is an action to reset the device emulation.
//
// Resets the browser's viewport, screen orientation, user-agent, and
// mobile/touch emulation settings to the original values the browser was
// started with.
func EmulateReset() EmulateAction {
	return Emulate(device.Reset)
}

// FullScreenshot takes a full screenshot with the specified image quality of
// the entire browser viewport. Calls emulation.SetDeviceMetricsOverride (see
// note below).
//
// Implementation liberally sourced from puppeteer.
//
// Note: after calling this action, reset the browser's viewport using
// ResetViewport, EmulateReset, or page.SetDeviceMetricsOverride.
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
		width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))
		// force viewport emulation
		err = emulation.SetDeviceMetricsOverride(width, height, 1, false).
			WithScreenOrientation(&emulation.ScreenOrientation{
				Type:  emulation.OrientationTypePortraitPrimary,
				Angle: 0,
			}).
			Do(ctx)
		if err != nil {
			return err
		}
		// capture screenshot
		*res, err = page.CaptureScreenshot().
			WithQuality(int64(quality)).
			WithClip(&page.Viewport{
				X:      contentSize.X,
				Y:      contentSize.Y,
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
