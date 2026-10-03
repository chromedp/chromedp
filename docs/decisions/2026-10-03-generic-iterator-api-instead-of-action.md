# Replace the Action interface with a generic and iterator API

Status: Decided.

The maintainer asked on 2026-10-03 whether an API built on generics and
iterators must replace the `chromedp.Action` interface, which is old and not
friendly. The maintainer then asked us to build the change. Then the maintainer
saw the final API before approving it. We built it on a branch. The
maintainer approved it, and the work was merged on 2026-10-04.

## What the change has

The code uses the typed `cdproto` that `pdlgen` writes. A protocol command is a value, and `cdp.Call` runs it on a session.
`chromedp.Browser` and `chromedp.Target` are sessions.

- `type Action[T any] func(ctx context.Context, t *Target) (T, error)`.
  `Run[T]` runs one action and returns its value. `Do` runs actions that return
  nothing, in order. `Void` is the empty struct. `Steps` joins actions, and `Func`
  makes an action from a func that returns only an error.
- Every action returns its value, so the out pointers are gone. `Text`, `Nodes`,
  `Attributes`, `Screenshot` and the others return what they read. `Evaluate[T]`
  decodes the result into a type.
- `Events[E]` and `BrowserEvents[E]` return an `iter.Seq2` and subscribe when
  they return. A program can subscribe, trigger the event, and then read it.
  `WaitEvent` does this for one event.
- `Legacy` runs an action of the old kind. `chromedp.Call` and `CallBrowser` send a
  command from a context.
- The query actions take a typed selector. `Selectable` is a string type or a
  `[]cdp.NodeID` type. The type of the selector chooses the lookup. `Search`
  is the default and the type of a plain string. `CSS`, `CSSAll`, `ID`,
  `JSPath` and `NodeIDs` replace the lookup options that start with `By`.
  `ByFunc` stays. An int or a `*Node` as a selector does not compile, and the
  run time panic of the old node id option is gone. The action `NodeIDs` is now
  `QueryNodeIDs`, because `NodeIDs` is the selector type.
- The old `Action` interface, `ActionFunc`, `Tasks`, `ListenTarget` and
  `ListenBrowser` are gone. `docs/MIGRATION.md` lists every renamed and removed
  name, and `docs/API.md` shows the old and the new code side by side.

The full suite passed with the same tests, ported to the new API.

## What the maintainer looked at

1. Every user of `chromedp` changes code. `*cdp.Node` is `*chromedp.Node`, an
   action returns its value, a protocol command is called with `cdp.Call`, and
   listeners are iterators.
2. A new tab is still created with `newWindow` set. Without the parameter,
   Chrome puts a tab in a shared window. A tab that is not the active one gets
   no animation frames, so `Poll` never returns. In headed Chrome this opens a
   window for each new tab. The typed API does not remove this Chrome
   behavior.
3. `RunResponse`, `NavigateResponse` and the `Navigate` actions still use an
   internal listener. It keeps the order of events across methods, and a
   subscription to one method does not.
4. `Poll` still polls in the page. It is not an iterator yet.
5. The selector types are a second change of the public API. Every call that
   passed a `By...` lookup option changes, and the action `NodeIDs` has a new
   name.
6. `Func` and `Steps` are not in the first design. They are the smallest way to
   put an action that returns an error into a `Do`.

## What happened next

The `pdlgen` work was merged first, because the typed `cdproto` comes from it.
`cdproto` v0.157.3 is the first release of the typed API. `chromedp` v0.17.0
uses it.
