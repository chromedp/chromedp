// This file is automatically generated by qtc from "domain.qtpl".
// See https://github.com/valyala/quicktemplate for details.

//line templates/domain.qtpl:1
package templates

//line templates/domain.qtpl:1
import (
	"github.com/knq/chromedp/cmd/chromedp-gen/internal"
)

// DomainTemplate is the template for a single domain.

//line templates/domain.qtpl:6
import (
	qtio422016 "io"

	qt422016 "github.com/valyala/quicktemplate"
)

//line templates/domain.qtpl:6
var (
	_ = qtio422016.Copy
	_ = qt422016.AcquireByteBuffer
)

//line templates/domain.qtpl:6
func StreamDomainTemplate(qw422016 *qt422016.Writer, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:6
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:7
	qw422016.N().S(FileImportTemplate(map[string]string{
		*internal.FlagPkg: "cdp",
	}))
	//line templates/domain.qtpl:9
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:10
	for _, c := range d.Commands {
		//line templates/domain.qtpl:10
		qw422016.N().S(`
`)
		//line templates/domain.qtpl:11
		qw422016.N().S(CommandTemplate(c, d, domains))
		//line templates/domain.qtpl:11
		qw422016.N().S(`
`)
		//line templates/domain.qtpl:12
	}
	//line templates/domain.qtpl:12
	qw422016.N().S(`
`)
//line templates/domain.qtpl:13
}

//line templates/domain.qtpl:13
func WriteDomainTemplate(qq422016 qtio422016.Writer, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:13
	qw422016 := qt422016.AcquireWriter(qq422016)
	//line templates/domain.qtpl:13
	StreamDomainTemplate(qw422016, d, domains)
	//line templates/domain.qtpl:13
	qt422016.ReleaseWriter(qw422016)
//line templates/domain.qtpl:13
}

//line templates/domain.qtpl:13
func DomainTemplate(d *internal.Domain, domains []*internal.Domain) string {
	//line templates/domain.qtpl:13
	qb422016 := qt422016.AcquireByteBuffer()
	//line templates/domain.qtpl:13
	WriteDomainTemplate(qb422016, d, domains)
	//line templates/domain.qtpl:13
	qs422016 := string(qb422016.B)
	//line templates/domain.qtpl:13
	qt422016.ReleaseByteBuffer(qb422016)
	//line templates/domain.qtpl:13
	return qs422016
//line templates/domain.qtpl:13
}

// CommandTemplate is the general command template.

//line templates/domain.qtpl:16
func StreamCommandTemplate(qw422016 *qt422016.Writer, c *internal.Type, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:16
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:17
	/* add *Param type */

	//line templates/domain.qtpl:17
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:18
	qw422016.N().S(TypeTemplate(c, internal.CommandTypePrefix, internal.CommandTypeSuffix, d, domains, nil, false, true))
	//line templates/domain.qtpl:18
	qw422016.N().S(`

`)
	//line templates/domain.qtpl:20
	/* add Command func */

	//line templates/domain.qtpl:20
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:21
	qw422016.N().S(CommandFuncTemplate(c, d, domains))
	//line templates/domain.qtpl:21
	qw422016.N().S(`

`)
	//line templates/domain.qtpl:23
	/* add param funcs (only if it has parameters and a returns). */

	//line templates/domain.qtpl:23
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:24
	if len(c.Parameters) != 0 {
		//line templates/domain.qtpl:24
		for _, p := range c.Parameters {
			//line templates/domain.qtpl:24
			if !p.Optional {
				//line templates/domain.qtpl:24
				continue
				//line templates/domain.qtpl:24
			}
			//line templates/domain.qtpl:24
			qw422016.N().S(`
`)
			//line templates/domain.qtpl:25
			qw422016.N().S(CommandOptionFuncTemplate(p, c, d, domains))
			//line templates/domain.qtpl:25
			qw422016.N().S(`
`)
			//line templates/domain.qtpl:26
		}
		//line templates/domain.qtpl:26
	}
	//line templates/domain.qtpl:26
	qw422016.N().S(`

`)
	//line templates/domain.qtpl:28
	/* add *Returns type */

	//line templates/domain.qtpl:28
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:29
	if len(c.Returns) != 0 {
		//line templates/domain.qtpl:29
		qw422016.N().S(`
`)
		//line templates/domain.qtpl:30
		qw422016.N().S(TypeTemplate(&internal.Type{
			ID:          c.Name,
			Type:        internal.TypeObject,
			Description: "Return values.",
			Properties:  c.Returns,
		}, internal.CommandReturnsPrefix, internal.CommandReturnsSuffix, d, domains, nil, false, false))
		//line templates/domain.qtpl:35
		qw422016.N().S(`
`)
		//line templates/domain.qtpl:36
	}
	//line templates/domain.qtpl:36
	qw422016.N().S(`

`)
	//line templates/domain.qtpl:38
	/* add CommandParams.Do func */

	//line templates/domain.qtpl:38
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:39
	qw422016.N().S(CommandDoFuncTemplate(c, d, domains))
	//line templates/domain.qtpl:39
	qw422016.N().S(`
`)
//line templates/domain.qtpl:40
}

