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

## Release

### Remove the replace directives before the release

The `go.mod` of `remote` and of `test` hold `replace github.com/chromedp/chromedp => ../`
and require `github.com/chromedp/chromedp v0.18.0`, a version that does not
exist yet. When the maintainer tags the core, set the real version and remove
the directive in both files, and run `go mod tidy` in both directories. Do this
before the tag `remote/v0.1.0`. Nothing in the repository fails the release when
a directive stays. A Dependabot configuration, if the repository gets one, must
list the directories `/`, `/remote` and `/test`.

## API

### Return the exit error of a dead browser from Cancel

`Run`, `Do`, `Call` and `CallBrowser` return the exit error of a browser that
died on its own. `Cancel` still returns only the errors of its own work. Make
`Cancel` return the exit error too. The error is known only after the process
is reaped, so the code must wait for it with a bound.

### Send the headers of a remote allocator to the version request

`remote.WithDialHTTPHeader` sets the headers of the websocket request only. The
request to `/json/version`, which `remote.NewAllocator` sends when the address
has no `/devtools/browser/` part, has none. A service that needs a header on
both requests needs `remote.NoModifyURL` today. Ask the maintainer whether the option
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
