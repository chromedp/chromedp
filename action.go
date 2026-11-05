package chromedp

import (
	"context"
	jsonv2 "github.com/chromedp/cdproto/cdp/jsonv2"
	"iter"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
)

// Void is the result type of an action that returns no value.
type Void = struct{}

// Action is a step that runs against the target of a chromedp context and
// returns a value of the type T. An action that returns no value has the type
// Action[Void].
//
// An action is a plain func, so a program can write its own:
//
//	title := func(ctx context.Context, t *chromedp.Target) (string, error) {
//		res, err := cdp.Call(ctx, t, runtime.Evaluate, runtime.EvaluateParams{
//			Expression:    "document.title",
//			ReturnByValue: ptr(true),
//		})
//		...
//	}
//
// The target t is a [cdp.Session]. Use it with [cdp.Call] to send a protocol
// command, and with [cdp.Events] to receive protocol events.
type Action[T any] func(ctx context.Context, t *Target) (T, error)

// Run runs the action against the target of the context, and returns its
// value. The context must be a chromedp context, typically made with
// [NewContext].
//
// The first time that you call Run on a context, Run allocates a browser with
// the Allocator, and the browser lives as long as the context of that call. Do
// not set a timeout on the context of that first call, because the timeout
// stops the whole browser. To limit one action, run it with a context that you
// derive with [context.WithTimeout] from a context that has run once. See
// [NewContext].
//
// For example:
//
//	title, err := chromedp.Run(ctx, chromedp.Title())
//
// If the browser process dies while nobody has asked it to stop, for example
// when it crashes or when the system kills it for lack of memory, then Run
// returns an error that wraps the exit error of the process and the error of
// the context. Use [errors.As] with a pointer to an [os/exec.ExitError] to read
// the signal or the status.
//
// Use [Do] to run several actions that return no value.
//
// Contexts that share a browser run in separate tabs, so goroutines can call
// Run in parallel when each one has its own context. Actions on one context
// share one tab, so they can race. See [NewContext].
func Run[T any](ctx context.Context, a Action[T]) (T, error) {
	c, err := initContextTarget(ctx)
	if err != nil {
		var zero T
		return zero, FromContext(ctx).withExitError(err)
	}
	v, err := a(ctx, c.Target)
	return v, c.withExitError(err)
}

// Do runs the actions against the target of the context, in order. It stops at
// the first error. See [Run] for the rules about the context.
//
// For example:
//
//	err := chromedp.Do(ctx,
//		chromedp.Navigate("https://example.com"),
//		chromedp.Click(chromedp.CSS("a")),
//	)
func Do(ctx context.Context, steps ...Action[Void]) error {
	_, err := Run(ctx, Steps(steps...))
	return err
}

// Steps joins the actions into one action that runs them in order and stops at
// the first error. It replaces the type Tasks.
func Steps(steps ...Action[Void]) Action[Void] {
	return func(ctx context.Context, t *Target) (Void, error) {
		for _, a := range steps {
			if _, err := a(ctx, t); err != nil {
				return Void{}, err
			}
		}
		return Void{}, nil
	}
}

// Func makes an action that returns no value from a func that returns only an
// error.
func Func(f func(ctx context.Context, t *Target) error) Action[Void] {
	return func(ctx context.Context, t *Target) (Void, error) {
		return Void{}, f(ctx, t)
	}
}

// OldAction is the interface of an action of the previous version of chromedp.
// The previous version named it Action.
type OldAction interface {
	// Do runs the action. The context holds the chromedp context.
	Do(context.Context) error
}

// Legacy makes an action from an action of the previous version. The old
// action receives the same context, so [Call] and [CallBrowser] work inside
// it.
func Legacy(a OldAction) Action[Void] {
	return func(ctx context.Context, t *Target) (Void, error) {
		return Void{}, a.Do(ctx)
	}
}

// Events subscribes to the event on the target of the context, and returns an
// iterator over its payloads. It replaces ListenTarget. Use [Listen] when the
// code needs the order of the events across event types.
//
// The subscription starts when Events returns, and not when the caller starts
// to range. A caller can subscribe, trigger the event, and then range, so that
// no event is lost:
//
//	events := chromedp.Events(ctx, page.LoadEventFired)
//	if err := chromedp.Do(ctx, chromedp.Navigate(url)); err != nil {
//		return err
//	}
//	for ev, err := range events {
//		...
//	}
//
// As [Run] does, Events starts the browser and opens the target when the
// context has none yet. If that fails, the iterator yields the error. See
// [cdp.Events] for the rules about ending the iteration. To stop, cancel the
// context or break out of the loop. Both end the subscription, also when the
// program never ranged over the iterator. The queue of the events has no limit,
// so a reader that does not read makes it grow until the subscription ends.
func Events[E any](ctx context.Context, ev cdp.Event[E]) iter.Seq2[E, error] {
	c, err := initContextTarget(ctx)
	if err != nil {
		return failed[E](err)
	}
	return cdp.Events(ctx, endWith(ctx, c.Target), ev)
}

// BrowserEvents is like [Events] for the events of the browser, such as the
// events of the target domain. It replaces ListenBrowser. Use [ListenBrowser]
// when the code needs the order of the events across event types.
func BrowserEvents[E any](ctx context.Context, ev cdp.Event[E]) iter.Seq2[E, error] {
	c, err := initContextBrowser(ctx)
	if err != nil {
		return failed[E](err)
	}
	return cdp.Events(ctx, endWith(ctx, c.Browser), ev)
}

