// Package inspector provides the Chrome Debugging Protocol
// commands, types, and events for the Inspector domain.
//
// Generated by the chromedp-gen command.
package inspector

// Code generated by chromedp-gen. DO NOT EDIT.

import (
	"context"

	cdp "github.com/igsky/chromedp/cdp"
)

// EnableParams enables inspector domain notifications.
type EnableParams struct{}

// Enable enables inspector domain notifications.
func Enable() *EnableParams {
	return &EnableParams{}
}

// Do executes Inspector.enable against the provided context and
// target handler.
func (p *EnableParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandInspectorEnable, nil, nil)
}

// DisableParams disables inspector domain notifications.
type DisableParams struct{}

// Disable disables inspector domain notifications.
func Disable() *DisableParams {
	return &DisableParams{}
}

// Do executes Inspector.disable against the provided context and
// target handler.
func (p *DisableParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandInspectorDisable, nil, nil)
}
