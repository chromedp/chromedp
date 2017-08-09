// Code generated by easyjson for marshaling/unmarshaling. DO NOT EDIT.

package io

import (
	json "encoding/json"
	runtime "github.com/igsky/chromedp/cdp/runtime"
	easyjson "github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
)

// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)

func easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo(in *jlexer.Lexer, out *ResolveBlobReturns) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "uuid":
			out.UUID = string(in.String())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo(out *jwriter.Writer, in ResolveBlobReturns) {
	out.RawByte('{')
	first := true
	_ = first
	if in.UUID != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"uuid\":")
		out.String(string(in.UUID))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v ResolveBlobReturns) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v ResolveBlobReturns) MarshalEasyJSON(w *jwriter.Writer) {
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *ResolveBlobReturns) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *ResolveBlobReturns) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo(l, v)
}
func easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo1(in *jlexer.Lexer, out *ResolveBlobParams) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "objectId":
			out.ObjectID = runtime.RemoteObjectID(in.String())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo1(out *jwriter.Writer, in ResolveBlobParams) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"objectId\":")
	out.String(string(in.ObjectID))
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v ResolveBlobParams) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo1(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v ResolveBlobParams) MarshalEasyJSON(w *jwriter.Writer) {
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo1(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *ResolveBlobParams) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo1(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *ResolveBlobParams) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo1(l, v)
}
func easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo2(in *jlexer.Lexer, out *ReadReturns) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "base64Encoded":
			out.Base64encoded = bool(in.Bool())
		case "data":
			out.Data = string(in.String())
		case "eof":
			out.EOF = bool(in.Bool())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo2(out *jwriter.Writer, in ReadReturns) {
	out.RawByte('{')
	first := true
	_ = first
	if in.Base64encoded {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"base64Encoded\":")
		out.Bool(bool(in.Base64encoded))
	}
	if in.Data != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"data\":")
		out.String(string(in.Data))
	}
	if in.EOF {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"eof\":")
		out.Bool(bool(in.EOF))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v ReadReturns) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo2(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v ReadReturns) MarshalEasyJSON(w *jwriter.Writer) {
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo2(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *ReadReturns) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo2(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *ReadReturns) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo2(l, v)
}
func easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo3(in *jlexer.Lexer, out *ReadParams) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "handle":
			out.Handle = StreamHandle(in.String())
		case "offset":
			out.Offset = int64(in.Int64())
		case "size":
			out.Size = int64(in.Int64())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo3(out *jwriter.Writer, in ReadParams) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"handle\":")
	out.String(string(in.Handle))
	if in.Offset != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"offset\":")
		out.Int64(int64(in.Offset))
	}
	if in.Size != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"size\":")
		out.Int64(int64(in.Size))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v ReadParams) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo3(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v ReadParams) MarshalEasyJSON(w *jwriter.Writer) {
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo3(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *ReadParams) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo3(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *ReadParams) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo3(l, v)
}
func easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo4(in *jlexer.Lexer, out *CloseParams) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "handle":
			out.Handle = StreamHandle(in.String())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo4(out *jwriter.Writer, in CloseParams) {
	out.RawByte('{')
	first := true
	_ = first
	if !first {
		out.RawByte(',')
	}
	first = false
	out.RawString("\"handle\":")
	out.String(string(in.Handle))
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v CloseParams) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo4(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v CloseParams) MarshalEasyJSON(w *jwriter.Writer) {
	easyjsonC5a4559bEncodeGithubComKnqChromedpCdpIo4(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *CloseParams) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo4(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *CloseParams) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjsonC5a4559bDecodeGithubComKnqChromedpCdpIo4(l, v)
}
