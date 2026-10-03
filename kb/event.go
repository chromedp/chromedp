package kb

import (
	"strconv"

	"github.com/chromedp/cdproto/input"
)

// Key event types for the Type field of an input.DispatchKeyEventParams.
const (
	KeyDown    = input.DispatchKeyEventTypeKeyDown
	KeyUp      = input.DispatchKeyEventTypeKeyUp
	KeyRawDown = input.DispatchKeyEventTypeRawKeyDown
	KeyChar    = input.DispatchKeyEventTypeChar
)

// Modifier is a bit field of pressed modifier keys. Convert it with int64 to
// set the Modifiers field of an input event.
type Modifier int64

// Modifier values.
const (
	ModifierNone  Modifier = 0
	ModifierAlt   Modifier = 1
	ModifierCtrl  Modifier = 2
	ModifierMeta  Modifier = 4
	ModifierShift Modifier = 8

	// ModifierCommand is an alias for ModifierMeta.
	ModifierCommand = ModifierMeta
)

// Int64 returns the Modifier as an int64 value.
func (m Modifier) Int64() int64 {
	return int64(m)
}

// String returns the Modifier as a string value.
func (m Modifier) String() string {
	switch m {
	case ModifierNone:
		return "None"
	case ModifierAlt:
		return "Alt"
	case ModifierCtrl:
		return "Ctrl"
	case ModifierMeta:
		return "Meta"
	case ModifierShift:
		return "Shift"
	}
	return "Modifier(" + strconv.FormatInt(int64(m), 10) + ")"
}
