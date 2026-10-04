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

The repository has three modules. The core, in the root, uses only the standard
library and `cdproto`. The module `remote` holds the code that needs a
WebSocket and owns the library `gobwas/ws`. The module `test` holds the tests
that need the libraries `pdf` and `pixelmatch`. The core never imports `remote`.
See `decisions/2026-10-04-the-core-uses-only-the-standard-library.md`.

The core package has five layers.

An `Allocator` gets a browser. `ExecAllocator` starts a browser process and
talks to it through a pipe. The pipe is the default, on Windows too. On Windows the pipes are handles that the switch `--remote-debugging-io-pipes` names. With a `Dialer`, which the
option `WithDialer` sets, it reads the WebSocket address from the
output of the browser and connects through the dialer instead. The core has no
dialer, and `remote.WebSocket` supplies one. The allocator of `remote` connects
to a browser that already runs, through a WebSocket. It implements `Attacher`,
so that `NewContext` knows that no process belongs to the context.

A `Browser` holds a `Transport`, which is the pipe or the WebSocket
connection. The WebSocket connection is `remote.Conn`. It reads every message, sends commands, and passes events to the
target that owns them. Messages use the `encoding/json/v2` package of the
standard library.

A `Target` is one page or tab. It tracks the frame tree and the DOM tree of
that page from events. A query can then read nodes without a round trip.

A `Context` ties a target to a Go `context.Context`. `NewContext` makes one.
Canceling it closes the tab. `Run` and `Do` start the browser and open the tab
when the context has none. Then they run actions in order.

An `Action[T]` is the unit of work. It is a func that runs against a `Target`
and returns a value of the type `T`. An action that returns no value has the
type `Action[Void]`. The query, navigation, input, emulation, screenshot and
evaluation functions each return an action. A selector finds nodes by a query
string. The type of the selector, such as `ID` or `CSS`, chooses the lookup. A
plain string is a `Search`. Then the query waits for the nodes to be ready.

Two folders hold generated data. `kb/` has the keyboard keys and `device/` has
the device descriptors. JavaScript snippets in `js/` are embedded with
`go:embed`.

## What exists

The package covers browser allocation, contexts, targets, navigation, element
queries, input, device emulation, screenshots, JavaScript evaluation, polling
and events. It also has the options for a visible window and for a browser that
stays open. The Go files in the root hold about 15,000 lines, tests included.
The README links the examples repository for larger tasks.

Release v0.17.0 uses the typed `cdproto` v0.157.3. It has the generic action
API, the iterator events, the typed selectors, the pipe transport and the
visible window options. `API.md` describes the API and `MIGRATION.md` lists
every change. The decisions in `decisions/` record the choices.

## Testing

The tests drive a real browser, so they need Chrome or `headless-shell`.
`TestMain` makes one exec allocator for the whole run and fails the run if a
temporary directory leaks. HTML pages and golden images are in `testdata/`.

CI, in `.github/workflows/test.yml`, runs `go test -v ./...` against Chrome in
the root, in `remote/` and in `test/`, on Linux, Windows and macOS. On Linux only,
it then runs `./contrib/docker-test.sh`, which tests the three modules against
the `chromedp/headless-shell` image. The
tests of `remote/` and of `test/` use the exported API of the core and the
helpers in `internal/chromedptest/`. It uses the newest stable Go release, which is Go 1.27 now.

A second workflow, `.github/workflows/nightly.yml`, runs the container script every
night against the channels `stable`, `beta` and `dev` of the `chromedp/headless-shell`
image, on Linux amd64.

The only test that needs no browser is `go test ./docs/`. It tests the
documents and the Go comments.

## Open questions

These are things that this repository does not settle. Do not decide them
without the maintainer.

1. Where does the release process live? The maintainer has a release script
   that changes `go.mod` and `go.sum`. No file in this repository describes
   it.
2. Does the maintainer want a linter? No configuration exists. See
   `BACKLOG.md`.
3. Can a new module go in `go.mod` of `remote` or `test`? The core takes none,
   see `decisions/2026-10-04-the-core-uses-only-the-standard-library.md`. The
   policy for the other two modules is not stated.
4. Does the maintainer want `go fix` and `modernize` runs recorded as a rule?
   The history shows them on 2025-04-18 and 2026-07-15.
