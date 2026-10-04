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
				MarginTop:           new(0.5),
				MarginBottom:        new(0.5),
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

// TestPrintToPDFHeaderAndFooter checks that the options of chromedp.PrintToPDF
// reach the browser. The header and the footer templates turn the header and
// the footer on, and the title and the page numbers show in the text.
func TestPrintToPDFHeaderAndFooter(t *testing.T) {
	t.Parallel()

	ctx, cancel := chromedptest.Allocate(t, "")
	defer cancel()

	if err := chromedp.Do(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			frameTree, err := chromedp.Call(ctx, page.GetFrameTree, cdp.Empty{})
			if err != nil {
				return err
			}
			_, err = chromedp.Call(ctx, page.SetDocumentContent, page.SetDocumentContentParams{
				FrameID: frameTree.FrameTree.Frame.ID,
				HTML:    `<html><head><title>PDF Options</title></head><body>Hello Options</body></html>`,
			})
			return err
		}),
	); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		opts []chromedp.PDFOption
		want []string
		not  []string
	}{
		{
			"header and footer",
			[]chromedp.PDFOption{
				chromedp.PDFPaper(chromedp.PaperA4),
				chromedp.PDFMargin(0.5),
				chromedp.PDFHeaderTemplate(`<div style="font-size:8px;width:100%;text-align:center;"><span class="title"></span></div>`),
				chromedp.PDFFooterTemplate(`<div style="font-size:8px;width:100%;text-align:center;">page <span class="pageNumber"></span> of <span class="totalPages"></span></div>`),
				chromedp.PDFOutlineAndTagged(),
			},
			[]string{"Hello Options", "PDF Options", "page 1 of 1"},
			nil,
		},
		{
			"no header and no footer by default",
			nil,
			[]string{"Hello Options"},
			[]string{"PDF Options", "page 1 of 1"},
		},
	}
	for _, test := range tests {
		buf, err := chromedp.Run(ctx, chromedp.PrintToPDF(test.opts...))
		if err != nil {
			t.Fatalf("%s: got error: %v", test.name, err)
		}
		r, err := pdf.NewReader(bytes.NewReader(buf), int64(len(buf)))
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		b, err := r.GetPlainText()
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		text, err := io.ReadAll(b)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		for _, w := range test.want {
			if !bytes.Contains(text, []byte(w)) {
				t.Errorf("%s: expected the text to hold %q, got: %q", test.name, w, text)
			}
		}
		for _, n := range test.not {
			if bytes.Contains(text, []byte(n)) {
				t.Errorf("%s: expected the text not to hold %q, got: %q", test.name, n, text)
			}
		}
	}
}
