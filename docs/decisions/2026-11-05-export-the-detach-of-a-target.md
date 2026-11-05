# Export the detach of a target as Detached

Status: Proposed.

This change lets a program see that the browser detached the target of a
context. onsi, the maintainer of the testing framework Biloba, asked for it
after the discussion https://github.com/chromedp/chromedp/discussions/1645. The
maintainer has not decided on it yet.

## Why

The fix for that discussion gave `Target` an internal channel, `detached`. The
target closes it after it handled every message that the browser routed to it
before the detach. `Call` uses it to end a command that waits for a reply that
never comes.

A program also needs to know when its tab goes away, for example when the page
closes itself. Biloba needs two things:

1. A check that does not block and tells whether the target detached.
2. A func that runs once when the target detaches, also when the detach
   happened before the program asked.

Today a program finds this out from the event `Target.detachedFromTarget` of
the browser. A listener of `ListenBrowser` that starts after the detach never
sees that event. A listener that sees it runs on the goroutine that reads the
messages, before the target handled its last messages. Biloba wraps the
`Transport` of the browser to read the event from the connection and to keep a
record of the sessions that detached.

## What was done

- `Detached(ctx) (<-chan struct{}, error)` returns the channel of the target of
  the context. The channel closes after the target handled every message that
  the browser routed to it before the detach. So a func of `Listen` saw every
  event of the target before the channel closes.
- A receive-only channel gives both needs. A `select` with a `default` case is
  the check that does not block. A goroutine that waits on the channel runs a
  func once, and a channel that already closed lets it run at once.
- `Detached` does not open a target. It returns the new error `ErrNoTarget`
  when the context has none yet, and `ErrInvalidContext` when ctx is not a
  chromedp context. It reads the target under the lock that `attachTarget`
  holds when it sets the target, so it can run on any goroutine.
- The first version opened the target, as `Listen` does. Biloba then built on
  it, and every check had to make sure first that the context had a target.
  Without that, a check on a new context opened a tab that the program did not
  ask for, and the context of the check, which can be short, owned the tab. The
  check also read `Target` outside the lock. A program listens to drive a tab,
  so `Listen` opens one. A program asks whether a tab went away about a tab
  that it already has.
- The channel does not close when the context ends first. The target then
  stops to read its messages, and a detach after that does not reach it. When
  the context ends at about the time of the detach, a func that waits can see
  only `ctx.Done`. Use `ctx.Done` for the end of the context.

## What was not done

- A method `Detached` on `*Target`. It needs no error. But the decision
  `2026-11-04-export-the-ordered-event-listener.md` kept the method `listen`
  unexported, because a program has a `*Target` only inside an action. A
  package-level func finds the target from the context, as `Listen` does. The
  exported methods `Call` and `Subscribe` exist because `Target` implements
  `cdp.Session`.
- A callback option such as `OnDetach(ctx, fn)`. A channel gives the same thing
  with a `select` or a goroutine, and it composes with other channels.
