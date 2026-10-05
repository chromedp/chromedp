package chromedp

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	cdpio "github.com/chromedp/cdproto/io"
	"github.com/chromedp/cdproto/page"
)

// PaperSize is the size of a sheet of paper in inches. Use it with [PDFPaper].
// A field that is 0 gives the default of the browser for that field, so
// PaperSize{Width: 5} is 5 inches wide and 11 inches high, and the zero value
// is Letter paper.
type PaperSize struct {
	Width, Height float64
}

// Common paper sizes, in inches.
var (
	PaperLetter  = PaperSize{8.5, 11}
	PaperLegal   = PaperSize{8.5, 14}
	PaperTabloid = PaperSize{11, 17}
	PaperA3      = PaperSize{11.69, 16.54}
	PaperA4      = PaperSize{8.27, 11.69}
	PaperA5      = PaperSize{5.83, 8.27}
)

// PDFOption is an option of [PrintToPDF]. It changes the parameters of the
// protocol command [page.PrintToPDF].
type PDFOption = func(*page.PrintToPDFParams)

// PrintToPDF is an action that prints the page to a PDF file, and returns the
// bytes of the file. With no option, the browser prints on Letter paper, in
// portrait, with the margins of 1 cm, and with no header, footer or
// background.
//
// A length is in inches, and a CSS pixel is 1/96 of an inch. For example, a
// landscape A4 page with a header and a page number:
//
//	buf, err := chromedp.Run(ctx, chromedp.PrintToPDF(
//		chromedp.PDFPaper(chromedp.PaperA4),
//		chromedp.PDFLandscape(),
//		chromedp.PDFMargin(0.5),
//		chromedp.PDFPrintBackground(),
//		chromedp.PDFHeaderTemplate(`<div style="font-size:8px"><span class="title"></span></div>`),
//		chromedp.PDFFooterTemplate(`<div style="font-size:8px"><span class="pageNumber"></span></div>`),
//	))
//
// For a field that no option sets, send [page.PrintToPDF] yourself with
// [cdp.Call], and give an option that changes the parameters. An option is a
// func, so it can do that:
//
//	func(p *page.PrintToPDFParams) { p.TransferMode = page.PrintToPDFTransferModeReturnAsStream }
//
// The browser returns an error when the page ranges select no page. Wait for
// the page to load before the call.
func PrintToPDF(opts ...PDFOption) Action[[]byte] {
	return func(ctx context.Context, t *Target) ([]byte, error) {
		var p page.PrintToPDFParams
		for _, o := range opts {
			o(&p)
		}
		res, err := cdp.Call(ctx, t, page.PrintToPDF, p)
		if err != nil {
			return nil, err
		}
		if p.TransferMode != page.PrintToPDFTransferModeReturnAsStream {
			return res.Data, nil
		}
		return readStream(ctx, t, res.Stream)
	}
}

// readStream reads a whole stream of the browser, and closes it. It closes the
// stream also when the read fails or the context ended, with a new context that
// has a limit of 5 seconds, because the browser keeps the stream until then.
func readStream(ctx context.Context, t *Target, h cdpio.StreamHandle) (data []byte, err error) {
	defer func() {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, cerr := cdp.Call(cctx, t, cdpio.Close, cdpio.CloseParams{Handle: h}); cerr != nil && err == nil {
			data, err = nil, fmt.Errorf("closing the stream of the PDF: %w", cerr)
		}
	}()
	var buf bytes.Buffer
	for {
		res, err := cdp.Call(ctx, t, cdpio.Read, cdpio.ReadParams{Handle: h})
		if err != nil {
			return nil, fmt.Errorf("reading the stream of the PDF: %w", err)
		}
		buf.Write(res.Data)
		if res.EOF {
			return buf.Bytes(), nil
		}
	}
}

// PDFLandscape is an option of [PrintToPDF] that prints in landscape. The
// width and the height of the paper swap.
func PDFLandscape() PDFOption {
	return func(p *page.PrintToPDFParams) { p.Landscape = ptr(true) }
}

