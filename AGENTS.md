# chromedp

`chromedp` is a Go package that drives Chrome and other browsers that speak
the Chrome DevTools Protocol. The protocol is the JSON message format that
Chrome uses for remote control. `chromedp` needs no external driver. It
starts the browser or connects to one, then sends protocol commands over a
pipe or a WebSocket.

The generated protocol types live in a separate module, `cdproto`, which
`pdlgen` writes. The core module of this repository holds the high level API on
top of it: allocators, contexts, actions, selectors, input and screenshots. The
core uses only the Go standard library and `cdproto`. See
`docs/decisions/2026-10-04-the-core-uses-only-the-standard-library.md`.

The repository holds three modules:

| Directory | Module | Holds |
| --- | --- | --- |
| the root | `github.com/chromedp/chromedp` | the core, with the pipe transport |
| `remote/` | `github.com/chromedp/chromedp/remote` | everything that needs a websocket, and the library `gobwas/ws` |
| `test/` | `github.com/chromedp/chromedp/test` | the tests that need `pdf` or `pixelmatch` |

## Standing rules

These hold in every `chromedp` repository, for every coding agent.

1. Stage changes for review. Commit and push only when the maintainer says so.
2. Load the `simple-english` skill before you write text that a person reads.
   Examples are a document, a code comment, an error message and a commit
   message. Follow the skill for that text.
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
| changing the public API | `docs/PLAN.md` under Architecture, `docs/API.md`, `docs/MIGRATION.md`, then the decisions in `docs/decisions/` |
| looking for the old and the new name of an API | `docs/MIGRATION.md` |
| looking for the API with before and after code | `docs/API.md` |
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
3. Hold the lock before you read a field of a `chromedp.Node` or a
   `chromedp.Frame`. The code in `query.go` and `target.go` calls `RLock`
   first, because the target updates the node tree from events while an action
   reads it.
4. Run an action with `chromedp.Run` or `chromedp.Do`. They start the browser
   and the tab when the context has none, and they pass the `Target` to the
   action. Do not call an action with a `Target` that these funcs did not
   prepare.
5. Wrap every error with `%w`. See the decision
   `docs/decisions/2021-04-29-wrap-errors-with-w.md`.
6. Keep the tests compatible with `headless-shell`. CI runs them against Chrome on
   Linux, Windows and macOS, and against the `chromedp/headless-shell` image on
   Linux. A test that fails on one
   of them must skip with a comment that says why. `input_test.go` shows how.
7. Embed JavaScript from the `js/` folder with `go:embed`. Do not write
   JavaScript inside a Go string.
8. Do not change the type `Action[T]` or the signature of an exported
   function without asking the maintainer. See `docs/decisions/2026-10-03-generic-iterator-api-instead-of-action.md`.
9. Do not add a helper that the `cdproto` module is dropping. See
   `docs/decisions/2026-10-03-stop-relying-on-removed-cdproto-helpers.md`.

## Layout

The root package `chromedp` holds the API. The files group by topic.

| Path | Holds |
| --- | --- |
| `allocate.go`, `allocate_linux.go`, `allocate_other.go`, `allocate_pipe.go`, `allocate_detach_unix.go`, `allocate_detach_windows.go`, `keepopen.go` | `Allocator`, `ExecAllocator`, `Dialer` and `Attacher`, which start or reach a browser, and the options that keep a browser open |
| `browser.go`, `pipe.go`, `pipe_unix.go`, `pipe_windows.go` | `Browser`, the `Transport` interface, the pipe connection and the way each platform passes the pipes to the browser |
| `remote/` | the module `remote`: the WebSocket connection `Conn`, the remote `Allocator`, `WebSocket` and `WithKeepOpen` |
| `test/` | the module `test`: the screenshot tests and the PDF text test, with their `testdata/` |
| `internal/testenv/`, `internal/chromedptest/` | the variables and the helpers that the tests of the three modules share |
| `chromedp.go`, `action.go` | `Context`, `NewContext`, `RunResponse`, `Action[T]`, `Run`, `Do` and the events |
| `target.go`, `util.go` | `Target`, which tracks frames and the DOM tree from events |
| `frame.go`, `node.go` | `Frame` and `Node`, which add the tree state to the protocol types |
| `session.go` | the event subscriptions of a `Browser` and of a `Target` |
| `errors.go` | the error values and `ExceptionError` |
| `query.go` | selectors, query options and the element actions |
| `nav.go`, `input.go`, `drag.go`, `console.go`, `expose.go`, `pdf.go`, `emulate.go`, `screenshot.go`, `eval.go`, `call.go`, `poll.go` | actions by topic |
| `js.go`, `js/` | embedded JavaScript snippets |
| `kb/` | keyboard key definitions, generated |
| `device/` | device descriptors for emulation, generated |
| `testdata/` | HTML pages for the tests of the core. `remote/testdata/` and `test/testdata/` hold the files of the other modules |
| `contrib/docker-test.sh` | runs the tests inside the `headless-shell` image |
| `docs/` | plan, progress, backlog, API and migration guides, and decisions |

The root of the repository holds `README.md`, `AGENTS.md`, `CLAUDE.md`,
`CONTRIBUTING.md` and `LICENSE` as text documents. Every other document goes
in `docs/`.

## Terms

These words have one meaning in every document and in every Go comment.

- Maintainer: the person who owns the project and decides the open questions.
- Target: a page, a tab or a worker that the browser exposes. The `Target` type
  tracks one.
- Tab: a target of the type page, as a person sees it in the browser. Text for
  users says tab. Text about the code says target.
