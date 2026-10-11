// Package websocket is a WebSocket client as RFC 6455 defines it. It has no
// dependency outside the standard library.
//
// The package supports what a DevTools client needs. It dials with net/http,
// writes one masked text frame per message, reads text and binary messages
// that the server can split into fragments, and handles the control frames
// ping, pong and close.
//
// The package does not support extensions, so it never negotiates
// permessage-deflate. It does not support a server role either.
//
// One goroutine can call [Conn.ReadMessage] while another goroutine calls
// [Conn.WriteText]. [Conn.Close] is safe to call at any time.
package websocket