// PDFPaper is an option of [PrintToPDF] that sets the size of the paper. Give
// one of the sizes of the package, such as [PaperA4], or a [PaperSize] of your
// own. The default is [PaperLetter]. A zero width or height gives the default
// for that side, so PDFPaper(PaperSize{}) changes nothing. The size is for
// portrait, and [PDFLandscape] swaps it.
func PDFPaper(size PaperSize) PDFOption {
	return func(p *page.PrintToPDFParams) {
		p.PaperWidth, p.PaperHeight = size.Width, size.Height
	}
}

// PDFMargin is an option of [PrintToPDF] that sets the four margins to the same
// length in inches. The default is 1 cm, which is about 0.4 inches. A margin of
// 0 is valid.
func PDFMargin(inches float64) PDFOption {
	return PDFMargins(inches, inches, inches, inches)
}

// PDFMargins is an option of [PrintToPDF] that sets the margins in inches. The
// order is the same as in CSS: top, right, bottom and left.
func PDFMargins(top, right, bottom, left float64) PDFOption {
	return func(p *page.PrintToPDFParams) {
		p.MarginTop, p.MarginRight, p.MarginBottom, p.MarginLeft = &top, &right, &bottom, &left
	}
}

// PDFScale is an option of [PrintToPDF] that sets the scale of the page. The
// default is 1, and 0 gives the default, so PDFScale(0) changes nothing. The
// browser accepts 0.1 to 2.
func PDFScale(scale float64) PDFOption {
	return func(p *page.PrintToPDFParams) { p.Scale = scale }
}

// PDFPageRanges is an option of [PrintToPDF] that selects the pages to print,
// for example "1-3, 5". The numbers start at 1. The default is all pages.
func PDFPageRanges(ranges string) PDFOption {
	return func(p *page.PrintToPDFParams) { p.PageRanges = ranges }
}

// PDFPreferCSSPageSize is an option of [PrintToPDF] that uses the page size of
// the CSS rule @page, when the page has one. Otherwise the browser scales the
// content to fit the paper.
func PDFPreferCSSPageSize() PDFOption {
	return func(p *page.PrintToPDFParams) { p.PreferCSSPageSize = ptr(true) }
}

// PDFPrintBackground is an option of [PrintToPDF] that prints the background
// colors and images. By default, the browser leaves them out.
func PDFPrintBackground() PDFOption {
	return func(p *page.PrintToPDFParams) { p.PrintBackground = ptr(true) }
}

// PDFHeaderTemplate is an option of [PrintToPDF] that sets the HTML of the
// header, and turns on the header and the footer. The template can use these
// classes, and the browser puts the value in an element that has one: date,
// title, url, pageNumber and totalPages. The font size of the browser for a
// header is 0, so the template must set one. The header and the footer use
// the page margins, so a margin that is too small hides them.
//
// When you give only a header, the browser prints its own footer. Give
// [PDFFooterTemplate] with `<span></span>` to print no footer.
func PDFHeaderTemplate(html string) PDFOption {
	return func(p *page.PrintToPDFParams) {
		p.HeaderTemplate = html
		p.DisplayHeaderFooter = ptr(true)
	}
}

// PDFFooterTemplate is like [PDFHeaderTemplate] for the footer.
func PDFFooterTemplate(html string) PDFOption {
	return func(p *page.PrintToPDFParams) {
		p.FooterTemplate = html
		p.DisplayHeaderFooter = ptr(true)
	}
}

// PDFOutlineAndTagged is an option of [PrintToPDF] that adds the outline of
// the document, which is a table of contents that a PDF viewer shows for the
// headings, and that makes a tagged PDF, which holds the structure of the
// document for a screen reader. Chrome 154 and later make a tagged PDF by
// default, so with them the option adds the outline.
func PDFOutlineAndTagged() PDFOption {
	return func(p *page.PrintToPDFParams) {
		p.GenerateDocumentOutline = ptr(true)
		p.GenerateTaggedPDF = ptr(true)
	}
}

// PDFStream is an option of [PrintToPDF] that makes the browser send the file as
// a stream that the action reads in pieces, and not in one message. Use it for
// a large file. The result is the same.
func PDFStream() PDFOption {
	return func(p *page.PrintToPDFParams) {
		p.TransferMode = page.PrintToPDFTransferModeReturnAsStream
	}
}
