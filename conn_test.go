package chromedp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gobwas/ws"
)

func TestDialHTTPHeader(t *testing.T) {
	t.Parallel()

	const want = "Bearer secret-token"
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()
		<-time.After(5 * time.Second)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	hdr := http.Header{"Authorization": []string{want}}
	c, err := dialContext(context.Background(), wsURL, hdr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if got != want {
		t.Fatalf("got Authorization %q, want %q", got, want)
	}
}

func TestDialHTTPHeaderAbsent(t *testing.T) {
	t.Parallel()

	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()
		<-time.After(5 * time.Second)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	c, err := DialContext(context.Background(), wsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if got != "" {
		t.Fatalf("got Authorization %q, want empty", got)
	}
}
