# chromedp

`chromedp` is a Go package that drives Chrome and other browsers that speak
the Chrome DevTools Protocol. The protocol is the JSON message format that
Chrome uses for remote control. `chromedp` needs no external driver. It
starts the browser or connects to one, then sends protocol commands over a
WebSocket.

The generated protocol types live in a separate module, `cdproto`, which
`pdlgen` writes. This module holds the high level API on top of it:
allocators, contexts, actions, selectors, input and screenshots.

## Standing rules

These hold in every `chromedp` repository, for every coding agent.

1. Stage changes for review. Commit and push only when the maintainer says so.
2. Load the `simple-english` skill before you write any text that a person
   reads: a document, a code comment, an error message or a commit message.
   Follow it for that text.
3. Load the `go-pedantry` skill before you write or review Go code. Follow it
   where it does not conflict with a rule in this file. A rule here wins.

Questions and feature ideas go to GitHub Discussions, and bugs go to issues. Do
not open an issue for a question. See
`docs/decisions/2026-10-03-questions-and-ideas-go-to-discussions.md`.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Edit this file, not that one.

## Which document to read

Read `docs/PLAN.md` first. It describes the purpose and the architecture. Its
last section lists the open questions. Do not decide an open question on your
own. Ask the maintainer.

`docs/decisions/README.md` is the index of every decision. A decision is one
file named by its date and a short title. Read the status of a decision before
you trust it, because a later decision can amend or replace it.

| If you are | Read |
| --- | --- |
| starting work | `docs/PLAN.md`, then `docs/PROGRESS.md` |
| looking for known work that is not done | `docs/BACKLOG.md` |
| asking why something is the way it is | the index in `docs/decisions/README.md` |
| changing the public API | `docs/PLAN.md` under Architecture, then the proposed decisions in `docs/decisions/` |
| changing the cdproto dependency | `docs/decisions/2026-10-03-stop-relying-on-removed-cdproto-helpers.md` |
| changing the README | `docs/decisions/2026-10-03-readme-shows-the-gear-logo-and-discord-badge.md` |
| recording a decision | `docs/decisions/README.md` and `docs/decisions/2026-10-03-use-dated-decision-files.md` |
| running the tests | the section Before you commit, in this file |

## Hard rules

1. Do not run the tests in a session that has no browser. They need Chrome or
   the `chromedp/headless-shell` container. A session without one must run
   `go build ./...`, `go vet ./...` and `go test ./docs/` only.
2. Do not edit `kb/kb.go` or `device/device.go` by hand. `go generate` writes
   them from `kb/gen.go` and `device/gen.go`. Change the generator and run it.
3. Hold the lock before you read a field of a `cdp.Node` or a `cdp.Frame`.
   The code in `query.go` and `target.go` calls `RLock` first, because the
   target updates the node tree from events while an action reads it.
4. Run an action with `chromedp.Run`. An action needs an executor in its
   context, and `Run` sets it. Do not call `Do` on a context that `Run` did
   not prepare.
5. Wrap every error with `%w`. See the decision
   `docs/decisions/2021-04-29-wrap-errors-with-w.md`.
6. Keep the tests compatible with `headless-shell`. CI runs them against both
   Chrome and the `chromedp/headless-shell` image. A test that fails on one
   of them must skip with a comment that says why. `input_test.go` shows how.
7. Embed JavaScript from the `js/` folder with `go:embed`. Do not write
   JavaScript inside a Go string.
8. Do not change the `Action` interface or the signature of an exported
   function without asking the maintainer. See `docs/decisions/2026-10-03-generic-iterator-api-instead-of-action.md`.
9. Do not add a helper that the `cdproto` module is dropping. See
   `docs/decisions/2026-10-03-stop-relying-on-removed-cdproto-helpers.md`.

## Layout

The root package `chromedp` holds the API. The files group by topic.

