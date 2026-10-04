# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Changes since v0.18.0

These commits come after the release v0.18.0. Each one has its own test, and a documentation change has none.

- Added: `Tap` and `TapXY` send a touch tap (issue 1174).
- Fixed: `Evaluate[T]` returned the zero value and no error for the expression `null` when `T` cannot be nil, for example `Evaluate[int]` with the expression `null`. It returns `ErrJSNull` now, as the documentation says. The example `ExampleEvaluate` had a case for it, and its output did not show the error.
- Added: `EvalAwaitPromise`, an evaluate option that waits for a promise that the expression returns.
- Fixed: `Text` on a text node, which a search for plain text finds, failed with a JavaScript error. It returns the text of the node now, when its parent element is visible.
- Fixed: the exec allocator keeps the order of the flags, so `--flag-switches-begin` and `--flag-switches-end` can surround the switches that Chrome needs between them (issue 1483).
- Fixed: a call on a `Target` or a `Browser` with a context that never ends hung for ever after the browser died. It returns an error now (issue 1529).
- Added: the allocator option `NoInheritEnv` starts the browser with only the variables of `Env` (issue 1584).
- Fixed: `WaitNotPresent` with a `JSPath` that gives null, undefined or an empty `NodeList` never succeeded. A `JSPath` can give a `NodeList` of nodes, and a value that is not a node is an error at once (issue 1600).
- Added: `WithDetachOnCancel` leaves the tab open when the context ends (issue 1613).
- Documented: contexts that share a browser run in separate tabs and are safe in parallel, and actions on one context can race (issue 541).
- Documented: the limit for a cross-site iframe, and two ways around it: `WithTargetID` and the flags that turn off site isolation (issues 1398, 1467 and 1471).
- Documented: `SendKeys` sends `\n` as Enter with a char event, an editor such as Lexical can insert two line breaks, and a rawKeyDown event avoids it. Sending `kb.Enter` does not help, because it is the same as `\n` (issue 1401).
- Documented: the first `Run` binds the life of the browser to its context, so a timeout on that context closes the browser, and a derived context limits one action (issue 1539).
- Documented: the default flag `disable-dev-shm-usage`, why it is on, its cost, and `Flag("disable-dev-shm-usage", false)` to turn it off (issue 1627).
- Documented: the set of selector types is closed, and `ByFunc` is the extension point for a custom lookup (issue 1641).
- Added: `DragAndDrop` and `DragAndDropXY` drag an element and drop it on another element or on a point. They work for a page with mouse events and for a page with HTML5 drag and drop. They are new in the next release.
- Added: `Console` turns the console API calls, the uncaught exceptions, the unhandled promise rejections and the browser log into one iterator of `ConsoleMessage` values, in order. It is new in the next release.
- Added: `ExposeFunc` makes a Go func available to the page as `window.<name>`. A call from the page returns a promise that the result or the error of the func settles. It works after a navigation and in an iframe. It is new in the next release.
- Added: `PrintToPDF` with the options `PDFLandscape`, `PDFPaper`, `PDFMargin`, `PDFMargins`, `PDFScale`, `PDFPageRanges`, `PDFPreferCSSPageSize`, `PDFPrintBackground`, `PDFHeaderTemplate`, `PDFFooterTemplate`, `PDFOutlineAndTagged` and `PDFStream`, so that a program does not build `page.PrintToPDFParams` by hand. It is new in the next release, and `cdp.Call` with `page.PrintToPDF` still works.

## Changes in v0.18.0

These commits are in the release v0.18.0. Each one has its own test.

