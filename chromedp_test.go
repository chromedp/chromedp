package chromedp

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"
)

var pool *Pool

var defaultContext = context.Background()

func TestMain(m *testing.M) {
	var err error

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

func TestNavigate(t *testing.T) {
	var err error

	c, err := pool.Allocate(defaultContext)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()

	err = c.Run(defaultContext, Navigate("https://www.google.com/"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Run(defaultContext, WaitVisible(`#hplogo`, ByID))
	if err != nil {
		t.Fatal(err)
	}

	var urlstr string
	err = c.Run(defaultContext, Location(&urlstr))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(urlstr, "https://www.google.") {
		t.Errorf("expected to be on google, got: %v", urlstr)
	}
}

func TestSendKeys(t *testing.T) {
	var err error

	c, err := pool.Allocate(defaultContext)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
}
