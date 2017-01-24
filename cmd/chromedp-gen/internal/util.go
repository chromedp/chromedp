package internal

import (
	"fmt"
	"regexp"
	"strings"

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

// ForceCamel forces camel case specific to go.
func ForceCamel(s string) string {
	if s == "" {
		return ""
	}

	return snaker.SnakeToCamelIdentifier(snaker.CamelToSnake(s))
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
	if !IsInternalType(dtyp, typ) && dtyp != d.Domain {
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
			s += " // " + CodeRE.ReplaceAllString(v.Description, "")
		}
	}
	if len(types) > 0 {
		s += "\n"
	}
	s += "}"

	return s
}

// internalTypes is the list of internal types.
var internalTypes map[string]bool

// SetInternalTypes sets the internal types.
func SetInternalTypes(types map[string]bool) {
	internalTypes = types
}

// IsInternalType determines if the specified domain and typ constitute an
// internal type.
func IsInternalType(dtyp DomainType, typ string) bool {
	if _, ok := internalTypes[dtyp.String()+"."+typ]; ok {
		return true
	}

	return false
}

// InternalTypeList returns the list of internal types.
func InternalTypeList() []string {
	var types []string
	for k := range internalTypes {
		n := strings.SplitN(k, ".", 2)
		types = append(types, ForceCamel(n[1]))
	}

	return types
}

// goReservedNames is the list of reserved names.
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
