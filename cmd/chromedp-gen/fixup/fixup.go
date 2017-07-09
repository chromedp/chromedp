// Package fixup modifies/alters/fixes and adds to the low level type
// definitions for the Chrome Debugging Protocol domains, as generated from
// protocol.json.
//
// The goal of package fixup is to fix the issues associated with generating Go
// code from the existing Chrome domain definitions, and is wrapped up in one
// high-level func, FixDomains.
//
// Currently, FixDomains will do the following:
//  - add 'Inspector.MethodType' type as a string enumeration of all the event/command names.
//  - add 'Inspector.MessageError' type as a object with code (integer), and message (string).
//  - add 'Inspector.Message' type as a object with id (integer), method (MethodType), params (interface{}), error (MessageError).
//  - add 'Inspector.DetachReason' type and change event 'Inspector.detached''s parameter reason's type.
//  - add 'Inspector.ErrorType' type.
//  - change type of Network.TimeSinceEpoch, Network.MonotonicTime, and
//    Runtime.Timestamp to internal Timestamp type.
//  - convert object properties and event/command parameters that are enums into independent types.
//  - change '*.modifiers' parameters to type Input.Modifier.
//  - add 'DOM.NodeType' type and convert "nodeType" parameters to it.
//  - change Page.Frame.id/parentID to FrameID type.
//  - add additional properties to 'Page.Frame' and 'DOM.Node' for use by higher level packages.
//  - add special unmarshaler to NodeId, BackendNodeId, FrameId to handle
//    values from older (v1.1) protocol versions. -- NOTE: this might need to be
//    applied to more types, such as network.LoaderId
//  - rename 'Input.GestureSourceType' -> 'Input.GestureType'.
//  - rename CSS.CSS* types.
//  - add Error() method to 'Runtime.ExceptionDetails' type so that it can be used as error.
//  - change 'Network.Headers' type to map[string]interface{}.
//
// Please note that the above is not an exhaustive list of all modifications
// applied to the domains, however it does attempt to give a comprehensive
// overview of the most important changes to the definition vs the vanilla
// specification.
package fixup

import (
	"fmt"
	"strings"

	"github.com/knq/chromedp/cmd/chromedp-gen/internal"
	"github.com/knq/chromedp/cmd/chromedp-gen/templates"
)

func setup() {
	types := map[string]bool{
		"DOM.BackendNodeId":      true,
		"DOM.BackendNode":        true,
		"DOM.NodeId":             true,
		"DOM.Node":               true,
		"DOM.NodeType":           true,
		"DOM.PseudoType":         true,
		"DOM.RGBA":               true,
		"DOM.ShadowRootType":     true,
		"Inspector.ErrorType":    true,
		"Inspector.MessageError": true,
		"Inspector.Message":      true,
		"Inspector.MethodType":   true,
		"Network.LoaderId":       true,
		"Network.MonotonicTime":  true,
		"Network.TimeSinceEpoch": true,
		"Page.FrameId":           true,
		"Page.Frame":             true,
	}

	if *internal.FlagRedirect {
		types["Network.Cookie"] = true
		types["Network.CookieSameSite"] = true
		types["Page.ResourceType"] = true
	}

	// set the cdp types
	internal.SetCDPTypes(types)
}

// Specific type names to use for the applied fixes to the protocol domains.
//
// These need to be here in case the location of these types change (see above)
// relative to the generated 'cdp' package.
const (
	domNodeIDRef = "NodeID"
	domNodeRef   = "*Node"
)