- Fixed: the data race on `Context.Target` between the cancellation watcher and `attachTarget` (issue 1638).
- Fixed: the kernel killed Chrome when the OS thread that started it ended. Chrome now starts from a goroutine that stays on its thread (issue 1566).
- Fixed: the default options passed the name `site-per-process` as a feature. It did nothing and is removed (issue 1605). The decision is in `decisions/2026-10-04-keep-site-isolation-on.md`.
- Fixed: the target logged `unhandled node event` for four DOM events, and `DOM.scrollableFlagUpdated` changed nothing. A test fails when a new event of the `dom` or `page` package is neither handled nor ignored (issue 1530).
- Fixed: the websocket did not answer ping frames, and `DialContext` panicked when the server sent frames with the handshake (pull request 1611).
- Fixed: a key with Ctrl, Alt or Meta typed its character (issue 1384).
- Added: `remote.WithDialHTTPHeader` and `remote.WithConnHTTPHeader` (pull request 1631). The commit named them `WithRemoteDialHTTPHeader` and `WithConnHTTPHeader` in the core, and the split below moved and renamed them.
- Fixed: on macOS, helper processes of a browser that was killed stayed alive and made the allocator leave its temporary user data directory. `killLeftovers` now works on macOS, with the list of processes from `ps`, as it works on Linux with `/proc`.
- Added: each test binary starts a browser once before the tests and prints one line that starts with `browser:`. The line has the product and version, the revision, the executable path and the operating system, so the log of a CI run says which browser ran the tests.
- Added: on Windows, the default browser lookup falls back to Microsoft Edge (`msedge`) when it finds no Chrome.
- Added: `WithNewWindow(bool)`. By default, each new tab opens in a new window, as before in this version. `WithNewWindow(false)` opens it as a tab in the window of the browser. Switch tabs with `target.ActivateTarget`.
- Fixed: a tab in a browser context that has no window, with `WithNewWindow(false)`, failed with `Failed to open new tab - no browser is open`. The first tab of a new browser context now opens in a new window, and a tab for an existing browser context retries in a new window (the error of issue 1601).
- Added: the exit error of a browser process that died is in the error of `Run`, `Do`, `Call` and `CallBrowser` (issue 408).
- Added: `LoadError` and `ErrPageLoad` for a page that did not load (issue 793).
- Not reproduced: `WaitReady` after a fresh navigation (issue 1593). A test covers it.
- Not reproduced: the remote allocator error `no browser is open` (issue 1601). It works with Chrome 154 in the new headless mode, and with headless-shell 131.
- Already fixed: the race in the removal of the user data directory (issue 1544). A test covers the retry, and the example test no longer leaves a directory.
- Checked: a result of 30 MB works on the pipe and on the websocket (issue 401).
- Changed: the core module uses only the standard library and `cdproto`. The websocket code, the remote allocator and the websocket mode of the exec allocator moved to the new module `github.com/chromedp/chromedp/remote`. The tests that need `pdf` or `pixelmatch` moved to the new module `github.com/chromedp/chromedp/test`. The core has the new names `Dialer`, `WithDialer`, `WithAllocatorOptions`, `Attacher`, `NewAllocatorContext` and `ErrNoDialer`. `KeepOpen` and the flags for a debugging port need `remote.WebSocket`. `docs/MIGRATION.md` lists the old and new names, and the decision is in `decisions/2026-10-04-the-core-uses-only-the-standard-library.md`. The tag `remote/v0.1.0` does not exist yet, and the `go.mod` of `remote` and `test` still have a `replace` directive for the core.
- Added: the pipe transport works on Windows. The allocator passes two inheritable handles to the browser and names them in the switch `--remote-debugging-io-pipes`, so a program for Windows needs no `remote.WebSocket`. The tests ran on Windows 11 with Chrome 154 and Edge 154. The decision is in `decisions/2026-10-04-the-pipe-works-on-windows.md`.
- Fixed: a canceled context gave an `exec` error on Windows when the browser program did not exist, and the exit error of a dead browser sometimes arrived too late on Windows. The middle button test skips on Windows, because autoscroll swallows `auxclick`.

## The first runs on three systems

The workflow `test.yml` runs the core, `remote` and `test` on Ubuntu, Windows and macOS with the Chrome of the runner, and the container script on Ubuntu. The first runs failed on all three systems, and the tests pass on Linux and on Windows 11 with Chrome 154 and Edge 154. The runner images list Google Chrome, version 154 on Windows and 152 on macOS. The steps of the three modules and of the container now run even when an earlier step failed, so one run shows every failure.

What the logs showed, and what changed:

- Container: `TestTargetHandlesEveryEvent` needs the `go` command, which the image does not have. It skips. `TestWithNewWindow/SharedWindow` and `/Inherited` skip when `HEADLESS_SHELL` is set, because headless-shell gives each target its own window id. The failure message now prints the window ids.
- All systems, probable cause: the runners are slow when many tests run at once. Several limits were too short: one second to close a tab, two seconds to start a browser in `TestStartsWithNonBlankTab`, five seconds in `TestRunResponse`, and one second per step in `ExampleNewContext_reuseBrowser`. On Windows that example ended the whole test run with `log.Fatal`. The limits are 5 to 30 seconds now. A failure to close a tab names the call that failed.
- Windows, probable cause: `TestExecAllocatorKillBrowser` and `TestExecAllocatorPipeStartFailure/pipe_not_open` also fit a slow machine. The browser needed more than two seconds to print its reason, so the allocator killed it and the error had no output. The wait is 10 seconds now. Both tests pass in under a second on the Windows 11 VM, also with `CI=true`.
- macOS: Ctrl+A is not select all there. `TestKeyEventModifier/ctrl+a` still checks the keydown event, and it skips the check of the selection on macOS.
- macOS: a start that the context ended gave `chrome failed to start` with the messages of the helper processes that lost their parent. It returns the error of the context now.
- Not explained: the macOS run left one empty temporary directory, `chromedp-runner...`, once. The allocator does not kill leftover processes on macOS. The directory did not leave in the second run.
- Not decided: nobody ran the tests on a real Mac. The next run on `macos-latest` must show whether the longer limits are enough.

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
