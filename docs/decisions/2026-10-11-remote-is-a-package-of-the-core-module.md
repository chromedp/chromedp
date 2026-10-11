# The remote package is part of the core module

Status: Amends 2026-10-04-the-core-uses-only-the-standard-library.md, 2026-10-03-use-a-pipe-to-the-browser-by-default.md, 2026-10-03-a-visible-window-is-an-opt-in.md and 2026-10-06-support-go-1-25-and-later.md.

The maintainer decided on 2026-10-11 that `remote` uses its own WebSocket client
and not `github.com/gobwas/ws`. The nested module and its dependencies go away.
The package `github.com/chromedp/chromedp/remote` is now a normal package of the
core module. The import path does not change, and the public API of the package
does not change.

## Why

The decision `2026-10-04-the-core-uses-only-the-standard-library.md` made
`remote` a module so that the core did not depend on `gobwas/ws`. The split had a
cost. The repository had a second set of tags, a second `go.mod`, a version
dance before each release, and a `replace` directive that CI added at run time.
A program that used a remote browser had to run an extra `go get`.

What `chromedp` needs from a WebSocket library is small. It is a client that
sends one text frame per message and reads text frames. RFC 6455 describes this in
a few pages. A client of fewer than 1,000 lines with comments does it with the
standard library, so the module split has no purpose left.

## What changed

- `remote/go.mod` and `remote/go.sum` are gone. `remote` is a package of the core module.
- The core module still requires only `cdproto`. `go mod tidy` changes nothing in its `go.mod` and `go.sum`.
- `remote/internal/websocket` is the new client, written from RFC 6455 and the standard library. `remote/conn.go` is a thin adapter that decodes and encodes the `cdproto` messages.
- `remote/internal/wstest` is a small server for the tests. It shares no code with the client.
- The libraries `gobwas/ws`, `gobwas/httphead`, `gobwas/pool` and `golang.org/x/sys` are no longer in any `go.mod` of this repository.
- CI and `contrib/docker-test.sh` run `remote` with the core. Only `test` keeps the `replace` directive that CI adds at run time.

The client closes the connection as RFC 6455 section 7 says. When the server
sends a close frame, the client echoes it and closes the network connection at
once. `Close` sends a close frame with the code 1000. If a goroutine reads, it
then waits at most one second for the close frame of the server or for the end
of the stream, and closes the connection. A write that runs after the close
frame fails, so no data frame follows it. The dial has a timeout of 30 seconds
for the connect and 10 seconds for the TLS handshake. The deadline of the
context wins when it is shorter. A message can have at most 1,000,000 frames,
and the limit for its size is 256 MiB.

The client does not support extensions, so it never negotiates
`permessage-deflate`. Chrome does not use it for the DevTools protocol.

## Versions and tags

The tag `remote/v0.2.0` is the last tag of the nested module. Nobody will create
a new `remote/` tag. The first core release that has the package is v0.21.0.

## What this changes for users

A program that has `require github.com/chromedp/chromedp/remote v0.x` in its
`go.mod` must remove that line. Go reports an ambiguous import while both the old
nested module and the core module provide the package
`github.com/chromedp/chromedp/remote`. Run `go mod tidy`, which removes the line.
A program that never required the module needs no change, because `go get` of
the core gives the package.

The error that `Conn.Read` returns for a normal close is still `io.EOF`. A close
with another status code gives an error with the code and the reason.

## Consequences

- The repository has two modules: the core, which has `remote`, and `test`.
- A Dependabot configuration, if the repository gets one, lists `/` and `/test`.
- The maintainer owns a WebSocket client now. The client has a fuzz test, tests for each protocol error and a test server that it does not share code with.
