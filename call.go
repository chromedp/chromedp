package chromedp

import (
	"context"

	"github.com/chromedp/cdproto/runtime"
)

// CallAction are actions that calls a Javascript function using
// runtime.CallFunctionOn.
type CallAction Action

// CallFunctionOn is an action to call a Javascript function, unmarshaling
// the result of the function to res.
//
// The handling of res is the same as that of Evaluate.
//
// Do not call the following methods on runtime.CallFunctionOnParams:
// - WithReturnByValue: it will be set depending on the type of res;
// - WithArguments: pass the arguments with args instead.
//
// Note: any exception encountered will be returned as an error.
func CallFunctionOn(functionDeclaration string, res interface{}, opt CallOption, args ...interface{}) CallAction {
	return ActionFunc(func(ctx context.Context) error {
		// set up parameters
		p := runtime.CallFunctionOn(functionDeclaration).
			WithSilent(true)

		switch res.(type) {
		case nil, **runtime.RemoteObject:
		default:
			p = p.WithReturnByValue(true)
		}

		// apply opt
		if opt != nil {
			p = opt(p)
		}

		// arguments
		if len(args) > 0 {
			ea := &errAppender{args: make([]*runtime.CallArgument, 0, len(args))}
			for _, arg := range args {
				ea.append(arg)
			}
			if ea.err != nil {
				return ea.err
			}
			p = p.WithArguments(ea.args)
		}

		// call
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

// CallOption is a function to modify the runtime.CallFunctionOnParams
// to provide more information.
type CallOption = func(params *runtime.CallFunctionOnParams) *runtime.CallFunctionOnParams
