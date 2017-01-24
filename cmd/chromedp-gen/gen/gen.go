package gen

import (
	"bytes"
	"path"

	"github.com/gedex/inflector"
	qtpl "github.com/valyala/quicktemplate"

	. "github.com/knq/chromedp/cmd/chromedp-gen/internal"
	. "github.com/knq/chromedp/cmd/chromedp-gen/templates"
)

// fileBuffers is a type to manage buffers for file data.
type fileBuffers map[string]*bytes.Buffer

// GenerateDomains generates domains for the Chrome Debugging Protocol domain
// definitions, returning generated file buffers.
func GenerateDomains(domains []*Domain) map[string]*bytes.Buffer {
	fb := make(fileBuffers)

	var w *qtpl.Writer

	// determine base (also used for the domains manager type name)
	pkgBase := path.Base(*FlagPkg)
	DomainTypeSuffix = inflector.Singularize(ForceCamel(pkgBase))

	// generate internal types
	fb.generateInternalTypes(domains)

	// generate util package
	fb.generateUtilPackage(domains)

	// do individual domain templates
	for _, d := range domains {
		pkgName := d.PackageName()
		pkgOut := pkgName + "/" + pkgName + ".go"

		// do command template
		w = fb.get(pkgOut, pkgName, d)
		StreamDomainTemplate(w, d, domains)
		fb.release(w)

		// generate domain types
		if len(d.Types) != 0 {
			fb.generateTypes(
				pkgName+"/types.go",
				d.Types, TypePrefix, TypeSuffix, d, domains,
				"", "", "", "", "",
			)
		}

		// generate domain event types
		if len(d.Events) != 0 {
			fb.generateTypes(
				pkgName+"/events.go",
				d.Events, EventTypePrefix, EventTypeSuffix, d, domains,
				"EventTypes", "MethodType", EventMethodPrefix+d.String(), EventMethodSuffix,
				"EventTypes is all event types in the domain.",
			)
		}
	}

	return map[string]*bytes.Buffer(fb)
}

// generateInternalTypes generates the internal types for domain d.
//
// because there are circular package dependencies, some types need to be moved
// to the shared internal package.
func (fb fileBuffers) generateInternalTypes(domains []*Domain) {
	pkg := path.Base(*FlagPkg)
	w := fb.get(pkg+".go", pkg, nil)

	for _, d := range domains {
		// process internal types
		for _, t := range d.Types {
			if IsInternalType(d.Domain, t.IdOrName()) {
				StreamTypeTemplate(w, t, TypePrefix, TypeSuffix, d, domains, nil, false, false)
			}
		}
	}

	fb.release(w)
}

// generateUtilPackage generates the util package.
//
// currently only contains the message unmarshaler: if this wasn't in a
// separate package, there would be circular dependencies.
func (fb fileBuffers) generateUtilPackage(domains []*Domain) {
	// generate imports
	importMap := map[string]string{
		*FlagPkg: ".",
	}
	for _, d := range domains {
		importMap[*FlagPkg+"/"+d.PackageName()] = d.PackageImportAlias()
	}

	w := fb.get("util/util.go", "util", nil)
	StreamFileImportTemplate(w, importMap)
	StreamExtraUtilTemplate(w, domains)
	fb.release(w)
}

// generateTypes generates the types.
func (fb fileBuffers) generateTypes(
	path string,
	types []*Type, prefix, suffix string, d *Domain, domains []*Domain,
	emit, emitType, emitPrefix, emitSuffix, emitDesc string,
) {
	w := fb.get(path, d.PackageName(), d)

	// add internal import
	StreamFileLocalImportTemplate(w, *FlagPkg)
	StreamFileEmptyVarTemplate(w, InternalTypeList()...)

	// process type list
	var names []string
	for _, t := range types {
		if IsInternalType(d.Domain, t.IdOrName()) {
			continue
		}
		StreamTypeTemplate(w, t, prefix, suffix, d, domains, nil, false, false)
		names = append(names, t.TypeName(emitPrefix, emitSuffix))
	}

	// emit var
	if emit != "" {
		s := "[]" + emitType + "{"
		for _, n := range names {
			s += "\n" + n + ","
		}
		s += "\n}"
		StreamFileVarTemplate(w, emit, s, emitDesc)
	}

	fb.release(w)
}

// get retrieves the file buffer for s, or creates it if it is not yet available.
func (fb fileBuffers) get(s string, pkgName string, d *Domain) *qtpl.Writer {
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
	StreamFileHeader(w, pkgName, v)

	return w
}

// release releases a template writer.
func (fb fileBuffers) release(w *qtpl.Writer) {
	qtpl.ReleaseWriter(w)
}
