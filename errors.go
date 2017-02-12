package chromedp

import (
	"errors"
)

// Error types.
var (
	// ErrInvalidDimensions is the error returned when the retrieved box model is
	// invalid.
	ErrInvalidDimensions = errors.New("invalid dimensions")

	// ErrNoResults is the error returned when there are no matching nodes.
	ErrNoResults = errors.New("no results")

	// ErrHasResults is the error returned when there should not be any
	// matching nodes.
	ErrHasResults = errors.New("has results")

	// ErrNotVisible is the error returned when a non-visible node should be
	// visible.
	ErrNotVisible = errors.New("not visible")

	// ErrVisible is the error returned when a visible node should be
	// non-visible.
	ErrVisible = errors.New("visible")

	// ErrDisabled is the error returned when a disabled node should be
	// enabled.
	ErrDisabled = errors.New("disabled")

	// ErrNotSelected is the error returned when a non-selected node should be
	// selected.
	ErrNotSelected = errors.New("not selected")

	// ErrInvalidBoxModel is the error returned when the retrieved box model
	// data is invalid.
	ErrInvalidBoxModel = errors.New("invalid box model")
)