// FixDomains modifies, updates, alters, fixes, and adds to the types defined
// in the domains, so that the generated Chrome Debugging Protocol domain code
// is more Go-like and easier to use.
//
// Please see package-level documentation for the list of changes made to the
// various debugging protocol domains.
func FixDomains(domains []*internal.Domain) {
	// set up the internal types
	setup()

	// method type
	methodType := &internal.Type{
		ID:               "MethodType",
		Type:             internal.TypeString,
		Description:      "Chrome Debugging Protocol method type (ie, event and command names).",
		EnumValueNameMap: make(map[string]string),
		Extra:            templates.ExtraMethodTypeDomainDecoder(),
	}

	// message error type
	messageErrorType := &internal.Type{
		ID:          "MessageError",
		Type:        internal.TypeObject,
		Description: "Message error type.",
		Properties: []*internal.Type{
			{
				Name:        "code",
				Type:        internal.TypeInteger,
				Description: "Error code.",
			},
			{
				Name:        "message",
				Type:        internal.TypeString,
				Description: "Error message.",
			},
		},
		Extra: "// Error satisfies error interface.\nfunc (e *MessageError) Error() string {\nreturn fmt.Sprintf(\"%s (%d)\", e.Message, e.Code)\n}\n",
	}

	// message type
	messageType := &internal.Type{
		ID:          "Message",
		Type:        internal.TypeObject,
		Description: "Chrome Debugging Protocol message sent to/read over websocket connection.",
		Properties: []*internal.Type{
			{
				Name:        "id",
				Type:        internal.TypeInteger,
				Description: "Unique message identifier.",
				Optional:    true,
			},
			{
				Name:        "method",
				Ref:         "Inspector.MethodType",
				Description: "Event or command type.",
				Optional:    true,
			},
			{
				Name:        "params",
				Type:        internal.TypeAny,
				Description: "Event or command parameters.",
				Optional:    true,
			},
			{
				Name:        "result",
				Type:        internal.TypeAny,
				Description: "Command return values.",
				Optional:    true,
			},
			{
				Name:        "error",
				Ref:         "MessageError",
				Description: "Error message.",
				Optional:    true,
			},
		},
	}

	// detach reason type
	detachReasonType := &internal.Type{
		ID:          "DetachReason",
		Type:        internal.TypeString,
		Enum:        []string{"target_closed", "canceled_by_user", "replaced_with_devtools", "Render process gone."},
		Description: "Detach reason.",
	}

	// cdp error types
	errorValues := []string{"channel closed", "invalid result", "unknown result"}
	errorValueNameMap := make(map[string]string)
	for _, e := range errorValues {
		errorValueNameMap[e] = "Err" + internal.ForceCamel(e)
	}
	errorType := &internal.Type{
		ID:               "ErrorType",
		Type:             internal.TypeString,
		Enum:             errorValues,
		EnumValueNameMap: errorValueNameMap,
		Description:      "Error type.",
		Extra:            templates.ExtraCDPTypes(),
	}

	// modifier type
	modifierType := &internal.Type{
		ID:          "Modifier",
		Type:        internal.TypeInteger,
		EnumBitMask: true,
		Description: "Input key modifier type.",
		Enum:        []string{"None", "Alt", "Ctrl", "Meta", "Shift"},
		Extra:       "// ModifierCommand is an alias for ModifierMeta.\nconst ModifierCommand Modifier = ModifierMeta",
	}

	// node type type -- see: https://developer.mozilla.org/en/docs/Web/API/Node/nodeType
	nodeTypeType := &internal.Type{
		ID:          "NodeType",
		Type:        internal.TypeInteger,
		Description: "Node type.",
		Enum: []string{
			"Element", "Attribute", "Text", "CDATA", "EntityReference",
			"Entity", "ProcessingInstruction", "Comment", "Document",
			"DocumentType", "DocumentFragment", "Notation",
		},
	}

	// process domains
	for _, d := range domains {
		switch d.Domain {
		case internal.DomainInspector:
			// add Inspector types
			d.Types = append(d.Types, messageErrorType, messageType, methodType, detachReasonType, errorType)

			// find detached event's reason parameter and change type
			for _, e := range d.Events {
				if e.Name == "detached" {
					for _, t := range e.Parameters {
						if t.Name == "reason" {
							t.Ref = "DetachReason"
							t.Type = internal.TypeEnum("")
							break
						}
					}
					break
				}
			}

		case internal.DomainCSS:
			for _, t := range d.Types {
				if t.ID == "CSSComputedStyleProperty" {
					t.ID = "ComputedProperty"
				}
			}

		case internal.DomainInput:
			// add Input types
			d.Types = append(d.Types, modifierType)
			for _, t := range d.Types {
				switch t.ID {
				case "GestureSourceType":
					t.ID = "GestureType"

				case "TimeSinceEpoch":
					t.Type = internal.TypeTimestamp
					t.TimestampType = internal.TimestampTypeSecond
					t.Extra = templates.ExtraTimestampTemplate(t, d)
				}
			}

		case internal.DomainDOM:
			// add DOM types
			d.Types = append(d.Types, nodeTypeType)

			for _, t := range d.Types {
				switch t.ID {
				case "NodeId", "BackendNodeId":
					t.Extra += templates.ExtraFixStringUnmarshaler(internal.ForceCamel(t.ID), "ParseInt", ", 10, 64")

				case "Node":
					t.Properties = append(t.Properties,
						&internal.Type{
							Name:        "Parent",
							Ref:         domNodeRef,
							Description: "Parent node.",
							NoResolve:   true,
							NoExpose:    true,
						},
						&internal.Type{
							Name:        "Invalidated",
							Ref:         "chan struct{}",
							Description: "Invalidated channel.",
							NoResolve:   true,
							NoExpose:    true,
						},
						&internal.Type{
							Name:        "State",
							Ref:         "NodeState",
							Description: "Node state.",
							NoResolve:   true,
							NoExpose:    true,
						},
						&internal.Type{
							Name:        "",
							Ref:         "sync.RWMutex",
							Description: "Read write mutex.",
							NoResolve:   true,
							NoExpose:    true,
						},
					)
					t.Extra += templates.ExtraNodeTemplate()
				}
			}

		case internal.DomainPage:
			for _, t := range d.Types {
				switch t.ID {
				case "FrameId":
					t.Extra += templates.ExtraFixStringUnmarshaler(internal.ForceCamel(t.ID), "", "")

				case "Frame":
					t.Properties = append(t.Properties,
						&internal.Type{
							Name:        "State",
							Ref:         "FrameState",
							Description: "Frame state.",
							NoResolve:   true,
							NoExpose:    true,
						},
						&internal.Type{
							Name:        "Root",
							Ref:         domNodeRef,
							Description: "Frame document root.",
							NoResolve:   true,
							NoExpose:    true,
						},
						&internal.Type{
							Name:        "Nodes",
							Ref:         "map[" + domNodeIDRef + "]" + domNodeRef,
							Description: "Frame nodes.",
							NoResolve:   true,
							NoExpose:    true,
						},
						&internal.Type{
							Name:        "",
							Ref:         "sync.RWMutex",
							Description: "Read write mutex.",
							NoResolve:   true,
							NoExpose:    true,
						},
					)
					t.Extra += templates.ExtraFrameTemplate()

					// convert Frame.id/parentId to $ref of FrameID
					for _, p := range t.Properties {
						if p.Name == "id" || p.Name == "parentId" {
							p.Ref = "FrameId"
							p.Type = internal.TypeEnum("")
						}
					}
				}
			}

		case internal.DomainNetwork:
			for _, t := range d.Types {
				// change Monotonic to TypeTimestamp and add extra unmarshaling template
				if t.ID == "TimeSinceEpoch" {
					t.Type = internal.TypeTimestamp
					t.TimestampType = internal.TimestampTypeSecond
					t.Extra = templates.ExtraTimestampTemplate(t, d)
				}

				// change Monotonic to TypeTimestamp and add extra unmarshaling template
				if t.ID == "MonotonicTime" {
					t.Type = internal.TypeTimestamp
					t.TimestampType = internal.TimestampTypeMonotonic
					t.Extra = templates.ExtraTimestampTemplate(t, d)
				}

				// change Headers to be a map[string]interface{}
				if t.ID == "Headers" {
					t.Type = internal.TypeAny
					t.Ref = "map[string]interface{}"
				}
			}

		case internal.DomainRuntime:
			var types []*internal.Type
			for _, t := range d.Types {
				switch t.ID {
				case "Timestamp":
					t.Type = internal.TypeTimestamp
					t.TimestampType = internal.TimestampTypeMillisecond
					t.Extra += templates.ExtraTimestampTemplate(t, d)

				case "ExceptionDetails":
					t.Extra += templates.ExtraExceptionDetailsTemplate()
				}

				types = append(types, t)
			}
			d.Types = types
		}

		for _, t := range d.Types {
			// convert object properties
			if t.Properties != nil {
				t.Properties = convertObjectProperties(t.Properties, d, t.ID)
			}
		}

		// process events and commands
		processTypesWithParameters(methodType, d, d.Events, internal.EventMethodPrefix, internal.EventMethodSuffix)
		processTypesWithParameters(methodType, d, d.Commands, internal.CommandMethodPrefix, internal.CommandMethodSuffix)

		// fix input enums
		if d.Domain == internal.DomainInput {
			for _, t := range d.Types {
				if t.Enum != nil && t.ID != "Modifier" {
					t.EnumValueNameMap = make(map[string]string)
					for _, v := range t.Enum {
						prefix := ""
						switch t.ID {
						case "GestureType":
							prefix = "Gesture"
						case "ButtonType":
							prefix = "Button"
						}
						n := prefix + internal.ForceCamel(v)
						if t.ID == "KeyType" {
							n = "Key" + strings.Replace(n, "Key", "", -1)
						}
						t.EnumValueNameMap[v] = strings.Replace(n, "Cancell", "Cancel", -1)
					}
				}
			}
		}

		for _, t := range d.Types {
			// fix type stuttering
			if !t.NoExpose && !t.NoResolve {
				id := strings.TrimPrefix(t.ID, d.Domain.String())
				if id == "" {
					continue
				}

				t.ID = id
			}
		}
	}
}

