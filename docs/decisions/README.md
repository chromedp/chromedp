# Decisions

Every decision this project made is a file in this folder. A file is named by
its date and a short title, as in `2026-10-03-use-dated-decision-files.md`. This
table is the index.

Each file opens with its status. "Decided" means the maintainer chose it. "Proposed"
means somebody suggested it and the maintainer has not confirmed it. "Open" means nobody
has chosen yet. A decision that changes an earlier one says so in its status,
as "Amends 2021-04-29-wrap-errors-with-w.md". The earlier one says it back, as
"Superseded by <file>". Read the status before the decision.

Refer to a decision by its file name, never by a number. A new decision is a
new file with the date of the decision. Add its row here. `go test ./docs/`
fails when a decision has no row or a row is wrong, and it prints the row to
add.

| Date | Decision | Status |
| --- | --- | --- |
| 2019-04-30 | [Run the tests under headless-shell as well as Chrome](2019-04-30-test-under-headless-shell.md) | Decided |
| 2021-04-29 | [Wrap errors with %w](2021-04-29-wrap-errors-with-w.md) | Decided |
| 2025-02-22 | [Use the experimental JSON v2 package](2025-02-22-use-json-v2.md) | Decided |
| 2026-10-03 | [A visible window is an opt-in](2026-10-03-a-visible-window-is-an-opt-in.md) | Decided |
| 2026-10-03 | [Replace the Action interface with a generic and iterator API](2026-10-03-generic-iterator-api-instead-of-action.md) | Decided |
| 2026-10-03 | [Questions and ideas go to Discussions, and issues are for bugs](2026-10-03-questions-and-ideas-go-to-discussions.md) | Decided |
| 2026-10-03 | [The README shows the gear logo and the Discord badge](2026-10-03-readme-shows-the-gear-logo-and-discord-badge.md) | Decided |
| 2026-10-03 | [Stop relying on helpers that cdproto no longer generates](2026-10-03-stop-relying-on-removed-cdproto-helpers.md) | Decided |
| 2026-10-03 | [The minimum Go version is 1.27](2026-10-03-the-minimum-go-version-is-1-27.md) | Decided |
| 2026-10-03 | [chromedp projects use dated decision files](2026-10-03-use-dated-decision-files.md) | Decided |
| 2026-10-03 | [Use a pipe to the browser by default](2026-10-03-use-a-pipe-to-the-browser-by-default.md) | Decided |
| 2026-10-04 | [Keep site isolation on](2026-10-04-keep-site-isolation-on.md) | Decided |
| 2026-10-04 | [The core uses only the standard library](2026-10-04-the-core-uses-only-the-standard-library.md) | Decided |
| 2026-10-04 | [The pipe works on Windows](2026-10-04-the-pipe-works-on-windows.md) | Amends 2026-10-03-use-a-pipe-to-the-browser-by-default.md and 2026-10-04-the-core-uses-only-the-standard-library.md |
