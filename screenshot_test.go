package chromedp

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestScreenshot(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image2.html")
	defer cancel()

	tests := []struct {
		sel  string
		by   QueryOption
		size int
	}{
		{`/html/body/img`, BySearch, 239},
		{`img`, ByQueryAll, 239},
		{`#icon-github`, ByID, 120},
		{`document.querySelector('#imagething').shadowRoot.querySelector('.container')`, ByJSPath, 190},
	}

	// a smaller viewport speeds up this test
	if err := Run(ctx, EmulateViewport(600, 400)); err != nil {
		t.Fatal(err)
	}

	for i, test := range tests {
		var buf []byte
		if err := Run(ctx, Screenshot(test.sel, &buf, test.by)); err != nil {
			t.Fatalf("test %d got error: %v", i, err)
		}

		if len(buf) == 0 {
			t.Fatalf("test %d failed to capture screenshot", i)
		}
		img, err := png.Decode(bytes.NewReader(buf))
		if err != nil {
			t.Fatal(err)
		}
		size := img.Bounds().Size()
		if size.X != test.size || size.Y != test.size {
			t.Errorf("expected dimensions to be %d*%d, got %d*%d",
				test.size, test.size, size.X, size.Y)
		}
	}
}

func TestScreenshotHighDPI(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	// Use a weird screen dimension with a 1.5 scale factor, so that
	// cropping the screenshot is forced to use floating point arithmetic
	// and keep the high DPI in mind.
	// We also want the dimensions to be large enough to see the element we
	// want, since we're not scrolling to ensure it's in view.
	if err := Run(ctx, EmulateViewport(905, 705, EmulateScale(1.5))); err != nil {
		t.Fatal(err)
	}

	var buf []byte
	if err := Run(ctx, Screenshot("#half-color", &buf, ByID)); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	size := img.Bounds().Size()
	wantSize := 300 // 200px at 1.5 scaling factor
	if size.X != wantSize || size.Y != wantSize {
		t.Fatalf("expected dimensions to be %d*%d, got %d*%d",
			wantSize, wantSize, size.X, size.Y)
	}
	wantColor := func(x, y int, r, g, b, a uint32) {
		color := img.At(x, y)
		r_, g_, b_, a_ := color.RGBA()
		if r_ != r || g_ != g || b_ != b || a_ != a {
			t.Errorf("got 0x%04x%04x%04x%04x at (%d,%d), want 0x%04x%04x%04x%04x",
				r_, g_, b_, a_, x, y, r, g, b, a)
		}
	}
	// The left half is blue.
	wantColor(5, 5, 0x0, 0x0, 0xffff, 0xffff)
	wantColor(5, 295, 0x0, 0x0, 0xffff, 0xffff)
	// The right half is red.
	wantColor(295, 5, 0xffff, 0x0, 0x0, 0xffff)
	wantColor(295, 295, 0xffff, 0x0, 0x0, 0xffff)
}

func TestCaptureScreenshot(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "image.html")
	defer cancel()

	const width, height = 650, 450

	// set the viewport size, to know what screenshot size to expect
	var buf []byte
	if err := Run(ctx,
		EmulateViewport(width, height),
		CaptureScreenshot(&buf),
	); err != nil {
		t.Fatal(err)
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if want := "png"; format != want {
		t.Fatalf("expected format to be %q, got %q", want, format)
	}
	if config.Width != width || config.Height != height {
		t.Fatalf("expected dimensions to be %d*%d, got %d*%d",
			width, height, config.Width, config.Height)
	}
}
