# About chromedp-proxy

`chromedp-proxy` is a simple cli tool to log/intercept Chrome Debugging
Protocol sessions, notably the websocket messages sent to/from Chrome DevTools
and a Chromium/Chrome/headless_shell/etc. instance.

This is useful for finding problems/issues with the [`chromedp`](https://github.com/knq/chromedp)
package or to review/log/capture Chrome Debugging Protocol commands, command
results, and events sent/received by DevTools, Selenium, or any other
application speaking the Chrome Debugging Protocol.

## Installation

Install in the usual Go way:

```sh
go get -u github.com/knq/chromedp/cmd/chromedp-proxy
```

## Use

By default, `chromedp-proxy` will listen on localhost:9223 and will proxy requests to/from localhost:9222:
```sh
chromedp-proxy
```

`chromedp-proxy` can also be used to expose a local Chrome instance on an
external address/port:
```sh
chromedp-proxy -l 192.168.1.10:9222
```

By default, `chromedp-proxy` will log to both `stdout` and to `logs/cdp-<id>.log`, and can be modified using cli flags:
```sh
# only log to stdout
chromedp-proxy -n

# another way to only log to stdout
chromedp-proxy -log ''

# log to /var/log/cdp/session-<id>.log
chromedp-proxy -log '/var/log/cdp/session-%s.log'
```

## Flags:

```sh
$ ./chromedp-proxy -help
Usage of ./chromedp-proxy:
  -l string
    	listen address (default "localhost:9223")
  -log string
    	log file mask (default "logs/cdp-%s.log")
  -n	disable logging to file
  -r string
    	remote address (default "localhost:9222")
```
