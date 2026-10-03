# chromedp projects use dated decision files

Status: Decided.

The maintainer decided on 2026-10-03 that `chromedp` projects record decisions by date and
not by number.

Each decision is a file named `YYYY-MM-DD-short-slug.md` in `docs/decisions/`.
It opens with `# <Title>`, a blank line and a status line. The status is
`Decided.`, `Proposed.`, `Open.`, `Amends <file>.` or `Superseded by <file>.`.

A decision is named by its file name, never by a number. An amendment is
stated in both files. `docs/decisions/README.md` is the index, with the columns
Date, Decision and Status. `go test ./docs/` tests the index and prints the
row to add.
