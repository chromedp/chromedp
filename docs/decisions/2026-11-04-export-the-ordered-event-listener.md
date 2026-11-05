# Export the ordered event listener as Listen and ListenBrowser

Status: Amends 2026-10-03-generic-iterator-api-instead-of-action.md. Amended by 2026-11-05-listen-from-the-start-of-a-target.md, which is proposed.

The maintainer decided on 2026-11-04 that the package exports its ordered event
listener as the funcs `Listen` and `ListenBrowser`. The change started with
the discussion https://github.com/chromedp/chromedp/discussions/1645, which
onsi, the maintainer of the testing framework Biloba, opened on 2026-11-03.

## Why

The decision `2026-10-03-generic-iterator-api-instead-of-action.md` removed
`ListenTarget` and `ListenBrowser`, and `Events` and `BrowserEvents` replaced
them. These iterators give one stream for each event type. They queue the events
and deliver them from another goroutine, so they have two limits:

1. They give no order between event types.
2. A command can return before a subscriber has seen an event that arrived
   before the reply to that command.

Code that tracks the state of a page from many event types needs both
guarantees. The old `ListenTarget` gave them, and the package still uses the
same mechanism inside, for `Navigate`, `RunResponse`, `NavigateResponse` and
`WaitNewTarget`. The decision above kept that mechanism for this reason. It did
not export it.

## What was done

- `Listen(ctx, fn func(ev any)) error` calls `fn` for each event of the target
  of the context. `ListenBrowser` does the same for the events of the browser.
  Both start the browser, and `Listen` opens the target, as `Events` does.
  Both return `ErrInvalidContext` for a context that is not a chromedp context.
- The target calls `fn` on the goroutine that reads its messages, in the order
  of arrival, before it reads the next message. A reply to a command is
  delivered through the same loop. So `fn` sees an event that arrived before a
  reply before the command returns.
- `fn` gets the typed events of `cdproto` only. The replies to commands stay
  internal.
- The names are `Listen` and `ListenBrowser`. The package-level funcs find the
  target from the context, as `Events` does. The methods stay unexported,
  because a program has a `*Target` only inside an action.
- The documentation of `Listen` says that `Events` is the recommended API, and
  that `Listen` is the lower level tool for code that needs the order.

## Rules that the documentation states

`fn` runs inside the loop that reads the messages of the tab. The code does not
enforce these rules, because it cannot tell a call from the loop from a call from
another goroutine:

- `fn` must return fast. A slow `fn` delays every command and event of the tab.
- `fn` must not run a command or call `Listen`. The first waits for a reply that
  only the blocked goroutine can read. The second takes the lock that is held
  while the listeners run. Both stop the program.
- `fn` must not panic. The loop does not recover, so a panic ends the program.
- A listener sees the events that arrive after `Listen` returns.

## What was not done

- A context option that sets the listener when `NewContext` runs, so that no
  event of the start is missed. One model that the maintainer consulted
  recommended it as the only API. It cannot serve code that must start to
  listen on a tab that already runs, which is what `ListenTarget` did. It can be
  added later if a program needs the events of the start of a tab. The
  proposed decision `2026-11-05-listen-from-the-start-of-a-target.md` adds it
  as `WithListener`, next to `Listen`.
- A typed `Listen[E]`. Several typed listeners that write to one place also
  see the order across the types, but a func that switches on `any` is simpler
  for this use.
- A check that stops a command from inside `fn`. It needs a flag for each
  goroutine, and Go has none. A flag for each tab also stops the commands of
  other goroutines that are right.
- A recover of a panic of `fn`. A panic is a bug of the caller, and the program
  must show it.
