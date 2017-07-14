package client

import (
	"errors"

	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
)

// Target is the common interface for a Chrome Debugging Protocol target.
type Target interface {
	String() string
	GetID() string
	GetType() TargetType
	GetWebsocketURL() string
}

// TargetType are the types of targets available in Chrome.
type TargetType string

// TargetType values.
const (
	BackgroundPage TargetType = "background_page"
	Other          TargetType = "other"
	Page           TargetType = "page"
	ServiceWorker  TargetType = "service_worker"
	Node           TargetType = "node"
)

// String satisfies stringer.
func (tt TargetType) String() string {
	return string(tt)
}

// MarshalEasyJSON satisfies easyjson.Marshaler.
func (tt TargetType) MarshalEasyJSON(out *jwriter.Writer) {
	out.String(string(tt))
}

// MarshalJSON satisfies json.Marshaler.
func (tt TargetType) MarshalJSON() ([]byte, error) {
	return easyjson.Marshal(tt)
}

// UnmarshalEasyJSON satisfies easyjson.Unmarshaler.
func (tt *TargetType) UnmarshalEasyJSON(in *jlexer.Lexer) {
	switch TargetType(in.String()) {
	case BackgroundPage:
		*tt = BackgroundPage
	case Other:
		*tt = Other
	case Page:
		*tt = Page
	case ServiceWorker:
		*tt = ServiceWorker
	case Node:
		*tt = Node

	default:
		in.AddError(errors.New("unknown TargetType"))
	}
}

// UnmarshalJSON satisfies json.Unmarshaler.
func (tt *TargetType) UnmarshalJSON(buf []byte) error {
	return easyjson.Unmarshal(buf, tt)
}
