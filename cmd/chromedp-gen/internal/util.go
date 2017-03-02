package internal

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/client9/misspell"
	"github.com/knq/snaker"
)

const (
	// Base64EncodedParamName is the base64encoded variable name in command
	// return values when they are optionally base64 encoded.
	Base64EncodedParamName = "base64Encoded"

	// Base64EncodedDescriptionPrefix is the prefix for command return
	// description prefix when base64 encoded.
	Base64EncodedDescriptionPrefix = "Base64-encoded"
)

// MisspellReplacer is the misspelling replacer
var MisspellReplacer *misspell.Replacer

func init() {
	MisspellReplacer = misspell.New()
	MisspellReplacer.Compile()
}

// ForceCamel forces camel case specific to go.
func ForceCamel(s string) string {
	if s == "" {
		return ""
	}

	return snaker.SnakeToCamelIdentifier(snaker.CamelToSnake(s))
}

// ForceCamelWithFirstLower forces the first portion to be lower case.
func ForceCamelWithFirstLower(s string) string {
	if s == "" {
		return ""
	}

	s = snaker.CamelToSnake(s)
	first := strings.SplitN(s, "_", -1)[0]
	s = snaker.SnakeToCamelIdentifier(s)

	return strings.ToLower(first) + s[len(first):]
}

// CodeRE is a regexp to match <code> and </code> tags.
var CodeRE = regexp.MustCompile(`<\/?code>`)

// resolve finds the ref in the provided domains, relative to domain d when ref
// is not namespaced.
func resolve(ref string, d *Domain, domains []*Domain) (DomainType, *Type, string) {
	n := strings.SplitN(ref, ".", 2)

	// determine domain
	dtyp := d.Domain
	typ := n[0]
	if len(n) == 2 {
		err := (&dtyp).UnmarshalJSON([]byte(`"` + n[0] + `"`))
		if err != nil {
			panic(err)
		}
		typ = n[1]
	}

	// determine if ref points to an object
	var other *Type
	for _, z := range domains {
		if dtyp == z.Domain {
			for _, j := range z.Types {
				if j.ID == typ {
					other = j
					break
				}
			}
			break
		}
	}

	if other == nil {
		panic(fmt.Sprintf("could not resolve type %s in domain %s", ref, d))
	}

	var s string
	// add prefix if not an internal type and not defined in the domain
	if IsCDPType(dtyp, typ) {
		if d.Domain != DomainType("cdp") {
			s += "cdp."
		}
	} else if dtyp != d.Domain {
		s += strings.ToLower(dtyp.String()) + "."
	}

	return dtyp, other, s + ForceCamel(typ)
}

// structDef returns a struct definition for a list of types.
func structDef(types []*Type, d *Domain, domains []*Domain, noExposeOverride, omitOnlyWhenOptional bool) string {
	s := "struct"
	if len(types) > 0 {
		s += " "
	}
	s += "{"
	for _, v := range types {
		s += "\n\t" + v.GoName(noExposeOverride) + " " + v.GoType(d, domains)

		omit := ",omitempty"
		if omitOnlyWhenOptional && !v.Optional.Bool() {
			omit = ""
		}

		// add json tag
		if v.NoExpose {
			s += " `json:\"-\"`"
		} else {
			s += " `json:\"" + v.Name + omit + "\"`"
		}

		// add comment
		if v.Type != TypeObject && v.Description != "" {
			comment := CodeRE.ReplaceAllString(v.Description, "")
			comment, _ = MisspellReplacer.Replace(comment)
			s += " // " + comment
		}
	}
	if len(types) > 0 {
		s += "\n"
	}
	s += "}"

	return s
}

// cdpTypes is the list of internal types.
var cdpTypes map[string]bool

// SetCDPTypes sets the internal types.
func SetCDPTypes(types map[string]bool) {
	cdpTypes = types
}

// IsCDPType determines if the specified domain and typ constitute an
// internal type.
func IsCDPType(dtyp DomainType, typ string) bool {
	if _, ok := cdpTypes[dtyp.String()+"."+typ]; ok {
		return true
	}

	return false
}

// CDPTypeList returns the list of internal types.
func CDPTypeList() []string {
	var types []string
	for k := range cdpTypes {
		n := strings.SplitN(k, ".", 2)
		types = append(types, ForceCamel(n[1]))
	}

	return types
}

// goReservedNames is the list of reserved names in Go.
var goReservedNames = map[string]bool{
	// language words
	"break":       true,
	"case":        true,
	"chan":        true,
	"const":       true,
	"continue":    true,
	"default":     true,
	"defer":       true,
	"else":        true,
	"fallthrough": true,
	"for":         true,
	"func":        true,
	"go":          true,
	"goto":        true,
	"if":          true,
	"import":      true,
	"interface":   true,
	"map":         true,
	"package":     true,
	"range":       true,
	"return":      true,
	"select":      true,
	"struct":      true,
	"switch":      true,
	"type":        true,
	"var":         true,

	// go types
	"error":      true,
	"bool":       true,
	"string":     true,
	"byte":       true,
	"rune":       true,
	"uintptr":    true,
	"int":        true,
	"int8":       true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"uint":       true,
	"uint8":      true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"float32":    true,
	"float64":    true,
	"complex64":  true,
	"complex128": true,
}
