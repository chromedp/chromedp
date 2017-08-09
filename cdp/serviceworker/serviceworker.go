// Package serviceworker provides the Chrome Debugging Protocol
// commands, types, and events for the ServiceWorker domain.
//
// Generated by the chromedp-gen command.
package serviceworker

// Code generated by chromedp-gen. DO NOT EDIT.

import (
	"context"

	cdp "github.com/igsky/chromedp/cdp"
)

// EnableParams [no description].
type EnableParams struct{}

// Enable [no description].
func Enable() *EnableParams {
	return &EnableParams{}
}

// Do executes ServiceWorker.enable against the provided context and
// target handler.
func (p *EnableParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerEnable, nil, nil)
}

// DisableParams [no description].
type DisableParams struct{}

// Disable [no description].
func Disable() *DisableParams {
	return &DisableParams{}
}

// Do executes ServiceWorker.disable against the provided context and
// target handler.
func (p *DisableParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerDisable, nil, nil)
}

// UnregisterParams [no description].
type UnregisterParams struct {
	ScopeURL string `json:"scopeURL"`
}

// Unregister [no description].
//
// parameters:
//   scopeURL
func Unregister(scopeURL string) *UnregisterParams {
	return &UnregisterParams{
		ScopeURL: scopeURL,
	}
}

// Do executes ServiceWorker.unregister against the provided context and
// target handler.
func (p *UnregisterParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerUnregister, p, nil)
}

// UpdateRegistrationParams [no description].
type UpdateRegistrationParams struct {
	ScopeURL string `json:"scopeURL"`
}

// UpdateRegistration [no description].
//
// parameters:
//   scopeURL
func UpdateRegistration(scopeURL string) *UpdateRegistrationParams {
	return &UpdateRegistrationParams{
		ScopeURL: scopeURL,
	}
}

// Do executes ServiceWorker.updateRegistration against the provided context and
// target handler.
func (p *UpdateRegistrationParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerUpdateRegistration, p, nil)
}

// StartWorkerParams [no description].
type StartWorkerParams struct {
	ScopeURL string `json:"scopeURL"`
}

// StartWorker [no description].
//
// parameters:
//   scopeURL
func StartWorker(scopeURL string) *StartWorkerParams {
	return &StartWorkerParams{
		ScopeURL: scopeURL,
	}
}

// Do executes ServiceWorker.startWorker against the provided context and
// target handler.
func (p *StartWorkerParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerStartWorker, p, nil)
}

// SkipWaitingParams [no description].
type SkipWaitingParams struct {
	ScopeURL string `json:"scopeURL"`
}

// SkipWaiting [no description].
//
// parameters:
//   scopeURL
func SkipWaiting(scopeURL string) *SkipWaitingParams {
	return &SkipWaitingParams{
		ScopeURL: scopeURL,
	}
}

// Do executes ServiceWorker.skipWaiting against the provided context and
// target handler.
func (p *SkipWaitingParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerSkipWaiting, p, nil)
}

// StopWorkerParams [no description].
type StopWorkerParams struct {
	VersionID string `json:"versionId"`
}

// StopWorker [no description].
//
// parameters:
//   versionID
func StopWorker(versionID string) *StopWorkerParams {
	return &StopWorkerParams{
		VersionID: versionID,
	}
}

// Do executes ServiceWorker.stopWorker against the provided context and
// target handler.
func (p *StopWorkerParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerStopWorker, p, nil)
}

// InspectWorkerParams [no description].
type InspectWorkerParams struct {
	VersionID string `json:"versionId"`
}

// InspectWorker [no description].
//
// parameters:
//   versionID
func InspectWorker(versionID string) *InspectWorkerParams {
	return &InspectWorkerParams{
		VersionID: versionID,
	}
}

// Do executes ServiceWorker.inspectWorker against the provided context and
// target handler.
func (p *InspectWorkerParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerInspectWorker, p, nil)
}

// SetForceUpdateOnPageLoadParams [no description].
type SetForceUpdateOnPageLoadParams struct {
	ForceUpdateOnPageLoad bool `json:"forceUpdateOnPageLoad"`
}

// SetForceUpdateOnPageLoad [no description].
//
// parameters:
//   forceUpdateOnPageLoad
func SetForceUpdateOnPageLoad(forceUpdateOnPageLoad bool) *SetForceUpdateOnPageLoadParams {
	return &SetForceUpdateOnPageLoadParams{
		ForceUpdateOnPageLoad: forceUpdateOnPageLoad,
	}
}

// Do executes ServiceWorker.setForceUpdateOnPageLoad against the provided context and
// target handler.
func (p *SetForceUpdateOnPageLoadParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerSetForceUpdateOnPageLoad, p, nil)
}

// DeliverPushMessageParams [no description].
type DeliverPushMessageParams struct {
	Origin         string `json:"origin"`
	RegistrationID string `json:"registrationId"`
	Data           string `json:"data"`
}

// DeliverPushMessage [no description].
//
// parameters:
//   origin
//   registrationID
//   data
func DeliverPushMessage(origin string, registrationID string, data string) *DeliverPushMessageParams {
	return &DeliverPushMessageParams{
		Origin:         origin,
		RegistrationID: registrationID,
		Data:           data,
	}
}

// Do executes ServiceWorker.deliverPushMessage against the provided context and
// target handler.
func (p *DeliverPushMessageParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerDeliverPushMessage, p, nil)
}

// DispatchSyncEventParams [no description].
type DispatchSyncEventParams struct {
	Origin         string `json:"origin"`
	RegistrationID string `json:"registrationId"`
	Tag            string `json:"tag"`
	LastChance     bool   `json:"lastChance"`
}

// DispatchSyncEvent [no description].
//
// parameters:
//   origin
//   registrationID
//   tag
//   lastChance
func DispatchSyncEvent(origin string, registrationID string, tag string, lastChance bool) *DispatchSyncEventParams {
	return &DispatchSyncEventParams{
		Origin:         origin,
		RegistrationID: registrationID,
		Tag:            tag,
		LastChance:     lastChance,
	}
}

// Do executes ServiceWorker.dispatchSyncEvent against the provided context and
// target handler.
func (p *DispatchSyncEventParams) Do(ctxt context.Context, h cdp.Handler) (err error) {
	return h.Execute(ctxt, cdp.CommandServiceWorkerDispatchSyncEvent, p, nil)
}
