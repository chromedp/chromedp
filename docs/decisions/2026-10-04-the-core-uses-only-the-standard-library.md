# The core uses only the standard library

Status: Decided.

The maintainer decided that the core module `github.com/chromedp/chromedp` uses
no library other than the Go standard library and `github.com/chromedp/cdproto`.
`cdproto` itself uses only the standard library. Before this decision, `go.mod`
also required `gobwas/ws` for the websocket, and `ledongthuc/pdf` and
`orisano/pixelmatch` for two groups of tests. This file records what moved and
why.

## Why

A program that imports `chromedp` pulls every module in the `go.mod` of the core
into its build graph. The pipe is now the default transport, so most programs
need no websocket library. The two test libraries were only for tests, and they
were still in the graph of every user. A smaller graph means fewer updates, fewer
security reports for code that the program never runs, and a clearer promise.

The library `golang.org/x/sys` was an indirect requirement of `gobwas/ws`. No
code of the core used it, so it left with `gobwas/ws`.

## The three modules

The repository holds three Go modules.

- The root, `github.com/chromedp/chromedp`, is the core. It requires only `cdproto`.
- `remote/`, `github.com/chromedp/chromedp/remote`, holds the websocket. It owns `gobwas/ws`.
- `test/`, `github.com/chromedp/chromedp/test`, holds the tests that need `pdf` or `pixelmatch`. It has no exported names.

The release tag of `remote` is `remote/v0.1.0`. The core never imports `remote`.
`remote` imports the core, and `test` uses the exported API of the core only.

## What moved

The module `remote` has the websocket connection `Conn`, `DialContext` and the
dial options, the allocator for a browser that runs already, the websocket mode
of the exec allocator, and the option that keeps a browser open. The names
changed in this way:

| Old name | New name |
| --- | --- |
| `chromedp.NewRemoteAllocator` | `remote.NewAllocator` |
| `chromedp.RemoteAllocator` | `remote.Allocator` |
| `chromedp.RemoteAllocatorOption` | `remote.Option` |
| `chromedp.WithRemoteDialHTTPHeader` | `remote.WithDialHTTPHeader` |
| `chromedp.WebSocket` | `remote.WebSocket` |
| `chromedp.WithKeepOpen` | `remote.WithKeepOpen` |

`NoModifyURL`, `Conn`, `DialContext`, `DialOption`, `WithConnHTTPHeader` and
`WithConnDebugf` keep their names in the new package. The old browser options
`WithDialHTTPHeader` and `WithDialTimeout` are now options of the remote
allocator. `NewBrowser` is gone, and `remote.DialContext` followed by
`NewBrowserTransport` replaces it. `ErrInvalidWebsocketMessage` is now
`remote.ErrInvalidMessage`. `docs/MIGRATION.md` lists every name.

The tests of the moved code moved with it. The screenshot tests and the PDF text
test moved to `test/` with the files that only they use. The helpers that the
tests of the three modules share are in two packages of the core module. The
package `internal/testenv` reads the environment variables, and the core tests
use it. The package `internal/chromedptest` makes browsers for the tests, and the
tests of `remote` and `test` use it. The core tests cannot import it, because it
imports the core package. The Go rule for `internal` works by import path
prefix, so the other modules of the repository can import both packages.

## The hooks in the core

The core has the smallest set of public names that `remote` needs.

- `Dialer` is a func that connects to a websocket address and returns a `Transport`. `WithDialer` is the `ExecAllocatorOption` that sets one. With a dialer, the exec allocator starts the browser with a debugging port and connects through the dialer. This is the old websocket mode.
- `WithAllocatorOptions` is a `ContextOption` that adds `ExecAllocatorOption` values to the default allocator that `NewContext` builds. `remote.WithKeepOpen` uses it, so that the core knows nothing about websockets.
- `Attacher` is an interface with the method `Attaches() bool`. An allocator that attaches to a running browser implements it. `NewContext` used a type assertion to `*RemoteAllocator` before. It now uses `Attacher`. The core also closes the allocation signal and watches the lost connection for such an allocator, because those fields are private.
- `NewAllocatorContext` makes the context of an allocator that another module implements.
- `ErrNoDialer` is the error of an allocator that needs a websocket and has no dialer.
- `PipeConn.SetDebugf` is now exported, and a `Transport` can have it. `NewBrowserTransport` calls it with the protocol logger, so that `remote.Conn` logs the messages as before.

The keep-open mechanics need no websocket library, so `KeepOpen`, `KeptOpen` and
`WaitClosed` stay in the core, with the detach and the no-kill code. Only the
connection needs a dialer. `KeepOpen` without a dialer returns `ErrNoDialer`, and
the message names `remote.WebSocket`. The flags `remote-debugging-port` and
`remote-debugging-address` need a dialer too.

## What this changes for users

- A program that uses the default allocator needs no change.
- Run `go get github.com/chromedp/chromedp/remote` only for a remote browser, for a websocket exec allocator, or for keep-open.
- On Windows the exec allocator always used the websocket, because `os/exec` cannot pass the file descriptors 3 and 4. It now needs `remote.WebSocket`. Without it, the first `Run` returns `ErrNoDialer`. The backlog has an item for a pipe on Windows.
- The tests of `remote` use a browser started through a debugging port. The two subtests of `TestBrowserContext` that attach to the browser of the tests were skipped before, because the browser of the tests used the pipe. They now run in `remote` as `TestAllocatorBrowserContext`.

## Development setup

The `go.mod` of `remote` and of `test` require `github.com/chromedp/chromedp
v0.18.0`, the next release of the core. That version does not exist yet. Each
file has the directive `replace github.com/chromedp/chromedp => ../`, so the go
command uses the root directory, and `go mod tidy` works. The directives are for
development. When the maintainer tags the core, the maintainer sets the real
version and removes the directive, and then tags `remote/v0.1.0`. A program
outside this repository that depends on `remote` is not affected by a `replace`
directive in the `go.mod` of `remote`, because the go command ignores the
directives of a dependency.

Nobody commits a `go.work` file. `.gitignore` lists `go.work` and `go.work.sum`,
and the workflow of CI uses the replace directives. Each module runs
`go test ./...` in its own directory.

## Tests that changed

- `TestDialTimeout` tested `NewBrowser` with `WithDialTimeout`. It now tests `remote.NewAllocator` with `remote.WithDialTimeout`, with the same two cases.
- `TestBrowserDialHTTPHeader` tested the browser option `WithDialHTTPHeader`. That option is gone, and `TestAllocatorDialHTTPHeader` covers the same header through the allocator option.
- `TestExecAllocatorWebSocket` checked the transport type with a private field. It now checks that the output of the browser has the websocket address line.
- The tests of the websocket mode of the exec allocator that never need a connection stay in the core with a stub dialer. The tests that need a connection are in `remote`.
- Every other test keeps its assertions.

## Consequences

- The core has one dependency, and `go mod tidy` leaves `go.sum` with the lines of `cdproto` only.
- CI runs three modules. A Dependabot configuration, if the repository gets one, must list `/`, `/remote` and `/test`.
- The maintainer must remember the version and the directive step before the release. `docs/BACKLOG.md` has an item for it.
