# Backlog

This document lists work that is known and not done. A decision is not a
backlog item. It goes in [`decisions/`](decisions/README.md). A question for
the maintainer goes at the end of [`PLAN.md`](PLAN.md). When an item here is
done, delete it. Record in `decisions/` anything that you decided on the way.

## Tooling

### Add a linter configuration

The repository has no `.golangci.yml` and CI runs no linter. Add a
configuration and a CI step, then describe both in the Linting section of
[`../AGENTS.md`](../AGENTS.md). Ask the maintainer which linters to use.

### Run the tests under a race detector in CI

`chromedp_test.go` says that 50 contexts are enough for `go test -race` to find
problems. The workflow runs `go test -v ./...` with no `-race`. Ask the
maintainer whether CI must add it.

## API

### Test the Windows behavior

No Windows machine ran the tests. The pipe transport does not apply on
Windows, and the allocator uses the websocket there. The flags that detach a
kept browser on Windows are not tested. Run the tests on Windows and fix what
fails. See `decisions/2026-10-03-use-a-pipe-to-the-browser-by-default.md` and
`decisions/2026-10-03-a-visible-window-is-an-opt-in.md`.

### Return the exit error of a dead browser from Cancel

`Run`, `Do`, `Call` and `CallBrowser` return the exit error of a browser that
died on its own. `Cancel` still returns only the errors of its own work. Make
`Cancel` return the exit error too. The error is known only after the process
is reaped, so the code must wait for it with a bound.

### Send the headers of a remote allocator to the version request

`WithRemoteDialHTTPHeader` sets the headers of the websocket request only. The
request to `/json/version`, which `NewRemoteAllocator` sends when the address
has no `/devtools/browser/` part, has none. A service that needs a header on
both requests needs `NoModifyURL` today. Ask the maintainer whether the option
must cover the version request too.

### Keep the URL of a frame after a navigation in the same document

The target ignores `Page.navigatedWithinDocument`, so `Frame.URL` keeps the
address of the last full navigation. No code of the package reads it. Update
it from the event if a program needs it.

## Documentation

### Move the long examples out of the package

Move the long examples of `example_test.go` into the `examples` repository or
into tests. Remove code and files that nothing uses. `ExampleFullScreenshot`
also uses a live website, `https://google.com`, and `go test` runs it.

### Stop the tests from writing files in the root

The tests and the examples write `background.pdf`, `page.pdf`,
`fullScreenshot.jpeg` and `iphone7-ua.png` in the repository root. Git does not
track them, and `.gitignore` lists them. Make each test write its file in a
temporary directory.
