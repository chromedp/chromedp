package internal

import (
	"fmt"
	"strconv"
)

// DomainType is the Chrome domain type.
type DomainType string

/*
Accessibility
Animation
ApplicationCache
Browser
CacheStorage
Console
CSS
Database
Debugger
DeviceOrientation
DOM
DOMDebugger
DOMStorage
Emulation
HeapProfiler
IndexedDB
Input
Inspector
IO
LayerTree
Log
Memory
Network
Page
Profiler
Rendering
Runtime
Schema
Security
ServiceWorker
Storage
SystemInfo
Target
Tethering
Tracing
*/

// generated with:
// '<,'>s/^\(.*\)$/Domain\1 DomainType = "\1"/
const (
	DomainAccessibility     DomainType = "Accessibility"
	DomainAnimation         DomainType = "Animation"
	DomainApplicationCache  DomainType = "ApplicationCache"
	DomainBrowser           DomainType = "Browser"
	DomainCacheStorage      DomainType = "CacheStorage"
	DomainConsole           DomainType = "Console"
	DomainCSS               DomainType = "CSS"
	DomainDatabase          DomainType = "Database"
	DomainDebugger          DomainType = "Debugger"
	DomainDeviceOrientation DomainType = "DeviceOrientation"
	DomainDOM               DomainType = "DOM"
	DomainDOMDebugger       DomainType = "DOMDebugger"
	DomainDOMStorage        DomainType = "DOMStorage"
	DomainEmulation         DomainType = "Emulation"
	DomainHeapProfiler      DomainType = "HeapProfiler"
	DomainIndexedDB         DomainType = "IndexedDB"
	DomainInput             DomainType = "Input"
	DomainInspector         DomainType = "Inspector"
	DomainIO                DomainType = "IO"
	DomainLayerTree         DomainType = "LayerTree"
	DomainLog               DomainType = "Log"
	DomainMemory            DomainType = "Memory"
	DomainNetwork           DomainType = "Network"
	DomainPage              DomainType = "Page"
	DomainProfiler          DomainType = "Profiler"
	DomainRendering         DomainType = "Rendering"
	DomainRuntime           DomainType = "Runtime"
	DomainSchema            DomainType = "Schema"
	DomainSecurity          DomainType = "Security"
	DomainServiceWorker     DomainType = "ServiceWorker"
	DomainStorage           DomainType = "Storage"
	DomainSystemInfo        DomainType = "SystemInfo"
	DomainTarget            DomainType = "Target"
	DomainTethering         DomainType = "Tethering"
	DomainTracing           DomainType = "Tracing"
)

// String satisfies Stringer.
func (dt DomainType) String() string {
	return string(dt)
}

// MarshalJSON satisfies json.Marshaler.
func (dt DomainType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + dt + `"`), nil
}

// UnmarshalJSON satisfies json.Unmarshaler.
func (dt *DomainType) UnmarshalJSON(buf []byte) error {
	s, err := strconv.Unquote(string(buf))
	if err != nil {
		return err
	}

	switch DomainType(s) {

	// generated with:
	// '<,'>s/^\(.*\)$/case Domain\1:\r\t*dt = Domain\1/
	case DomainAccessibility:
		*dt = DomainAccessibility
	case DomainAnimation:
		*dt = DomainAnimation
	case DomainApplicationCache:
		*dt = DomainApplicationCache
	case DomainBrowser:
		*dt = DomainBrowser
	case DomainCacheStorage:
		*dt = DomainCacheStorage
	case DomainConsole:
		*dt = DomainConsole
	case DomainCSS:
		*dt = DomainCSS
	case DomainDatabase:
		*dt = DomainDatabase
	case DomainDebugger:
		*dt = DomainDebugger
	case DomainDeviceOrientation:
		*dt = DomainDeviceOrientation
	case DomainDOM:
		*dt = DomainDOM
	case DomainDOMDebugger:
		*dt = DomainDOMDebugger
	case DomainDOMStorage:
		*dt = DomainDOMStorage
	case DomainEmulation:
		*dt = DomainEmulation
	case DomainHeapProfiler:
		*dt = DomainHeapProfiler
	case DomainIndexedDB:
		*dt = DomainIndexedDB
	case DomainInput:
		*dt = DomainInput
	case DomainInspector:
		*dt = DomainInspector
	case DomainIO:
		*dt = DomainIO
	case DomainLayerTree:
		*dt = DomainLayerTree
	case DomainLog:
		*dt = DomainLog
	case DomainMemory:
		*dt = DomainMemory
	case DomainNetwork:
		*dt = DomainNetwork
	case DomainPage:
		*dt = DomainPage
	case DomainProfiler:
		*dt = DomainProfiler
	case DomainRendering:
		*dt = DomainRendering
	case DomainRuntime:
		*dt = DomainRuntime
	case DomainSchema:
		*dt = DomainSchema
	case DomainSecurity:
		*dt = DomainSecurity
	case DomainServiceWorker:
		*dt = DomainServiceWorker
	case DomainStorage:
		*dt = DomainStorage
	case DomainSystemInfo:
		*dt = DomainSystemInfo
	case DomainTarget:
		*dt = DomainTarget
	case DomainTethering:
		*dt = DomainTethering
	case DomainTracing:
		*dt = DomainTracing

	default:
		return fmt.Errorf("unknown domain type %s", string(buf))
	}

	return nil
}

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
