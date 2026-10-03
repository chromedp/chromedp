# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

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

`cdproto` v0.157.2 is the first release of the typed API. `chromedp` v0.17.0
uses it. The full test suite passes. The comments of the Go files and the
documents follow the `simple-english` skill, and `go test ./docs/` tests them.

## Waiting

- The maintainer must answer the open questions at the end of [`PLAN.md`](PLAN.md).
- Known work that is not done is in [`BACKLOG.md`](BACKLOG.md).
