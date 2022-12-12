package chromedp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/runtime"
)

func TestExposeFunc(t *testing.T) {

	tests := []struct {
		name   string
		Func   BindingFunc
		param  string
		result string
	}{
		{
			name: "func1",
			Func: func(param string) (string, error) {
				return param + param, nil
			},
			param:  "param1",
			result: "param1param1",
		},
		{
			name: "func2",
			Func: func(param string) (string, error) {
				return strings.ToUpper(param), nil
			},
			param:  "param2",
			result: "PARAM2",
		},
	}

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	for _, test := range tests {
		err := ExposeFunc(ctx, test.name, test.Func)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := Run(ctx,
		Navigate(testdataDir+"/expose.html"),
	); err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		var res []byte

		cmd := fmt.Sprintf(`%s("%s");`, test.name, test.param)
		err := Run(ctx, Evaluate(cmd, &res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}))

		if err != nil {
			t.Fatal(err)
		}
		// When res is a *[]byte, the raw JSON-encoded value of the script result will be placed in res.
		data, _ := json.Marshal(test.result)
		if string(data) != string(res) {
			t.Fatalf("want result: %s, got : %s", string(data), string(res))
		}
	}
}
