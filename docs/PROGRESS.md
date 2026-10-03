# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Changes since v0.17.1

These commits are on `main` and are not released. Each one has its own test.

- Fixed: the data race on `Context.Target` between the cancellation watcher and `attachTarget` (issue 1638).
- Fixed: the kernel killed Chrome when the OS thread that started it ended. Chrome now starts from a goroutine that stays on its thread (issue 1566).
- Fixed: the default options passed the name `site-per-process` as a feature. It did nothing and is removed (issue 1605). The decision is proposed in `decisions/2026-10-04-keep-site-isolation-on.md`.
- Fixed: the target logged `unhandled node event` for four DOM events, and `DOM.scrollableFlagUpdated` changed nothing. A test fails when a new event of the `dom` or `page` package is neither handled nor ignored (issue 1530).
- Fixed: the websocket did not answer ping frames, and `DialContext` panicked when the server sent frames with the handshake (pull request 1611).
- Fixed: a key with Ctrl, Alt or Meta typed its character (issue 1384).
- Added: `WithDialHTTPHeader`, `WithRemoteDialHTTPHeader` and `WithConnHTTPHeader` (pull request 1631).
- Added: `WithNewWindow(bool)`. By default, each new tab opens in a new window, as before in this version. `WithNewWindow(false)` opens it as a tab in the window of the browser. Switch tabs with `target.ActivateTarget`.
- Fixed: a tab in a browser context that has no window, with `WithNewWindow(false)`, failed with `Failed to open new tab - no browser is open`. The first tab of a new browser context now opens in a new window, and a tab for an existing browser context retries in a new window (the error of issue 1601).
- Added: the exit error of a browser process that died is in the error of `Run`, `Do`, `Call` and `CallBrowser` (issue 408).
- Added: `LoadError` and `ErrPageLoad` for a page that did not load (issue 793).
- Not reproduced: `WaitReady` after a fresh navigation (issue 1593). A test covers it.
- Not reproduced: the remote allocator error `no browser is open` (issue 1601). It works with Chrome 154 in the new headless mode, and with headless-shell 131.
- Already fixed: the race in the removal of the user data directory (issue 1544). A test covers the retry, and the example test no longer leaves a directory.
- Checked: a result of 30 MB works on the pipe and on the websocket (issue 401).

## Where the work stands

On 2026-10-03 `main` was ported to the tagged `cdproto` `v0.157.1`, and the
tests were fixed for current Chrome. See the commits of that day. The `Test`
workflow on GitHub is not stable. The first browser start sometimes does not
print its DevTools address, and Chrome only prints DBus errors. The tests pass
on the local Chrome.

On 2026-10-04 the typed API work was merged into `main`. The maintainer
approved these parts:

- the typed `cdproto` of `pdlgen`, with generics and iterators
- the generic action API and the iterator events of `chromedp`
- the pipe transport as the default
- the visible window options `WithVisibleWindow` and `WithKeepOpen`, and the
  variable `CHROMEDP_VISIBLEWINDOW`
- the port of the `examples` repository

`cdproto` v0.157.3 is the first release of the typed API. `chromedp` v0.17.0
uses it. The full test suite passes. The comments of the Go files and the
documents follow the `simple-english` skill, and `go test ./docs/` tests them.

## Waiting

- The maintainer must answer the open questions at the end of [`PLAN.md`](PLAN.md).
- Known work that is not done is in [`BACKLOG.md`](BACKLOG.md).
