package chromedp

// Error is a chromedp error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// Error types.
const (
	// ErrInvalidWebsocketMessage is the invalid websocket message.
	ErrInvalidWebsocketMessage Error = "invalid websocket message"

	// ErrInvalidDimensions is the invalid dimensions error.
	ErrInvalidDimensions Error = "invalid dimensions"

	// ErrNoResults is the no results error.
	ErrNoResults Error = "no results"

	// ErrHasResults is the has results error.
	ErrHasResults Error = "has results"

	// ErrNotVisible is the not visible error.
	ErrNotVisible Error = "not visible"

	// ErrVisible is the visible error.
	ErrVisible Error = "visible"

	// ErrDisabled is the disabled error.
	ErrDisabled Error = "disabled"

	// ErrNotSelected is the not selected error.
	ErrNotSelected Error = "not selected"

	// ErrInvalidBoxModel is the invalid box model error.
	ErrInvalidBoxModel Error = "invalid box model"

	// ErrChannelClosed is the channel closed error.
	ErrChannelClosed Error = "channel closed"

	// ErrInvalidTarget is the invalid target error.
	ErrInvalidTarget Error = "invalid target"

	// ErrInvalidContext is the invalid context error.
	ErrInvalidContext Error = "invalid context"

	// ErrPollingTimeout is the error that the timeout reached before the pageFunction returns a truthy value.
	ErrPollingTimeout Error = "waiting for function failed: timeout"
)
