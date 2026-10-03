# Migrate to the new API

This document lists the public API changes of the branch `typed-api`. It has
three parts, in the order of the changes: the move to `cdproto` v0.157.1, the
move to the typed `cdproto`, and the move to the generic action API. Apply the
parts in this order. A later part can replace a rule of an earlier part.

## Migrate to cdproto v0.157.1

The new `cdproto` no longer holds the hand-written additions that the old
generator made. `chromedp` now defines these types itself. The lists in this
part give the old name first, then the new name.

### Types

- `*cdp.Node` becomes `*chromedp.Node`. It embeds `*cdp.Node`, so every protocol field still works. The tree links `Parent`, `Children`, `ContentDocument`, `ShadowRoots`, `TemplateContent` and `PseudoElements` hold `*chromedp.Node` values.
- `*cdp.Frame` becomes `*chromedp.Frame`. It embeds `*cdp.Frame` and adds `State`, `Root` and `Nodes`.
- The methods `Attribute`, `AttributeValue`, `PartialXPath`, `PartialXPathByID`, `FullXPath`, `FullXPathByID`, `WriteTo` and `Dump` keep their names and move to `chromedp.Node`.
- The fields `Parent`, `Invalidated` and `State` of `cdp.Node` move to `chromedp.Node`.
- `cdp.NodeState` becomes `chromedp.NodeState`.
- `cdp.NodeReady`, `cdp.NodeVisible` and `cdp.NodeHighlighted` become `chromedp.NodeStateReady`, `chromedp.NodeStateVisible` and `chromedp.NodeStateHighlighted`. The names `NodeReady` and `NodeVisible` already name query options in `chromedp`.
- `cdp.FrameState` and its constants `FrameAttached`, `FrameNavigated`, `FrameLoading` and the others become `chromedp.FrameState` and the same constant names.
- `cdp.EmptyNodeID` becomes `chromedp.EmptyNodeID`.
- `cdp.EmptyFrameID` becomes `chromedp.EmptyFrameID`.
- `cdp.NodeType` becomes `chromedp.NodeType`. The constants `cdp.NodeTypeElement`, `cdp.NodeTypeText` and the others become `chromedp.NodeTypeElement`, `chromedp.NodeTypeText` and the others.

### Functions that take or return nodes and frames

Every public function that took or returned `*cdp.Node` or `*cdp.Frame` now uses `*chromedp.Node` or `*chromedp.Frame`. These are `Nodes`, `FromNode`, `ByFunc`, `WaitFunc`, `After`, `QueryAfter`, `MouseClickNode`, `KeyEventNode`, `ScreenshotNodes` and `WithPollingInFrame`. The types `QueryOption` and `Selector` change in the same way.

### Input

- `input.Modifier` becomes `chromedp.Modifier`, which is an alias of `kb.Modifier`.
- `input.ModifierNone`, `input.ModifierAlt`, `input.ModifierCtrl`, `input.ModifierMeta`, `input.ModifierShift` and `input.ModifierCommand` become `chromedp.ModifierNone`, `chromedp.ModifierAlt`, `chromedp.ModifierCtrl`, `chromedp.ModifierMeta`, `chromedp.ModifierShift` and `chromedp.ModifierCommand`. The same names are in `kb`.
- `input.KeyDown`, `input.KeyUp`, `input.KeyChar` and `input.KeyRawDown` become `kb.KeyDown`, `kb.KeyUp`, `kb.KeyChar` and `kb.KeyRawDown`. They are aliases of `input.DispatchKeyEventTypeKeyDown` and the other constants of the type `input.DispatchKeyEventType`.
- `input.MouseType` becomes `input.DispatchMouseEventType`. The function `MouseEvent` takes that type.
- `input.MousePressed`, `input.MouseReleased`, `input.MouseMoved` and `input.MouseWheel` become `chromedp.MousePressed`, `chromedp.MouseReleased`, `chromedp.MouseMoved` and `chromedp.MouseWheel`. They are aliases of `input.DispatchMouseEventTypeMousePressed` and the other constants.
- `input.Left`, `input.Right`, `input.Middle` and `input.None` become `input.MouseButtonLeft`, `input.MouseButtonRight`, `input.MouseButtonMiddle` and `input.MouseButtonNone`. The functions `chromedp.ButtonLeft`, `chromedp.ButtonRight`, `chromedp.ButtonMiddle` and `chromedp.ButtonNone` keep their names. They are mouse options and not constants.
- The `Modifiers` field of an input event is `int64`. Convert a `Modifier` with `int64(m)`.

