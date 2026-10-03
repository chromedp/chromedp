package chromedp

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
)

// Evaluate is an action to evaluate the JavaScript expression, and decode the
// result of the script into the type T.
//
// When T is [Void], the script result is ignored.
//
// When T is []byte, the result is the raw JSON-encoded value of the script
// result.
//
// When T is *runtime.RemoteObject, the result is the low-level protocol type,
// and no attempt is made to convert the result. The original objects could be
// maintained in memory until the page is navigated or closed.
// `runtime.ReleaseObject` or `runtime.ReleaseObjectGroup` can be used to ask
// the browser to release the original objects.
//
// For all other types, the result of the script is returned "by value" (i.e.,
// JSON-encoded), and subsequently an attempt is made to decode it into a
// value of type T. When the script result is "undefined" or "null", and T can
// not be nil (only a chan, func, interface, map, pointer, or slice type can
// be nil), the action returns [ErrJSUndefined] or [ErrJSNull] respectively.
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
			p.ReturnByValue = new(true)
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
	if value == nil {
		switch reflect.TypeFor[T]().Kind() {
		// Common kinds that can be nil.
		case reflect.Pointer, reflect.Map, reflect.Slice:
		// It's weird that T is one of the following kinds,
		// but they can be nil too.
		case reflect.Chan, reflect.Func, reflect.Interface:
		default:
			// When the value of the type T can not be nil, return
			// [ErrJSUndefined] or [ErrJSNull] respectively.
			if v.Type == "undefined" {
				return res, ErrJSUndefined
			}
			return res, ErrJSNull
		}
		// Change the value to the json literal null to make json.Unmarshal happy.
		value = []byte("null")
	}

	err := json.Unmarshal(value, &res)
	return res, err
}

// EvaluateAsDevTools is an action that evaluates a JavaScript expression as
// Chrome DevTools would, evaluating the expression in the "console" context,
// and making the Command Line API available to the script.
//
// See [Evaluate] for more information on how script expressions are evaluated.
//
// Note: this should not be used with untrusted JavaScript.
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

// EvalWithCommandLineAPI is an evaluate option to make the DevTools Command
// Line API available to the evaluated script.
//
// See [Evaluate] for more information on how evaluate actions work.
//
// Note: this should not be used with untrusted JavaScript.
func EvalWithCommandLineAPI(p *runtime.EvaluateParams) {
	p.IncludeCommandLineAPI = new(true)
}

// EvalIgnoreExceptions is an evaluate option that will cause JavaScript
// evaluation to ignore exceptions.
func EvalIgnoreExceptions(p *runtime.EvaluateParams) {
	p.Silent = new(true)
}

// EvalAsValue is an evaluate option that will cause the evaluated JavaScript
// expression to encode the result of the expression as a JSON-encoded value.
func EvalAsValue(p *runtime.EvaluateParams) {
	p.ReturnByValue = new(true)
}
