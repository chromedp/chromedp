// Package internal contains the types and util funcs common to the
// chromedp-gen command.
package internal

import (
	"strconv"
	"strings"
	"unicode"
)

// ProtocolInfo holds information about the Chrome Debugging Protocol.
type ProtocolInfo struct {
	// Version contains the protocol version information.
	Version *Version `json:"version,omitempty"`

	// Domains lists the available domains in the protocol.
	Domains []*Domain `json:"domains,omitempty"`
}

// Version holds information about the protocol version.
type Version struct {
	// Major is the version major.
	Major string `json:"major,omitempty"`

	// Minor is the version minor.
	Minor string `json:"minor,omitempty"`
}

// Domain represents a Chrome Debugging Protocol domain.
type Domain struct {
	// Domain is the name of the domain.
	Domain DomainType `json:"domain,omitempty"`

	// Description is the domain description.
	Description string `json:"description,omitempty"`

	// Experimental indicates whether or not the domain is experimental.
	Experimental Bool `json:"experimental,omitempty"`

	// Deprecated indicates whether or not the domain is deprecated.
	Deprecated Bool `json:"deprecated,omitempty"`

	// Types are the list of types in the domain.
	Types []*Type `json:"types,omitempty"`

	// Commands are the list of command types in the domain.
	Commands []*Type `json:"commands,omitempty"`

	// Events is the list of events types in the domain.
	Events []*Type `json:"events,omitempty"`
}

// Strings satisfies stringer.
func (d *Domain) String() string {
	return d.Domain.String()
}

// GetDescription returns the formatted description of the domain.
func (d *Domain) GetDescription() string {
	return CodeRE.ReplaceAllString(d.Description, "")
}

// PackageName returns the Go package name to use for the domain.
func (d *Domain) PackageName() string {
	return strings.ToLower(d.String())
}

// DomainType returns the name of the type to use for the domain.
func (d *Domain) DomainType() string {
	return DomainTypePrefix + d.String() + DomainTypeSuffix
}

// PackageImportAlias returns the Go import package name alias to use for the
// domain, or the empty string.
func (d *Domain) PackageImportAlias() string {
	switch d.PackageName() {
	case "io":
		return "iodom"

	case "log":
		return "logdom"
	}

	return ""
}

// PackageRefName returns the Go package name to use for the domain.
func (d *Domain) PackageRefName() string {
	pkgAlias := d.PackageImportAlias()
	if pkgAlias == "" {
		return d.PackageName()
	}

	return pkgAlias
}

// Type represents a type available to the domain.
type Type struct {
	// Type is the provided type enum.
	Type TypeEnum `json:"type,omitempty"`

	// ID is the ID of the type when type is an object.
	ID string `json:"id,omitempty"`

	// Name is the type name.
	Name string `json:"name,omitempty"`

	// Description is the type description.
	Description string `json:"description,omitempty"`

	// Optional indicates if the type is optional. Used for commands and event parameters.
	Optional Bool `json:"optional,omitempty"`

	// Deprecated indicates if the type is deprecated. Used for commands and event parameters.
	Deprecated Bool `json:"deprecated,omitempty"`

	// Enum are the enum values for the type (only used when type is string --
	// a non empty slice marks the type as a "enum").
	Enum []string `json:"enum,omitempty"`

	// Ref is the ID of a referenced type when type points to another type.
	Ref string `json:"$ref,omitempty"`

	// Items is the type of contained values in the array when type is array.
	Items *Type `json:"items,omitempty"`

	// Properties are the properties of the object when type is object.
	Properties []*Type `json:"properties,omitempty"`

	// Parameters is the command or event parameters.
	Parameters []*Type `json:"parameters,omitempty"`

	// Returns is the return value types.
	Returns []*Type `json:"returns,omitempty"`

	// MinItems is the minimum number of items when type is array.
	MinItems int64 `json:"minItems,omitempty"`

	// MaxItems is the maximum number of items when type is array.
	MaxItems int64 `json:"maxItems,omitempty"`

	// Handlers are the listed handlers for the command or event.
	Handlers []HandlerType `json:"handlers,omitempty"`

	// Redirect is the redirect value for the command or event.
	Redirect DomainType `json:"redirect,omitempty"`

	// TimestampType is the timestamp subtype.
	TimestampType TimestampType `json:"-"`

	// NoExpose toggles whether or not to expose the type.
	NoExpose bool `json:"-"`

	// NoResolve toggles not resolving the type to a domain (ie, for special internal types).
	NoResolve bool `json:"-"`

	// EnumValueNameMap is a map to override the generated enum value name.
	EnumValueNameMap map[string]string `json:"-"`

	// EnumBitMask toggles it as a bit mask enum for TypeInteger enums.
	EnumBitMask bool `json:"-"`

	// Extra will be added as output after the the type is emitted.
	Extra string `json:"-"`
}

