package chromedp

import (
	"context"
	"errors"
	"iter"
	"slices"
	"strings"
	"testing"
	"time"

	jsonv2 "encoding/json/v2"
	"github.com/chromedp/cdproto/runtime"
)

// collectConsole reads the messages until the message with the text stop.
func collectConsole(t *testing.T, messages iter.Seq2[ConsoleMessage, error], stop string) []ConsoleMessage {
	t.Helper()
	var got []ConsoleMessage
	for m, err := range messages {
		if err != nil {
			t.Fatalf("got error after %d messages: %v", len(got), err)
		}
		got = append(got, m)
		if m.Text == stop {
			return got
		}
	}
	t.Fatalf("the messages ended before %q", stop)
	return nil
}

func TestConsole(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "console.html")
	defer cancel()
	cctx, ccancel := context.WithTimeout(ctx, 30*time.Second)
	defer ccancel()

	// Subscribe, run the actions, then range.
	messages := Console(cctx)
	if err := Do(ctx, Click(ID("run"))); err != nil {
		t.Fatal(err)
	}
	got := collectConsole(t, messages, "done")

	type typeText struct {
		typ  ConsoleType
		text string
	}
	var summary []typeText
	for _, m := range got {
		text := m.Text
		// The text of an error holds its stack, which has file names.
		if first, _, ok := strings.Cut(text, "\n"); ok {
			text = first
		}
		summary = append(summary, typeText{m.Type, text})
	}
	want := []typeText{
		{ConsoleLog, "hello 42 true null undefined"},
		{ConsoleWarning, "careful"},
		{ConsoleInfo, "list has 3 items"},
		{ConsoleDebug, "debug text"},
		{ConsoleError, `failed {code: 7, name: "x"} [1, 2, 3]`},
		{ConsoleLog, "Error: inner"},
		{ConsoleException, "Uncaught Error: boom"},
		{ConsoleException, "Uncaught (in promise) Error: rejected"},
		{ConsoleException, "Uncaught (in promise) plain"},
		{ConsoleLog, "done"},
	}
	if !slices.Equal(summary, want) {
		t.Fatalf("expected messages:\n%v\ngot:\n%v", want, summary)
	}

	// The raw arguments.
	m := got[4]
	if len(m.Args) != 3 {
		t.Fatalf("expected 3 arguments, got: %d", len(m.Args))
	}
	if m.Args[1].Type != runtime.RemoteObjectTypeObject || m.Args[1].Preview == nil {
		t.Errorf("expected an object with a preview, got: %+v", m.Args[1])
	}
	var s string
	if err := jsonv2.Unmarshal(m.Args[0].Value, &s); err != nil || s != "failed" {
		t.Errorf("expected the first argument to be %q, got: %q (%v)", "failed", s, err)
	}

	// The place of a call.
	if !strings.HasSuffix(m.URL, "/console.html") || m.Line != 13 || m.Stack == nil {
		t.Errorf("expected the place of the call on line 14 of console.html (line 13 in the protocol), got: %q %d %v", m.URL, m.Line, m.Stack)
	}
	if m.Time.IsZero() || time.Since(m.Time) > time.Minute {
		t.Errorf("expected a recent time, got: %v", m.Time)
	}
	if m.IsException() || m.Exception != nil {
		t.Errorf("expected a call of the console API, not an exception")
	}

	// An exception.
	for _, i := range []int{6, 7, 8} {
		m := got[i]
		if !m.IsException() || m.Exception == nil || m.Exception.Exception == nil {
			t.Errorf("message %d: expected an exception with its object, got: %+v", i, m)
			continue
		}
		if !strings.HasSuffix(m.URL, "/console.html") || m.Line < 12 {
			t.Errorf("message %d: expected a place in console.html, got: %q %d:%d", i, m.URL, m.Line, m.Column)
		}
		if len(m.Args) != 0 {
			t.Errorf("message %d: expected no arguments, got: %d", i, len(m.Args))
		}
	}
	// The error that a function threw has a stack, and the object of the
	// rejected error has its description.
	if got[6].Stack == nil || len(got[6].Stack.CallFrames) == 0 {
		t.Errorf("expected a stack for the thrown error, got: %+v", got[6].Stack)
	}
	if d := got[7].Exception.Exception.Description; !strings.HasPrefix(d, "Error: rejected") {
		t.Errorf("expected the description of the rejected error, got: %q", d)
	}
}

func TestConsoleBrowserLog(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "console.html")
	defer cancel()
	cctx, ccancel := context.WithTimeout(ctx, 30*time.Second)
	defer ccancel()

	messages := Console(cctx)
	if err := Do(ctx, Click(ID("missing"))); err != nil {
		t.Fatal(err)
	}
	for m, err := range messages {
		if err != nil {
			t.Fatal(err)
		}
		if m.Source != "network" {
			continue
		}
		if m.Type != ConsoleError || !strings.Contains(m.Text, "missing-image.png") && !strings.Contains(m.URL, "missing-image.png") {
			t.Fatalf("expected a network error about the image, got: %+v", m)
		}
		return
	}
	t.Fatal("the messages ended before the network error")
}

