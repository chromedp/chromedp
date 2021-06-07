package chromedp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/cdproto/runtime"
)

// EvaluateAction are actions that evaluate Javascript expressions using
// runtime.Evaluate.
type EvaluateAction Action

// Evaluate is an action to evaluate the Javascript expression, unmarshaling
// the result of the script evaluation to res.
//
// When res is nil, the script result will be ignored.
//
// When res is a *[]byte, the raw JSON-encoded value of the script
// result will be placed in res.
//
// When res is a **runtime.RemoteObject, res will be set to the low-level
// protocol type, and no attempt will be made to convert the result.
// Original objects are maintained in memory until the page navigated or closed,
// unless they are either explicitly released or are released along with the
// other objects in their object group. runtime.ReleaseObject or
// runtime.ReleaseObjectGroup can be used to ask the browser to release
// original objects.
//
// For all other cases, the result of the script will be returned "by value" (ie,
// JSON-encoded), and subsequently an attempt will be made to json.Unmarshal
// the script result to res. It returns an error if the script result is
// "undefined" in this case.
//
// Note: any exception encountered will be returned as an error.
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

		return parseRemoteObject(v, res)
	})
}

func parseRemoteObject(v *runtime.RemoteObject, res interface{}) error {
	if res == nil {
		return nil
	}

	switch x := res.(type) {
	case **runtime.RemoteObject:
		*x = v
		return nil

	case *[]byte:
		*x = v.Value
		return nil
	}

	if v.Type == "undefined" {
		// The unmarshal below would fail with the cryptic
		// "unexpected end of JSON input" error, so try to give
		// a better one here.
		return fmt.Errorf("encountered an undefined value")
	}

	// unmarshal
	return json.Unmarshal(v.Value, res)
}

// EvaluateAsDevTools is an action that evaluates a Javascript expression as
// Chrome DevTools would, evaluating the expression in the "console" context,
// and making the Command Line API available to the script.
//
// See Evaluate for more information on how script expressions are evaluated.
//
// Note: this should not be used with untrusted Javascript.
func EvaluateAsDevTools(expression string, res interface{}, opts ...EvaluateOption) EvaluateAction {
	return Evaluate(expression, res, append(opts, EvalObjectGroup("console"), EvalWithCommandLineAPI)...)
}

// EvaluateOption is the type for Javascript evaluation options.
type EvaluateOption = func(*runtime.EvaluateParams) *runtime.EvaluateParams

// EvalObjectGroup is a evaluate option to set the object group.
func EvalObjectGroup(objectGroup string) EvaluateOption {
	return func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithObjectGroup(objectGroup)
	}
}

// EvalWithCommandLineAPI is an evaluate option to make the DevTools Command
// Line API available to the evaluated script.
//
// See Evaluate for more information on how evaluate actions work.
//
// Note: this should not be used with untrusted Javascript.
func EvalWithCommandLineAPI(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithIncludeCommandLineAPI(true)
}

// EvalIgnoreExceptions is a evaluate option that will cause Javascript
// evaluation to ignore exceptions.
func EvalIgnoreExceptions(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithSilent(true)
}

// EvalAsValue is a evaluate option that will cause the evaluated Javascript
// expression to encode the result of the expression as a JSON-encoded value.
func EvalAsValue(p *runtime.EvaluateParams) *runtime.EvaluateParams {
	return p.WithReturnByValue(true)
}
