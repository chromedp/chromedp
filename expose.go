package chromedp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
)

// ExposedFunc is the function type that can be exposed to the browser env.
type ExposedFunc func(args string) (string, error)

// ExposeAction are actions which expose Go functions to the browser env.
type ExposeAction Action

// Expose is an action to add a function called fnName on the browser page's
// window object. When called, the function executes fn in the Go env and
// returns a Promise which resolves to the return value of fn.
//
// Note:
// 1. This is the lite version of puppeteer's [page.exposeFunction].
// 2. It adds "chromedpExposeFunc" to the page's window object too.
// 3. The exposed function survives page navigation until the tab is closed?
// 4. (iframe?)
// 5. Avoid exposing multiple funcs with the same name.
// 6. Maybe you just need runtime.AddBinding.
//
// [page.exposeFunction]: https://github.com/puppeteer/puppeteer/blob/v19.2.2/docs/api/puppeteer.page.exposefunction.md
func Expose(fnName string, fn ExposedFunc) ExposeAction {
	return ActionFunc(func(ctx context.Context) error {

		expression := fmt.Sprintf(`chromedpExposeFunc.wrapBinding("exposedFun","%s");`, fnName)
		err := Run(ctx,
			runtime.AddBinding(fnName),
			Evaluate(exposeJS, nil),
			Evaluate(expression, nil),
			// Make it effective after navigation.
			addScriptToEvaluateOnNewDocument(exposeJS),
			addScriptToEvaluateOnNewDocument(expression),
		)
		if err != nil {
			return err
		}

		ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *runtime.EventBindingCalled:
				if ev.Payload == "" {
					return
				}

				var payload struct {
					Type string `json:"type"`
					Name string `json:"name"`
					Seq  int64  `json:"seq"`
					Args string `json:"args"`
				}

				err := json.Unmarshal([]byte(ev.Payload), &payload)
				if err != nil {
					return
				}

				if payload.Type != "exposedFun" || payload.Name != fnName {
					return
				}

				result, err := fn(payload.Args)

				callback := "chromedpExposeFunc.deliverResult"
				if err != nil {
					result = err.Error()
					callback = "chromedpExposeFunc.deliverError"
				}

				// Prevent the message from being processed by other functions
				ev.Payload = ""

				go func() {
					err := Run(ctx,
						CallFunctionOn(callback,
							nil,
							func(p *runtime.CallFunctionOnParams) *runtime.CallFunctionOnParams {
								return p.WithExecutionContextID(ev.ExecutionContextID)
							},
							payload.Name,
							payload.Seq,
							result,
						),
					)

					if err != nil {
						c := FromContext(ctx)
						c.Browser.errf("failed to deliver result to exposed func %s: %s", fnName, err)
					}
				}()
			}
		})

		return nil
	})
}

func addScriptToEvaluateOnNewDocument(script string) Action {
	return ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(script).Do(ctx)
		return err
	})
}
