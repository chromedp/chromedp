package chromedp

import (
	"context"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"time"

	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/runtime"
)

// ConsoleType is the kind of a [ConsoleMessage]. The values for a call of the
// console API are the names of the protocol, such as "log", "warning" and
// "error". The package defines a constant for the common ones.
type ConsoleType string

// Console message types.
const (
	ConsoleLog       ConsoleType = "log"
	ConsoleDebug     ConsoleType = "debug"
	ConsoleInfo      ConsoleType = "info"
	ConsoleWarning   ConsoleType = "warning"
	ConsoleError     ConsoleType = "error"
	ConsoleException ConsoleType = "exception"
)

// ConsoleMessage is a message that the page wrote to the console. It is a call
// of the console API, an exception that nothing caught, or an entry of the
// browser log.
type ConsoleMessage struct {
	// Type is the kind of the message. An uncaught exception and an unhandled
	// promise rejection have the type [ConsoleException]. An entry of the browser
	// log has the type "debug", "info", "warning" or "error", for the levels
	// verbose, info, warning and error.
	Type ConsoleType

	// Text is the text that the DevTools console shows. A string argument is
	// there as it is, and another value has its description or a short form of
	// its properties. A format such as %s and %d in the first argument takes the
	// arguments that follow it. For an exception, the text is the error and its
	// message, and the stack is in Stack.
	Text string

	// Args are the arguments of the call to the console API, as the protocol
	// reports them. The objects have no value, only a description and a
	// preview. Use [runtime.GetProperties] with the object id to read one. Args
	// is empty for an exception and for an entry of the browser log.
	Args []*runtime.RemoteObject

	// Time is the time of the message, as the browser reports it.
	Time time.Time

	// URL, Line and Column tell where the message came from. A call of the
	// console API has the place of the call. An exception has the place of the
	// throw. They can be empty. The line and the column start at 0, as in the
	// protocol.
	URL    string
	Line   int64
	Column int64

	// Stack is the stack trace of the call or of the exception. It is nil when
	// the browser reports none.
	Stack *runtime.StackTrace

	// Exception holds the details of an exception, and it is nil for another
	// message.
	Exception *runtime.ExceptionDetails

	// Source tells which part of the browser wrote a message of the browser
	// log, for example "network" or "security". It is empty for a call of the
	// console API and for an exception.
	Source string
}

// IsException reports whether the message is an uncaught exception or an
// unhandled promise rejection.
func (m ConsoleMessage) IsException() bool {
	return m.Type == ConsoleException
}

// String returns the type and the text of the message.
func (m ConsoleMessage) String() string {
	return string(m.Type) + ": " + m.Text
}

// Console subscribes to the console of the target of the context, and returns
// an iterator over its messages. A message is a call of the console API such
// as console.log, an uncaught exception, an unhandled promise rejection, or an
// entry of the browser log. The iterator keeps the order of the messages.
//
// Console reads the events [runtime.ConsoleAPICalled], [runtime.ExceptionThrown]
// and [log.EntryAdded]. [Events] gives the same events one by one, as the
// protocol sends them, and Console joins them in one stream of simple values.
// The old API used [ListenTarget] with a func that tested the type of each
// event.
//
// As [Events] does, Console subscribes when it returns, and not when the
// caller starts to range. A caller can subscribe, run the actions, and then
// range, so that no message is lost:
//
//	messages := chromedp.Console(ctx)
//	if err := chromedp.Do(ctx, chromedp.Navigate(url)); err != nil {
//		return err
//	}
//	for m, err := range messages {
//		if err != nil {
//			break
//		}
//		fmt.Println(m)
//	}
//
// A browser sends the messages only from the moment that the target exists, so
// a message that the page wrote before the first call of Console or of [Run]
// is lost. If the browser cannot start, the iterator yields the error. See
// [cdp.Events] for the rules about ending the iteration.
//
// To stop, cancel the context or break out of the loop. Both end the
// subscription and free its queue, also when the program never ranged over the
// iterator.
//
// The queue of the messages has no limit, so that a slow reader never blocks
// the page. The page keeps working when the program does not read the messages,
// but the queue grows with each message until the subscription ends. A program
// that reads slowly or stops to read must cancel the context or break out of
// the loop.
func Console(ctx context.Context) iter.Seq2[ConsoleMessage, error] {
	c, err := initContextTarget(ctx)
	if err != nil {
		return failed[ConsoleMessage](err)
	}
	events, unsubscribe := c.Target.events.subscribeMany(
		runtime.ConsoleAPICalled.Method,
		runtime.ExceptionThrown.Method,
		log.EntryAdded.Method,
	)
	// End the subscription when the context ends, also when nobody ranges
	// over the iterator.
	stop := context.AfterFunc(ctx, unsubscribe)
	return func(yield func(ConsoleMessage, error) bool) {
		defer unsubscribe()
		defer stop()
		for {
			select {
			case <-ctx.Done():
				yield(ConsoleMessage{}, ctx.Err())
				return
			case raw, ok := <-events:
				if !ok {
					return
				}
				m, err := decodeConsole(raw)
				if err != nil {
					yield(ConsoleMessage{}, err)
					return
				}
				if !yield(m, nil) {
					return
				}
			}
		}
	}
}