//line templates/domain.qtpl:40
func WriteCommandTemplate(qq422016 qtio422016.Writer, c *internal.Type, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:40
	qw422016 := qt422016.AcquireWriter(qq422016)
	//line templates/domain.qtpl:40
	StreamCommandTemplate(qw422016, c, d, domains)
	//line templates/domain.qtpl:40
	qt422016.ReleaseWriter(qw422016)
//line templates/domain.qtpl:40
}

//line templates/domain.qtpl:40
func CommandTemplate(c *internal.Type, d *internal.Domain, domains []*internal.Domain) string {
	//line templates/domain.qtpl:40
	qb422016 := qt422016.AcquireByteBuffer()
	//line templates/domain.qtpl:40
	WriteCommandTemplate(qb422016, c, d, domains)
	//line templates/domain.qtpl:40
	qs422016 := string(qb422016.B)
	//line templates/domain.qtpl:40
	qt422016.ReleaseByteBuffer(qb422016)
	//line templates/domain.qtpl:40
	return qs422016
//line templates/domain.qtpl:40
}

// CommandFuncTemplate is the command func template.

//line templates/domain.qtpl:43
func StreamCommandFuncTemplate(qw422016 *qt422016.Writer, c *internal.Type, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:44
	cmdName := c.CamelName()
	typ := c.CommandType()

	//line templates/domain.qtpl:46
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:47
	qw422016.N().S(formatComment(c.GetDescription(), "", cmdName+" "))
	//line templates/domain.qtpl:47
	if len(c.Parameters) > 0 {
		//line templates/domain.qtpl:47
		qw422016.N().S(`
//
// parameters:`)
		//line templates/domain.qtpl:49
		for _, p := range c.Parameters {
			//line templates/domain.qtpl:49
			if p.Optional {
				//line templates/domain.qtpl:49
				continue
				//line templates/domain.qtpl:49
			}
			//line templates/domain.qtpl:49
			qw422016.N().S(`
//   `)
			//line templates/domain.qtpl:50
			qw422016.N().S(p.String())
			//line templates/domain.qtpl:50
			if p.Optional {
				//line templates/domain.qtpl:50
				qw422016.N().S(` (optional)`)
				//line templates/domain.qtpl:50
			}
			//line templates/domain.qtpl:50
		}
		//line templates/domain.qtpl:50
	}
	//line templates/domain.qtpl:50
	qw422016.N().S(`
func `)
	//line templates/domain.qtpl:51
	qw422016.N().S(cmdName)
	//line templates/domain.qtpl:51
	qw422016.N().S(`(`)
	//line templates/domain.qtpl:51
	qw422016.N().S(c.ParamList(d, domains, false))
	//line templates/domain.qtpl:51
	qw422016.N().S(`) *`)
	//line templates/domain.qtpl:51
	qw422016.N().S(typ)
	//line templates/domain.qtpl:51
	qw422016.N().S(`{
	return &`)
	//line templates/domain.qtpl:52
	qw422016.N().S(typ)
	//line templates/domain.qtpl:52
	qw422016.N().S(`{`)
	//line templates/domain.qtpl:52
	for _, t := range c.Parameters {
		//line templates/domain.qtpl:52
		if !t.Optional {
			//line templates/domain.qtpl:52
			qw422016.N().S(`
		`)
			//line templates/domain.qtpl:53
			qw422016.N().S(t.GoName(false))
			//line templates/domain.qtpl:53
			qw422016.N().S(`: `)
			//line templates/domain.qtpl:53
			qw422016.N().S(t.GoName(true))
			//line templates/domain.qtpl:53
			qw422016.N().S(`,`)
			//line templates/domain.qtpl:53
		}
		//line templates/domain.qtpl:53
	}
	//line templates/domain.qtpl:53
	qw422016.N().S(`
	}
}
`)
//line templates/domain.qtpl:56
}

