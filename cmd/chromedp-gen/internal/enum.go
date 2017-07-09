package internal

import (
	"fmt"
	"strconv"
)

// HandlerType are the handler targets for commands and events.
type HandlerType string

// HandlerType values.
const (
	HandlerTypeBrowser  HandlerType = "browser"
	HandlerTypeRenderer HandlerType = "renderer"
)

// String satisfies stringer.
func (ht HandlerType) String() string {
	return string(ht)
}

// MarshalJSON satisfies json.Marshaler.
func (ht HandlerType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + ht + `"`), nil
}

// UnmarshalJSON satisfies json.Unmarshaler.
func (ht *HandlerType) UnmarshalJSON(buf []byte) error {
	s, err := strconv.Unquote(string(buf))
	if err != nil {
		return err
	}

	switch HandlerType(s) {
	case HandlerTypeBrowser:
		*ht = HandlerTypeBrowser
	case HandlerTypeRenderer:
		*ht = HandlerTypeRenderer

	default:
		return fmt.Errorf("unknown handler type %s", string(buf))
	}

	return nil
}

// TypeEnum is the Chrome domain type enum.
type TypeEnum string

// TypeEnum values.
const (
	TypeAny       TypeEnum = "any"
	TypeArray     TypeEnum = "array"
	TypeBoolean   TypeEnum = "boolean"
	TypeInteger   TypeEnum = "integer"
	TypeNumber    TypeEnum = "number"
	TypeObject    TypeEnum = "object"
	TypeString    TypeEnum = "string"
	TypeTimestamp TypeEnum = "timestamp"
)

// String satisfies stringer.
func (te TypeEnum) String() string {
	return string(te)
}

// MarshalJSON satisfies json.Marshaler.
func (te TypeEnum) MarshalJSON() ([]byte, error) {
	return []byte(`"` + te + `"`), nil
}

// UnmarshalJSON satisfies json.Unmarshaler.
func (te *TypeEnum) UnmarshalJSON(buf []byte) error {
	s, err := strconv.Unquote(string(buf))
	if err != nil {
		return err
	}

	switch TypeEnum(s) {
	case TypeAny:
		*te = TypeAny
	case TypeArray:
		*te = TypeArray
	case TypeBoolean:
		*te = TypeBoolean
	case TypeInteger:
		*te = TypeInteger
	case TypeNumber:
		*te = TypeNumber
	case TypeObject:
		*te = TypeObject
	case TypeString:
		*te = TypeString

	default:
		return fmt.Errorf("unknown type enum %s", string(buf))
	}

	return nil
}

// GoType returns the Go type for the TypeEnum.
func (te TypeEnum) GoType() string {
	switch te {
	case TypeAny:
		return "easyjson.RawMessage"

	case TypeBoolean:
		return "bool"

	case TypeInteger:
		return "int64"

	case TypeNumber:
		return "float64"

	case TypeString:
		return "string"

	case TypeTimestamp:
		return "time.Time"

	default:
		panic(fmt.Sprintf("called GoType on non primitive type %s", te.String()))
	}

	return ""
}

// GoEmptyValue returns the Go empty value for the TypeEnum.
func (te TypeEnum) GoEmptyValue() string {
	switch te {
	case TypeBoolean:
		return `false`

	case TypeInteger:
		return `0`

	case TypeNumber:
		return `0`

	case TypeString:
		return `""`

	case TypeTimestamp:
		return `time.Time{}`
	}

	return `nil`
}

// TimestampType are the various timestamp subtypes.
type TimestampType int

// TimestampType values.
const (
	TimestampTypeMillisecond TimestampType = 1 + iota
	TimestampTypeSecond
	TimestampTypeMonotonic
)
