// Package device contains device emulation definitions for use with chromedp's
// Emulate action.
package device

//go:generate go run gen.go

// Info holds device information for use with chromedp.Emulate.
type Info struct {
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
func (i Info) String() string {
	return i.Name
}

// Device satisfies chromedp.Device.
func (i Info) Device() Info {
	return i
}

// infoType provides the enumerated device type.
type infoType int

// String satisfies fmt.Stringer.
func (i infoType) String() string {
	return devices[i].String()
}

// Device satisfies chromedp.Device.
func (i infoType) Device() Info {
	return devices[i]
}
