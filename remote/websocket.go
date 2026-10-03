package remote

import (
	"context"

	"github.com/chromedp/chromedp"
)

// WebSocket is a chromedp.ExecAllocatorOption that makes the exec allocator
// connect to the browser with a websocket, as chromedp did before the pipe
// became the default. The default is a pipe: the allocator starts the browser
// with --remote-debugging-pipe, and the browser reads the commands from a pipe
// and writes the responses and the events to another pipe. On Unix they are the
// file descriptors 3 and 4, and on Windows they are two handles. A pipe needs
// no port.
//
// Use WebSocket when the browser must open a debugging port, for example to let
// a second program connect to it. The Flag options "remote-debugging-port" and
// "remote-debugging-address" also select the websocket, and they need this
// option. The option is also required for chromedp.KeepOpen.
//
// For example:
//
//	opts := append(chromedp.DefaultExecAllocatorOptions[:], remote.WebSocket)
//	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
func WebSocket(a *chromedp.ExecAllocator) {
	chromedp.WithDialer(dial)(a)
}

// dial is the chromedp.Dialer of WebSocket. It dials with the default timeout.
func dial(ctx context.Context, wsURL string) (chromedp.Transport, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()
	conn, err := DialContext(ctx, wsURL)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
