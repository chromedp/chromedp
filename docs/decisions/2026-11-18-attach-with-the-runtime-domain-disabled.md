# Attach with the Runtime domain disabled

Status: Decided.

The maintainer decided on 2026-11-18 to add the context option
`WithRuntimeDisabled`. Dušan Panić proposed it in the discussion
https://github.com/chromedp/chromedp/discussions/1647, as
`WithoutRuntimeEnable`, with a branch. The maintainer reviewed the branch and
wrote the change again in a smaller form. The maintainer chose the name, so
that the option says what it sets and not what it leaves out.

## Context

The first `Run` of a context sends `Runtime.enable`. While the Runtime domain
is enabled, Chrome prepares each argument of a console call for the protocol.
It formats the stack of an error that the page logs, so a func of the page in
`Error.prepareStackTrace` runs. A page that does not run under the protocol
does not see this call. So any page can tell that a program drives it. The test
`TestRuntimeDisabledHidesTheProtocol` shows this with Chrome 156.

The queries and `Poll` need an execution context in each frame. The target
learns the main world of each frame from the Runtime events.

## Decision

1. `WithRuntimeDisabled()` makes `attachTarget` skip `Runtime.enable`. The
   default does not change.
2. With the option, the queries and `Poll` run in an isolated world, which
   `Page.createIsolatedWorld` creates in the frame on the first use. The target
   forgets the world when the frame navigates or detaches, or when the document
   is updated. A world that arrives after the frame navigated is used for that
   call and not kept. Puppeteer and Playwright run their queries in an isolated
   world too.
3. An isolated world shares the DOM with the page, but not its globals. So a
   predicate of `Poll` sees only the DOM. `Evaluate` runs in the main world, as
   before.
4. Chrome sends no Runtime events, so `Console` gets only the entries of the
   browser log, and `Listen`, `Events` and `WithListener` get no Runtime
   events.
5. `ExposeFunc` returns an error that wraps the new `ErrRuntimeDisabled`. Chrome
   adds no binding to the page while the domain is disabled, and the
   maintainer prefers an error to a func that does nothing.
6. A child context does not inherit the option, as for `WithListener`.

The proposal also found a separate bug: `runPoll` held a read lock on
`frameMu` and then called `enclosingFrame`, which takes the same read lock. A
writer that waits between the two locks blocks both, and the `Poll` in a frame
hangs. `runPoll` now takes no lock around `enclosingFrame`.

## What was not done

- The branch kept the last failure of `Page.createIsolatedWorld` for each
  frame, sent the command again only after 200 ms, logged the failure when the
  action ended, and had a fake clock for its tests. The loops of the queries
  and of `Poll` already try again at their interval, and the command fails only
  for a short time, such as during a navigation. So this change has none of
  that state.
- The main world of the top frame is reachable without the Runtime domain,
  through an object that `Runtime.evaluate` returns. The queries name an
  execution context and not an object, so this needs a larger change.
