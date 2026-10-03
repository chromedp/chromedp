package chromedp

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp/device"
)

// EmulateViewport is an action that changes the browser viewport.
//
// It wraps calls to emulation.SetDeviceMetricsOverride and
// emulation.SetTouchEmulationEnabled.
//
// Note: by default, this action sets the screen orientation to landscape and
// disables mobile and touch emulation. To change that, use the emulate
// viewport options EmulateOrientation (or EmulateLandscape or
// EmulatePortrait), EmulateMobile, and EmulateTouch.
func EmulateViewport(width, height int64, opts ...EmulateViewportOption) Action[Void] {
	p1 := &emulation.SetDeviceMetricsOverrideParams{Width: width, Height: height, DeviceScaleFactor: 1.0}
	p2 := &emulation.SetTouchEmulationEnabledParams{}
	for _, o := range opts {
		o(p1, p2)
	}
	return Func(func(ctx context.Context, t *Target) error {
		if _, err := cdp.Call(ctx, t, emulation.SetDeviceMetricsOverride, *p1); err != nil {
			return err
		}
		_, err := cdp.Call(ctx, t, emulation.SetTouchEmulationEnabled, *p2)
		return err
	})
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
func EmulateOrientation(orientation emulation.ScreenOrientationType, angle int64) EmulateViewportOption {
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
	EmulateOrientation(emulation.ScreenOrientationTypeLandscapePrimary, 90)(p1, p2)
}

// EmulatePortrait is an emulate viewport option to set the device viewport
// screen orientation in portrait primary mode and an angle of 0.
func EmulatePortrait(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	EmulateOrientation(emulation.ScreenOrientationTypePortraitPrimary, 0)(p1, p2)
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

// ResetViewport is an action that resets the browser viewport to the values
// that the browser started with.
//
// Note: it does not change the emulated User-Agent of the browser, if any.
func ResetViewport() Action[Void] {
	return EmulateViewport(0, 0, EmulatePortrait)
}

// Device is the shared interface for known device types.
//
// See [device] for a set of ready-made devices and modes.
type Device interface {
	// Device returns the device info.
	Device() device.Info
}

// Emulate is an action to emulate a specific device.
//
// See [device] for a set of ready-made devices and modes.
func Emulate(device Device) Action[Void] {
	d := device.Device()

	var angle int64
	orientation := emulation.ScreenOrientationTypePortraitPrimary
	if d.Landscape {
		orientation, angle = emulation.ScreenOrientationTypeLandscapePrimary, 90
	}

	return Func(func(ctx context.Context, t *Target) error {
		if _, err := cdp.Call(ctx, t, emulation.SetUserAgentOverride, emulation.SetUserAgentOverrideParams{UserAgent: d.UserAgent}); err != nil {
			return err
		}
		if _, err := cdp.Call(ctx, t, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
			Width:             d.Width,
			Height:            d.Height,
			DeviceScaleFactor: d.Scale,
			Mobile:            d.Mobile,
			ScreenOrientation: &emulation.ScreenOrientation{
				Type:  orientation,
				Angle: angle,
			},
		}); err != nil {
			return err
		}
		_, err := cdp.Call(ctx, t, emulation.SetTouchEmulationEnabled, emulation.SetTouchEmulationEnabledParams{Enabled: d.Touch})
		return err
	})
}

// EmulateReset is an action that resets the device emulation.
//
// It resets the viewport, the screen orientation, the user agent, and the
// mobile and touch emulation. The new values are the original values from the
// start of the browser.
func EmulateReset() Action[Void] {
	return Emulate(device.Reset)
}
