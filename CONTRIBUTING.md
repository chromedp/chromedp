# Contributing to chromedp

`chromedp` is a Go package that drives Chrome through the Chrome DevTools
Protocol. Read these three things before you change anything.

[`AGENTS.md`](AGENTS.md) holds the rules, the layout, the Go conventions and
the test commands. It is written for a coding agent and everything in it
applies to a person.

[`docs/PLAN.md`](docs/PLAN.md) describes the purpose and the architecture.
Its last section lists the open questions. Do not decide an open question on
your own. Ask the maintainer.

[`docs/decisions/`](docs/decisions/README.md) holds every decision, one file
each, named by date. Read the status of a decision before you trust it.

## Before you send a change

```bash
gofmt -l .
go vet ./...
go test -v ./...
```

`gofmt -l .` must print nothing. The tests need Chrome. If Chrome is not on
your machine, run `./contrib/docker-test.sh`. It runs the tests inside the
`chromedp/headless-shell` image. CI runs the tests with Chrome on Linux, Windows and macOS, and with the image on Linux only, on every push.

If you do not have a browser, run `go test ./docs/`. It needs none and tests
the documents and the Go comments.

## Writing

Write in plain English. Use short sentences and the active voice. The
`simple-english` skill in `.agents/skills` has the full rules, and
`go test ./docs/` finds the violations that a machine can find, in the
documents and in the Go comments.

To record a decision, add a file to `docs/decisions/`. Name it with the date, as
in `2026-10-03-short-title.md`. Then add its row to the index. The test prints
the row for you.

## Questions

Ask in [Discussions](https://github.com/chromedp/chromedp/discussions), and not
in an issue. The issue tracker is for bugs, and a new issue starts from the bug
report template. Suggest a feature in Discussions first. The
[README](README.md) links the project resources.
