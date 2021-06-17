package chromedp

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"testing"

	"github.com/orisano/pixelmatch"
)

func TestScreenshot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sel  string
		want string
	}{
		{
			name: "padding border",
			sel:  "#padding-border",
			want: "element-padding-border.png",
		},
		{
			name: "larger than viewport",
			sel:  "#larger-than-viewport",
			want: "element-larger-than-viewport.png",
		},
		{
			name: "outside viewport",
			sel:  "#outside-viewport",
			want: "element-scrolled-into-view.png",
		},
		{
			name: "rotate element",
			sel:  "#rotated",
			want: "element-rotate.png",
		},
		{
			name: "fractional dimensions",
			sel:  "#fractional-dimensions",
			want: "element-fractional.png",
		},
		{
			name: "fractional offset",
			sel:  "#fractional-offset",
			want: "element-fractional-offset.png",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := testAllocate(t, "screenshot.html")
			defer cancel()

			var buf []byte
			if err := Run(ctx,
				EmulateViewport(500, 500),
				EvaluateAsDevTools("document.documentElement.scrollTo(20,  30)", nil),
				Screenshot(test.sel, &buf, ByQuery),
			); err != nil {
				t.Fatal(err)
			}
			diff, err := matchPixel(buf, test.want)
			if err != nil {
				t.Fatal(err)
			}
			if diff != 0 {
				t.Fatalf("screenshot does not match. diff: %v", diff)
			}
		})
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

	diff, err := matchPixel(buf, "half-color.png")
	if err != nil {
		t.Fatal(err)
	}
	if diff != 0 {
		t.Fatalf("screenshot does not match. diff: %v", diff)
	}
}

func TestCaptureScreenshot(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "grid.html")
	defer cancel()

	var buf []byte
	if err := Run(ctx,
		EmulateViewport(500, 500),
		CaptureScreenshot(&buf),
	); err != nil {
		t.Fatal(err)
	}

	diff, err := matchPixel(buf, "sanity.png")
	if err != nil {
		t.Fatal(err)
	}
	if diff != 0 {
		t.Fatalf("screenshot does not match. diff: %v", diff)
	}
}

func TestFullScreenshot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		quality int
		want    string
	}{
		{
			name:    "quality 100",
			quality: 100,
			want:    "grid-fullpage.png",
		},
		{
			name:    "quality 90",
			quality: 90,
			want:    "grid-fullpage-90.jpeg",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := testAllocate(t, "grid.html")
			defer cancel()

			var buf []byte
			if err := Run(ctx,
				EmulateViewport(500, 500),
				EvaluateAsDevTools("document.documentElement.scrollTo(20,  30)", nil),
				FullScreenshot(&buf, test.quality),
			); err != nil {
				t.Fatal(err)
			}

			diff, err := matchPixel(buf, test.want)
			if err != nil {
				t.Fatal(err)
			}
			if diff != 0 {
				t.Fatalf("screenshot does not match. diff: %v", diff)
			}
		})
	}
}

func matchPixel(buf []byte, want string) (int, error) {
	img1, format1, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return 0, err
	}

	img2, format2, err := openImage(want)
	if err != nil {
		return 0, err
	}

	if format1 != format2 {
		return 0, fmt.Errorf("image formats not matched: %s != %s", format1, format2)
	}

	return pixelmatch.MatchPixel(img1, img2, pixelmatch.Threshold(0.1))
}

func openImage(screenshot string) (image.Image, string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	p := path.Join(wd, "testdata", "screenshots", screenshot)
	f, err := os.Open(p)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}
	return img, format, nil
}
