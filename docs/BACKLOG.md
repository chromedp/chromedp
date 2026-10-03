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

## Transport

### Use a pipe to the browser by default

`chromedp` starts the browser with a remote debugging port and connects to it
over a websocket. Change the default to a pipe to the browser process, as
Puppeteer and the other DevTools packages do with `--remote-debugging-pipe`.
The browser reads and writes the protocol messages on two extra file
descriptors, 3 and 4, that the parent process opens. Each message ends with a
zero byte. The pipe needs no port, so there is no port to find, to secure or to
clash with another process. Keep the websocket as a choice for a browser that
`chromedp` does not start, such as a remote one. The change touches
`allocate.go` and `conn.go`.

## API

### Review and merge the new API

The new API is built on the local branch `typed-api`, and nobody has approved
it. See `decisions/2026-10-03-generic-iterator-api-instead-of-action.md` and
[`API.md`](API.md). The maintainer pushes the branch after the review. Merge
the `pdlgen` branch `typed-api` first, because the typed `cdproto` comes from
it. The decision changes from Proposed to Decided when the maintainer approves
it.

## Documentation

### Move the long examples out of the package

After the transport and the API change, move the long examples into the
`examples` repository or into tests. Remove code and files that nothing uses.

### Stop the tests from writing files in the root

The tests and the examples write `background.pdf`, `page.pdf`,
`fullScreenshot.jpeg` and `iphone7-ua.png` in the repository root. Git does not
track them, and `.gitignore` lists them. Make each test write its file in a
temporary directory.
