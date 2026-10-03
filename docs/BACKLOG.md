# Backlog

This document lists work that is known and not done. A decision is not a
backlog item. It goes in [`decisions/`](decisions/README.md). A question for
The maintainer goes at the end of [`PLAN.md`](PLAN.md). When an item here is done, delete
it, and record in `decisions/` anything that was decided on the way.

## Tooling

### Add a linter configuration

The repository has no `.golangci.yml` and CI runs no linter. Add a
configuration and a CI step, then describe both in the Linting section of
[`../AGENTS.md`](../AGENTS.md). Ask the maintainer which linters the maintainer wants.

### Run the tests under a race detector in CI

`chromedp_test.go` says that 50 tabs is enough for `go test -race` to find
problems. The workflow runs `go test -v ./...` with no `-race`. Ask the maintainer
whether CI must add it.

## Dependencies

### Stop using the cdproto helpers that are going away

See `decisions/2026-10-03-stop-relying-on-removed-cdproto-helpers.md`. This
waits for the maintainer to confirm the proposal. The list of uses is in
[`PLAN.md`](PLAN.md), under Open questions.
