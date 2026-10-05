# Support Go 1.25 and later

Status: Decided.

The maintainer decided on 2026-10-06 that `chromedp` supports Go 1.25 and
later. The `go` line of `go.mod` was 1.27 and is now 1.25. This replaces the
minimum version in `2026-10-03-the-minimum-go-version-is-1-27.md`.

## Why

Issue https://github.com/chromedp/chromedp/issues/1536 reports that the
maintainers of `google/pprof` were not able to upgrade `chromedp`, because it declared
Go 1.27. A library that needs the newest Go release forces every program that
imports it to use that release. Many projects cannot do that yet.

## What was done

`cdproto` v0.157.8 has the package `github.com/chromedp/cdproto/cdp/jsonv2`. It
is the only place in the `cdproto` code that imports a JSON package. On Go 1.27
and later it uses `encoding/json/v2` and `encoding/json/jsontext` of the
standard library. On Go 1.25 and 1.26 it uses the module
`github.com/go-json-experiment/json`. Its names are aliases or thin wrappers, so
the types of the public API of `chromedp` are the same types on Go 1.27 as
before. For example, `DefaultUnmarshalOptions` and `DefaultMarshalOptions` still
have the type of the options of the standard library.

The code of `chromedp` imports `cdp/jsonv2` and no longer imports the standard
packages for JSON version 2. The code has no feature of Go 1.26. The helper
`ptr` replaces `new(true)` and similar calls, and `errors.As` replaces
`errors.AsType`. The documents can still show `new(true)` and say that it needs
Go 1.26 or later.

The `Test` workflow runs Go 1.25, Go 1.26 and the newest stable release on
Linux. It runs the newest stable release on Windows and macOS. Two more Linux
jobs test the two JSON layers of `cdproto`. One uses Go 1.25 with
`GOEXPERIMENT=jsonv2`. The other uses the newest stable release with
`GOEXPERIMENT=nojsonv2` and the build tag `cdproto_jsoncompat`.

## What remains

The `go.mod` files of `remote` and `test` require `chromedp` v0.19.1, which
declares Go 1.27. The maintainer must tag a new release of the core module and
update those requirements. Until then, a program that uses `remote` needs a
`replace` directive or Go 1.27.
