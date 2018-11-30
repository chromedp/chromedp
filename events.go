package chromedp

import (
	"github.com/chromedp/cdproto"
)

// CDPEvent is a CDP event originating from a target handler
type CDPEvent struct {
	id  string
	msg *cdproto.Message
}

// cdpEventForwarder returns a function that can be called for a target handler
// to forward *CDPEvents over the given channel
func cdpEventForwarder(id string, ch chan<- *CDPEvent) func(msg *cdproto.Message) {
	return func(msg *cdproto.Message) {
		ev := &CDPEvent{
			id:  id,
			msg: msg,
		}
		if ch != nil {
			ch <- ev
		}
	}
}

// ID returns the target ID of the event
func (e *CDPEvent) ID() string {
	return e.id
}

// Message returns the message associated with the event
func (e *CDPEvent) Message() *cdproto.Message {
	return e.msg
}
