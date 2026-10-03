package chromedp

import (
	"strings"
	"sync"

	"github.com/chromedp/cdproto/cdp"
)

// Frame is a frame of a page. It embeds the protocol frame and adds the node
// tree of the frame and the state that chromedp keeps up to date from the
// browser events.
//
// Hold the lock of a Frame before you read a field that chromedp updates.
type Frame struct {
	*cdp.Frame

	// State is the state of the frame.
	State FrameState
	// Root is the document root of the frame.
	Root *Node
	// Nodes holds the nodes of the frame by node id.
	Nodes map[cdp.NodeID]*Node

	sync.RWMutex
}

// FrameState is the state of a Frame.
type FrameState uint16

// FrameState enum values.
const (
	FrameDOMContentEventFired FrameState = 1 << (15 - iota)
	FrameLoadEventFired
	FrameAttached
	FrameNavigated
	FrameLoading
	FrameScheduledNavigation
)

// frameStateNames are the names of the frame states.
var frameStateNames = map[FrameState]string{
	FrameDOMContentEventFired: "DOMContentEventFired",
	FrameLoadEventFired:       "LoadEventFired",
	FrameAttached:             "Attached",
	FrameNavigated:            "Navigated",
	FrameLoading:              "Loading",
	FrameScheduledNavigation:  "ScheduledNavigation",
}

// String satisfies stringer interface.
func (fs FrameState) String() string {
	var s []string
	for k, v := range frameStateNames {
		if fs&k != 0 {
			s = append(s, v)
		}
	}
	return "[" + strings.Join(s, " ") + "]"
}

// EmptyFrameID is the "non-existent" frame id.
const EmptyFrameID = cdp.FrameID("")