| Path | Holds |
| --- | --- |
| `allocate.go`, `allocate_linux.go`, `allocate_other.go` | `Allocator`, `ExecAllocator` and `RemoteAllocator`, which start or reach a browser |
| `browser.go`, `conn.go` | `Browser` and the WebSocket connection |
| `chromedp.go` | `Context`, `NewContext`, `Run`, the `Action` interface and the listeners |
| `target.go`, `util.go` | `Target`, which tracks frames and the DOM tree from events |
| `query.go` | selectors, query options and the element actions |
| `nav.go`, `input.go`, `emulate.go`, `screenshot.go`, `eval.go`, `call.go`, `poll.go` | actions by topic |
| `js.go`, `js/` | embedded JavaScript snippets |
| `kb/` | keyboard key definitions, generated |
| `device/` | device descriptors for emulation, generated |
| `testdata/` | HTML pages and golden images for the tests |
| `contrib/docker-test.sh` | runs the tests inside the `headless-shell` image |
| `docs/` | plan, backlog, progress and decisions |

The root of the repository holds `README.md`, `AGENTS.md`, `CLAUDE.md`,
`CONTRIBUTING.md` and `LICENSE` as text documents. Every other document goes
in `docs/`.

## Go conventions

The module needs Go 1.27 or newer, as `go.mod` says. The generated `cdproto`
uses `encoding/json/v2`, which is in the standard library from Go 1.27. CI runs
Go 1.27 and the newest stable release. See
`docs/decisions/2026-10-03-the-minimum-go-version-is-1-27.md`.

Wrap every error with `%w`, never `%s` or `%v`:

```go
if err != nil {
	return fmt.Errorf("reading the node tree: %w", err)
}
```

Write error messages in lower case. Name the object that failed. Do not start
with "failed to" or "error".

Put `context.Context` first in every parameter list and name it `ctx`. The
`Context` type of `chromedp` is not a `context.Context` and it is not the same
thing. Never store a `context.Context` in a struct.

Accept interfaces and return concrete structs. Name a receiver with one or two
lower case letters, and use the same name on every method of the type. Group
struct fields by purpose, with exported fields first and a blank line between
groups.

Run `gofmt` on every file. The history shows `go fix` and `modernize` runs, so
keep the code in the form they produce.

## Linting

No linter configuration exists in this repository. There is no
`.golangci.yml` and CI runs no linter. Use `gofmt` and `go vet`. Adding a
linter configuration is in `docs/BACKLOG.md`.

## Before you commit

Run these commands in the repository root:

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./docs/
```

`gofmt -l .` must print nothing.

The full test suite needs a browser, and it is the only way to test the
package. Run it where Chrome is installed:

```bash
go test -v ./...
```

If Chrome is not on the machine, use the container. This builds the test
binary and runs it inside the `chromedp/headless-shell` image with `docker`,
or with `podman` when it is installed:

```bash
./contrib/docker-test.sh
```

The `IMAGE` variable chooses another image. CI runs both commands on every
push and pull request, once with the oldest and once with the newest stable Go
release. See `.github/workflows/test.yml`.

These variables change the tests: `CHROMEDP_TEST_RUNNER` names the browser
binary, `CHROMEDP_NO_HEADLESS` shows the window, `CHROMEDP_NO_SANDBOX` set to
`false` turns the sandbox on, and `CHROMEDP_DEBUG` logs every message.

## Writing documentation

Follow the `simple-english` skill for every word. A document that a person
reads is prose, so write sentences of 20 words or fewer for a procedure and 25
for a description.

Record a decision in a file in `docs/decisions/`. Name it
`YYYY-MM-DD-short-slug.md` with the date of the decision. Open it with
`# <Title>`, a blank line and `Status: Decided.`. Use `Proposed.`, `Open.`,
`Amends <file>.` or `Superseded by <file>.` when that is the status. Refer to a
decision by its file name, never by a number. State an amendment in both
files. Add a row to `docs/decisions/README.md`.

`go test ./docs/` checks the links, the decision index, the root layout, the
skill copies and the prose rules. It needs no browser.

The skills live in `.agents/skills` and `.claude/skills` as identical copies.
Never replace a copy with a symbolic link. `skills-lock.json` names the source
of each skill. The Claude Code permissions of one person go in
`.claude/settings.local.json`, which `.gitignore` lists.