// decodeConsole turns a tagged event into a message.
func decodeConsole(raw jsontext.Value) (ConsoleMessage, error) {
	var ev struct {
		Method string         `json:"method"`
		Params jsontext.Value `json:"params"`
	}
	if err := jsonv2.Unmarshal(raw, &ev, DefaultUnmarshalOptions); err != nil {
		return ConsoleMessage{}, fmt.Errorf("decoding a console event: %w", err)
	}
	switch ev.Method {
	case runtime.ConsoleAPICalled.Method:
		var e runtime.EventConsoleAPICalled
		if err := jsonv2.Unmarshal(ev.Params, &e, DefaultUnmarshalOptions); err != nil {
			return ConsoleMessage{}, fmt.Errorf("decoding %s: %w", ev.Method, err)
		}
		return consoleCall(&e), nil
	case runtime.ExceptionThrown.Method:
		var e runtime.EventExceptionThrown
		if err := jsonv2.Unmarshal(ev.Params, &e, DefaultUnmarshalOptions); err != nil {
			return ConsoleMessage{}, fmt.Errorf("decoding %s: %w", ev.Method, err)
		}
		return consoleException(&e), nil
	default:
		var e log.EventEntryAdded
		if err := jsonv2.Unmarshal(ev.Params, &e, DefaultUnmarshalOptions); err != nil {
			return ConsoleMessage{}, fmt.Errorf("decoding %s: %w", ev.Method, err)
		}
		return consoleLog(&e), nil
	}
}

// consoleTime converts a protocol timestamp, in milliseconds, to a time.
func consoleTime(ts runtime.Timestamp) time.Time {
	ms := float64(ts)
	return time.Unix(int64(ms/1000), int64(ms*1e6)%1e9)
}

func consoleCall(e *runtime.EventConsoleAPICalled) ConsoleMessage {
	m := ConsoleMessage{
		Type:  ConsoleType(e.Type),
		Text:  consoleText(e.Args),
		Args:  e.Args,
		Time:  consoleTime(e.Timestamp),
		Stack: e.StackTrace,
	}
	if st := e.StackTrace; st != nil && len(st.CallFrames) > 0 {
		f := st.CallFrames[0]
		m.URL, m.Line, m.Column = f.URL, f.LineNumber, f.ColumnNumber
	}
	return m
}

func consoleException(e *runtime.EventExceptionThrown) ConsoleMessage {
	d := e.ExceptionDetails
	m := ConsoleMessage{
		Type:      ConsoleException,
		Text:      d.Text,
		Time:      consoleTime(e.Timestamp),
		URL:       d.URL,
		Line:      d.LineNumber,
		Column:    d.ColumnNumber,
		Stack:     d.StackTrace,
		Exception: d,
	}
	if obj := d.Exception; obj != nil {
		// The text is "Uncaught" or "Uncaught (in promise)", and the object is
		// the error. The description of an error holds its stack, which
		// is in Stack as well.
		desc, _, _ := strings.Cut(consoleValue(obj), "\n    at ")
		m.Text = strings.TrimSpace(d.Text + " " + desc)
	}
	if m.URL == "" && d.StackTrace != nil && len(d.StackTrace.CallFrames) > 0 {
		m.URL = d.StackTrace.CallFrames[0].URL
	}
	return m
}

func consoleLog(e *log.EventEntryAdded) ConsoleMessage {
	en := e.Entry
	m := ConsoleMessage{
		Text:   en.Text,
		Args:   en.Args,
		Time:   consoleTime(en.Timestamp),
		URL:    en.URL,
		Line:   en.LineNumber,
		Stack:  en.StackTrace,
		Source: string(en.Source),
	}
	switch en.Level {
	case log.EntryLevelVerbose:
		m.Type = ConsoleDebug
	default:
		m.Type = ConsoleType(en.Level)
	}
	return m
}

// consoleText renders the arguments of a call of the console API as the
// DevTools console does.
func consoleText(args []*runtime.RemoteObject) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	rest := args
	if first := args[0]; first.Type == runtime.RemoteObjectTypeString {
		var s string
		if err := jsonv2.Unmarshal(first.Value, &s); err == nil {
			if strings.Contains(s, "%") {
				var n int
				s, n = consoleFormat(s, args[1:])
				rest = args[1+n:]
			} else {
				rest = args[1:]
			}
			parts = append(parts, s)
		}
	}
	for _, a := range rest {
		parts = append(parts, consoleValue(a))
	}
	return strings.Join(parts, " ")
}