// Listen calls fn for each event of the target of the context, in the order in
// which the events arrive from the browser. As [Events] does, it starts the
// browser and opens the target when the context has none yet, and it returns
// the error if that fails. Canceling the context stops the calls. It returns
// [ErrInvalidContext] when ctx is not a chromedp context.
//
// Use [Events] when you can. Use Listen when your code needs the order of the
// events across event types, or must see an event before the command that
// follows it returns. The iterators of Events have neither guarantee, because
// they queue the events and deliver them from another goroutine. The target
// calls fn on the goroutine that reads the messages of the browser, and it
// calls fn before it reads the next message. So when an event arrives before
// the reply to a command, fn sees the event before that command returns.
//
// fn gets the events and nothing else, as pointers to the event types of
// cdproto, such as *page.EventLoadEventFired. It gets no reply to a command.
// It sees the events that arrive after Listen returns, so call Listen before
// the action that causes the events.
//
// fn runs inside the loop that reads the messages of the tab. For this reason:
//
//   - fn must return fast. A slow fn delays every command and every event of
//     the tab.
//   - fn must not run a command with [Run], [Do], [Call] or [cdp.Call], and it
//     must not call Listen. The call waits for a message that only the blocked
//     goroutine can read, so the program stops.
//   - fn must not panic. A panic ends the program.
//
// To do slow work, put the event in a slice under a lock, or send it on a
// channel that has room, and handle it in another goroutine.
func Listen(ctx context.Context, fn func(ev any)) error {
	c, err := initContextTarget(ctx)
	if err != nil {
		return err
	}
	c.Target.listen(ctx, eventsOnly(fn))
	return nil
}

// ListenBrowser is like [Listen] for the events of the browser, such as the
// events of the target domain. The same rules hold for fn. As [BrowserEvents]
// does, it starts the browser when the context has none yet.
func ListenBrowser(ctx context.Context, fn func(ev any)) error {
	c, err := initContextBrowser(ctx)
	if err != nil {
		return err
	}
	c.Browser.listen(ctx, eventsOnly(fn))
	return nil
}

// Detached returns a channel that closes when the browser detaches the target
// of the context, for example when the page closes. It returns
// [ErrInvalidContext] when ctx is not a chromedp context. As [Listen]
// does, it starts the browser and opens the target when the context has none
// yet, and it returns the error if that fails.
//
// The channel closes after the target handled every message that the browser
// sent to it before the detach. So a func of Listen saw every event of the
// target, and every reply that the browser sent reached its command, before
// the channel closes. A command that still waits for a reply then returns an
// error that wraps [ErrTargetDetached].
//
// A detach that happened before the call gives a channel that is closed. To
// check without a wait, use a select with a default case:
//
//	ch, err := chromedp.Detached(ctx)
//	if err != nil {
//		return err
//	}
//	select {
//	case <-ch:
//		// the tab is gone
//	default:
//	}
//
// The channel does not close when the context ends first, because the target
// stops to read its messages then. Use ctx.Done for the end of the context.
func Detached(ctx context.Context) (<-chan struct{}, error) {
	c, err := initContextTarget(ctx)
	if err != nil {
		return nil, err
	}
	return c.Target.detached, nil
}

// eventsOnly returns a func that calls fn for each event, and skips the replies
// to commands that the internal listeners also get.
func eventsOnly(fn func(ev any)) func(ev any) {
	return func(ev any) {
		if _, ok := ev.(*cdproto.Message); ok {
			return
		}
		fn(ev)
	}
}

// endingSession is a [cdp.Session] that ends each subscription when a context
// ends, also when nobody ranges over the iterator.
type endingSession struct {
	cdp.Session
	ctx context.Context
}

// endWith returns the session s with subscriptions that end when ctx ends.
// Without it, an iterator that nobody reads keeps its subscription, and a
// goroutine that waits to deliver a queued event, for ever.
func endWith(ctx context.Context, s cdp.Session) cdp.Session {
	return endingSession{Session: s, ctx: ctx}
}

// Subscribe subscribes on the session, and ends the subscription when the
// context ends.
func (e endingSession) Subscribe(method string) (<-chan jsonv2.Value, func()) {
	events, cancel := e.Session.Subscribe(method)
	stop := context.AfterFunc(e.ctx, cancel)
	return events, func() {
		stop()
		cancel()
	}
}

// failed returns an iterator that yields the error and ends.
func failed[E any](err error) iter.Seq2[E, error] {
	return func(yield func(E, error) bool) {
		var zero E
		yield(zero, err)
	}
}

// WaitEvent makes an action that waits for an event. It subscribes to the
// event, runs the trigger action, and then returns the first payload for which
// match returns true. A nil match accepts every payload. The trigger runs after
// the subscription, so the event cannot arrive too early.
//
// For example, to wait for the load event that a click causes:
//
//	_, err := chromedp.Run(ctx, chromedp.WaitEvent(page.LoadEventFired, nil,
//		chromedp.Click(chromedp.CSS("a"))))
//
// When the context ends before a matching event arrives, WaitEvent returns the
// error of the context.
func WaitEvent[E, T any](ev cdp.Event[E], match func(E) bool, trigger Action[T]) Action[E] {
	return func(ctx context.Context, t *Target) (E, error) {
		lctx, cancel := context.WithCancel(ctx)
		defer cancel()
		events := cdp.Events(lctx, t, ev)
		if _, err := trigger(ctx, t); err != nil {
			var zero E
			return zero, err
		}
		for e, err := range events {
			if err != nil {
				return e, err
			}
			if match == nil || match(e) {
				return e, nil
			}
		}
		var zero E
		return zero, ErrChannelClosed
	}
}

// Sleep is an action that waits for the duration, or until the context ends.
func Sleep(d time.Duration) Action[Void] {
	return func(ctx context.Context, t *Target) (Void, error) {
		return Void{}, sleepContext(ctx, d)
	}
}
