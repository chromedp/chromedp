package chromedp

import (
	"bytes"
	"context"
	"errors"
	"math"
	"regexp"
	"strconv"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	cdpio "github.com/chromedp/cdproto/io"
	"github.com/chromedp/cdproto/page"
)

// pdfPage is the size of a page of a PDF file, in points. A point is 1/72 of
// an inch.
type pdfPage struct {
	width, height float64
}

var (
	pdfPageRE     = regexp.MustCompile(`/Type\s*/Page\b[^s]`)
	pdfMediaBoxRE = regexp.MustCompile(`/MediaBox\s*\[\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\]`)
)

// pdfPages reads the sizes of the pages from the media boxes of the file. It
// works because the browser writes the page objects without compression.
func pdfPages(t *testing.T, buf []byte) []pdfPage {
	t.Helper()
	if !bytes.HasPrefix(buf, []byte("%PDF-")) {
		t.Fatalf("expected a PDF file, got: %.20q", buf)
	}
	var pages []pdfPage
	for _, m := range pdfMediaBoxRE.FindAllSubmatch(buf, -1) {
		var v [4]float64
		for i := range v {
			var err error
			if v[i], err = strconv.ParseFloat(string(m[i+1]), 64); err != nil {
				t.Fatal(err)
			}
		}
		pages = append(pages, pdfPage{v[2] - v[0], v[3] - v[1]})
	}
	if n := len(pdfPageRE.FindAll(buf, -1)); n != len(pages) {
		t.Fatalf("found %d page objects and %d media boxes", n, len(pages))
	}
	return pages
}

func near(a, b float64) bool { return math.Abs(a-b) < 1.5 }

func printPDF(t *testing.T, page string, opts ...PDFOption) []pdfPage {
	t.Helper()
	ctx, cancel := testAllocate(t, page)
	defer cancel()
	buf, err := Run(ctx, PrintToPDF(opts...))
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	return pdfPages(t, buf)
}

func TestPrintToPDF(t *testing.T) {
	t.Parallel()

	letter := pdfPage{8.5 * 72, 11 * 72}
	tests := []struct {
		name string
		page string
		opts []PDFOption
		size pdfPage
	}{
		{"default", "pdf.html", nil, letter},
		{"landscape", "pdf.html", []PDFOption{PDFLandscape()}, pdfPage{letter.height, letter.width}},
		{"A4", "pdf.html", []PDFOption{PDFPaper(PaperA4)}, pdfPage{8.27 * 72, 11.69 * 72}},
		{"A4 landscape", "pdf.html", []PDFOption{PDFPaper(PaperA4), PDFLandscape()}, pdfPage{11.69 * 72, 8.27 * 72}},
		{"legal", "pdf.html", []PDFOption{PDFPaper(PaperLegal)}, pdfPage{8.5 * 72, 14 * 72}},
		{"own size", "pdf.html", []PDFOption{PDFPaper(PaperSize{4, 6})}, pdfPage{4 * 72, 6 * 72}},
		{"width only", "pdf.html", []PDFOption{PDFPaper(PaperSize{Width: 5})}, pdfPage{5 * 72, 11 * 72}},
		{"empty paper size", "pdf.html", []PDFOption{PDFPaper(PaperSize{})}, letter},
		{"scale zero", "pdf.html", []PDFOption{PDFScale(0)}, letter},
		{"CSS page size is ignored", "pdf_css.html", nil, letter},
		{"prefer CSS page size", "pdf_css.html", []PDFOption{PDFPreferCSSPageSize()}, pdfPage{5 * 72, 7 * 72}},
		{"stream", "pdf.html", []PDFOption{PDFStream()}, letter},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pages := printPDF(t, test.page, test.opts...)
			if len(pages) == 0 {
				t.Fatal("expected at least one page")
			}
			if got := pages[0]; !near(got.width, test.size.width) || !near(got.height, test.size.height) {
				t.Fatalf("expected a page of %.1f x %.1f points, got: %.1f x %.1f", test.size.width, test.size.height, got.width, got.height)
			}
		})
	}
}

func TestPrintToPDFPageCount(t *testing.T) {
	t.Parallel()

	count := func(opts ...PDFOption) func(t *testing.T) int {
		return func(t *testing.T) int { return len(printPDF(t, "pdf.html", opts...)) }
	}
	tests := []struct {
		name string
		// fewer and more are the options for two prints, and the first must
		// give fewer pages than the second.
		fewer, more func(t *testing.T) int
	}{
		{"scale", count(PDFScale(0.5)), count()},
		{"margins", count(PDFMargin(0)), count(PDFMargin(2))},
		{"top and bottom margins", count(PDFMargins(0, 2, 0, 2)), count(PDFMargins(2, 0, 2, 0))},
		{"landscape has less height", count(), count(PDFLandscape())},
		{"page ranges", count(PDFPageRanges("1")), count()},
		{"page range of two", count(PDFPageRanges("1")), count(PDFPageRanges("1-2"))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fewer, more := test.fewer(t), test.more(t)
			if fewer >= more {
				t.Fatalf("expected fewer than %d pages, got: %d", more, fewer)
			}
		})
	}

	t.Run("page ranges give that many pages", func(t *testing.T) {
		t.Parallel()

		if n := len(printPDF(t, "pdf.html", PDFPageRanges("2-3"))); n != 2 {
			t.Fatalf("expected 2 pages, got: %d", n)
		}
	})
}

func TestPrintToPDFStreamIsTheSameFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "pdf.html")
	defer cancel()

	whole, err := Run(ctx, PrintToPDF())
	if err != nil {
		t.Fatal(err)
	}
	stream, err := Run(ctx, PrintToPDF(PDFStream()))
	if err != nil {
		t.Fatal(err)
	}
	if a, b := len(pdfPages(t, whole)), len(pdfPages(t, stream)); a != b {
		t.Fatalf("expected the same number of pages, got: %d and %d", a, b)
	}
	if !bytes.HasSuffix(bytes.TrimSpace(stream), []byte("%%EOF")) {
		t.Fatalf("expected the stream to hold the whole file, got the end: %q", stream[max(0, len(stream)-20):])
	}
}

func TestPrintToPDFPageRangeError(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "pdf.html")
	defer cancel()
	if _, err := Run(ctx, PrintToPDF(PDFPageRanges("99"))); err == nil {
		t.Fatal("expected an error for a range after the last page")
	}
}

// TestPDFOptions checks that each option sets its fields of the command.
func TestPDFOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []PDFOption
		want page.PrintToPDFParams
	}{
		{"none", nil, page.PrintToPDFParams{}},
		{"landscape", []PDFOption{PDFLandscape()}, page.PrintToPDFParams{Landscape: new(true)}},
		{"paper", []PDFOption{PDFPaper(PaperA4)}, page.PrintToPDFParams{PaperWidth: 8.27, PaperHeight: 11.69}},
		{"margin zero", []PDFOption{PDFMargin(0)}, page.PrintToPDFParams{MarginTop: new(0.0), MarginRight: new(0.0), MarginBottom: new(0.0), MarginLeft: new(0.0)}},
		{"margins", []PDFOption{PDFMargins(1, 2, 3, 4)}, page.PrintToPDFParams{MarginTop: new(1.0), MarginRight: new(2.0), MarginBottom: new(3.0), MarginLeft: new(4.0)}},
		{"scale", []PDFOption{PDFScale(1.5)}, page.PrintToPDFParams{Scale: 1.5}},
		{"ranges", []PDFOption{PDFPageRanges("1-2")}, page.PrintToPDFParams{PageRanges: "1-2"}},
		{"CSS size", []PDFOption{PDFPreferCSSPageSize()}, page.PrintToPDFParams{PreferCSSPageSize: new(true)}},
		{"background", []PDFOption{PDFPrintBackground()}, page.PrintToPDFParams{PrintBackground: new(true)}},
		{"header", []PDFOption{PDFHeaderTemplate("h")}, page.PrintToPDFParams{HeaderTemplate: "h", DisplayHeaderFooter: new(true)}},
		{"footer", []PDFOption{PDFFooterTemplate("f")}, page.PrintToPDFParams{FooterTemplate: "f", DisplayHeaderFooter: new(true)}},
		{"outline", []PDFOption{PDFOutlineAndTagged()}, page.PrintToPDFParams{GenerateDocumentOutline: new(true), GenerateTaggedPDF: new(true)}},
		{"stream", []PDFOption{PDFStream()}, page.PrintToPDFParams{TransferMode: page.PrintToPDFTransferModeReturnAsStream}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got page.PrintToPDFParams
			for _, o := range test.opts {
				o(&got)
			}
			if !pdfParamsEqual(got, test.want) {
				t.Fatalf("expected %+v, got: %+v", test.want, got)
			}
		})
	}
}

// pdfParamsEqual compares the parameters, and follows the pointers.
func pdfParamsEqual(a, b page.PrintToPDFParams) bool {
	eq := func(x, y *bool) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	eqf := func(x, y *float64) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	return eq(a.Landscape, b.Landscape) && eq(a.DisplayHeaderFooter, b.DisplayHeaderFooter) &&
		eq(a.PrintBackground, b.PrintBackground) && eq(a.PreferCSSPageSize, b.PreferCSSPageSize) &&
		eq(a.GenerateTaggedPDF, b.GenerateTaggedPDF) && eq(a.GenerateDocumentOutline, b.GenerateDocumentOutline) &&
		eqf(a.MarginTop, b.MarginTop) && eqf(a.MarginRight, b.MarginRight) &&
		eqf(a.MarginBottom, b.MarginBottom) && eqf(a.MarginLeft, b.MarginLeft) &&
		a.Scale == b.Scale && a.PaperWidth == b.PaperWidth && a.PaperHeight == b.PaperHeight &&
		a.PageRanges == b.PageRanges && a.HeaderTemplate == b.HeaderTemplate &&
		a.FooterTemplate == b.FooterTemplate && a.TransferMode == b.TransferMode
}

// TestPrintToPDFStreamClosedAfterError makes sure that readStream closes the
// stream when the read fails, here because the context of the read ended.
func TestPrintToPDFStreamClosedAfterError(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "pdf.html")
	defer cancel()

	var target *Target
	if _, err := Run(ctx, func(ctx context.Context, t *Target) (Void, error) {
		target = t
		return Void{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	res, err := cdp.Call(ctx, target, page.PrintToPDF, page.PrintToPDFParams{
		TransferMode: page.PrintToPDFTransferModeReturnAsStream,
	})
	if err != nil {
		t.Fatal(err)
	}

	dead, deadCancel := context.WithCancel(ctx)
	deadCancel()
	if _, err := readStream(dead, target, res.Stream); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected a canceled context, got: %v", err)
	}
	// The stream is closed, so a read of its handle fails.
	if _, err := cdp.Call(ctx, target, cdpio.Read, cdpio.ReadParams{Handle: res.Stream}); err == nil {
		t.Fatal("expected an error for a closed stream")
	}
}
