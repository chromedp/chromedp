package chromedp

import (
	"fmt"
	"strings"

	"github.com/chromedp/cdproto/runtime"
)

// ExceptionError is the error for a JavaScript exception. It wraps the
// protocol details of the exception.
type ExceptionError struct {
	*runtime.ExceptionDetails
}

// Error satisfies the error interface.
func (e *ExceptionError) Error() string {
	return exceptionString(e.ExceptionDetails)
}

// exceptionString returns a readable description of an exception.
func exceptionString(e *runtime.ExceptionDetails) string {
	var b strings.Builder
	fmt.Fprintf(&b, "exception %q (%d:%d)", e.Text, e.LineNumber, e.ColumnNumber)
	if obj := e.Exception; obj != nil {
		fmt.Fprintf(&b, ": %s", obj.Description)
	}
	return b.String()
}

// LoadError is the error for a page that did not load. Navigate and
// RunResponse return it when the browser reports an error for the request of
// the page, for example when the host name does not exist. Use [errors.As] to
// read the text of the browser, or [errors.Is] with [ErrPageLoad] to test for
// any load error.
type LoadError struct {
	// ErrorText is the error text of the browser, such as
	// "net::ERR_NAME_NOT_RESOLVED". Chromium lists these texts in
	// net/base/net_error_list.h.
	ErrorText string
}

// Error satisfies the error interface. The text is the same as in earlier
// versions.
func (e *LoadError) Error() string {
	return "page load error " + e.ErrorText
}

// Unwrap returns ErrPageLoad, so that errors.Is finds it.
func (e *LoadError) Unwrap() error {
	return ErrPageLoad
}

// Error is a chromedp error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// Error types.
const (
	// ErrNoDialer is the error of an ExecAllocator that needs a websocket and
	// has no dialer. See [WithDialer].
	ErrNoDialer Error = "the allocator needs a websocket, so it needs a dialer: add remote.WebSocket from the module github.com/chromedp/chromedp/remote to its options"

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

	// ErrPageLoad is the error that every [LoadError] wraps.
	ErrPageLoad Error = "page load error"

	// ErrNoDisplay is the error when a visible window is requested on Linux
	// and the environment has no display.
	ErrNoDisplay Error = "a visible window needs a display, but DISPLAY and WAYLAND_DISPLAY are not set: start a display, or unset CHROMEDP_VISIBLEWINDOW and remove WithVisibleWindow and VisibleWindow to run headless"

	// ErrInvalidContext is the invalid context error.
	ErrInvalidContext Error = "invalid context"

	// ErrPollingTimeout is the error when the timeout ends before the pageFunction returns a truthy value.
	ErrPollingTimeout Error = "waiting for function failed: timeout"

	// ErrJSUndefined is the error that the type of RemoteObject is "undefined".
	ErrJSUndefined Error = "encountered an undefined value"

	// ErrJSNull is the error that the value of RemoteObject is null.
	ErrJSNull Error = "encountered a null value"
)
