package chromedp

import (
	"context"
	"log"
	"os"
	"testing"
)

var pool *Pool
var defaultContext = context.Background()
var testdataDir string

func testAllocate(t *testing.T, path string) *Res {
	c, err := pool.Allocate(defaultContext)
	if err != nil {
		t.Fatalf("could not allocate from pool: %v", err)
	}

	err = WithLogf(t.Logf)(c.c)
	if err != nil {
		t.Fatalf("could not set logf: %v", err)
	}

	err = WithDebugf(t.Logf)(c.c)
	if err != nil {
		t.Fatalf("could not set debugf: %v", err)
	}

	err = WithErrorf(t.Errorf)(c.c)
	if err != nil {
		t.Fatalf("could not set errorf: %v", err)
	}

	h := c.c.GetHandlerByIndex(0)
	th, ok := h.(*TargetHandler)
	if !ok {
		t.Fatalf("handler is invalid type")
	}

	th.logf = t.Logf
	th.debugf = t.Logf
	th.errorf = func(s string, v ...interface{}) {
		t.Logf("target handler error: "+s, v...)
	}

	if path != "" {
		err = c.Run(defaultContext, Navigate(testdataDir+"/"+path))
		if err != nil {
			t.Fatalf("could not navigate to testdata/%s: %v", path, err)
		}
	}

	return c
}

func TestMain(m *testing.M) {
	var err error

	testdataDir = "file:" + os.Getenv("GOPATH") + "/src/github.com/knq/chromedp/testdata"

	//pool, err = NewPool(PoolLog(log.Printf, log.Printf, log.Printf))
	pool, err = NewPool()
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()

	err = pool.Shutdown()
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(code)
}
