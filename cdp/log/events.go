package log

// Code generated by chromedp-gen. DO NOT EDIT.

import (
	cdp "github.com/igsky/chromedp/cdp"
)

// EventEntryAdded issued when new message was logged.
type EventEntryAdded struct {
	Entry *Entry `json:"entry"` // The entry.
}

// EventTypes all event types in the domain.
var EventTypes = []cdp.MethodType{
	cdp.EventLogEntryAdded,
}
