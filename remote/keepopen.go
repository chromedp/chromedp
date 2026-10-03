package remote

import "github.com/chromedp/chromedp"

// WithKeepOpen makes chromedp.NewContext build the default allocator with
// chromedp.KeepOpen and with [WebSocket]. The browser then stays open when the
// program ends or when the context is canceled. It works with headless mode
// too. See also chromedp.KeptOpen and chromedp.WithVisibleWindow.
//
// The option only applies when the parent context has no allocator. It has no
// effect on an allocator that the caller made, and it has no effect on a
// context that inherits a browser. For an allocator that you make, add
// chromedp.KeepOpen and [WebSocket] to the options of chromedp.NewExecAllocator
// instead.
func WithKeepOpen() chromedp.ContextOption {
	return chromedp.WithAllocatorOptions(WebSocket, chromedp.KeepOpen)
}
