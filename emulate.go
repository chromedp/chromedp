package chromedp

import (
	"context"

	"github.com/chromedp/cdproto/emulation"
)

// EmulateViewport is an action to change the browser viewport.
//
// Wraps calls to emulation.SetDeviceMetricsOverride and emulation.SetTouchEmulationEnabled.
func EmulateViewport(width, height int64, opts ...EmulateViewportOption) Action {
	return ActionFunc(func(ctx context.Context) error {
		p1 := emulation.SetDeviceMetricsOverride(width, height, 1.0, false)
		p2 := emulation.SetTouchEmulationEnabled(false)

		// apply opts
		for _, o := range opts {
			o(p1, p2)
		}

		// execute
		if err := p1.Do(ctx); err != nil {
			return err
		}
		return p2.Do(ctx)
	})
}

// EmulateViewportOption is the type for emulate viewport options.
type EmulateViewportOption func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams)

// EmulateScale is an emulate viewport option to set the device viewport scaling
// factor.
func EmulateScale(scale float64) EmulateViewportOption {
	return func(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
		p1.DeviceScaleFactor = scale
	}
}

// EmulateOrientation is an emulate viewport option to set the device viewport
// orientation.
func EmulateOrientation(orientation emulation.OrientationType, angle int64) EmulateViewportOption {
	return func(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
		p1.ScreenOrientation = &emulation.ScreenOrientation{
			Type:  orientation,
			Angle: angle,
		}
	}
}

// EmulateLandscape is an emulate viewport option to set the device viewport
// orientation in landscape primary mode and an angle of 90.
func EmulateLandscape(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	EmulateOrientation(emulation.OrientationTypeLandscapePrimary, 90)(p1, p2)
}

// EmulatePortrait is an emulate viewport option to set the device viewport
// orentation in portrait primary mode and an angle of 0.
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

// ResetViewport is an action to reset the browser viewport.
func ResetViewport() Action {
	return EmulateViewport(0, 0, EmulatePortrait)
}

// Device is a interface for a known device.
//
// See: github.com/chromedp/chromedp/device for a set of off-the-shelf devices
// and modes.
type Device interface {
	// ViewportParams returns paramaters for use with the EmulateViewport
	// action.
	ViewportParams() (int64, int64, string, []func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams))
}

// Emulate is an action to emulate a specific device.
//
// See: github.com/chromedp/chromedp/device for a set of off-the-shelf devices
// and modes.
func Emulate(device Device) Action {
	width, height, userAgent, deviceOpts := device.ViewportParams()
	opts := make([]EmulateViewportOption, len(deviceOpts))
	for i := 0; i < len(deviceOpts); i++ {
		opts[i] = deviceOpts[i]
	}
	return ActionFunc(func(ctx context.Context) error {
		// set user agent
		if err := emulation.SetUserAgentOverride(userAgent).Do(ctx); err != nil {
			return err
		}
		return EmulateViewport(width, height, opts...).Do(ctx)
	})
}
