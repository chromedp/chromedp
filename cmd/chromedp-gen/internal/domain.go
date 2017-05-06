package internal

import (
	"fmt"
	"strconv"
)

// DomainType is the Chrome domain type.
type DomainType string

// DomainType values.
const (
	DomainAccessibility     DomainType = "Accessibility"
	DomainAnimation         DomainType = "Animation"
	DomainApplicationCache  DomainType = "ApplicationCache"
	DomainBrowser           DomainType = "Browser"
	DomainCSS               DomainType = "CSS"
	DomainCacheStorage      DomainType = "CacheStorage"
	DomainConsole           DomainType = "Console"
	DomainDOM               DomainType = "DOM"
	DomainDOMDebugger       DomainType = "DOMDebugger"
	DomainDOMStorage        DomainType = "DOMStorage"
	DomainDatabase          DomainType = "Database"
	DomainDebugger          DomainType = "Debugger"
	DomainDeviceOrientation DomainType = "DeviceOrientation"
	DomainEmulation         DomainType = "Emulation"
	DomainHeapProfiler      DomainType = "HeapProfiler"
	DomainIO                DomainType = "IO"
	DomainIndexedDB         DomainType = "IndexedDB"
	DomainInput             DomainType = "Input"
	DomainInspector         DomainType = "Inspector"
	DomainLayerTree         DomainType = "LayerTree"
	DomainLog               DomainType = "Log"
	DomainMemory            DomainType = "Memory"
	DomainNetwork           DomainType = "Network"
	DomainOverlay           DomainType = "Overlay"
	DomainPage              DomainType = "Page"
	DomainProfiler          DomainType = "Profiler"
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
	return []byte("\"" + dt + "\""), nil
}

// UnmarshalJSON satisfies json.Unmarshaler.
func (dt *DomainType) UnmarshalJSON(buf []byte) error {
	s, err := strconv.Unquote(string(buf))
	if err != nil {
		return err
	}

	switch DomainType(s) {
	case DomainAccessibility:
		*dt = DomainAccessibility
	case DomainAnimation:
		*dt = DomainAnimation
	case DomainApplicationCache:
		*dt = DomainApplicationCache
	case DomainBrowser:
		*dt = DomainBrowser
	case DomainCSS:
		*dt = DomainCSS
	case DomainCacheStorage:
		*dt = DomainCacheStorage
	case DomainConsole:
		*dt = DomainConsole
	case DomainDOM:
		*dt = DomainDOM
	case DomainDOMDebugger:
		*dt = DomainDOMDebugger
	case DomainDOMStorage:
		*dt = DomainDOMStorage
	case DomainDatabase:
		*dt = DomainDatabase
	case DomainDebugger:
		*dt = DomainDebugger
	case DomainDeviceOrientation:
		*dt = DomainDeviceOrientation
	case DomainEmulation:
		*dt = DomainEmulation
	case DomainHeapProfiler:
		*dt = DomainHeapProfiler
	case DomainIO:
		*dt = DomainIO
	case DomainIndexedDB:
		*dt = DomainIndexedDB
	case DomainInput:
		*dt = DomainInput
	case DomainInspector:
		*dt = DomainInspector
	case DomainLayerTree:
		*dt = DomainLayerTree
	case DomainLog:
		*dt = DomainLog
	case DomainMemory:
		*dt = DomainMemory
	case DomainNetwork:
		*dt = DomainNetwork
	case DomainOverlay:
		*dt = DomainOverlay
	case DomainPage:
		*dt = DomainPage
	case DomainProfiler:
		*dt = DomainProfiler
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
