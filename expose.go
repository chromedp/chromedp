package chromedp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
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
			_, err := page.AddScriptToEvaluateOnNewDocument(exposedFunJS).Do(ctx)
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

				var expression string

				c.Target.bindingFuncMu.RLock()
				defer c.Target.bindingFuncMu.RUnlock()
				if fn, ok := c.Target.bindingFuncs[payload.Name]; ok {
					res, err := fn(payload.Args)
					if err != nil {
						expression = deliverError(payload.Name, payload.Seq, err.Error(), err.Error())
					} else {
						expression = deliverResult(payload.Name, payload.Seq, res)
					}
				} else {
					expression = deliverError(payload.Name, payload.Seq, "bindingCall name not exsit", "")
				}

				go func() {
					Run(ctx, Evaluate(expression, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
						return p.WithContextID(ev.ExecutionContextID)
					}))
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

	expression := addPageBinding("exposedFun", fnName)
	err = Run(ctx, ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(expression).Do(ctx)
		return err
	}))
	if err != nil {
		return err
	}
	return nil
}

const exposedFunJS = `
function deliverError(name, seq, message, stack) {
	const error = new Error(message);
	error.stack = stack;
	window[name].callbacks.get(seq).reject(error);
	window[name].callbacks.delete(seq);
}

function deliverResult(name, seq, result) {
	window[name].callbacks.get(seq).resolve(result);
	window[name].callbacks.delete(seq);
}

function addPageBinding(type, name) {
	// This is the CDP binding.
	const callCDP = self[name];
	console.log("callCDP",callCDP)
	// We replace the CDP binding with a Puppeteer binding.
	Object.assign(self, {
		[name](args) {
			if(typeof args != "string"){
				return Promise.reject(new Error('function takes exactly one argument, this argument should be string'))
			}
			var _a, _b;
			// This is the Puppeteer binding.
			const callPuppeteer = self[name];
			(_a = callPuppeteer.callbacks) !== null && _a !== void 0 ? _a : (callPuppeteer.callbacks = new Map());
			const seq = ((_b = callPuppeteer.lastSeq) !== null && _b !== void 0 ? _b : 0) + 1;
			callPuppeteer.lastSeq = seq;
			callCDP(JSON.stringify({ type, name, seq, args }));
			return new Promise((resolve, reject) => {
				callPuppeteer.callbacks.set(seq, { resolve, reject });
			});
		},
	});
}
`

func deliverError(name string, seq int64, message, stack string) string {
	var cmd string = `deliverError("%s",%d,"%s","%s");`
	return fmt.Sprintf(cmd, name, seq, message, stack)
}

func deliverResult(name string, seq int64, result string) string {
	var cmd string = `deliverResult("%s",%d,"%s");`
	return fmt.Sprintf(cmd, name, seq, result)
}

func addPageBinding(typeS, name string) string {
	var cmd string = `addPageBinding("%s","%s");`
	return fmt.Sprintf(cmd, typeS, name)
}
