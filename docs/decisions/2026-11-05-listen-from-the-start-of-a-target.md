# Listen to a target from its start with WithListener

Status: Proposed. Amends 2026-11-04-export-the-ordered-event-listener.md.

This change adds the context option `WithListener`. onsi, the maintainer of
the testing framework Biloba, asked for it after the discussion
https://github.com/chromedp/chromedp/discussions/1645. The maintainer has not
decided on it yet.

## Why

The decision `2026-11-04-export-the-ordered-event-listener.md` lists this
option under "What was not done". It says that the option can be added later
if a program needs the events of the start of a tab. Biloba needs them.

The first `Run` attaches the target and sends `Runtime.enable`. Chrome answers
with a `runtime.EventExecutionContextCreated` for each JavaScript context that
the target already has. A func of `Listen` can start only after the first
`Run`, so it never sees these events. For a tab that runs already, which
`WithTargetID` names, these events are the only list of the contexts that
exist. Biloba keeps a list of the contexts of each frame, in the order of the
events. Today it wraps the `Transport` of the browser and reads the events from
the connection.

## What was done

- `WithListener(fn func(ev any)) ContextOption` adds fn to the target in
  `attachTarget`, before the target reads a message and before chromedp sends
  a command to the target. So fn sees every event that reaches the target.
- A new tab, the first tab of a browser and a target of `WithTargetID` all go
  through `attachTarget`, so the option works for each of them.
- The rules of `Listen` hold for fn. It gets the events only, through
  `eventsOnly`. It runs on the goroutine that reads the messages of the tab, in
  the order of arrival. It must return fast, must not run a command or call
  `Listen`, and must not panic.
- fn gets events as long as the target runs. Its context is the context that
  runs the target, which is the context of the first `Run`.
- The option can be given more than once. The target calls the funcs in the
  order of the options. A child context does not inherit them, because it has
  its own target.
- `Listen` stays. It serves code that starts to listen on a tab that runs
  already, which the option cannot do.

## What was not done

- A `ListenBrowser` option for the browser. Nothing needs it yet.
- A way to remove fn before the target ends. A func that must stop early can
  check a flag of its own. `Listen` with a context that the program cancels
  serves the other cases.
