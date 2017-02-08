package chromedp

import (
	"context"
	"encoding/json"

	"github.com/knq/chromedp/cdp"
	rundom "github.com/knq/chromedp/cdp/runtime"
)

// Evaluate evaluates the Javascript expression, unmarshaling the result of the
// script evaluation to res.
//
// If res is *[]byte, then the result of the script evaluation will be returned
// "by value" (ie, JSON-encoded) and res will be set to the raw value.
//
// Alternatively, if res is **chromedp/cdp/runtime.RemoteObject, then it will
// be set to the returned RemoteObject and no attempt will be made to convert
// the value to an equivalent Go type.
//
// Otherwise, if res is any other Go type, the result of the script evaluation
// will be returned "by value" (ie, JSON-encoded), and subsequently will be
// json.Unmarshal'd into res.
//
// Note: any exception encountered will be returned as an error.
func Evaluate(expression string, res interface{}, opts ...EvaluateOption) Action {
	if res == nil {
		panic("res cannot be nil")
	}

	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		var err error

		// set up parameters
		p := rundom.Evaluate(expression)
		switch res.(type) {
		case **rundom.RemoteObject:
		default:
			p = p.WithReturnByValue(true)
		}

		// apply opts
		for _, o := range opts {
			p = o(p)
		}

		// evaluate
		v, exp, err := p.Do(ctxt, h)
		if err != nil {
			return err
		}
		if exp != nil {
			return exp
		}

		switch x := res.(type) {
		case **rundom.RemoteObject:
			*x = v
			return nil

		case *[]byte:
			*x = []byte(v.Value)
			return nil
		}

		// unmarshal
		return json.Unmarshal(v.Value, res)
	})
}

// EvaluateAsDevTools is an action that evaluates a Javascript expression in
// the same context as Chrome DevTools would, exposing the Command Line API to
// the script evaluating the expression in the "console" context.
//
// Note: this should not be used with any untrusted code.
func EvaluateAsDevTools(expression string, res interface{}, opts ...EvaluateOption) Action {
	return Evaluate(expression, res, append(opts, EvalObjectGroup("console"), EvalWithCommandLineAPI)...)
}

// EvaluateOption is an Evaluate call option.
type EvaluateOption func(*rundom.EvaluateParams) *rundom.EvaluateParams

// EvalObjectGroup is a evaluate option to set the object group.
func EvalObjectGroup(objectGroup string) EvaluateOption {
	return func(p *rundom.EvaluateParams) *rundom.EvaluateParams {
		return p.WithObjectGroup(objectGroup)
	}
}

// EvalWithCommandLineAPI is an evaluate option to make the DevTools Command
// Line API available to the evaluated script.
//
// Note: this should not be used with any untrusted code.
func EvalWithCommandLineAPI(p *rundom.EvaluateParams) *rundom.EvaluateParams {
	return p.WithIncludeCommandLineAPI(true)
}

// EvalSilent is a evaluate option that will cause script evaluation to ignore
// exceptions.
func EvalSilent(p *rundom.EvaluateParams) *rundom.EvaluateParams {
	return p.WithSilent(true)
}

// EvalAsValue is a evaluate option that will case the script to encode its
// result as a value.
func EvalAsValue(p *rundom.EvaluateParams) *rundom.EvaluateParams {
	return p.WithReturnByValue(true)
}
