package chromedp

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	jsonv2 "github.com/chromedp/cdproto/cdp/jsonv2"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
)

// exposePrefix is the start of the name of each binding that ExposeFunc adds.
// The name of an exposed func cannot start with it.
const exposePrefix = "__chromedp_"

// ExposeFunc is an action that makes the Go func fn available to the page as
// window.<name>. A page script calls it like an async function. The call
// returns a promise that the browser settles with the result of fn, or rejects
// with an Error when fn returns an error. It is like exposeFunction in
// Puppeteer.
//
// The function is there in the current document and in every document that the
// target loads later, for the main frame and for every iframe of the same
// site, so it survives a navigation. An iframe of another site runs in another
// process and is another target, so the function is not there. It stays until
// the target ends. The listener for the calls
// stops when the context of the target ends. A second call of ExposeFunc with
// the same name on the same target returns an error. So does a name that is
// empty, that is not valid UTF-8, or that starts with "__chromedp_", the prefix
// of the private names of the package.
//
// A script passes its arguments as JSON, so a value such as a function or a
// DOM node does not work. How the arguments reach fn depends on the type A:
//
//   - When A is a slice type, such as []any or []float64, fn gets all the
//     arguments of the call in the slice. Use it for a call with several
//     arguments, or with none.
//   - For every other A, the call must have one argument, and the action
//     decodes it into A. Use a struct for an object, or a basic type. A call
//     with no argument gives the zero value of A, and a call with more than one
//     argument is an error that rejects the promise.
//
// The action encodes the result of fn as JSON for the page. When R is [Void],
// the promise resolves with undefined.
//
// The action runs fn in a new goroutine for each call, so fn can run several
// times at the same time and must be safe for that. The ctx of fn has the
// values of the context that ExposeFunc received, and it ends when the target
// ends. If fn panics, the promise rejects with an Error whose message is "panic:"
// and the panic value, for example "panic: boom". The message of an error that
// fn returns replaces each byte that is not valid UTF-8 with U+FFFD.
//
// For example, a page that calls window.square(7):
//
//	err := chromedp.Do(ctx,
//		chromedp.ExposeFunc("square", func(ctx context.Context, n float64) (float64, error) {
//			return n * n, nil
//		}),
//		chromedp.Navigate(url),
//	)
func ExposeFunc[A, R any](name string, fn func(ctx context.Context, arg A) (R, error)) Action[Void] {
	return Func(func(ctx context.Context, t *Target) (err error) {
		if name == "" {
			return errors.New("exposing a func: the name is empty")
		}
		if strings.HasPrefix(name, exposePrefix) {
			return fmt.Errorf("exposing func %q: the name starts with %q", name, exposePrefix)
		}
		jsName, err := jsonv2.Marshal(name)
		if err != nil {
			return fmt.Errorf("exposing func %q: the name is not valid: %w", name, err)
		}
		t.exposedMu.Lock()
		if _, ok := t.exposed[name]; ok {
			t.exposedMu.Unlock()
			return fmt.Errorf("exposing func %q: the name is in use already", name)
		}
		if t.exposed == nil {
			t.exposed = make(map[string]struct{})
		}
		t.exposed[name] = struct{}{}
		t.exposedSeq++
		// The binding has a number and not the name, so that no two names make
		// the same binding or the same reply func.
		binding := exposePrefix + strconv.Itoa(t.exposedSeq)
		t.exposedMu.Unlock()
		defer func() {
			if err != nil {
				t.exposedMu.Lock()
				delete(t.exposed, name)
				t.exposedMu.Unlock()
			}
		}()

		// The page script calls a private binding. The wrapper script defines
		// window[name] on top of it.
		jsBinding, err := jsonv2.Marshal(binding)
		if err != nil {
			return fmt.Errorf("exposing func %q: %w", name, err)
		}
		script := "(" + exposeFuncJS + ")(" + string(jsName) + "," + string(jsBinding) + ")"

		// Subscribe first, so that no call is lost.
		lctx, stop := context.WithCancel(context.WithoutCancel(ctx))
		calls, unsubscribe := t.Subscribe(runtime.BindingCalled.Method)
		var undo []func(context.Context)
		defer func() {
			if err != nil {
				unsubscribe()
				stop()
				// Take back the binding and the script, so that a retry starts
				// clean. The context of the call can be the reason for the
				// failure, so the cleanup has its own context.
				cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()
				for i := len(undo) - 1; i >= 0; i-- {
					undo[i](cctx)
				}
			}
		}()

		if _, err := cdp.Call(ctx, t, runtime.AddBinding, runtime.AddBindingParams{Name: binding}); err != nil {
			return fmt.Errorf("exposing func %q: %w", name, err)
		}
		undo = append(undo, func(ctx context.Context) {
			_, _ = cdp.Call(ctx, t, runtime.RemoveBinding, runtime.RemoveBindingParams{Name: binding})
		})
		if !t.isWorker {
			// Run the script in each new document. Where the browser knows
			// the flag, it runs the script in the open documents, such as an
			// iframe that is there already.
			added, err := cdp.Call(ctx, t, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{
				Source:         script,
				RunImmediately: ptr(true),
			})
			if err != nil {
				return fmt.Errorf("exposing func %q: %w", name, err)
			}
			undo = append(undo, func(ctx context.Context) {
				_, _ = cdp.Call(ctx, t, page.RemoveScriptToEvaluateOnNewDocument, page.RemoveScriptToEvaluateOnNewDocumentParams{Identifier: added.Identifier})
			})
		}
		// The main document is open already. The script does nothing the second
		// time that it runs in a document.
		res, err := cdp.Call(ctx, t, runtime.Evaluate, runtime.EvaluateParams{Expression: script})
		if err != nil {
			return fmt.Errorf("exposing func %q: %w", name, err)
		}
		if res.ExceptionDetails != nil {
			return fmt.Errorf("exposing func %q: %w", name, &ExceptionError{res.ExceptionDetails})
		}

		go func() {
			// The channel closes when the target ends.
			defer unsubscribe()
			defer stop()
			for raw := range calls {
				var ev runtime.EventBindingCalled
				if err := jsonv2.Unmarshal(raw, &ev, DefaultUnmarshalOptions); err != nil || ev.Name != binding {
					continue
				}
				go exposedCall(lctx, t, binding, &ev, fn)
			}
		}()
		return nil
	})
}

