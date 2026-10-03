# The minimum Go version is 1.27

Status: Decided.

The maintainer decided on 2026-10-03 that the minimum Go version of `chromedp`
is 1.27. The `go` line of `go.mod` was 1.26 and is now 1.27.

## Why

The `cdproto` package is generated with `encoding/json/v2` from the standard
library, which is available from Go 1.27. See
`2026-10-03-the-generated-code-uses-encoding-json-v2.md` in the `pdlgen`
repository. `cdproto` needs Go 1.27, so `chromedp` needs it too.

## What was done

The `go` line of `go.mod` and the Go versions in the `Test` workflow changed.
The workflow tests Go 1.27 and the newest stable release.

## What remains

`go.mod` still requires `github.com/go-json-experiment/json` and a `cdproto`
pseudo-version from before the change. Moving the code to `encoding/json/v2`,
and requiring a tagged release of `cdproto`, wait for the first release of
`cdproto`. Until then `chromedp` does not build against the new `cdproto`. See
`2026-10-03-stop-relying-on-removed-cdproto-helpers.md`.
