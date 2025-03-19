package chromedp

import (
	"context"

	"github.com/chromedp/cdproto/runtime"
	jsonv2 "github.com/go-json-experiment/json"
)

// CallAction are actions that calls a JavaScript function using
// runtime.CallFunctionOn.
type CallAction Action

// CallFunctionOn is an action to call a JavaScript function, unmarshaling
// the result of the function to res.
//
// The handling of res is the same as that of Evaluate.
//
// Do not call the following methods on runtime.CallFunctionOnParams:
// - WithReturnByValue: it will be set depending on the type of res;
// - WithArguments: pass the arguments with args instead.
//
// Note: any exception encountered will be returned as an error.
func CallFunctionOn(functionDeclaration string, res any, opt CallOption, args ...any) CallAction {
	return ActionFunc(func(ctx context.Context) error {
		_, err := callFunctionOn(ctx, functionDeclaration, res, opt, args...)
		return err
	})
}

func callFunctionOn(ctx context.Context, functionDeclaration string, res any, opt CallOption, args ...any) (*runtime.RemoteObject, error) {
	// set up parameters
	p := runtime.CallFunctionOn(functionDeclaration).
		WithSilent(true)

	switch res.(type) {
	case **runtime.RemoteObject:
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
			return nil, ea.err
		}
		p = p.WithArguments(ea.args)
	}

	// call
	v, exp, err := p.Do(ctx)
	if err != nil {
		return nil, err
	}
	if exp != nil {
		return nil, exp
	}

	return v, parseRemoteObject(v, res)
}

// CallOption is a function to modify the runtime.CallFunctionOnParams to
// provide more information.
type CallOption = func(params *runtime.CallFunctionOnParams) *runtime.CallFunctionOnParams

// errAppender is to help accumulating the arguments and simplifying error checks.
//
// see https://blog.golang.org/errors-are-values
type errAppender struct {
	args []*runtime.CallArgument
	err  error
}

// append method calls the jsonv2.Marshal method to marshal the value and
// appends it to the slice. It records the first error for future reference.
//
// As soon as an error occurs, the append method becomes a no-op but the error
// value is saved.
func (ea *errAppender) append(v any) {
	if ea.err != nil {
		return
	}
	var b []byte
	b, ea.err = jsonv2.Marshal(v, DefaultMarshalOptions)
	ea.args = append(ea.args, &runtime.CallArgument{Value: b})
}
