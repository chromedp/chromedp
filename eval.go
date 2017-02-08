package chromedp

import (
	"context"
	"encoding/json"

	"github.com/knq/chromedp/cdp"
	rundom "github.com/knq/chromedp/cdp/runtime"
)

// Evaluate evaluates the supplied Javascript expression, attempting to
// unmarshal the resulting value into res.
//
// If res is a **chromedp/cdp/runtime.RemoteObject, then it will be set to the
// raw, returned RuntimeObject, Otherwise, the result value be json.Unmarshal'd
// to res.
func Evaluate(expression string, res interface{}, opts ...EvaluateOption) Action {
	if res == nil {
		panic("res cannot be nil")
	}

	return ActionFunc(func(ctxt context.Context, h cdp.FrameHandler) error {
		var err error

		// check if we want a 'raw' result
		obj, raw := res.(**rundom.RemoteObject)

		// set up parameters
		p := rundom.Evaluate(expression)
		if !raw {
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

		if raw {
			*obj = v
			return nil
		}

		// unmarshal
		return json.Unmarshal(v.Value, res)
	})
}

// EvaluateOption is an Evaluate call option.
type EvaluateOption func(*rundom.EvaluateParams) *rundom.EvaluateParams

// EvalObjectGroup is a Evaluate option to set the object group.
func EvalObjectGroup(objectGroup string) EvaluateOption {
	return func(p *rundom.EvaluateParams) *rundom.EvaluateParams {
		return p.WithObjectGroup(objectGroup)
	}
}

// EvalWithCommandLineAPI is an Evaluate option to include the DevTools Command
// Line API.
//
// Note: this should not be used with any untrusted code.
func EvalWithCommandLineAPI(p *rundom.EvaluateParams) *rundom.EvaluateParams {
	return p.WithIncludeCommandLineAPI(true)
}

// EvalSilent is a Evaluate option that will cause script evaluation to ignore
// exceptions.
func EvalSilent(p *rundom.EvaluateParams) *rundom.EvaluateParams {
	return p.WithSilent(true)
}

// EvalAsValue is a Evaluate option that will case the script to encode its
// result as a value.
func EvalAsValue(p *rundom.EvaluateParams) *rundom.EvaluateParams {
	return p.WithReturnByValue(true)
}

// EvaluateAsDevTools evaluates a Javascript expression in the same
//
// Note: this should not be used with any untrusted code.
func EvaluateAsDevTools(expression string) Action {
	return Evaluate(expression, EvalObjectGroup("console"), EvalWithCommandLineAPI)
}
