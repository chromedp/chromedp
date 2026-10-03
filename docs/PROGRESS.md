# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

On 2026-10-03 `main` was ported to the tagged `cdproto` `v0.157.1`, and the tests
were fixed for current Chrome. See the commits of that day. The `Test` workflow on
GitHub is not stable: the first browser start sometimes does not print its DevTools
address, and Chrome only prints DBus errors. The tests pass on the local Chrome.

The new API is built on the local branch `typed-api`, which is not pushed. It uses
the typed `cdproto` that the `pdlgen` branch `typed-api` writes. The branch holds
the generic actions, the iterator events, `docs/API.md` and `docs/MIGRATION.md`.
The full suite passes on it. It waits for the review of the maintainer.

## Waiting

- The maintainer must review the branch `typed-api`, and then approve or reject
  the decision `2026-10-03-generic-iterator-api-instead-of-action.md`.
- The maintainer must answer the open questions at the end of [`PLAN.md`](PLAN.md).
