package test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/internal/chromedptest"
	"github.com/ledongthuc/pdf"
)

// When it is loaded correctly, the header and footer templates that use these
// values work as expected:
//   - title
//   - url
//   - pageNumber
//   - totalPages
//
// This is a regression test for https://github.com/chromedp/chromedp/issues/922.
func TestPDFTemplate(t *testing.T) {
	t.Parallel()

	ctx, cancel := chromedptest.Allocate(t, "")
	defer cancel()

	var buf []byte
	if err := chromedp.Do(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			frameTree, err := chromedp.Call(ctx, page.GetFrameTree, cdp.Empty{})
			if err != nil {
				return err
			}

			_, err = chromedp.Call(ctx, page.SetDocumentContent, page.SetDocumentContentParams{
				FrameID: frameTree.FrameTree.Frame.ID,
				HTML: `
				<html>
					<head>
						<title>PDF Template</title>
					</head>
					<body>
						Hello World!
					</body>
				</html>
			`,
			})
			return err
		}),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			res, err := chromedp.Call(ctx, page.PrintToPDF, page.PrintToPDFParams{
				MarginTop:           0.5,
				MarginBottom:        0.5,
				DisplayHeaderFooter: new(true),
				HeaderTemplate:      `<div style="font-size:8px;width:100%;text-align:center;"><span class="title"></span> -- <span class="url"></span></div>`,
				FooterTemplate:      `<div style="font-size:8px;width:100%;text-align:center;">(<span class="pageNumber"></span> / <span class="totalPages"></span>)</div>`,
			})
			buf = res.Data
			return err
		}),
	); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.GetPlainText()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte("Hello World!PDF Template -- about:blank(1 / 1)")
	l := len(want)
	// try to reuse buf
	if len(buf) >= l {
		buf = buf[0:l]
	} else {
		buf = make([]byte, l)
	}
	n, err := io.ReadFull(b, buf)
	if err != nil && !(errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) {
		t.Fatal(err)
	}
	buf = buf[:n]

	if !bytes.Equal(buf, want) {
		t.Errorf("page.PrintToPDF produces unexpected content. got: %q, want: %q", buf, want)
	}
}
