# chromedp Plan

This document records the plan for `github.com/chromedp/chromedp`. The
decisions are in [`decisions/`](decisions/README.md), one file each, named by
date. Coding agents must read this file before they change code here.
[`../AGENTS.md`](../AGENTS.md) holds the rules.

## Purpose

`chromedp` is a Go package that drives Chrome and other browsers that speak
the Chrome DevTools Protocol. A Go program uses it to load pages, read and
change the DOM, send input, take screenshots and listen for events. It needs
no external driver program.

The package is the high level API. The generated protocol types are in the
`cdproto` module, which the `pdlgen` tool writes from the protocol
definition files of Chromium.

## Architecture

The package has five layers.

An `Allocator` gets a browser. `ExecAllocator` starts a browser process and
reads its WebSocket address from the output. `RemoteAllocator` connects to a
browser that already runs.

A `Browser` holds the WebSocket connection. It reads every message, sends
commands, and passes events to the target that owns them. Messages use the
`go-json-experiment` JSON package.

A `Target` is one page or tab. It tracks the frame tree and the DOM tree of
that page from events, so that a query can read nodes without a round trip.

A `Context` ties a target to a Go `context.Context`. `NewContext` makes one.
Canceling it closes the tab. `Run` puts the executor in the context and runs
actions in order.

An `Action` is the unit of work. The interface has one method, `Do`. The
query, navigation, input, emulation, screenshot and evaluation functions each
return an action. A selector finds nodes by a query string and an option such
as `ByID` or `BySearch`, and waits for them to be ready.

Two folders hold generated data. `kb/` has the keyboard keys and `device/` has
the device descriptors. JavaScript snippets in `js/` are embedded with
`go:embed`.

## What exists

The package covers browser allocation, contexts, targets, navigation, element
queries, input, device emulation, screenshots, JavaScript evaluation, polling
and event listeners. The Go files in the root hold about 11,000 lines, tests
included. The README links the examples repository for larger tasks.

## Testing

The tests drive a real browser, so they need Chrome or `headless-shell`.
`TestMain` makes one exec allocator for the whole run and fails the run if a
temporary directory leaks. HTML pages and golden images are in `testdata/`.

CI, in `.github/workflows/test.yml`, runs `go test -v ./...` against Chrome
and then `./contrib/docker-test.sh` against the `chromedp/headless-shell`
image. It does both on the oldest and the newest stable Go release.

The only test that needs no browser is `go test ./docs/`. It checks the
documents.

## Open questions

These are things that this repository does not settle. Do not decide them
without the maintainer.

1. Which `cdproto` helpers does `chromedp` use? The proposal is in
   `decisions/2026-10-03-stop-relying-on-removed-cdproto-helpers.md`. A search
   of this repository found these uses.
   In the package code:
   `cdp.FrameState` with `cdp.FrameAttached` and `cdp.FrameLoading`, in
   `util.go`.
   `cdp.NodeTypeElement` and `cdp.NodeTypeText`, in `query.go`.
   `input.Modifier`, in the signatures of `ButtonModifiers` and
   `KeyModifiers` in `input.go`.
   The embedded `RLock` and `RUnlock` of `cdp.Node`, which `query.go` and
   `target.go` use.
   In the tests and examples only:
   `Node.Dump` in `util_test.go` and `example_test.go`.
   `Node.FullXPath` in `input_test.go` and `query_test.go`.
   `Node.AttributeValue` in `query_test.go`.
   `ExceptionDetails.Error` in `example_test.go`.
   `ResponseTime.Time`, a timestamp type, in `chromedp_test.go`.
   The package code uses no other helper from the proposal. `Node.Attribute`
   and the partial XPath helpers do not appear. Which of these must move into
   `chromedp`, and which must stay out of its API?
2. Does the maintainer want the generics and iterator API in
   `decisions/2026-10-03-generic-iterator-api-instead-of-action.md`? It
   replaces the `Action` interface, so it is a major change.
3. How is `chromedp` released and tagged? The history holds no release
   process, and no tag was checked.
4. Is a linter wanted? No configuration exists. See `BACKLOG.md`.
5. Can a new module go in `go.mod`? Five modules are direct dependencies
   today. Nothing here states a policy.
6. Does the maintainer want `go fix` and `modernize` runs recorded as a rule? The history
   shows them on 2025-04-18 and 2026-07-15.
7. The skills lock file came from `dbmeta`. Does its hash match the skill
   copies that `pdlgen` holds? The two lock files differ.
8. Three untracked files sit in the root, `background.pdf`, `page.pdf` and
    `fullScreenshot.jpeg`. `.gitignore` hides them. They look like test
    output. Can the maintainer delete them?
