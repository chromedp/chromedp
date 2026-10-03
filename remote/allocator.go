package remote

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// defaultDialTimeout is how long the dial of a websocket can take, unless
// [WithDialTimeout] sets another time.
const defaultDialTimeout = 10 * time.Second

// NewAllocator creates a new context with an [Allocator]. Use it with
// chromedp.NewContext. The url must point to the websocket address of the
// browser, such as "ws://127.0.0.1:$PORT/devtools/browser/...".
//
// If the url does not contain "/devtools/browser/", NewAllocator tries to find
// the correct url. It sends a request to "http://$HOST:$PORT/json/version".
//
// NewAllocator accepts urls of these formats:
//   - ws://127.0.0.1:9222/
//   - http://127.0.0.1:9222/
//
// It does not accept "ws://127.0.0.1:9222/devtools/browser/", because the
// allocator does not try to modify it and it is invalid.
//
// Use [NoModifyURL] to stop NewAllocator from changing the url.
func NewAllocator(parent context.Context, url string, opts ...Option) (context.Context, context.CancelFunc) {
	a := &Allocator{
		wsURL:         url,
		modifyURLFunc: modifyURL,
		dialTimeout:   defaultDialTimeout,
	}
	for _, o := range opts {
		o(a)
	}
	return chromedp.NewAllocatorContext(parent, a)
}

// Option is an option of an [Allocator].
type Option = func(*Allocator)

// Allocator is a chromedp.Allocator which connects to an already running Chrome
// process through a websocket URL.
type Allocator struct {
	wsURL         string
	modifyURLFunc func(ctx context.Context, wsURL string) (string, error)

	// dialHTTPHeader is set by WithDialHTTPHeader. Allocate sends it with the
	// websocket handshake.
	dialHTTPHeader http.Header

	// dialTimeout is set by WithDialTimeout.
	dialTimeout time.Duration

	wg sync.WaitGroup
}

// Allocate satisfies the chromedp.Allocator interface.
func (a *Allocator) Allocate(ctx context.Context, opts ...chromedp.BrowserOption) (*chromedp.Browser, error) {
	if chromedp.FromContext(ctx) == nil {
		return nil, chromedp.ErrInvalidContext
	}

	wsURL := a.wsURL
	if a.modifyURLFunc != nil {
		var err error
		wsURL, err = a.modifyURLFunc(ctx, wsURL)
		if err != nil {
			return nil, fmt.Errorf("modifying the websocket address: %w", err)
		}
	}

	// Use a different context for the websocket, so that we can close the
	// relevant pages before we close the websocket connection.
	wctx, cancel := context.WithCancel(context.Background())

	// for the entire allocator
	a.wg.Go(func() {
		<-ctx.Done()
		chromedp.Cancel(ctx) // block until all pages are closed
		cancel()             // close the websocket connection
	})

	dialCtx := wctx
	if a.dialTimeout > 0 {
		var dialCancel context.CancelFunc
		dialCtx, dialCancel = context.WithTimeout(wctx, a.dialTimeout)
		defer dialCancel()
	}
	var dialOpts []DialOption
	if len(a.dialHTTPHeader) > 0 {
		dialOpts = append(dialOpts, WithConnHTTPHeader(a.dialHTTPHeader))
	}
	conn, err := DialContext(dialCtx, wsURL, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not dial %q: %w", wsURL, err)
	}
	return chromedp.NewBrowserTransport(wctx, conn, opts...)
}

// Wait satisfies the chromedp.Allocator interface.
func (a *Allocator) Wait() {
	a.wg.Wait()
}

// Attaches satisfies the chromedp.Attacher interface. It always returns true,
// because the allocator connects to a browser that runs already.
func (a *Allocator) Attaches() bool {
	return true
}

// WithDialHTTPHeader is an [Option] that sets HTTP headers on the websocket
// handshake request to the remote browser. A hosted browser service can need an
// Authorization header. The option copies the header, so a later change of h has
// no effect.
//
// The headers go with the websocket request only. The request to
// "/json/version", which NewAllocator sends to find the websocket address, has
// no headers. When the service needs headers, give it the full websocket address
// and use [NoModifyURL]. For example:
//
//	remote.NewAllocator(ctx, wsURL, remote.NoModifyURL,
//		remote.WithDialHTTPHeader(http.Header{
//			"Authorization": {"Bearer " + token},
//		}))
func WithDialHTTPHeader(h http.Header) Option {
	return func(a *Allocator) {
		a.dialHTTPHeader = h.Clone()
	}
}

// WithDialTimeout is an [Option] that sets the timeout for the dial of the
// websocket address of the browser. The default is ten seconds. Use a zero
// duration for no timeout.
func WithDialTimeout(d time.Duration) Option {
	return func(a *Allocator) {
		a.dialTimeout = d
	}
}

// NoModifyURL is an [Option] that prevents the allocator from modifying the
// websocket debugger URL passed to it.
func NoModifyURL(a *Allocator) {
	a.modifyURLFunc = nil
}
