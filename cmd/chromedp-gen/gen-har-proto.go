// +build ignore

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gedex/inflector"
	"github.com/knq/chromedp/cmd/chromedp-gen/internal"
)

const (
	specURL = "http://www.softwareishard.com/blog/har-12-spec/"

	cacheDataID = "CacheData"
)

var (
	flagOut = flag.String("out", "har.json", "out file")
)

// propRefMap is the map of property names to their respective type.
var propRefMap = map[string]string{
	"Log.creator":         "Creator",
	"Log.browser":         "Creator",
	"Log.pages":           "Page",
	"Log.entries":         "Entry",
	"Page.pageTimings":    "PageTimings",
	"Entry.request":       "Request",
	"Entry.response":      "Response",
	"Entry.cache":         "Cache",
	"Entry.timings":       "Timings",
	"Request.cookies":     "Cookie",
	"Request.headers":     "NameValuePair",
	"Request.queryString": "NameValuePair",
	"Request.postData":    "PostData",
	"Response.cookies":    "Cookie",
	"Response.headers":    "NameValuePair",
	"Response.content":    "Content",
	"PostData.params":     "Param",
	"Cache.beforeRequest": cacheDataID,
	"Cache.afterRequest":  cacheDataID,
}

func main() {
	var err error

	flag.Parse()

	// initial type map
	typeMap := map[string]internal.Type{
		"HAR": {
			ID:          "HAR",
			Type:        internal.TypeObject,
			Description: "Parent container for HAR log.",
			Properties: []*internal.Type{{
				Name: "log",
				Ref:  "Log",
			}},
		},
		"NameValuePair": {
			ID:          "NameValuePair",
			Type:        internal.TypeObject,
			Description: "Describes a name/value pair.",
			Properties: []*internal.Type{{
				Name:        "name",
				Type:        internal.TypeString,
				Description: "Name of the pair.",
			}, {
				Name:        "value",
				Type:        internal.TypeString,
				Description: "Value of the pair.",
			}, {
				Name:        "comment",
				Type:        internal.TypeString,
				Description: "A comment provided by the user or the application.",
				Optional:    internal.Bool(true),
			}},
		},
	}

	// load remote definition
	doc, err := goquery.NewDocument(specURL)
	if err != nil {
		log.Fatal(err)
	}

	// loop over type definitions
	doc.Find(`h3:contains("HAR Data Structure") + p + p + ul a`).Each(func(i int, s *goquery.Selection) {
		n := s.Text()

		// skip browser (same as creator)
		switch n {
		case "browser", "queryString", "headers":
			return
		}

		// generate the object ID
		id := inflector.Singularize(internal.ForceCamel(n))
		if strings.HasSuffix(id, "um") {
			id = strings.TrimSuffix(id, "um") + "a"
		}
		if strings.HasSuffix(id, "Timing") {
			id += "s"
		}

		log.Printf("processing '%s', id: '%s'", n, id)

		// base selector
		sel := fmt.Sprintf(".harType#%s", n)

		// grab description
		desc := strings.TrimSpace(doc.Find(sel + " + p").Text())
		if desc == "" {
			panic(fmt.Sprintf("%s (%s) has no description", n, id))
		}

		// grab properties and scan
		props, err := scanProps(id, readPropText(sel, doc))
		if err != nil {
			log.Fatal(err)
		}

		// add to type map
		typeMap[id] = internal.Type{
			ID:          id,
			Type:        internal.TypeObject,
			Description: desc,
			Properties:  props,
		}
	})

	// grab and scan cachedata properties
	cacheDataPropText := readPropText(`p:contains("Both beforeRequest and afterRequest object share the following structure.")`, doc)
	cacheDataProps, err := scanProps(cacheDataID, cacheDataPropText)
	if err != nil {
		log.Fatal(err)
	}
	typeMap[cacheDataID] = internal.Type{
		ID:          cacheDataID,
		Type:        internal.TypeObject,
		Description: "Describes the cache data for beforeRequest and afterRequest.",
		Properties:  cacheDataProps,
	}

	// sort by type names
	var typeNames []string
	for n := range typeMap {
		typeNames = append(typeNames, n)
	}
	sort.Strings(typeNames)

	// add to type list
	var types []*internal.Type
	for _, n := range typeNames {
		typ := typeMap[n]
		types = append(types, &typ)
	}

	// create the protocol info
	def := internal.ProtocolInfo{
		Version: &internal.Version{Major: "1", Minor: "2"},
		Domains: []*internal.Domain{{
			Domain:      internal.DomainType("HAR"),
			Description: "HTTP Archive Format",
			Types:       types,
		}},
	}

	// json marshal
	buf, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		log.Fatal(buf)
	}

	// write
	err = ioutil.WriteFile(*flagOut, buf, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func scanProps(id string, propText string) ([]*internal.Type, error) {
	// scan properties
	var props []*internal.Type
	scanner := bufio.NewScanner(strings.NewReader(propText))
	i := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// grab prop stuff
		propName := strings.TrimSpace(line[:strings.IndexAny(line, "[")])
		propDesc := strings.TrimSpace(line[strings.Index(line, "-")+1:])
		if propName == "" || propDesc == "" {
			return nil, fmt.Errorf("line %d missing either name or description", i)
		}
		opts := strings.TrimSpace(line[strings.Index(line, "[")+1 : strings.Index(line, "]")])

		// determine type
		typ := internal.TypeEnum(opts)
		if z := strings.Index(opts, ","); z != -1 {
			typ = internal.TypeEnum(strings.TrimSpace(opts[:z]))
		}

		// convert some fields to integers
		if strings.Contains(strings.ToLower(propName), "size") ||
			propName == "compression" || propName == "status" ||
			propName == "hitCount" {
			typ = internal.TypeInteger
		}

		// fix object/array refs
		var ref string
		var items *internal.Type
		fqPropName := fmt.Sprintf("%s.%s", id, propName)
		switch typ {
		case internal.TypeObject:
			typ = internal.TypeEnum("")
			ref = propRefMap[fqPropName]

		case internal.TypeArray:
			items = &internal.Type{
				Ref: propRefMap[fqPropName],
			}
		}

		// add property
		props = append(props, &internal.Type{
			Name:        propName,
			Type:        typ,
			Description: propDesc,
			Ref:         ref,
			Items:       items,
			Optional:    internal.Bool(strings.Contains(opts, "optional")),
		})

		i++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return props, nil
}

func readPropText(sel string, doc *goquery.Document) string {
	text := strings.TrimSpace(doc.Find(sel).NextAllFiltered("ul").Text())
	j := strings.Index(text, "\n\n")
	if j == -1 {
		panic(fmt.Sprintf("could not find property description for `%s`", sel))
	}
	return text[:j]
}