// processTypesWithParameters adds the types to t's enum values, setting the
// enum value map for m. Also, converts the Parameters and Returns properties.
func processTypesWithParameters(m *internal.Type, d *internal.Domain, types []*internal.Type, prefix, suffix string) {
	for _, t := range types {
		n := t.ProtoName(d)
		m.Enum = append(m.Enum, n)
		m.EnumValueNameMap[n] = t.TypeName(prefix+d.String(), suffix)

		t.Parameters = convertObjectProperties(t.Parameters, d, t.Name)
		if t.Returns != nil {
			t.Returns = convertObjectProperties(t.Returns, d, t.Name)
		}
	}
}

// convertObjectProperties converts object properties.
func convertObjectProperties(params []*internal.Type, d *internal.Domain, name string) []*internal.Type {
	r := make([]*internal.Type, 0)
	for _, p := range params {
		switch {
		case p.Items != nil:
			r = append(r, &internal.Type{
				Name:        p.Name,
				Type:        internal.TypeArray,
				Description: p.Description,
				Optional:    p.Optional,
				Items:       convertObjectProperties([]*internal.Type{p.Items}, d, name+"."+p.Name)[0],
			})

		case p.Enum != nil:
			r = append(r, fixupEnumParameter(name, p, d))

		case p.Name == "modifiers":
			r = append(r, &internal.Type{
				Name:        p.Name,
				Ref:         "Modifier",
				Description: p.Description,
				Optional:    p.Optional,
			})

		case p.Name == "nodeType":
			r = append(r, &internal.Type{
				Name:        p.Name,
				Ref:         "DOM.NodeType",
				Description: p.Description,
				Optional:    p.Optional,
			})

		case p.Ref == "GestureSourceType":
			r = append(r, &internal.Type{
				Name:        p.Name,
				Ref:         "GestureType",
				Description: p.Description,
				Optional:    p.Optional,
			})

		case p.Ref == "CSSComputedStyleProperty":
			r = append(r, &internal.Type{
				Name:        p.Name,
				Ref:         "ComputedProperty",
				Description: p.Description,
				Optional:    p.Optional,
			})

		case p.Ref != "" && !p.NoExpose && !p.NoResolve:
			ref := strings.SplitN(p.Ref, ".", 2)
			if len(ref) == 1 {
				ref[0] = strings.TrimPrefix(ref[0], d.Domain.String())
			} else {
				ref[1] = strings.TrimPrefix(ref[1], ref[0])
			}

			z := strings.Join(ref, ".")
			if z == "" {
				z = p.Ref
			}

			r = append(r, &internal.Type{
				Name:        p.Name,
				Ref:         z,
				Description: p.Description,
				Optional:    p.Optional,
			})

		default:
			r = append(r, p)
		}
	}

	return r
}