// ResolveType resolves the type relative to the Go domain.
//
// Returns the DomainType of the underlying type, the underlying type (or the
// original passed type if not a reference) and the fully qualified name type
// name.
func (t *Type) ResolveType(d *Domain, domains []*Domain) (DomainType, *Type, string) {
	switch {
	case t.NoExpose || t.NoResolve || strings.HasPrefix(t.Ref, "*"):
		return d.Domain, t, t.Ref

	case t.Ref != "":
		dtyp, typ, z := resolve(t.Ref, d, domains)

		// add ptr if object
		ptr := ""
		switch typ.Type {
		case TypeObject, TypeTimestamp:
			ptr = "*"
		}

		return dtyp, typ, ptr + z

	case t.ID != "":
		return resolve(t.ID, d, domains)

	case t.Type == TypeArray:
		dtyp, typ, z := t.Items.ResolveType(d, domains)
		return dtyp, typ, "[]" + z

	case t.Type == TypeObject && (t.Properties == nil || len(t.Properties) == 0):
		return d.Domain, t, TypeAny.GoType()

	case t.Type == TypeObject:
		panic("should not encounter an object with defined properties that does not have Ref and ID")
	}

	return d.Domain, t, t.Type.GoType()
}

// IDorName returns either the ID or the Name for the type.
func (t Type) IDorName() string {
	if t.ID != "" {
		return t.ID
	}

	return t.Name
}

// String satisfies stringer.
func (t Type) String() string {
	desc := t.GetDescription()
	if desc != "" {
		desc, _ = MisspellReplacer.Replace(desc)
		desc = " - " + desc
	}

	return ForceCamelWithFirstLower(t.IDorName()) + desc
}

// GetDescription returns the cleaned description for the type.
func (t *Type) GetDescription() string {
	return CodeRE.ReplaceAllString(t.Description, "")
}

// EnumValues returns enum values for the type.
func (t *Type) EnumValues() []string {
	return t.Enum
}

// GoName returns the Go name.
func (t *Type) GoName(noExposeOverride bool) string {
	if t.NoExpose || noExposeOverride {
		n := t.Name
		if n != "" && !unicode.IsUpper(rune(n[0])) {
			if goReservedNames[n] {
				n += "Val"
			}
			n = ForceCamelWithFirstLower(n)
		}

		return n
	}

	return ForceCamel(t.IDorName())
}

// EnumValueName returns the name for a enum value.
func (t *Type) EnumValueName(v string) string {
	if t.EnumValueNameMap != nil {
		if e, ok := t.EnumValueNameMap[v]; ok {
			return e
		}
	}

	// special case for "negative" value
	neg := ""
	if strings.HasPrefix(v, "-") {
		neg = "Negative"
	}

	return ForceCamel(t.IDorName()) + neg + ForceCamel(v)
}

// GoTypeDef returns the Go type definition for the type.
func (t *Type) GoTypeDef(d *Domain, domains []*Domain, extra []*Type, noExposeOverride, omitOnlyWhenOptional bool) string {
	switch {
	case t.Parameters != nil:
		return structDef(append(extra, t.Parameters...), d, domains, noExposeOverride, omitOnlyWhenOptional)

	case t.Type == TypeArray:
		_, o, _ := t.Items.ResolveType(d, domains)
		return "[]" + o.GoTypeDef(d, domains, nil, false, false)

	case t.Type == TypeObject:
		return structDef(append(extra, t.Properties...), d, domains, noExposeOverride, omitOnlyWhenOptional)

	case t.Type == TypeAny && t.Ref != "":
		return t.Ref
	}

	return t.Type.GoType()
}

// GoType returns the Go type for the type.
func (t *Type) GoType(d *Domain, domains []*Domain) string {
	_, _, z := t.ResolveType(d, domains)
	return z
}

// GoEmptyValue returns the empty Go value for the type.
func (t *Type) GoEmptyValue(d *Domain, domains []*Domain) string {
	typ := t.GoType(d, domains)

	switch {
	case strings.HasPrefix(typ, "[]") || strings.HasPrefix(typ, "*"):
		return "nil"
	}

	return t.Type.GoEmptyValue()
}

// ParamList returns the list of parameters.
func (t *Type) ParamList(d *Domain, domains []*Domain, all bool) string {
	var s string
	for _, p := range t.Parameters {
		if !all && p.Optional.Bool() {
			continue
		}

		_, _, z := p.ResolveType(d, domains)
		s += p.GoName(true) + " " + z + ","
	}

	return strings.TrimSuffix(s, ",")
}