- Session: a channel that sends commands and receives events for one scope.
  `cdp.Session` is the interface. `Browser` and `Target` implement it.
- Action: a func of the type `Action[T]` that runs against a target and returns
  a value of the type `T`.
- Selector: the value that picks the elements of a query action. It is a
  `Selectable`, a string type or a `[]cdp.NodeID` type. The type chooses the
  lookup. A plain string is a `Search`.

## Go conventions

The module needs Go 1.27 or newer, as `go.mod` says. The generated `cdproto`
uses `encoding/json/v2`, which is in the standard library from Go 1.27. CI runs
the newest stable release, which is Go 1.27 now. See
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
lower case letters. Use the same name on every method of the type. Group struct
fields by purpose, with exported fields first and a blank line between groups.

Run `gofmt` on every file. The history shows `go fix` and `modernize` runs, so
keep the code in the form they produce.

## Linting

No linter configuration exists in this repository. There is no
`.golangci.yml` and CI runs no linter. Use `gofmt` and `go vet`. Adding a
linter configuration is in `docs/BACKLOG.md`.

## Before you commit

Run these commands in the repository root, which is the core module:

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./docs/
```

`gofmt -l .` must print nothing. Run `go vet ./...` in `remote/` and in `test/`
too. Run `GOOS=windows go vet ./...` and `GOOS=darwin go vet ./...` in the root
and in `remote/`.

`go.mod` of the core requires the released typed `cdproto`, so these commands
need no other setup. The core must import no library other than the standard
library and `cdproto`. Check it with `go list -deps ./...`, and do not add a
module to its `go.mod`.

Never commit a `go.work` file or a `go.work.sum` file, in this repository or in
any other. `.gitignore` lists them. The maintainer can try an unreleased
`cdproto` with a local `go.work` file. Do not add a `replace` directive to the
`go.mod` of the core.

The `go.mod` of `remote/` and of `test/` require a released version of the core,
`github.com/chromedp/chromedp v0.18.0` at this time, and hold no `replace`
directive. To try a change of the core in `remote` or `test`, add
`replace github.com/chromedp/chromedp => ../` to that `go.mod` while you work,
and never commit it. CI adds the directive at run time, so that `remote` and
`test` run against the core of the commit under test. Before the maintainer tags
`remote`, make sure that no `go.mod` holds a `replace` directive, and set the
version of the core to the newest tag. Do not create a tag.

The full test suite needs a browser, and it is the only way to test the
package. Run it where Chrome is installed. Each module runs in its own
directory, with `GOWORK=off` when a `go.work` file exists:

```bash
go test -v ./...
(cd remote && go test -v ./...)
(cd test && go test -v ./...)
```

If Chrome is not on the machine, use the container. The script builds the test
binary of each of the three modules and runs it inside the
`chromedp/headless-shell` image. It uses `docker`, or `podman` when it is
installed:

```bash
./contrib/docker-test.sh
```

The `IMAGE` variable chooses another image. CI runs the first command on Linux, Windows and macOS, and the container
script on Linux only, on every push and pull request, with the newest stable Go
release. It runs the three modules in turn, each one in its own directory. The
steps of `remote`, `test` and the container run even when an earlier step
failed, so that one run shows every failure. See `.github/workflows/test.yml`.
A second workflow, `.github/workflows/nightly.yml`, runs the container script every
night at 04:17 UTC, and when you start it by hand, against the three channels of
the image: `stable` (the same as `latest`), `beta` and `dev`. It runs on Linux
amd64. The tests print a
line that starts with `browser:` with the version of each channel.
If the repository gets a Dependabot configuration, it must list the three
directories `/`, `/remote` and `/test`.

The tests of `remote/` and of `test/` use the exported API of the core only. The
helpers that the three modules share are in `internal/chromedptest/`, and the
core tests, which cannot import that package, read the variables through
`internal/testenv/`.

These variables change the tests:

- `CHROMEDP_TEST_RUNNER` names the browser binary.
- `CHROMEDP_VISIBLEWINDOW` shows the window. The old name `CHROMEDP_NO_HEADLESS` works in the tests too.
- `CHROMEDP_NO_SANDBOX`, set to `false`, turns the sandbox on.
- `CHROMEDP_DEBUG` logs every message.
- `CI` and `HEADLESS_SHELL` make the test `TestKillLeftoversSkipsKeepOpen` skip itself. It is not stable on a CI runner, and the headless-shell image has no `sh`.
- `HEADLESS_SHELL` also makes `TestWithNewWindow/SharedWindow` and `/Inherited` skip, because headless-shell gives each target its own window.

A CI runner is slow when many tests run at once. Give a limit in a test enough time for a slow start, because a limit only matters when the test fails.

## Writing documentation

Follow the `simple-english` skill for every word. Write sentences of 20 words
or fewer for a procedure, and 25 words or fewer for a description.

Record a decision in a file in `docs/decisions/`. Name it
`YYYY-MM-DD-short-slug.md` with the date of the decision. Open it with
`# <Title>`, a blank line and `Status: Decided.`. Use `Proposed.`, `Open.`,
`Amends <file>.` or `Superseded by <file>.` when that is the status. Refer to a
decision by its file name, never by a number. State an amendment in both
files. Add a row to `docs/decisions/README.md`.

`go test ./docs/` tests the links, the decision index, the root layout, the
skill copies, and the prose rules in the documents and in the Go comments. It
needs no browser.

The skills live in `.agents/skills` and `.claude/skills` as identical copies.
Never replace a copy with a symbolic link. `skills-lock.json` names the source
of each skill. The Claude Code permissions of one person go in
`.claude/settings.local.json`, which `.gitignore` lists.
