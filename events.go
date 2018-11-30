package chromedp

import (
	"github.com/chromedp/cdproto"
)

// CDPEvent is a CDP event originating from a target handler
type CDPEvent struct {
	id  string
	msg *cdproto.Message
}

// ID returns the target ID of the event
func (e *CDPEvent) ID() string {
	return e.id
}

// Message returns the message associated with the event
func (e *CDPEvent) Message() *cdproto.Message {
	return e.msg
}
