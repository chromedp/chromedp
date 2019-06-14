// Package device contains device emulation definitions for use with chromedp's
// Emulate action.
package device

//go:generate go run gen.go

import "github.com/chromedp/cdproto/emulation"

// Device holds device information for use with chromedp.Emulate.
type Device struct {
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
	return d.Name
}

// UserAgentString satisfies chromedp.Device.
func (d Device) UserAgentString() string {
	return d.UserAgent
}

// EmulateViewportOption satisfies chromedp.Device.
func (d Device) EmulateViewportOption() []func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams) {
	return []func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams){
		func(p1 *emulation.SetDeviceMetricsOverrideParams, p2 *emulation.SetTouchEmulationEnabledParams) {
			var angle int64
			orientation := emulation.OrientationTypePortraitPrimary
			if d.Landscape {
				orientation, angle = emulation.OrientationTypeLandscapePrimary, 90
			}

			// force parameters
			*p1 = emulation.SetDeviceMetricsOverrideParams{
				Width:             d.Width,
				Height:            d.Height,
				DeviceScaleFactor: d.Scale,
				ScreenOrientation: &emulation.ScreenOrientation{
					Type:  orientation,
					Angle: angle,
				},
				Mobile: d.Mobile,
			}
			*p2 = emulation.SetTouchEmulationEnabledParams{
				Enabled: d.Touch,
			}
		},
	}
}

// deviceType provides the enumerated device type.
type deviceType int

// String satisfies fmt.Stringer.
func (d deviceType) String() string {
	return devices[d].String()
}

// UserAgent satisfies chromedp.Device.
func (d deviceType) UserAgentString() string {
	return devices[d].UserAgentString()
}

// EmulateViewportOption satisfies chromedp.Device.
func (d deviceType) EmulateViewportOption() []func(*emulation.SetDeviceMetricsOverrideParams, *emulation.SetTouchEmulationEnabledParams) {
	return devices[d].EmulateViewportOption()
}
