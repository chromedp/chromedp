// Package test holds the tests of chromedp that need a library outside the
// standard library and cdproto. The screenshot tests compare images with
// pixelmatch, and the PDF test reads the text of a PDF file with a PDF reader.
// The module of this package keeps these libraries out of the core module. The
// package has no code, and its tests use the exported API of chromedp only.
package test
