// Package device contains device emulation definitions for use with chromedp's
// Emulate action.
package device

//go:generate go run gen.go

import "github.com/chromedp/cdproto/emulation"

// Device provides the common type for defined devices.
type Device int

// Device is the actual device.
type device struct {
	// Name is the device name.
	Name string

	// UserAgent is the device user agent string.
	UserAgent string

	// Width is the viewport width.
	Width int64

	// Height is the viewport height.
	Height int64

	// Scale is the device viewport scale factor.
	Scale float64

	// Landscape indicates whether or not the device is in landscape mode or
	// not.
	Landscape bool

	// Mobile indicates whether it is a mobile device or not.
	Mobile bool

	// Touch indicates whether the device has touch enabled.
	Touch bool
}

// String satisfies fmt.Stringer.
func (d Device) String() string {
	return devices[d].Name
}

// ViewportParams satisfies chromedp.Device.
func (d Device) ViewportParams() (int64, int64, string, []func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams)) {
	orientation := emulatePortrait
	if devices[d].Landscape {
		orientation = emulateLandscape
	}
	opts := []func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams){
		emulateScale(devices[d].Scale),
		orientation,
	}
	if devices[d].Mobile {
		opts = append(opts, emulateMobile)
	}
	if devices[d].Touch {
		opts = append(opts, emulateTouch)
	}
	return devices[d].Width, devices[d].Height, devices[d].UserAgent, opts
}

/*

	THE FOLLOWING ARE A COPY OF THE chromedp.EmulateViewport* options.

	Provided here in order to prevent circular imports.
*/

// emulateScale is an emulate viewport option to set the device viewport scaling
// factor.
func emulateScale(scale float64) func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams) {
	return func(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
		p1.DeviceScaleFactor = scale
	}
}

// emulateOrientation is an emulate viewport option to set the device viewport
// orientation.
func emulateOrientation(orientation emulation.OrientationType, angle int64) func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams) {
	return func(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
		p1.ScreenOrientation = &emulation.ScreenOrientation{
			Type:  orientation,
			Angle: angle,
		}
	}
}

// emulateLandscape is an emulate viewport option to set the device viewport
// orientation in landscape primary mode and an angle of 90.
func emulateLandscape(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	emulateOrientation(emulation.OrientationTypeLandscapePrimary, 90)(p1, p2)
}

// emulatePortrait is an emulate viewport option to set the device viewport
// orentation in portrait primary mode and an angle of 0.
func emulatePortrait(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	emulateOrientation(emulation.OrientationTypePortraitPrimary, 0)(p1, p2)
}

// emulateMobile is an emulate viewport option to toggle the device viewport to
// display as a mobile device.
func emulateMobile(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	p1.Mobile = true
}

// emulateTouch is an emulate viewport option to enable touch emulation.
func emulateTouch(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
	p2.Enabled = true
}
