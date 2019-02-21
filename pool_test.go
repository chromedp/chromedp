package chromedp

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestAllocatePortInUse(t *testing.T) {
	t.Parallel()

	// take a random available port
	l, err := net.Listen("tcp4", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// make the pool use the port already in use via a port range
	_, portStr, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.Atoi(portStr)
	pool, err := NewPool(
		PortRange(port, port+1),
		// skip the error log from the used port
		PoolLog(nil, nil, func(string, ...interface{}) {}),
	)
	if err != nil {
		t.Fatal(err)
	}

	c, err := pool.Allocate(ctxt)
	if err != nil {
		want := "address already in use"
		got := err.Error()
		if !strings.Contains(got, want) {
			t.Fatalf("wanted error to contain %q, but got %q", want, got)
		}
	} else {
		t.Fatal("wanted Allocate to error if port is in use")
		c.Release()
	}
}