// addEnumValues adds orig.Enum values to type named n's Enum values in domain.
func addEnumValues(d *internal.Domain, n string, p *internal.Type) {
	// find type
	var typ *internal.Type
	for _, t := range d.Types {
		if t.ID == n {
			typ = t
			break
		}
	}
	if typ == nil {
		typ = &internal.Type{
			ID:          n,
			Type:        internal.TypeString,
			Description: p.Description,
			Optional:    p.Optional,
		}
		d.Types = append(d.Types, typ)
	}

	// combine typ.Enum and vals
	v := make(map[string]bool)
	all := append(typ.Enum, p.Enum...)
	for _, z := range all {
		v[z] = false
	}

	var i int
	typ.Enum = make([]string, len(v))
	for _, z := range all {
		if !v[z] {
			typ.Enum[i] = z
			i++
		}
		v[z] = true
	}
}

// enumRefMap is the fully qualified parameter name to ref.
var enumRefMap = map[string]string{
	"Animation.Animation.type":                         "Type",
	"Console.ConsoleMessage.level":                     "MessageLevel",
	"Console.ConsoleMessage.source":                    "MessageSource",
	"CSS.CSSMedia.source":                              "MediaSource",
	"CSS.forcePseudoState.forcedPseudoClasses":         "PseudoClass",
	"Debugger.setPauseOnExceptions.state":              "ExceptionsState",
	"Emulation.ScreenOrientation.type":                 "OrientationType",
	"Emulation.setTouchEmulationEnabled.configuration": "EnabledConfiguration",
	"Input.dispatchKeyEvent.type":                      "KeyType",
	"Input.dispatchMouseEvent.button":                  "ButtonType",
	"Input.dispatchMouseEvent.type":                    "MouseType",
	"Input.dispatchTouchEvent.type":                    "TouchType",
	"Input.emulateTouchFromMouseEvent.button":          "ButtonType",
	"Input.emulateTouchFromMouseEvent.type":            "MouseType",
	"Input.TouchPoint.state":                           "TouchState",
	"Log.LogEntry.level":                               "Level",
	"Log.LogEntry.source":                              "Source",
	"Log.ViolationSetting.name":                        "Violation",
	"Network.Request.mixedContentType":                 "MixedContentType",
	"Network.Request.referrerPolicy":                   "ReferrerPolicy",
	"Page.startScreencast.format":                      "ScreencastFormat",
	"Runtime.consoleAPICalled.type":                    "APIType",
	"Runtime.ObjectPreview.subtype":                    "Subtype",
	"Runtime.ObjectPreview.type":                       "Type",
	"Runtime.PropertyPreview.subtype":                  "Subtype",
	"Runtime.PropertyPreview.type":                     "Type",
	"Runtime.RemoteObject.subtype":                     "Subtype",
	"Runtime.RemoteObject.type":                        "Type",
	"Tracing.start.transferMode":                       "TransferMode",
	"Tracing.TraceConfig.recordMode":                   "RecordMode",
}

// fixupEnumParameter takes an enum parameter, adds it to the domain and
// returns a type suitable for use in place of the type.
func fixupEnumParameter(typ string, p *internal.Type, d *internal.Domain) *internal.Type {
	fqname := strings.TrimSuffix(fmt.Sprintf("%s.%s.%s", d.Domain, typ, p.Name), ".")
	ref := internal.ForceCamel(typ + "." + p.Name)
	if n, ok := enumRefMap[fqname]; ok {
		ref = n
	}

	// add enum values to type name
	addEnumValues(d, ref, p)

	return &internal.Type{
		Name:        p.Name,
		Ref:         ref,
		Description: p.Description,
		Optional:    p.Optional,
	}
}
