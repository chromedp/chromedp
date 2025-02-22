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
// and the value that res points to can not be nil (only the value of a chan,
// func, interface, map, pointer, or slice can be nil), it returns [ErrJSUndefined]
// or [ErrJSNull] respectively.
func Evaluate(expression string, res any, opts ...EvaluateOption) EvaluateAction {
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

		return parseRemoteObject(v, res)
	})
}

func parseRemoteObject(v *runtime.RemoteObject, res any) (err error) {
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
	if value == nil {
		rv := reflect.ValueOf(res)
		if rv.Kind() == reflect.Ptr {
			switch rv.Elem().Kind() {
			// Common kinds that can be nil.
			case reflect.Ptr, reflect.Map, reflect.Slice:
			// It's weird that res is a pointer to the following kinds,
			// but they can be nil too.
			case reflect.Chan, reflect.Func, reflect.Interface:
			default:
				// When the value that `res` points to can not be set to nil,
				// return [ErrJSUndefined] or [ErrJSNull] respectively.
				if v.Type == "undefined" {
					return ErrJSUndefined
				}
				return ErrJSNull
			}
		}
		// Change the value to the json literal null to make json.Unmarshal happy.
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
func EvaluateAsDevTools(expression string, res any, opts ...EvaluateOption) EvaluateAction {
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
