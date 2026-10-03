# Replace the Action interface with a generic and iterator API

Status: Proposed.

The maintainer asked on 2026-10-03 whether an API built on generics and iterators
must replace the `chromedp.Action` interface, which is old and not friendly. The
maintainer then asked for the change to be built, so that the final API can be
seen before it is approved. It is built on the local branch `typed-api`. Nobody
has approved it, and it is not on GitHub.

## What the branch has

The branch uses the typed `cdproto` that `pdlgen` writes on its own branch
`typed-api`. A protocol command is a value, and `cdp.Call` runs it on a session.
`chromedp.Browser` and `chromedp.Target` are sessions.

- `type Action[T any] func(ctx context.Context, t *Target) (T, error)`.
  `Run[T]` runs one action and returns its value. `Do` runs actions that return
  nothing, in order. `Void` is the empty struct. `Steps` joins actions, and `Func`
  makes an action from a func that returns only an error.
- Every action returns its value, so the out pointers are gone. `Text`, `Nodes`,
  `Attributes`, `Screenshot` and the others return what they read. `Evaluate[T]`
  decodes the result into a type.
- `Events[E]` and `BrowserEvents[E]` return an `iter.Seq2` and subscribe when they
  return, so a program can subscribe, trigger the event, and then read it.
  `WaitEvent` does this for one event.
- `Legacy` runs an action of the old kind. `chromedp.Call` and `CallBrowser` send a
  command from a context.
- The old `Action` interface, `ActionFunc`, `Tasks`, `ListenTarget` and
  `ListenBrowser` are gone. `docs/MIGRATION.md` lists every renamed and removed
  name, and `docs/API.md` shows the old and the new code side by side.

The full suite passes on the branch with the same tests, ported to the new API.

## What the maintainer must look at

1. Every user of `chromedp` changes code. `*cdp.Node` is `*chromedp.Node`, an
   action returns its value, a protocol command is called with `cdp.Call`, and
   listeners are iterators.
2. A new tab is still created with `newWindow` set. With the parameter left out,
   Chrome puts a tab in a shared window, and a tab that is not the active one
   gets no animation frames, so `Poll` never returns. In headed Chrome this opens
   a window for each new tab. This is a Chrome behavior that the typed API does not
   remove.
3. `RunResponse`, `NavigateResponse` and the `Navigate` actions still use an
   internal listener that keeps the order of events across methods, because a
   subscription to one method does not keep order across methods.
4. `Poll` still polls in the page. It is not an iterator yet.
5. `Func` and `Steps` are not in the first design. They are the smallest way to
   put an action that returns an error into a `Do`.

## What happens next

If the maintainer approves, the branch is reviewed and pushed, and the `pdlgen`
branch is merged first, because the typed `cdproto` comes from it. If the
maintainer does not approve, the branch is dropped, and `chromedp` stays on the
current API with the pushed port to `cdproto` `v0.157.1`.
