package chromedp

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/chromedp/cdproto/runtime"
)

// EvaluateAction are actions that evaluate JavaScript expressions using
// runtime.Evaluate.
type EvaluateAction Action

// Evaluate is an action to evaluate the JavaScript expression, unmarshaling
// the result of the script evaluation to res.
//
// When res is nil, the script result will be ignored.
//
// When res is a *[]byte, the raw JSON-encoded value of the script
// result will be placed in res.
//
// When res is a **runtime.RemoteObject, res will be set to the low-level
// protocol type, and no attempt will be made to convert the result.
// The original objects could be maintained in memory until the page is
// navigated or closed. `runtime.ReleaseObject` or `runtime.ReleaseObjectGroup`
// can be used to ask the browser to release the original objects.
//
// For all other cases, the result of the script will be returned "by value" (i.e.,
// JSON-encoded), and subsequently an attempt will be made to json.Unmarshal
// the script result to res. When the script result is "undefined" or "null",
// and the value that res points to is not a pointer, or it points to a
// map/ptr/slice/interface/func/chan, it will handle by json.Unmarshal.
// for other type it returns [ErrJSUndefined] of [ErrJSNull] respectively.
func Evaluate(expression string, res interface{}, opts ...EvaluateOption) EvaluateAction {
	return ActionFunc(func(ctx context.Context) error {
		// set up parameters
		p := runtime.Evaluate(expression)
		switch res.(type) {
		case **runtime.RemoteObject:
		default:
			p = p.WithReturnByValue(true)
		}

		// apply opts
		for _, o := range opts {
			p = o(p)
		}

		// evaluate
		v, exp, err := p.Do(ctx)
		if err != nil {
			return err
		}
		if exp != nil {
			return exp
		}

		err = parseRemoteObject(v, res)
		return err
	})
}

func parseRemoteObject(v *runtime.RemoteObject, res interface{}) (err error) {
	if res == nil {
		return
	}

	switch x := res.(type) {
	case **runtime.RemoteObject:
		*x = v
		return

	case *[]byte:
		*x = v.Value
		return
	}

	value := v.Value
	if v.Type == "undefined" || value == nil {
		rv := reflect.ValueOf(res)
		if rv.Kind() == reflect.Pointer {
			rv = rv.Elem()
			switch rv.Kind() {
			case reflect.Map, reflect.Pointer, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
				// map, ptr, slice, func, chan, interface type can be handled correctly by json package
			default:
				// return [ErrJSUndefined] or [ErrJSNull] respectively.
				// the caller maybe need to redefined `res` type
				if v.Type == "undefined" {
					return ErrJSUndefined
				}
				return ErrJSNull
			}
		}
		// `res` is not a pointer, change the value to the json literal null, returns error by json.Unmarshal
		value = []byte("null")
	}

	return json.Unmarshal(value, res)
}

// EvaluateAsDevTools is an action that evaluates a JavaScript expression as
// Chrome DevTools would, evaluating the expression in the "console" context,
// and making the Command Line API available to the script.
//
// See [Evaluate] for more information on how script expressions are evaluated.
//
// Note: this should not be used with untrusted JavaScript.
func EvaluateAsDevTools(expression string, res interface{}, opts ...EvaluateOption) EvaluateAction {
	return Evaluate(expression, res, append(opts, EvalObjectGroup("console"), EvalWithCommandLineAPI)...)
}

// EvaluateOption is the type for JavaScript evaluation options.
type EvaluateOption = func(*runtime.EvaluateParams) *runtime.EvaluateParams

// EvalObjectGroup is an evaluate option to set the object group.
func EvalObjectGroup(objectGroup string) EvaluateOption {
	return func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithObjectGroup(objectGroup)
	}
}

// EvalWithCommandLineAPI is an evaluate option to make the DevTools Command
// Line API available to the evaluated script.
//
// See [Evaluate] for more information on how evaluate actions work.
//
// Note: this should not be used with untrusted JavaScript.
func EvalWithCommandLineAPI(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithIncludeCommandLineAPI(true)
}

// EvalIgnoreExceptions is an evaluate option that will cause JavaScript
// evaluation to ignore exceptions.
func EvalIgnoreExceptions(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithSilent(true)
}

// EvalAsValue is an evaluate option that will cause the evaluated JavaScript
// expression to encode the result of the expression as a JSON-encoded value.
func EvalAsValue(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithReturnByValue(true)
}