### Other changes

- `emulation.OrientationType` becomes `emulation.ScreenOrientationType`, and its constants become `emulation.ScreenOrientationTypePortraitPrimary` and the others. `chromedp.EmulateOrientation` takes the new type.
- `page.CaptureScreenshotFormatPng` and `page.CaptureScreenshotFormatJpeg` keep their names and now have the type `page.CaptureScreenshotFormat`.
- `browser.DownloadProgressStateCompleted` and `browser.SetDownloadBehaviorBehaviorAllowAndName` keep their names and have the types `browser.DownloadProgressState` and `browser.SetDownloadBehaviorBehavior`.
- `runtime.ExceptionDetails` has no `Error` method. The actions `Evaluate`, `CallFunctionOn` and the query actions return `*chromedp.ExceptionError`, which wraps the details and has an `Error` method.
- The time types of `cdp` and `network` are plain `float64` values, so a method such as `ResponseTime.Time()` is gone. Convert the seconds yourself with `time.Unix`.
- `cdproto` uses `encoding/json/v2` and `encoding/json/jsontext` from the standard library. Use them in place of `github.com/go-json-experiment/json` and its `jsontext` package. The module needs Go 1.27.
- `EventExecutionContextDestroyed` names a context by `ExecutionContextUniqueID`. `chromedp` tracks the unique identifier for you.

## Migrate to the typed cdproto

This part lists the public API changes that come with the typed `cdproto`.
In the typed `cdproto`, a command is a value and not a function. The value
has the type `cdp.Command[P, R]`, where `P` is the parameter struct and `R` is
the result struct. A command with no parameters or no result uses `cdp.Empty`.
The generated parameter types have no `Do` method, no `With...` methods and no
constructors. The old name comes first, then the new name.

### Run a command

- `page.Navigate(url).Do(ctx)` becomes `chromedp.Call(ctx, page.Navigate, page.NavigateParams{URL: url})`. `Call` returns the result struct and an error. The old `Do` returned the result fields one by one, so `frameID, loaderID, errorText, _, err := ...` becomes `res, err := ...` and `res.FrameID`, `res.LoaderID` and `res.ErrorText`.
- `Call` sends the command to the target of the context. As `Run` does, it starts the browser and opens the target when the context has none yet. It returns `chromedp.ErrInvalidContext` when the context is not a chromedp context.
- `chromedp.CallBrowser` is the same for the browser. Use it for the commands of the `target` and `browser` domains. It replaces `cmd.Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Browser))`.
- `cdp.WithExecutor`, `cdp.ExecutorFromContext` and `cdp.Execute` are gone. The context holds no executor. Use `Call` and `CallBrowser`, or call `cdp.Call(ctx, session, cmd, params)` with the session that you want. `chromedp.FromContext(ctx).Target` and `chromedp.FromContext(ctx).Browser` are sessions.
- A command is no longer an `Action`. `chromedp.Run(ctx, page.Navigate(url))` becomes an action that calls `cdp.Call` on its target. The last part of this document describes `Func`, which makes that action:

```go
chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
	_, err := cdp.Call(ctx, t, page.Navigate, page.NavigateParams{URL: url})
	return err
}))
```

### Sessions and events

- `chromedp.Browser` and `chromedp.Target` implement `cdp.Session`. The method `Execute` of both types becomes `Call`. It has the same arguments. It returns a `*cdproto.Error` when the browser answers with an error.
- `Browser.Call` still refuses `Browser.close`, and `Target.Call` still refuses `Target.closeTarget`. Use `chromedp.Cancel` to close.
- Both types have a new method `Subscribe(method string)`. It returns a channel of the raw event parameters and a func that cancels the subscription. The subscription buffers events from the moment that `Subscribe` returns, without a limit. `cdp.Events(ctx, session, page.LoadEventFired)` wraps it in an iterator of typed events. A caller can subscribe, trigger the event and then range over the events.
- In this part, `ListenTarget` and `ListenBrowser` keep their signatures. They still call the func with a pointer to the event struct, for example `*page.EventLoadEventFired`. The last part of this document replaces them with `Events` and `BrowserEvents`.

### Options

The generated `With...` methods are gone, so the option types of `chromedp` change. An option now changes the parameter struct that it receives and returns nothing. A field for an optional boolean has the type `*bool`. Use `new(true)` and `new(false)`.