// exposedCall runs one call of an exposed func, and sends the result back to
// the context of the page that made the call.
func exposedCall[A, R any](ctx context.Context, t *Target, binding string, ev *runtime.EventBindingCalled, fn func(context.Context, A) (R, error)) {
	var call struct {
		ID   int64          `json:"id"`
		Args []jsonv2.Value `json:"args"`
	}
	if err := jsonv2.Unmarshal([]byte(ev.Payload), &call, DefaultUnmarshalOptions); err != nil {
		// Without the id the promise cannot settle, and the payload did not come
		// from the wrapper.
		return
	}

	res, err := func() (res R, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		arg, err := exposedArg[A](call.Args)
		if err != nil {
			return res, err
		}
		return fn(ctx, arg)
	}()

	ok, value := "true", "undefined"
	if err != nil {
		ok = "false"
		b, merr := jsonv2.Marshal(strings.ToValidUTF8(err.Error(), "\uFFFD"))
		if merr != nil {
			b = []byte(`"the error message is not valid"`)
		}
		value = string(b)
	} else if reflect.TypeFor[R]() != reflect.TypeFor[Void]() {
		b, merr := jsonv2.Marshal(res, DefaultMarshalOptions)
		if merr != nil {
			ok = "false"
			b, _ = jsonv2.Marshal(fmt.Sprintf("encoding the result: %v", merr))
		}
		value = string(b)
	}
	reply, merr := jsonv2.Marshal(binding + "_reply")
	if merr != nil {
		return
	}
	expr := "window[" + string(reply) + "](" + strconv.FormatInt(call.ID, 10) + "," + ok + "," + value + ")"
	// The page can be gone, for example after a navigation. Then nobody waits
	// for the result, and the error does not matter.
	_, _ = cdp.Call(ctx, t, runtime.Evaluate, runtime.EvaluateParams{
		Expression: expr,
		ContextID:  ev.ExecutionContextID,
		Silent:     ptr(true),
	})
}

// exposedArg decodes the arguments of a call into the type A.
func exposedArg[A any](args []jsonv2.Value) (A, error) {
	var arg A
	if reflect.TypeFor[A]().Kind() == reflect.Slice {
		b, err := jsonv2.Marshal(args)
		if err != nil {
			return arg, err
		}
		if err := jsonv2.Unmarshal(b, &arg, DefaultUnmarshalOptions); err != nil {
			return arg, fmt.Errorf("decoding the arguments: %w", err)
		}
		return arg, nil
	}
	switch len(args) {
	case 0:
		return arg, nil
	case 1:
		if err := jsonv2.Unmarshal(args[0], &arg, DefaultUnmarshalOptions); err != nil {
			return arg, fmt.Errorf("decoding the argument: %w", err)
		}
		return arg, nil
	}
	return arg, fmt.Errorf("expected one argument, got %d", len(args))
}
