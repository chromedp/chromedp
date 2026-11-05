# Route the detach of a target after its messages

Status: Proposed.

This change fixes a race in the routing of the messages of a target. onsi,
the maintainer of the testing framework Biloba, found it after the fix for the
discussion https://github.com/chromedp/chromedp/discussions/1645. The
maintainer has not decided on it yet.

## The problem

The goroutine that reads the messages of the browser put each message of a
session in one queue, `incomingQueue`. It put the event
`Target.detachedFromTarget` of a session in a second queue, `delTabQueue`. The
routing loop in `Browser.run` selects over both queues. When both hold a value,
Go picks one at random.

So the loop sometimes took the detach before a reply or an event that the
browser sent earlier. It removed the target from `b.pages`, and then it dropped
the earlier message, because no target had its session. A command that Chrome
answered returned `ErrTargetDetached`, and a listener of `Listen` missed an
event. In the test `TestDetachAfterSessionMessages`, about half of the replies
and half of the events were lost.

## What was done

- The reader puts the detach in `incomingQueue`, behind the messages of the
  session that came before it. The queue holds a `sessionMessage`, which is a
  message or a detach. `delTabQueue` is gone.
- The loop routes the items in the order of the queue. So the target gets
  every message that the browser sent before the detach, and then
  `detachedMessage`.
- The browser listeners still see the detach on the reader goroutine before the
  loop routes it. A canceled context still stops the reader and the loop.
- The test uses a `Transport` that plays the browser. It keeps the routing loop
  busy while the reader queues a message and the detach, so both wait at the
  same time. Without the fix, all 100 of 100 runs of the test failed.

## What was not done

- A sentinel `*cdproto.Message` in place of the new type. The reader never
  queues a message of a session that has no id and no method, so such a message
  can mark a detach. A struct with a named field is clearer.