//line templates/domain.qtpl:56
func WriteCommandFuncTemplate(qq422016 qtio422016.Writer, c *internal.Type, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:56
	qw422016 := qt422016.AcquireWriter(qq422016)
	//line templates/domain.qtpl:56
	StreamCommandFuncTemplate(qw422016, c, d, domains)
	//line templates/domain.qtpl:56
	qt422016.ReleaseWriter(qw422016)
//line templates/domain.qtpl:56
}

//line templates/domain.qtpl:56
func CommandFuncTemplate(c *internal.Type, d *internal.Domain, domains []*internal.Domain) string {
	//line templates/domain.qtpl:56
	qb422016 := qt422016.AcquireByteBuffer()
	//line templates/domain.qtpl:56
	WriteCommandFuncTemplate(qb422016, c, d, domains)
	//line templates/domain.qtpl:56
	qs422016 := string(qb422016.B)
	//line templates/domain.qtpl:56
	qt422016.ReleaseByteBuffer(qb422016)
	//line templates/domain.qtpl:56
	return qs422016
//line templates/domain.qtpl:56
}

// CommandOptionFuncTemplate is the command option func template.

//line templates/domain.qtpl:59
func StreamCommandOptionFuncTemplate(qw422016 *qt422016.Writer, t *internal.Type, c *internal.Type, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:60
	n := t.GoName(false)
	optName := internal.OptionFuncPrefix + n + internal.OptionFuncSuffix
	typ := c.CommandType()
	v := t.GoName(true)

	//line templates/domain.qtpl:64
	qw422016.N().S(`
`)
	//line templates/domain.qtpl:65
	qw422016.N().S(formatComment(t.GetDescription(), "", optName+" "))
	//line templates/domain.qtpl:65
	qw422016.N().S(`
func (p `)
	//line templates/domain.qtpl:66
	qw422016.N().S(typ)
	//line templates/domain.qtpl:66
	qw422016.N().S(`) `)
	//line templates/domain.qtpl:66
	qw422016.N().S(optName)
	//line templates/domain.qtpl:66
	qw422016.N().S(`(`)
	//line templates/domain.qtpl:66
	qw422016.N().S(v)
	//line templates/domain.qtpl:66
	qw422016.N().S(` `)
	//line templates/domain.qtpl:66
	qw422016.N().S(t.GoType(d, domains))
	//line templates/domain.qtpl:66
	qw422016.N().S(`) *`)
	//line templates/domain.qtpl:66
	qw422016.N().S(typ)
	//line templates/domain.qtpl:66
	qw422016.N().S(`{
	p.`)
	//line templates/domain.qtpl:67
	qw422016.N().S(n)
	//line templates/domain.qtpl:67
	qw422016.N().S(` = `)
	//line templates/domain.qtpl:67
	qw422016.N().S(v)
	//line templates/domain.qtpl:67
	qw422016.N().S(`
	return &p
}
`)
//line templates/domain.qtpl:70
}

