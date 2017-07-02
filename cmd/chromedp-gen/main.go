// chromedp-gen is a tool to generate the low-level Chrome Debugging Protocol
// implementation types used by chromedp, based off Chrome's protocol.json.
//
// Please see README.md for more information on using this tool.
package main

//go:generate go run gen-domain.go
//go:generate qtc -dir templates -ext qtpl
//go:generate gofmt -w -s templates/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"

	"github.com/knq/chromedp/cmd/chromedp-gen/fixup"
	"github.com/knq/chromedp/cmd/chromedp-gen/gen"
	"github.com/knq/chromedp/cmd/chromedp-gen/internal"
)

func main() {
	// parse flags
	err := internal.FlagSet.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// load protocol data
	buf, err := ioutil.ReadFile(*internal.FlagFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// unmarshal protocol info
	var protoInfo internal.ProtocolInfo
	err = json.Unmarshal(buf, &protoInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// remove existing directory
	if !*internal.FlagNoRemove {
		err = os.RemoveAll(out())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	// determine what to process
	pkgs := []string{""}
	var processed []*internal.Domain
	for _, d := range protoInfo.Domains {
		// skip if not processing
		if (!*internal.FlagDep && d.Deprecated.Bool()) || (!*internal.FlagExp && d.Experimental.Bool()) {
			// extra info
			var extra []string
			if d.Deprecated.Bool() {
				extra = append(extra, "deprecated")
			}
			if d.Experimental.Bool() {
				extra = append(extra, "experimental")
			}

			log.Printf("skipping domain %s (%s) %v", d, d.PackageName(), extra)
			continue
		}

		// will process
		pkgs = append(pkgs, d.PackageName())
		processed = append(processed, d)

		// cleanup types, events, commands
		cleanup(d)
	}

	// fixup
	fixup.FixDomains(processed)

	// generate
	files := gen.GenerateDomains(processed)

	// write
	err = write(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// goimports
	err = goimports(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// easyjson
	err = easyjson(pkgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// gofmt
	err = gofmt(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	log.Printf("done.")
}

// cleanupTypes removes deprecated types.
func cleanupTypes(n string, dtyp string, types []*internal.Type) []*internal.Type {
	var ret []*internal.Type

	for _, t := range types {
		typ := dtyp + "." + t.IDorName()
		if !*internal.FlagDep && t.Deprecated.Bool() {
			log.Printf("skipping %s %s [deprecated]", n, typ)
			continue
		}

		if !*internal.FlagRedirect && string(t.Redirect) != "" {
			log.Printf("skipping %s %s [redirect:%s]", n, typ, t.Redirect)
			continue
		}

		if t.Properties != nil {
			t.Properties = cleanupTypes(n+" property", typ, t.Properties)
		}

		if t.Parameters != nil {
			t.Parameters = cleanupTypes(n+" param", typ, t.Parameters)
		}

		if t.Returns != nil {
			t.Returns = cleanupTypes(n+" return param", typ, t.Returns)
		}

		ret = append(ret, t)
	}

	return ret
}

// cleanup removes deprecated types, events, and commands from the domain.
func cleanup(d *internal.Domain) {
	d.Types = cleanupTypes("type", d.String(), d.Types)
	d.Events = cleanupTypes("event", d.String(), d.Events)
	d.Commands = cleanupTypes("command", d.String(), d.Commands)
}

// write writes all the file buffers.
func write(fileBuffers map[string]*bytes.Buffer) error {
	var err error

	out := out() + "/"
	for n, buf := range fileBuffers {
		// add out path
		n = out + n

		// create directory
		dir := filepath.Dir(n)
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}

		// write file
		err = ioutil.WriteFile(n, buf.Bytes(), 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

// goimports formats all the output file buffers on disk using goimports.
func goimports(fileBuffers map[string]*bytes.Buffer) error {
	log.Printf("running goimports")

	var keys []string
	for k := range fileBuffers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var wg sync.WaitGroup
	for _, n := range keys {
		wg.Add(1)
		go func(wg *sync.WaitGroup, n string) {
			defer wg.Done()
			buf, err := exec.Command("goimports", "-w", out()+"/"+n).CombinedOutput()
			if err != nil {
				log.Fatalf("error: could not format %s, got:\n%s", n, string(buf))
			}
		}(&wg, n)
	}
	wg.Wait()

	return nil
}

// easyjson runs easy json on the list of packages.
func easyjson(pkgs []string) error {
	p := []string{"-pkg", "-all", "-output_filename", "easyjson.go"}

	log.Printf("running easyjson (stubs)")

	// generate easyjson stubs
	var wg sync.WaitGroup
	for _, n := range pkgs {
		wg.Add(1)
		go func(wg *sync.WaitGroup, n string) {
			defer wg.Done()
			cmd := exec.Command("easyjson", append(p, "-stubs")...)
			cmd.Dir = out() + "/" + n
			buf, err := cmd.CombinedOutput()
			if err != nil {
				log.Fatalf("could not generate easyjson stubs for %s, got:\n%s", cmd.Dir, string(buf))
			}
		}(&wg, n)
	}
	wg.Wait()

	log.Printf("running easyjson")

	// generate actual easyjson types
	for _, n := range pkgs {
		wg.Add(1)
		go func(wg *sync.WaitGroup, n string) {
			defer wg.Done()
			cmd := exec.Command("easyjson", p...)
			cmd.Dir = out() + "/" + n
			buf, err := cmd.CombinedOutput()
			if err != nil {
				log.Fatalf("could not easyjson %s, got:\n%s", cmd.Dir, string(buf))
			}
		}(&wg, n)
	}
	wg.Wait()

	return nil
}

// gofmt formats all the output file buffers on disk using gofmt.
func gofmt(fileBuffers map[string]*bytes.Buffer) error {
	log.Printf("running gofmt")

	var keys []string
	for k := range fileBuffers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var wg sync.WaitGroup
	for _, n := range keys {
		wg.Add(1)
		go func(wg *sync.WaitGroup, n string) {
			defer wg.Done()
			buf, err := exec.Command("gofmt", "-w", "-s", out()+"/"+n).CombinedOutput()
			if err != nil {
				log.Fatalf("error: could not format %s, got:\n%s", n, string(buf))
			}
		}(&wg, n)
	}
	wg.Wait()

	return nil
}

// out returns the output path of the passed package flag.
func out() string {
	return os.Getenv("GOPATH") + "/src/" + *internal.FlagPkg
}
