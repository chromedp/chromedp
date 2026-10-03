package remote

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// forceIP tries to force the host component in urlstr to be an IP address.
//
// Since Chrome 66, clients of the Chrome DevTools Protocol that connect to a
// browser must send the "Host:" header as an IP address or "localhost".
// See https://github.com/chromium/chromium/commit/0e914b95f7cae6e8238e4e9075f248f801c686e6.
func forceIP(ctx context.Context, urlstr string) (string, error) {
	u, err := url.Parse(urlstr)
	if err != nil {
		return "", err
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}
	host, err = resolveHost(ctx, host)
	if err != nil {
		return "", err
	}
	u.Host = net.JoinHostPort(host, port)
	return u.String(), nil
}

// resolveHost tries to resolve a host to an IP address. If the host is an IP
// address or "localhost", it returns the host directly.
func resolveHost(ctx context.Context, host string) (string, error) {
	if host == "localhost" {
		return host, nil
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return host, nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}

	return addrs[0].IP.String(), nil
}

// modifyURL modifies the websocket debugger URL if the provided URL is not a
// valid websocket debugger URL.
//
// A websocket debugger URL that contains "/devtools/browser/" is valid. In
// this case, modifyURL changes urlstr only with forceIP.
//
// Otherwise, modifyURL builds a URL like http://[host]:[port]/json/version
// and queries the valid websocket debugger URL from this endpoint. It parses
// the [host] and [port] from urlstr. If the host component is not an IP, it
// resolves the host to an IP first. Example parameters:
//   - ws://127.0.0.1:9222/
//   - http://127.0.0.1:9222/
//   - http://container-name:9222/
func modifyURL(ctx context.Context, urlstr string) (string, error) {
	lctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if strings.Contains(urlstr, "/devtools/browser/") {
		return forceIP(lctx, urlstr)
	}

	// replace the scheme and path to construct a URL like:
	// http://127.0.0.1:9222/json/version
	u, err := url.Parse(urlstr)
	if err != nil {
		return "", err
	}
	u.Scheme = "http"
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}
	host, err = resolveHost(ctx, host)
	if err != nil {
		return "", err
	}
	u.Host = net.JoinHostPort(host, port)
	u.Path = "/json/version"

	// to get "webSocketDebuggerUrl" in the response
	req, err := http.NewRequestWithContext(lctx, "GET", u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	// the browser will construct the debugger URL using the "host" header of
	// the /json/version request. For example, run headless-shell in a container:
	//     docker run -d -p 9000:9222 chromedp/headless-shell:latest
	// then:
	//     curl http://127.0.0.1:9000/json/version
	// and the websocket debugger URL will be something like:
	// ws://127.0.0.1:9000/devtools/browser/...
	wsURL := result["webSocketDebuggerUrl"].(string)
	return wsURL, nil
}
