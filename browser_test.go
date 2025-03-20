package chromedp

import (
	"bytes"
	"github.com/chromedp/cdproto"
	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"testing"
)

func TestUnmarshalWithDefaultOptions(t *testing.T) {
	tests := []struct {
		Name      string
		Input     []byte
		WantError bool
	}{
		{Name: "simple json", Input: []byte(`{"id":1, "result": {"bar":"foo"}}`), WantError: false},
		{Name: "invalid utf-8 1", Input: []byte(`{"id":2, "result": "\udebe\\u018e8"}`), WantError: false},
		{Name: "invalid utf-8 2", Input: []byte(`{"id":2, "result": "y7vzPw=T6\u001a\u053a{\ud861=\u001b"}`), WantError: false},
		{Name: "empty json", Input: []byte(`{}`), WantError: false},
		{Name: "null result", Input: []byte(`{"id":4, "result": null}`), WantError: false},
		{Name: "nested json", Input: []byte(`{"id":5, "result": {"nested": {"foo": "bar"}}}`), WantError: false},
		{Name: "array in result", Input: []byte(`{"id":6, "result": ["foo", "bar"]}`), WantError: false},
		{Name: "boolean in result", Input: []byte(`{"id":7, "result": true}`), WantError: false},
		{Name: "number in result", Input: []byte(`{"id":8, "result": 12345}`), WantError: false},
		{Name: "string in result", Input: []byte(`{"id":9, "result": "foobar"}`), WantError: false},
		{Name: "extra fields", Input: []byte(`{"id":10, "result": {"bar":"foo"}, "extra": "field"}`), WantError: false},
		{Name: "invalid json", Input: []byte(`{"id":11, "result": {"bar":"foo"`), WantError: true},
		{Name: "empty input", Input: []byte(``), WantError: true},
		{Name: "whitespace input", Input: []byte(`   `), WantError: true},
		{Name: "null input", Input: nil, WantError: true},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var decoder jsontext.Decoder
			var b bytes.Buffer
			b.Write(test.Input)
			var msg cdproto.Message
			decoder.Reset(&b, DefaultUnmarshalOptions)
			err := jsonv2.UnmarshalDecode(&decoder, &msg, DefaultUnmarshalOptions)
			if test.WantError && err == nil {
				t.Error("expected error in unmarshal decode, but got none")
			} else if !test.WantError && err != nil {
				t.Errorf("expected no error in unmarshal decode, but got %s", err)
			}
			err = jsonv2.Unmarshal(test.Input, &msg, DefaultUnmarshalOptions)
			if test.WantError && err == nil {
				t.Error("expected error, but got none")
			} else if !test.WantError && err != nil {
				t.Errorf("expected no error, but got %s", err)
			}
		})
	}
}

func TestMarshalWithDefaultOptions(t *testing.T) {
	tests := []struct {
		Name      string
		Input     cdproto.Message
		WantError bool
	}{
		{Name: "simple json", Input: cdproto.Message{Result: []byte(`{"bar":"foo"}`)}, WantError: false},
		{Name: "invalid utf-8 1", Input: cdproto.Message{Result: []byte(`"\udebe\\u018e8"`)}, WantError: false},
		{Name: "invalid utf-8 2", Input: cdproto.Message{Result: []byte(`"y7vzPw=T6\u001a\u053a{\ud861=\u001b"`)}, WantError: false},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var encoder jsontext.Encoder
			var b bytes.Buffer
			encoder.Reset(&b, DefaultMarshalOptions)
			err := jsonv2.MarshalEncode(&encoder, &test.Input, DefaultMarshalOptions)
			if test.WantError && err == nil {
				t.Error("expected error in marshal encode, but got none")
			} else if !test.WantError && err != nil {
				t.Errorf("expected no error in marshal encode, but got %s", err)
			}
			_, err = jsonv2.Marshal(test.Input, DefaultMarshalOptions)
			if test.WantError && err == nil {
				t.Error("expected error, but got none")
			} else if !test.WantError && err != nil {
				t.Errorf("expected no error, but got %s", err)
			}
		})
	}
}