//line templates/domain.qtpl:70
func WriteCommandOptionFuncTemplate(qq422016 qtio422016.Writer, t *internal.Type, c *internal.Type, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:70
	qw422016 := qt422016.AcquireWriter(qq422016)
	//line templates/domain.qtpl:70
	StreamCommandOptionFuncTemplate(qw422016, t, c, d, domains)
	//line templates/domain.qtpl:70
	qt422016.ReleaseWriter(qw422016)
//line templates/domain.qtpl:70
}

//line templates/domain.qtpl:70
func CommandOptionFuncTemplate(t *internal.Type, c *internal.Type, d *internal.Domain, domains []*internal.Domain) string {
	//line templates/domain.qtpl:70
	qb422016 := qt422016.AcquireByteBuffer()
	//line templates/domain.qtpl:70
	WriteCommandOptionFuncTemplate(qb422016, t, c, d, domains)
	//line templates/domain.qtpl:70
	qs422016 := string(qb422016.B)
	//line templates/domain.qtpl:70
	qt422016.ReleaseByteBuffer(qb422016)
	//line templates/domain.qtpl:70
	return qs422016
//line templates/domain.qtpl:70
}

// CommandDoFuncTemplate is the command do func template.

//line templates/domain.qtpl:73
func StreamCommandDoFuncTemplate(qw422016 *qt422016.Writer, c *internal.Type, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:74
	typ := c.CommandType()

	hasEmptyParams := len(c.Parameters) == 0
	hasEmptyRet := len(c.Returns) == 0

	emptyRet := c.EmptyRetList(d, domains)
	if emptyRet != "" {
		emptyRet += ", "
	}

	retTypeList := c.RetTypeList(d, domains)
	if retTypeList != "" {
		retTypeList += ", "
	}

	retValueList := c.RetNameList("res", d, domains)
	if retValueList != "" {
		retValueList += ", "
	}

	b64ret := c.Base64EncodedRetParam()

	// determine if there's a conditional that indicates whether or not the
	// returned value is b64 encoded.
	var b64cond bool
	for _, p := range c.Returns {
		if p.Name == internal.Base64EncodedParamName {
			b64cond = true
			break
		}
	}

	pval := "p"
	if hasEmptyParams {
		pval = "nil"
	}

	//line templates/domain.qtpl:110
	qw422016.N().S(`
// Do executes `)
	//line templates/domain.qtpl:111
	qw422016.N().S(c.ProtoName(d))
	//line templates/domain.qtpl:111
	qw422016.N().S(` against the provided context and
// target handler.`)
	//line templates/domain.qtpl:112
	if !hasEmptyRet {
		//line templates/domain.qtpl:112
		qw422016.N().S(`
//
// returns:`)
		//line templates/domain.qtpl:114
		for _, p := range c.Returns {
			//line templates/domain.qtpl:114
			if p.Name == internal.Base64EncodedParamName {
				//line templates/domain.qtpl:114
				continue
				//line templates/domain.qtpl:114
			}
			//line templates/domain.qtpl:114
			qw422016.N().S(`
//   `)
			//line templates/domain.qtpl:115
			qw422016.N().S(p.String())
			//line templates/domain.qtpl:115
		}
		//line templates/domain.qtpl:115
	}
	//line templates/domain.qtpl:115
	qw422016.N().S(`
func (p *`)
	//line templates/domain.qtpl:116
	qw422016.N().S(typ)
	//line templates/domain.qtpl:116
	qw422016.N().S(`) Do(ctxt context.Context, h cdp.Handler) (`)
	//line templates/domain.qtpl:116
	qw422016.N().S(retTypeList)
	//line templates/domain.qtpl:116
	qw422016.N().S(`err error) {`)
	//line templates/domain.qtpl:116
	if hasEmptyRet {
		//line templates/domain.qtpl:116
		qw422016.N().S(`
	return h.Execute(ctxt, cdp.`)
		//line templates/domain.qtpl:117
		qw422016.N().S(c.CommandMethodType(d))
		//line templates/domain.qtpl:117
		qw422016.N().S(`, `)
		//line templates/domain.qtpl:117
		qw422016.N().S(pval)
		//line templates/domain.qtpl:117
		qw422016.N().S(`, nil)`)
		//line templates/domain.qtpl:117
	} else {
		//line templates/domain.qtpl:117
		qw422016.N().S(`
	// execute
	var res `)
		//line templates/domain.qtpl:119
		qw422016.N().S(c.CommandReturnsType())
		//line templates/domain.qtpl:119
		qw422016.N().S(`
	err = h.Execute(ctxt, cdp.`)
		//line templates/domain.qtpl:120
		qw422016.N().S(c.CommandMethodType(d))
		//line templates/domain.qtpl:120
		qw422016.N().S(`, `)
		//line templates/domain.qtpl:120
		qw422016.N().S(pval)
		//line templates/domain.qtpl:120
		qw422016.N().S(`, &res)
	if err != nil {
		return `)
		//line templates/domain.qtpl:122
		qw422016.N().S(emptyRet)
		//line templates/domain.qtpl:122
		qw422016.N().S(`err
	}
	`)
		//line templates/domain.qtpl:124
		if b64ret != nil {
			//line templates/domain.qtpl:124
			qw422016.N().S(`
	// decode
	var dec []byte`)
			//line templates/domain.qtpl:126
			if b64cond {
				//line templates/domain.qtpl:126
				qw422016.N().S(`
	if res.Base64encoded {`)
				//line templates/domain.qtpl:127
			}
			//line templates/domain.qtpl:127
			qw422016.N().S(`
		dec, err = base64.StdEncoding.DecodeString(res.`)
			//line templates/domain.qtpl:128
			qw422016.N().S(b64ret.GoName(false))
			//line templates/domain.qtpl:128
			qw422016.N().S(`)
		if err != nil {
			return nil, err
		}`)
			//line templates/domain.qtpl:131
			if b64cond {
				//line templates/domain.qtpl:131
				qw422016.N().S(`
	} else {
		dec = []byte(res.`)
				//line templates/domain.qtpl:133
				qw422016.N().S(b64ret.GoName(false))
				//line templates/domain.qtpl:133
				qw422016.N().S(`)
	}`)
				//line templates/domain.qtpl:134
			}
			//line templates/domain.qtpl:134
		}
		//line templates/domain.qtpl:134
		qw422016.N().S(`
	return `)
		//line templates/domain.qtpl:135
		qw422016.N().S(retValueList)
		//line templates/domain.qtpl:135
		qw422016.N().S(`nil`)
		//line templates/domain.qtpl:135
	}
	//line templates/domain.qtpl:135
	qw422016.N().S(`
}
`)
//line templates/domain.qtpl:137
}

//line templates/domain.qtpl:137
func WriteCommandDoFuncTemplate(qq422016 qtio422016.Writer, c *internal.Type, d *internal.Domain, domains []*internal.Domain) {
	//line templates/domain.qtpl:137
	qw422016 := qt422016.AcquireWriter(qq422016)
	//line templates/domain.qtpl:137
	StreamCommandDoFuncTemplate(qw422016, c, d, domains)
	//line templates/domain.qtpl:137
	qt422016.ReleaseWriter(qw422016)
//line templates/domain.qtpl:137
}

//line templates/domain.qtpl:137
func CommandDoFuncTemplate(c *internal.Type, d *internal.Domain, domains []*internal.Domain) string {
	//line templates/domain.qtpl:137
	qb422016 := qt422016.AcquireByteBuffer()
	//line templates/domain.qtpl:137
	WriteCommandDoFuncTemplate(qb422016, c, d, domains)
	//line templates/domain.qtpl:137
	qs422016 := string(qb422016.B)
	//line templates/domain.qtpl:137
	qt422016.ReleaseByteBuffer(qb422016)
	//line templates/domain.qtpl:137
	return qs422016
//line templates/domain.qtpl:137
}
