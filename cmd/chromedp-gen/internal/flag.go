package internal

import (
	"flag"
	"os"
)

// FlagSet is the set of application flags.
var FlagSet = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

// The application flags.
var (
	FlagPkg      = FlagSet.String("pkg", "github.com/knq/chromedp/cdp", "output package")
	FlagFile     = FlagSet.String("file", "protocol.json", "path to chrome protocol.json")
	FlagDep      = FlagSet.Bool("dep", false, "toggle deprecated API generation")
	FlagExp      = FlagSet.Bool("exp", true, "toggle experimental API generation")
	FlagRedirect = FlagSet.Bool("redirect", false, "toggle redirect API generation")
	FlagNoRemove = FlagSet.Bool("noremove", false, "toggle to not remove existing package directory")
)

// Prefix and suffix values.
var (
	DomainTypePrefix     = ""
	DomainTypeSuffix     = ""
	TypePrefix           = ""
	TypeSuffix           = ""
	EventMethodPrefix    = "Event"
	EventMethodSuffix    = ""
	CommandMethodPrefix  = "Command"
	CommandMethodSuffix  = ""
	EventTypePrefix      = "Event"
	EventTypeSuffix      = ""
	CommandTypePrefix    = ""
	CommandTypeSuffix    = "Params"
	CommandReturnsPrefix = ""
	CommandReturnsSuffix = "Returns"
	OptionFuncPrefix     = "With"
	OptionFuncSuffix     = ""
)
