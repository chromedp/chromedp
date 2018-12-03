package chromedp

import (
	"github.com/chromedp/cdproto"
)

// Event is a CDP event originating from a target handler
type Event struct {
	targetID string
	msg      *cdproto.Message
}

// eventForwarder returns a function that can be called for a target handler
// to forward *CDPEvents over the given channel
func eventForwarder(tid string, ch chan<- *Event) func(msg *cdproto.Message) {
	return func(msg *cdproto.Message) {
		if ch != nil {
			ev := &Event{
				targetID: tid,
				msg:      msg,
			}
			ch <- ev
		}
	}
}

// Message returns the message associated with the event
func (e *Event) Message() *cdproto.Message {
	return e.msg
}

// TargetID returns the target TargetID of the event
func (e *Event) TargetID() string {
	return e.targetID
}