- `EvaluateOption` becomes `func(*runtime.EvaluateParams)`. The options `EvalObjectGroup`, `EvalWithCommandLineAPI`, `EvalIgnoreExceptions` and `EvalAsValue` keep their names.
- `CallOption` becomes `func(*runtime.CallFunctionOnParams)`.
- `MouseOption` becomes `func(*input.DispatchMouseEventParams)`. The options `Button`, `ButtonType`, `ButtonLeft`, `ButtonMiddle`, `ButtonRight`, `ButtonNone`, `ButtonModifiers` and `ClickCount` keep their names.
- `KeyOption` becomes `func(*input.DispatchKeyEventParams)`. `KeyModifiers` keeps its name.
- `CreateBrowserContextOption` becomes `func(*target.CreateBrowserContextParams)`.
- A custom option such as `func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithSilent(true) }` becomes `func(p *runtime.EvaluateParams) { p.Silent = new(true) }`.

### Other changes

- `chromedp.MouseEvent` returns an action that runs `input.DispatchMouseEvent`. It no longer returns the parameters. The type `MouseAction` is unchanged.
- `MatchedStyle` takes a `**css.GetMatchedStylesForNodeResult`. The old name was `css.GetMatchedStylesForNodeReturns`.
- The result of a command that returns binary data, such as `page.CaptureScreenshot` and `page.PrintToPDF`, has a `Data []byte` field.
- `chromedp` no longer sends `focus: true` when it creates a target. It still sends `newWindow: true`. Without it, a new tab in a shared window is hidden when another tab is active. A hidden page gets no animation frames, so `Poll` never returns.

## Migrate to the generic action API

This part lists the public API changes of the generic action API. An action now returns its value, so the program does not pass a pointer to receive it. Events arrive as iterators. The old name comes first, then the new name. `docs/API.md` shows the old and the new code side by side.

### Actions

- The interface `Action` becomes the func type `Action[T]`. It is `func(ctx context.Context, t *Target) (T, error)`. The target `t` is a `cdp.Session`, so an action sends a command with `cdp.Call(ctx, t, command, params)`.
- `type Void = struct{}` is the value type of an action that returns no value. Such an action has the type `Action[Void]`.
- `Run(ctx, actions...) error` becomes `Run[T](ctx, action) (T, error)` for one action that returns a value, and `Do(ctx, steps...) error` for actions that return no value. `Do` runs the steps in order and stops at the first error.
- `Run(ctx)` with no action, which starts the browser and opens the tab, becomes `Do(ctx)`.
- `Tasks` is gone. `Steps(steps...)` joins actions into one `Action[Void]`.
- `ActionFunc` is gone. A func literal of the type `Action[T]` replaces it. `Func(f)` makes an `Action[Void]` from `func(ctx context.Context, t *Target) error`.
- `Legacy(a)` makes an `Action[Void]` from a value of the interface `OldAction`. `OldAction` is the old interface, `interface{ Do(context.Context) error }`. The old action receives the same context, so `Call` and `CallBrowser` work inside it.
- The types `QueryAction`, `NavigateAction`, `EvaluateAction`, `CallAction`, `PollAction`, `MouseAction`, `KeyAction` and `EmulateAction` are gone. Use `Action[T]`.
- `Selector.Do` is gone. Run a query with `Run` or `Do`.

### Actions that return a value

The old action took a pointer to receive the value. The new action returns the value, and the pointer argument is gone.

- `Text(sel, &s, opts...)` becomes `Text(sel, opts...)`, which is an `Action[string]`. `TextContent`, `Value`, `InnerHTML` and `OuterHTML` change in the same way.
- `Nodes(sel, &nodes, opts...)` becomes `Nodes(sel, opts...)`, an `Action[[]*Node]`. `NodeIDs` returns `[]cdp.NodeID`.
- `Attributes(sel, &m, opts...)` becomes `Attributes(sel, opts...)`, an `Action[map[string]string]`. `AttributesAll` returns `[]map[string]string`.
- `AttributeValue(sel, name, &value, &ok, opts...)` becomes `AttributeValue(sel, name, opts...)`, an `Action[AttributeResult]`. The struct `AttributeResult` has the fields `Value` and `Exists`.
- `JavascriptAttribute(sel, name, &res, opts...)` becomes `JavascriptAttribute[T](sel, name, opts...)`.
- `Dimensions` returns `*dom.BoxModel`, `ComputedStyle` returns `[]*css.ComputedStyleProperty` and `MatchedStyle` returns `*css.GetMatchedStylesForNodeResult`.
- `Screenshot(sel, &buf, opts...)` becomes `Screenshot(sel, opts...)`, an `Action[[]byte]`. `ScreenshotScale(sel, scale, opts...)`, `ScreenshotNodes(nodes, scale)`, `CaptureScreenshot()` and `FullScreenshot(quality)` change in the same way.
- `Location(&s)` and `Title(&s)` become `Location()` and `Title()`, both `Action[string]`.
- `NavigationEntries(&index, &entries)` becomes `NavigationEntries()`. It returns a `page.GetNavigationHistoryResult` with the fields `CurrentIndex` and `Entries`.
- `Evaluate(expr, &res, opts...)` becomes `Evaluate[T](expr, opts...)`. The type `T` replaces the kind of pointer. `Evaluate(expr, nil)` becomes `Evaluate[Void]` with the same expression, `&[]byte` becomes `[]byte` and `**runtime.RemoteObject` becomes `*runtime.RemoteObject`. `EvaluateAsDevTools` changes in the same way.
- `CallFunctionOn(fn, &res, opt, args...)` becomes `CallFunctionOn[T](fn, opt, args...)`.
- `Poll(expr, &res, opts...)` and `PollFunction(fn, &res, opts...)` become `Poll[T](expr, opts...)` and `PollFunction[T](fn, opts...)`.

