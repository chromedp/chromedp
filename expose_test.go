package chromedp

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/chromedp/cdproto/runtime"
)

func TestExposeFunc(t *testing.T) {

	ctx, cancel := testAllocate(t, "")
	defer cancel()

	if err := ExposeFunc(ctx, "md5", func(args string) (string, error) {
		h := md5.New()
		h.Write([]byte(args))
		return hex.EncodeToString(h.Sum(nil)), nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := Run(ctx,
		Navigate(testdataDir+"/expose.html"),
	); err != nil {
		t.Fatal(err)
	}

	var res string
	cmd := fmt.Sprintf(`%s("%s");`, "md5", "chromedp")
	if err := Run(ctx, Evaluate(cmd, &res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		t.Fatal(err)
	}

	h := md5.New()
	h.Write([]byte("chromedp"))
	md5Str := hex.EncodeToString(h.Sum(nil))

	if res != md5Str {
		t.Fatalf("want: %s, got: %s", md5Str, res)
	}
}
