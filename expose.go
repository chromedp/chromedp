package chromedp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
)

const (
	deliverError     = "deliverError"
	deliverResult    = "deliverResult"
	addTargetBinding = "addTargetBinding"
)

// BindingCalledPayload ...
type BindingCalledPayload struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Seq  int64  `json:"seq"`
	Args string `json:"args"`
}

// BindingFunc expose function type
type BindingFunc func(args string) (string, error)

// AddScriptToEvaluateOnNewDocument ...
func AddScriptToEvaluateOnNewDocument(script string) Action {
	return ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(script).Do(ctx)
		return err
	})
}

// ExposeAction are actions which expose local functions to browser env.
type ExposeAction Action

// Expose is an action to add a function called fnName on the browser page's window object.
// When called, the function executes BindingFunc in go env
// and returns a Promise which resolves to the return value of BindingFunc.
// Note. compared with puppeteer's exposeFunction.
// the BindingFunc takes exactly one argument, this argument should be string
// Note. Do not expose the same function name many times, it will only take effect for the first time.
func Expose(fnName string, fn BindingFunc) ExposeAction {
	return ActionFunc(func(ctx context.Context) error {

		// adds binding with the given name on the global objects of all inspected contexts
		err := Run(ctx, runtime.AddBinding(fnName))
		if err != nil {
			return err
		}

		expression := fmt.Sprintf(`%s("%s","%s");`, addTargetBinding, "cdpExposedFun", fnName)

		// inject bindingFunc wrapper into current window
		err = Run(ctx, Evaluate(exposeJS, nil))
		if err != nil {
			return err
		}

		err = Run(ctx, Evaluate(expression, nil))
		if err != nil {
			return err
		}

		// we also want to make it effective after nav url
		// it evaluates given script in every frame upon creation (before loading frame's scripts)
		err = Run(ctx, AddScriptToEvaluateOnNewDocument(exposeJS))
		if err != nil {
			return err
		}

		err = Run(ctx, AddScriptToEvaluateOnNewDocument(expression))
		if err != nil {
			return err
		}

		ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *runtime.EventBindingCalled:
				var payload BindingCalledPayload

				err := json.Unmarshal([]byte(ev.Payload), &payload)
				if err != nil {
					return
				}

				if payload.Type != "cdpExposedFun" {
					return
				}

				if payload.Name == fnName {
					callFnName := deliverResult
					result, err := fn(payload.Args)

					if err != nil {
						result = err.Error()
						callFnName = deliverError
					}

					// Prevent the message from being processed by other functions
					ev.Payload = ""

					go func() {
						Run(ctx, CallFunctionOn(callFnName, nil, func(p *runtime.CallFunctionOnParams) *runtime.CallFunctionOnParams {
							return p.WithExecutionContextID(ev.ExecutionContextID)
						}, payload.Name, payload.Seq, result))
					}()
				}
			}
		})

		return nil
	})
}
