package chromedp

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/chromedp/cdproto/runtime"
)

func md5SumFunc(args string) (string, error) {
	h := md5.New()
	h.Write([]byte(args))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func base64EncodeFunc(args string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(testString)), nil
}

func echoFunc(str string) (string, error) {
	return str, nil
}

const testString = "chromedp expose test"
const testStringMd5 = "a93d69002a286b46c8aa114362afb7ac"
const testStringBase64 = "Y2hyb21lZHAgZXhwb3NlIHRlc3Q="
const testIFrameHTMLTitle = "page with an iframe"
const testFormHTMLTitle = "this is form title"

func TestExposeToAllFrames(t *testing.T) {
	// allocate browser
	ctx, cancel := testAllocate(t, "iframe.html")
	defer cancel()

	// expose echoFunc function as  to browser current page and every frame
	if err := Run(ctx, Expose("echo", echoFunc)); err != nil {
		t.Fatal(err)
	}

	c := FromContext(ctx)

	c.Target.frameMu.RLock()
	executionContextIDs := make([]runtime.ExecutionContextID, 0, len(c.Target.execContexts))
	for _, executionContextID := range c.Target.execContexts {
		executionContextIDs = append(executionContextIDs, executionContextID)
	}
	c.Target.frameMu.RUnlock()

	var res1 string
	var res2 string
	callEchoFunc := fmt.Sprintf(`%s(document.title);`, "echo")
	for _, executionContextID := range executionContextIDs {
		id := executionContextID
		var res string
		if err := Run(ctx, Evaluate(callEchoFunc, &res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithContextID(id).WithAwaitPromise(true)
		})); err != nil {
			t.Fatal(err)
		}
		if len(res1) == 0 {
			res1 = res
		} else {
			res2 = res
		}
	}

	// we expect res1 or res2 = testIFrameHTMLTitle or testFormHTMLTitle
	if res1 == testIFrameHTMLTitle && res2 == testFormHTMLTitle || res1 == testFormHTMLTitle && res2 == testIFrameHTMLTitle {
		// pass
	} else {
		t.Fatalf("res1: %s, res2: %s", res1, res2)
	}
}

func TestExpose(t *testing.T) {
	// allocate browser
	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// expose md5SumFunc function as md5 to browser current page and every frame
	if err := Run(ctx, Expose("md5", md5SumFunc)); err != nil {
		t.Fatal(err)
	}

	// expose base64EncodeFunc function as base64 to browser current page and every frame
	if err := Run(ctx, Expose("base64", base64EncodeFunc)); err != nil {
		t.Fatal(err)
	}

	// 1. When on the current page
	var res string
	callMd5 := fmt.Sprintf(`%s("%s");`, "md5", testString)
	if err := Run(ctx, Evaluate(callMd5, &res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		t.Fatal(err)
	}

	if res != testStringMd5 {
		t.Fatalf("want: %s, got: %s", testStringMd5, res)
	}

	var res2 string
	callBase64 := fmt.Sprintf(`%s("%s");`, "base64", testString)
	if err := Run(ctx, Evaluate(callBase64, &res2, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		t.Fatal(err)
	}
	if res2 != testStringBase64 {
		t.Fatalf("want: %s, got: %s", testStringBase64, res)
	}

	// 2. Navigate another page
	if err := Run(ctx,
		Navigate(testdataDir+"/child1.html"),
	); err != nil {
		t.Fatal(err)
	}

	// we expect md5 can work properly.
	if err := Run(ctx, Evaluate(callMd5, &res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		t.Fatal(err)
	}
	if res != testStringMd5 {
		t.Fatalf("want: %s, got: %s", testStringMd5, res)
	}

	// we expect base64 can work properly.
	if err := Run(ctx, Evaluate(callBase64, &res2, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		t.Fatal(err)
	}
	if res2 != testStringBase64 {
		t.Fatalf("want: %s, got: %s", testStringBase64, res)
	}
}

func TestExposeMulti(t *testing.T) {
	// allocate browser
	ctx, cancel := testAllocate(t, "")
	defer cancel()

	// creates a new page.  about:blank
	Run(ctx)

	// expose md5SumFunc function as sameFunc to browser current page and every frame
	if err := Run(ctx, Expose("sameFunc", md5SumFunc)); err != nil {
		t.Fatal(err)
	}

	// expose base64EncodeFunc function as sameFunc to browser current page and every frame
	if err := Run(ctx, Expose("sameFunc", base64EncodeFunc)); err != nil {
		t.Fatal(err)
	}

	// we expect first expose function to handle
	var res string
	sameFunc := fmt.Sprintf(`%s("%s");`, "sameFunc", testString)
	if err := Run(ctx, Evaluate(sameFunc, &res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		t.Fatal(err)
	}

	if res != testStringMd5 {
		t.Fatalf("want md5SumFunc res:%s, got:%s", testStringMd5, res)
	}
	if res == testStringBase64 {
		t.Fatalf("want md5SumFunc res:%s, got base64EncodeFunc res :%s", testStringMd5, res)
	}
}