func TestConsoleSeveralReaders(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "console.html")
	defer cancel()
	cctx, ccancel := context.WithTimeout(ctx, 30*time.Second)
	defer ccancel()

	first, second := Console(cctx), Console(cctx)
	if err := Do(ctx, Click(ID("run"))); err != nil {
		t.Fatal(err)
	}
	a := collectConsole(t, first, "done")
	b := collectConsole(t, second, "done")
	if len(a) != len(b) || len(a) != 10 {
		t.Fatalf("expected 10 messages from each reader, got: %d and %d", len(a), len(b))
	}
}

func TestConsoleEndsWithContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := testAllocate(t, "console.html")
	defer cancel()
	cctx, ccancel := context.WithCancel(ctx)

	messages := Console(cctx)
	time.AfterFunc(100*time.Millisecond, ccancel)
	for _, err := range messages {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected the error of the context, got: %v", err)
		}
	}
}

func TestConsoleText(t *testing.T) {
	t.Parallel()

	str := func(s string) *runtime.RemoteObject {
		b, _ := jsonv2.Marshal(s)
		return &runtime.RemoteObject{Type: runtime.RemoteObjectTypeString, Value: b}
	}
	num := func(s string) *runtime.RemoteObject {
		return &runtime.RemoteObject{Type: runtime.RemoteObjectTypeNumber, Value: []byte(s), Description: s}
	}
	obj := &runtime.RemoteObject{
		Type:        runtime.RemoteObjectTypeObject,
		Description: "Object",
		Preview: &runtime.ObjectPreview{
			Type:        runtime.ObjectPreviewTypeObject,
			Description: "Object",
			Overflow:    true,
			Properties: []*runtime.PropertyPreview{
				{Name: "a", Type: runtime.PropertyPreviewTypeNumber, Value: "1"},
				{Name: "s", Type: runtime.PropertyPreviewTypeString, Value: "x"},
				{Name: "n", Type: runtime.PropertyPreviewTypeObject, ValuePreview: &runtime.ObjectPreview{
					Type: runtime.ObjectPreviewTypeObject, Subtype: runtime.ObjectPreviewSubtypeArray, Description: "Array(2)",
					Properties: []*runtime.PropertyPreview{
						{Name: "0", Type: runtime.PropertyPreviewTypeNumber, Value: "1"},
						{Name: "1", Type: runtime.PropertyPreviewTypeNumber, Value: "2"},
					},
				}},
			},
		},
	}
	fn := &runtime.RemoteObject{Type: runtime.RemoteObjectTypeFunction, Description: "function f() {}"}
	null := &runtime.RemoteObject{Type: runtime.RemoteObjectTypeObject, Subtype: runtime.RemoteObjectSubtypeNull}
	nan := &runtime.RemoteObject{Type: runtime.RemoteObjectTypeNumber, UnserializableValue: "NaN", Description: "NaN"}

	tests := []struct {
		name string
		args []*runtime.RemoteObject
		want string
	}{
		{"none", nil, ""},
		{"strings", []*runtime.RemoteObject{str("a"), str("b")}, "a b"},
		{"values", []*runtime.RemoteObject{num("1.5"), nan, null, fn}, "1.5 NaN null function f() {}"},
		{"object", []*runtime.RemoteObject{str("x"), obj}, `x {a: 1, s: "x", n: [1, 2], …}`},
		{"string verb", []*runtime.RemoteObject{str("%s!"), str("hi")}, "hi!"},
		{"number verbs", []*runtime.RemoteObject{str("%d %i %f"), num("3.7"), num("3.7"), num("3.7")}, "3.7 3 3.7"},
		{"percent", []*runtime.RemoteObject{str("100%% sure %s"), str("yes")}, "100% sure yes"},
		{"style", []*runtime.RemoteObject{str("%cbold"), str("font-weight: bold")}, "bold"},
		{"extra argument", []*runtime.RemoteObject{str("%s"), str("a"), num("2")}, "a 2"},
		{"missing argument", []*runtime.RemoteObject{str("%s and %s"), str("a")}, "a and %s"},
		{"object verb", []*runtime.RemoteObject{str("%o"), obj}, `{a: 1, s: "x", n: [1, 2], …}`},
		{"no verb", []*runtime.RemoteObject{str("50% off"), num("1")}, "50% off 1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := consoleText(test.args); got != test.want {
				t.Errorf("expected %q, got: %q", test.want, got)
			}
		})
	}
}
