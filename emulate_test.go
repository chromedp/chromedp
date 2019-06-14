package chromedp

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/chromedp/chromedp/device"
)

func TestEmulate(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	var buf []byte
	if err := Run(ctx,
		Emulate(device.IPhone7),
		Screenshot(`#half-color`, &buf, ByID),
	); err != nil {
		t.Fatal(err)
	}

	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	size := img.Bounds().Size()
	if size.X != 400 || size.Y != 400 {
		t.Errorf("expected size 400x400, got: %dx%d", size.X, size.Y)
	}
}

func TestEmulateInvalidDevice(t *testing.T) {
	t.Parallel()

	var dev device.Device

	want := "Invalid device"
	if got := dev.String(); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected a panic in Emulate(Device(0))")
		}
	}()
	_ = Emulate(dev)
}