### Actions that return no value

These actions keep their arguments and now have the type `Action[Void]`: `Navigate`, `NavigateToHistoryEntry`, `NavigateBack`, `NavigateForward`, `Reload`, `Stop`, `Sleep`, `Click`, `DoubleClick`, `SendKeys`, `SetValue`, `Clear`, `Focus`, `Blur`, `Submit`, `Reset`, `ScrollIntoView`, `SetAttributes`, `SetAttributeValue`, `RemoveAttribute`, `SetJavascriptAttribute`, `SetUploadFiles`, `Dump`, `DumpTo`, `Query`, the `Wait...` actions, `MouseEvent`, `MouseClickXY`, `MouseClickNode`, `KeyEvent`, `KeyEventNode`, `EmulateViewport`, `ResetViewport`, `Emulate` and `EmulateReset`.

`Navigate` still waits for the page to load. `NavigateResponse(url)` is new. It is an `Action[*network.Response]`. `RunResponse(ctx, steps...)` keeps its name, and it takes `Action[Void]` values.

### Selectors and query options

The selector options `ByQuery`, `ByID`, `NodeVisible`, `AtLeast`, `RetryInterval`, `FromNode` and the others keep their names and their use. Four option types receive the target, because the options send commands.

- `ByFunc` takes `func(context.Context, *Target, *Node) ([]cdp.NodeID, error)`.
- `WaitFunc` takes `func(context.Context, *Target, *Frame, runtime.ExecutionContextID, ...cdp.NodeID) ([]*Node, error)`.
- `After` takes `func(ctx context.Context, t *Target, nodes []*Node) error`. The old func had the execution context id as its second argument and took the nodes as a variadic argument. Read the nodes from the slice. Call `t` for a command.
- `QueryAfter(sel, f, opts...)` is generic. The func `f` is `func(ctx context.Context, t *Target, nodes []*Node) (T, error)` and the action is an `Action[T]`.
- `Query(sel, opts...)` returns `Action[Void]`. It only waits for the nodes.

### Events

- `ListenTarget(ctx, fn)` becomes `Events(ctx, page.LoadEventFired)`. It returns an iterator `iter.Seq2[E, error]`. The payload has its own type, for example `page.EventLoadEventFired`, so the func does not switch on `any`.
- `ListenBrowser(ctx, fn)` becomes `BrowserEvents(ctx, target.TargetCreated)`.
- The subscription starts when `Events` returns. A program subscribes, triggers the event and then ranges over the iterator, and no event is lost. The iterator ends when the context ends and yields the error of the context.
- A listener ran inside the loop that handles the events of the browser, so it had to be fast. The iterator buffers the events without a limit, so the program reads them at its own speed.
- `WaitEvent(event, match, trigger)` is new. It subscribes, runs the trigger action and returns the first payload for which `match` is true. It replaces the pattern of a listener that closes a channel.
- `WaitNewTarget` keeps its signature.

### Other changes

- `Call` and `CallBrowser` stay. Use them where only a context is at hand, for example in code that runs under `Legacy`. An action calls `cdp.Call(ctx, t, ...)` with its target.
- `Poll` and `PollFunction` keep the polling options. A poll still runs in the page and does not use an iterator.
- When the session closes, the subscriptions of `Browser` and `Target` deliver the events that they already hold. A reader then sees every event up to the end.