// RetTypeList returns a list of the return types.
func (t *Type) RetTypeList(d *Domain, domains []*Domain) string {
	var s string

	b64ret := t.Base64EncodedRetParam()
	for _, p := range t.Returns {
		if p.Name == Base64EncodedParamName {
			continue
		}

		n := p.Name
		_, _, z := p.ResolveType(d, domains)

		// if this is a base64 encoded item
		if b64ret != nil && b64ret.Name == p.Name {
			z = "[]byte"
		}

		s += ForceCamelWithFirstLower(n) + " " + z + ","
	}

	return strings.TrimSuffix(s, ",")
}

// EmptyRetList returns a list of the empty return values.
func (t *Type) EmptyRetList(d *Domain, domains []*Domain) string {
	var s string

	b64ret := t.Base64EncodedRetParam()
	for _, p := range t.Returns {
		if p.Name == Base64EncodedParamName {
			continue
		}

		_, o, z := p.ResolveType(d, domains)
		v := o.Type.GoEmptyValue()
		if strings.HasPrefix(z, "*") || strings.HasPrefix(z, "[]") || (b64ret != nil && b64ret.Name == p.Name) {
			v = "nil"
		}

		s += v + ", "
	}

	return strings.TrimSuffix(s, ", ")
}

// RetNameList returns a <valname>.<name> list for a command's return list.
func (t *Type) RetNameList(valname string, d *Domain, domains []*Domain) string {
	var s string
	b64ret := t.Base64EncodedRetParam()
	for _, p := range t.Returns {
		if p.Name == Base64EncodedParamName {
			continue
		}

		n := valname + "." + p.GoName(false)
		if b64ret != nil && b64ret.Name == p.Name {
			n = "dec"
		}

		s += n + ", "
	}

	return strings.TrimSuffix(s, ", ")
}

// Base64EncodedRetParam returns the base64 encoded return parameter, or nil if
// no parameters are base64 encoded.
func (t *Type) Base64EncodedRetParam() *Type {
	var last *Type
	for _, p := range t.Returns {
		if p.Name == Base64EncodedParamName {
			return last
		}
		if strings.HasPrefix(p.Description, Base64EncodedDescriptionPrefix) {
			return p
		}

		last = p
	}

	return nil
}

// CamelName returns the CamelCase name of the type.
func (t *Type) CamelName() string {
	return ForceCamel(t.IDorName())
}

// ProtoName returns the protocol name of the type.
func (t *Type) ProtoName(d *Domain) string {
	return d.String() + "." + t.Name
}

// EventMethodType returns the method type of the event.
func (t *Type) EventMethodType(d *Domain) string {
	return EventMethodPrefix + ForceCamel(t.ProtoName(d)) + EventMethodSuffix
}

// CommandMethodType returns the method type of the event.
func (t *Type) CommandMethodType(d *Domain) string {
	return CommandMethodPrefix + ForceCamel(t.ProtoName(d)) + CommandMethodSuffix
}

// TypeName returns the type name using the supplied prefix and suffix.
func (t *Type) TypeName(prefix, suffix string) string {
	return prefix + t.CamelName() + suffix
}

// EventType returns the type of the event.
func (t *Type) EventType() string {
	return t.TypeName(EventTypePrefix, EventTypeSuffix)
}

// CommandType returns the type of the command.
func (t *Type) CommandType() string {
	return t.TypeName(CommandTypePrefix, CommandTypeSuffix)
}

// CommandReturnsType returns the type of the command return type.
func (t *Type) CommandReturnsType() string {
	return t.TypeName(CommandReturnsPrefix, CommandReturnsSuffix)
}

// Bool provides a type for handling incorrectly quoted boolean values in the
// protocol.json file.
type Bool bool

// MarshalJSON marshals the bool into its corresponding JSON representation.
func (b Bool) MarshalJSON() ([]byte, error) {
	if b {
		return []byte("true"), nil
	}

	return []byte("false"), nil
}

// UnmarshalJSON unmarshals a possibly quoted string representation of a bool
// (ie, true, "true", false, "false").
func (b *Bool) UnmarshalJSON(buf []byte) error {
	var err error

	s := string(buf)

	// unquote
	if s != "true" && s != "false" {
		s, err = strconv.Unquote(s)
		if err != nil {
			return err
		}
	}

	// parse
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}

	*b = Bool(v)

	return nil
}

// Bool returns the bool as a Go bool.
func (b Bool) Bool() bool {
	return bool(b)
}