// consoleFormat applies the format specifiers %s, %d, %i, %f, %o, %O, %c and
// %% of the format to the arguments. It returns the text and the number of
// arguments that it used.
func consoleFormat(format string, args []*runtime.RemoteObject) (string, int) {
	var b strings.Builder
	used := 0
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || i+1 == len(format) {
			b.WriteByte(c)
			continue
		}
		verb := format[i+1]
		if verb == '%' {
			b.WriteByte('%')
			i++
			continue
		}
		if !strings.ContainsRune("sdifoOc", rune(verb)) || used == len(args) {
			b.WriteByte(c)
			continue
		}
		a := args[used]
		used++
		i++
		switch verb {
		case 's':
			if a.Type == runtime.RemoteObjectTypeString {
				var s string
				if jsonv2.Unmarshal(a.Value, &s) == nil {
					b.WriteString(s)
					continue
				}
			}
			b.WriteString(consoleValue(a))
		case 'd', 'i':
			b.WriteString(consoleNumber(a, verb == 'i'))
		case 'f':
			b.WriteString(consoleNumber(a, false))
		case 'o', 'O':
			b.WriteString(consoleValue(a))
		}
		// The verb c sets a CSS style. It prints nothing.
	}
	return b.String(), used
}

// consoleNumber renders an argument for %d, %i and %f.
func consoleNumber(a *runtime.RemoteObject, integer bool) string {
	if a.Type != runtime.RemoteObjectTypeNumber {
		return "NaN"
	}
	if a.UnserializableValue != "" {
		return string(a.UnserializableValue)
	}
	f, err := strconv.ParseFloat(string(a.Value), 64)
	if err != nil {
		return "NaN"
	}
	if integer {
		f = float64(int64(f))
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// consoleValue renders one value. A string stays as it is, a primitive has its
// value, and an object has its description, or a short form of its
// properties when the browser sent a preview.
func consoleValue(o *runtime.RemoteObject) string {
	if o == nil {
		return "null"
	}
	switch o.Type {
	case runtime.RemoteObjectTypeString:
		var s string
		if err := jsonv2.Unmarshal(o.Value, &s); err == nil {
			return s
		}
	case runtime.RemoteObjectTypeUndefined:
		return "undefined"
	case runtime.RemoteObjectTypeObject:
		if o.Subtype == runtime.RemoteObjectSubtypeNull {
			return "null"
		}
		if o.Preview != nil && o.Subtype != runtime.RemoteObjectSubtypeError {
			return consolePreview(o.Preview)
		}
	}
	if o.UnserializableValue != "" {
		return string(o.UnserializableValue)
	}
	if o.Description != "" {
		return o.Description
	}
	if len(o.Value) > 0 {
		return string(o.Value)
	}
	return string(o.Type)
}

// consolePreview renders the preview of an object, for example {a: 1, b: "x"}
// or [1, 2, 3]. A property that does not fit ends the list with an ellipsis.
func consolePreview(p *runtime.ObjectPreview) string {
	var items []string
	isArray := p.Subtype == runtime.ObjectPreviewSubtypeArray
	for _, prop := range p.Properties {
		v := consolePropertyValue(prop)
		if isArray {
			items = append(items, v)
		} else {
			items = append(items, prop.Name+": "+v)
		}
	}
	for _, e := range p.Entries {
		v := consoleNested(e.Value)
		if e.Key != nil {
			v = consoleNested(e.Key) + " => " + v
		}
		items = append(items, v)
	}
	if p.Overflow {
		items = append(items, "…")
	}
	body := strings.Join(items, ", ")
	switch {
	case isArray:
		return "[" + body + "]"
	case p.Description == "Object":
		return "{" + body + "}"
	default:
		return p.Description + " {" + body + "}"
	}
}

func consolePropertyValue(prop *runtime.PropertyPreview) string {
	if prop.ValuePreview != nil {
		return consolePreview(prop.ValuePreview)
	}
	if prop.Type == runtime.PropertyPreviewTypeString {
		return strconv.Quote(prop.Value)
	}
	return prop.Value
}

func consoleNested(p *runtime.ObjectPreview) string {
	if p == nil {
		return ""
	}
	if p.Type == runtime.ObjectPreviewTypeString {
		return strconv.Quote(p.Description)
	}
	if p.Type == runtime.ObjectPreviewTypeObject && len(p.Properties) > 0 {
		return consolePreview(p)
	}
	return p.Description
}
