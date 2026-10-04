package chromedp

import (
	"context"

	jsonv2 "encoding/json/v2"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
)

// Call runs the protocol command with the params on the target of the chromedp
// context, and returns its result. As [Run] does, it starts the browser and
// opens the target when the context has none yet.
//
// Use it where only the context is at hand, for example in code that runs
// under [Legacy]. An [Action] receives its target, and calls [cdp.Call] with
// it:
//
//	res, err := chromedp.Call(ctx, page.Navigate, page.NavigateParams{URL: urlstr})
//
// It returns [ErrInvalidContext] when ctx is not a chromedp context.
func Call[P, R any](ctx context.Context, cmd cdp.Command[P, R], params P) (R, error) {
	c, err := initContextTarget(ctx)
	if err != nil {
		var zero R
		return zero, FromContext(ctx).withExitError(err)
	}
	v, err := cdp.Call(ctx, c.Target, cmd, params)
	return v, c.withExitError(err)
}

// CallBrowser runs the protocol command with the params on the browser of the
// chromedp context, and returns its result. Use it for the commands of the
// browser and target domains. As [Run] does, it starts the browser when the
// context has none yet.
//
// It returns [ErrInvalidContext] when ctx is not a chromedp context.
func CallBrowser[P, R any](ctx context.Context, cmd cdp.Command[P, R], params P) (R, error) {
	c, err := initContextBrowser(ctx)
	if err != nil {
		var zero R
		return zero, FromContext(ctx).withExitError(err)
	}
	v, err := cdp.Call(ctx, c.Browser, cmd, params)
	return v, c.withExitError(err)
}

// CallFunctionOn is an action that calls a JavaScript function and decodes the
// result of the function into the type T.
//
// T is handled as in [Evaluate]. A function that returns null gives
// [ErrJSNull] when T cannot be nil.
//
// Do not set these fields of runtime.CallFunctionOnParams:
//   - ReturnByValue: CallFunctionOn sets it from the type T.
//   - Arguments: pass the arguments with args instead.
//
// If the function throws an exception, CallFunctionOn returns it as an error.
func CallFunctionOn[T any](functionDeclaration string, opt CallOption, args ...any) Action[T] {
	return func(ctx context.Context, t *Target) (T, error) {
		res, _, err := callFunctionOn[T](ctx, t, functionDeclaration, opt, args...)
		return res, err
	}
}

// callFunctionOn calls the function, and returns the decoded result and the
// remote object of the result.
func callFunctionOn[T any](ctx context.Context, t *Target, functionDeclaration string, opt CallOption, args ...any) (T, *runtime.RemoteObject, error) {
	var zero T

	// set up parameters
	p := &runtime.CallFunctionOnParams{
		FunctionDeclaration: functionDeclaration,
		Silent:              new(true),
	}
	if !wantsRemoteObject[T]() {
		p.ReturnByValue = new(true)
	}

	// apply opt
	if opt != nil {
		opt(p)
	}

	// arguments
	if len(args) > 0 {
		ea := &errAppender{args: make([]*runtime.CallArgument, 0, len(args))}
		for _, arg := range args {
			ea.append(arg)
		}
		if ea.err != nil {
			return zero, nil, ea.err
		}
		p.Arguments = ea.args
	}

	// call
	r, err := cdp.Call(ctx, t, runtime.CallFunctionOn, *p)
	if err != nil {
		return zero, nil, err
	}
	if r.ExceptionDetails != nil {
		return zero, nil, &ExceptionError{r.ExceptionDetails}
	}

	res, err := parseRemoteObject[T](r.Result)
	return res, r.Result, err
}

// CallOption is a func that changes the runtime.CallFunctionOnParams of a
// call.
type CallOption = func(params *runtime.CallFunctionOnParams)

// errAppender collects the arguments of a call and keeps the first error, so
// that the caller makes one error check. See
// https://blog.golang.org/errors-are-values.
type errAppender struct {
	args []*runtime.CallArgument
	err  error
}

// append calls the jsonv2.Marshal func on the value and appends the result to
// the slice. It records the first error. After an error, append does nothing
// and the error stays.
func (ea *errAppender) append(v any) {
	if ea.err != nil {
		return
	}
	var b []byte
	b, ea.err = jsonv2.Marshal(v, DefaultMarshalOptions)
	ea.args = append(ea.args, &runtime.CallArgument{Value: b})
}
