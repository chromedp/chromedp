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
