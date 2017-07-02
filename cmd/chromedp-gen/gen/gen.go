// Package gen takes the Chrome protocol domain definitions and applies the
// necessary code generation templates.
package gen

import (
	"bytes"
	"path"

	"github.com/gedex/inflector"
	qtpl "github.com/valyala/quicktemplate"

	"github.com/knq/chromedp/cmd/chromedp-gen/internal"
	"github.com/knq/chromedp/cmd/chromedp-gen/templates"
)

// fileBuffers is a type to manage buffers for file data.
type fileBuffers map[string]*bytes.Buffer

// GenerateDomains generates domains for the Chrome Debugging Protocol domain
// definitions, returning a set of file buffers as a map of the file name ->
// content.
func GenerateDomains(domains []*internal.Domain) map[string]*bytes.Buffer {
	fb := make(fileBuffers)

	var w *qtpl.Writer

	// determine base (also used for the domains manager type name)
	pkgBase := path.Base(*internal.FlagPkg)
	internal.DomainTypeSuffix = inflector.Singularize(internal.ForceCamel(pkgBase))

	// generate internal types
	fb.generateCDPTypes(domains)

	// generate util package
	fb.generateUtilPackage(domains)

	// do individual domain templates
	for _, d := range domains {
		pkgName := d.PackageName()
		pkgOut := pkgName + "/" + pkgName + ".go"

		// do command template
		w = fb.get(pkgOut, pkgName, d)
		templates.StreamDomainTemplate(w, d, domains)
		fb.release(w)

		// generate domain types
		if len(d.Types) != 0 {
			fb.generateTypes(
				pkgName+"/types.go",
				d.Types, internal.TypePrefix, internal.TypeSuffix, d, domains,
				"", "", "", "", "",
			)
		}

		// generate domain event types
		if len(d.Events) != 0 {
			fb.generateTypes(
				pkgName+"/events.go",
				d.Events, internal.EventTypePrefix, internal.EventTypeSuffix, d, domains,
				"EventTypes", "cdp.MethodType", "cdp."+internal.EventMethodPrefix+d.String(), internal.EventMethodSuffix,
				"All event types in the domain.",
			)
		}
	}

	return map[string]*bytes.Buffer(fb)
}

// generateCDPTypes generates the internal types for domain d.
//
// Because there are circular package dependencies, some types need to be moved
// to eliminate the circular dependencies. Please see the fixup package for a
// list of the "internal" CDP types.
func (fb fileBuffers) generateCDPTypes(domains []*internal.Domain) {
	var types []*internal.Type
	for _, d := range domains {
		// process internal types
		for _, t := range d.Types {
			if internal.IsCDPType(d.Domain, t.IDorName()) {
				types = append(types, t)
			}
		}
	}

	pkg := path.Base(*internal.FlagPkg)
	cdpDomain := &internal.Domain{
		Domain: internal.DomainType("cdp"),
		Types:  types,
	}
	doms := append(domains, cdpDomain)

	w := fb.get(pkg+".go", pkg, nil)
	for _, t := range types {
		templates.StreamTypeTemplate(w, t, internal.TypePrefix, internal.TypeSuffix, cdpDomain, doms, nil, false, true)
	}
	fb.release(w)
}

// generateUtilPackage generates the util package.
//
// Currently only contains the low-level message unmarshaler -- if this wasn't
// in a separate package, then there would be circular dependencies.
func (fb fileBuffers) generateUtilPackage(domains []*internal.Domain) {
	// generate import map data
	importMap := map[string]string{
		*internal.FlagPkg: "cdp",
	}
	for _, d := range domains {
		importMap[*internal.FlagPkg+"/"+d.PackageName()] = d.PackageImportAlias()
	}

	w := fb.get("cdputil/cdputil.go", "cdputil", nil)
	templates.StreamFileImportTemplate(w, importMap)
	templates.StreamExtraUtilTemplate(w, domains)
	fb.release(w)
}

// generateTypes generates the types for a domain.
func (fb fileBuffers) generateTypes(
	path string,
	types []*internal.Type, prefix, suffix string, d *internal.Domain, domains []*internal.Domain,
	emit, emitType, emitPrefix, emitSuffix, emitDesc string,
) {
	w := fb.get(path, d.PackageName(), d)

	// add internal import
	templates.StreamFileImportTemplate(w, map[string]string{*internal.FlagPkg: "cdp"})

	// process type list
	var names []string
	for _, t := range types {
		if internal.IsCDPType(d.Domain, t.IDorName()) {
			continue
		}
		templates.StreamTypeTemplate(w, t, prefix, suffix, d, domains, nil, false, true)
		names = append(names, t.TypeName(emitPrefix, emitSuffix))
	}

	// emit var
	if emit != "" {
		s := "[]" + emitType + "{"
		for _, n := range names {
			s += "\n" + n + ","
		}
		s += "\n}"
		templates.StreamFileVarTemplate(w, emit, s, emitDesc)
	}

	fb.release(w)
}

// get retrieves the file buffer for s, or creates it if it is not yet available.
func (fb fileBuffers) get(s string, pkgName string, d *internal.Domain) *qtpl.Writer {
	// check if it already exists
	if b, ok := fb[s]; ok {
		return qtpl.AcquireWriter(b)
	}

	// create buffer
	b := new(bytes.Buffer)
	fb[s] = b
	w := qtpl.AcquireWriter(b)

	v := d
	if b := path.Base(s); b != pkgName+".go" {
		v = nil
	}

	// add package header
	templates.StreamFileHeader(w, pkgName, v)

	return w
}

// release releases a template writer.
func (fb fileBuffers) release(w *qtpl.Writer) {
	qtpl.ReleaseWriter(w)
}
