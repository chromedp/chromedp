package chromedp

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
)

// Evaluate is an action that evaluates the JavaScript expression and decodes
// the result of the script into the type T.
//
// When T is [Void], Evaluate ignores the script result.
//
// When T is []byte, the result is the raw JSON-encoded value of the script
// result.
//
// When T is *runtime.RemoteObject, the result is the low-level protocol type,
// and Evaluate does not convert it. The original objects can stay in memory
// until the page navigates or closes. Use `runtime.ReleaseObject` or
// `runtime.ReleaseObjectGroup` to ask the browser to release them.
//
// For all other types, the script returns the result "by value" (that is,
// JSON-encoded). Evaluate then decodes it into a value of type T. Only a chan,
// func, interface, map, pointer, or slice type can be nil. When the script
// result is "undefined" or "null" and T cannot be nil, the action returns
// [ErrJSUndefined] or [ErrJSNull] respectively. The expression `null` gives
// [ErrJSNull] for an int, and a nil pointer for a *int. The expression
// `Promise.resolve(1)` needs the option [EvalAwaitPromise].
//
// For example:
//
//	n, err := chromedp.Run(ctx, chromedp.Evaluate[int](`1 + 2`))
func Evaluate[T any](expression string, opts ...EvaluateOption) Action[T] {
	return func(ctx context.Context, t *Target) (T, error) {
		var zero T

		// set up parameters
		p := &runtime.EvaluateParams{Expression: expression}
		if !wantsRemoteObject[T]() {
			p.ReturnByValue = ptr(true)
		}

		// apply opts
		for _, o := range opts {
			o(p)
		}

		// evaluate
		r, err := cdp.Call(ctx, t, runtime.Evaluate, *p)
		if err != nil {
			return zero, err
		}
		if r.ExceptionDetails != nil {
			return zero, &ExceptionError{r.ExceptionDetails}
		}

		return parseRemoteObject[T](r.Result)
	}
}

// wantsRemoteObject reports whether the type T is *runtime.RemoteObject.
func wantsRemoteObject[T any]() bool {
	var v T
	_, ok := any(&v).(**runtime.RemoteObject)
	return ok
}

// parseRemoteObject decodes the remote object into a value of the type T.
func parseRemoteObject[T any](v *runtime.RemoteObject) (T, error) {
	var res T
	switch x := any(&res).(type) {
	case *Void:
		return res, nil

	case **runtime.RemoteObject:
		*x = v
		return res, nil

	case *[]byte:
		*x = v.Value
		return res, nil
	}

	value := v.Value
	if value == nil || isJSONNull(value) {
		switch reflect.TypeFor[T]().Kind() {
		// Common kinds that can be nil.
		case reflect.Pointer, reflect.Map, reflect.Slice:
		// These kinds are unusual for T, but they can be nil too.
		case reflect.Chan, reflect.Func, reflect.Interface:
		default:
			// When the value of the type T cannot be nil, return
			// [ErrJSUndefined] or [ErrJSNull] respectively.
			if v.Type == "undefined" {
				return res, ErrJSUndefined
			}
			return res, ErrJSNull
		}
		// Change the value to the JSON literal null, so that json.Unmarshal accepts it.
		value = []byte("null")
	}

	err := json.Unmarshal(value, &res)
	return res, err
}

// isJSONNull reports whether value is the JSON literal null.
func isJSONNull(value []byte) bool {
	return string(bytes.TrimSpace(value)) == "null"
}

// EvaluateAsDevTools is an action that evaluates a JavaScript expression as
// Chrome DevTools does. It evaluates the expression in the "console" context
// and makes the Command Line API available to the script.
//
// See [Evaluate] for how the expressions are evaluated.
//
// Note: do not use this with untrusted JavaScript.
func EvaluateAsDevTools[T any](expression string, opts ...EvaluateOption) Action[T] {
	return Evaluate[T](expression, append(opts[:len(opts):len(opts)], EvalObjectGroup("console"), EvalWithCommandLineAPI)...)
}

// EvaluateOption is the type for JavaScript evaluation options.
type EvaluateOption = func(*runtime.EvaluateParams)

// EvalObjectGroup is an evaluate option to set the object group.
func EvalObjectGroup(objectGroup string) EvaluateOption {
	return func(p *runtime.EvaluateParams) {
		p.ObjectGroup = objectGroup
	}
}

// EvalWithCommandLineAPI is an evaluate option that makes the DevTools Command
// Line API available to the evaluated script.
//
// See [Evaluate] for how evaluate actions work.
//
// Note: do not use this with untrusted JavaScript.
func EvalWithCommandLineAPI(p *runtime.EvaluateParams) {
	p.IncludeCommandLineAPI = ptr(true)
}

// EvalIgnoreExceptions is an evaluate option that makes the evaluation ignore
// exceptions.
func EvalIgnoreExceptions(p *runtime.EvaluateParams) {
	p.Silent = ptr(true)
}

// EvalAwaitPromise is an evaluate option that makes the evaluation wait for a
// promise that the expression returns, and use its result. A promise that is
// rejected makes the action return an [*ExceptionError]. Without the option,
// the result of an expression that returns a promise is the promise object, and
// not its value.
//
//	v, err := chromedp.Run(ctx, chromedp.Evaluate[string](`fetch("/api").then(r => r.text())`, chromedp.EvalAwaitPromise))
func EvalAwaitPromise(p *runtime.EvaluateParams) {
	p.AwaitPromise = ptr(true)
}

// EvalAsValue is an evaluate option that makes the evaluation encode the
// result of the expression as a JSON-encoded value.
func EvalAsValue(p *runtime.EvaluateParams) {
	p.ReturnByValue = ptr(true)
}
