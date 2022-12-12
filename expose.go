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

// ExposeFunc The method adds a function called name on the browser page's window object.
// When called, the function executes BindingFunc in go env
// and returns a Promise which resolves to the return value of BindingFunc.
// Note. compared with puppeteer's exposeFunction.
// the BindingFunc takes exactly one argument, this argument should be string
func ExposeFunc(ctx context.Context, fnName string, fn BindingFunc) error {
	c := FromContext(ctx)
	if c == nil {
		return ErrInvalidContext
	}
	if c.Target == nil {
		if err := c.newTarget(ctx); err != nil {
			return err
		}
	}

	c.Target.bindingFuncListenOnce.Do(func() {
		c.Target.bindingFuncs = make(map[string]BindingFunc)

		err := Run(ctx, ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(exposeJS).Do(ctx)
			return err
		}))
		if err != nil {
			return
		}

		ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *runtime.EventBindingCalled:
				var payload BindingCalledPayload

				err := json.Unmarshal([]byte(ev.Payload), &payload)
				if err != nil {
					return
				}

				if payload.Type != "exposedFun" {
					return
				}

				c.Target.bindingFuncMu.RLock()
				defer c.Target.bindingFuncMu.RUnlock()

				result := "bindingCall name not exsit"
				callFnName := deliverError

				if fn, ok := c.Target.bindingFuncs[payload.Name]; ok {
					result, err = fn(payload.Args)
					if err != nil {
						result = err.Error()
					} else {
						callFnName = deliverResult
					}
				}

				go func() {
					Run(ctx, CallFunctionOn(callFnName, nil, func(p *runtime.CallFunctionOnParams) *runtime.CallFunctionOnParams {
						return p.WithExecutionContextID(ev.ExecutionContextID)
					}, payload.Name, payload.Seq, result))
				}()

			}
		})
	})

	c.Target.bindingFuncMu.Lock()
	if _, ok := c.Target.bindingFuncs[fnName]; ok {
		c.Target.bindingFuncMu.Unlock()
		return ErrExposeNameExist
	}
	c.Target.bindingFuncs[fnName] = fn
	c.Target.bindingFuncMu.Unlock()

	err := Run(ctx, runtime.AddBinding(fnName))
	if err != nil {
		return err
	}

	expression := fmt.Sprintf(`%s("%s","%s");`, addTargetBinding, "exposedFun", fnName)
	err = Run(ctx, ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(expression).Do(ctx)
		return err
	}))
	if err != nil {
		return err
	}
	return nil
}
