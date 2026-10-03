# Migrate to cdproto v0.157.0

This document lists the public API changes that come with the move to
`cdproto` v0.157.0. The new `cdproto` no longer holds the hand-written
additions that the old generator made. `chromedp` now defines these types
itself. The list gives the old name first, then the new name.

## Types

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

## Functions that take or return nodes and frames

Every public function that took or returned `*cdp.Node` or `*cdp.Frame` now uses `*chromedp.Node` or `*chromedp.Frame`. These are `Nodes`, `FromNode`, `ByFunc`, `WaitFunc`, `After`, `QueryAfter`, `MouseClickNode`, `KeyEventNode`, `ScreenshotNodes` and `WithPollingInFrame`. The types `QueryOption` and `Selector` change in the same way.

## Input

- `input.Modifier` becomes `chromedp.Modifier`, which is an alias of `kb.Modifier`.
- `input.ModifierNone`, `input.ModifierAlt`, `input.ModifierCtrl`, `input.ModifierMeta`, `input.ModifierShift` and `input.ModifierCommand` become `chromedp.ModifierNone`, `chromedp.ModifierAlt`, `chromedp.ModifierCtrl`, `chromedp.ModifierMeta`, `chromedp.ModifierShift` and `chromedp.ModifierCommand`. The same names are in `kb`.
- `input.KeyDown`, `input.KeyUp`, `input.KeyChar` and `input.KeyRawDown` become `kb.KeyDown`, `kb.KeyUp`, `kb.KeyChar` and `kb.KeyRawDown`. They are aliases of `input.DispatchKeyEventTypeKeyDown` and the other constants of the type `input.DispatchKeyEventType`.
- `input.MouseType` becomes `input.DispatchMouseEventType`. The function `MouseEvent` takes that type.
- `input.MousePressed`, `input.MouseReleased`, `input.MouseMoved` and `input.MouseWheel` become `chromedp.MousePressed`, `chromedp.MouseReleased`, `chromedp.MouseMoved` and `chromedp.MouseWheel`. They are aliases of `input.DispatchMouseEventTypeMousePressed` and the other constants.
- `input.Left`, `input.Right`, `input.Middle` and `input.None` become `input.MouseButtonLeft`, `input.MouseButtonRight`, `input.MouseButtonMiddle` and `input.MouseButtonNone`. The functions `chromedp.ButtonLeft`, `chromedp.ButtonRight`, `chromedp.ButtonMiddle` and `chromedp.ButtonNone` keep their names. They are mouse options and not constants.
- The `Modifiers` field of an input event is `int64`. Convert a `Modifier` with `int64(m)`.

## Other changes

- `emulation.OrientationType` becomes `emulation.ScreenOrientationType`, and its constants become `emulation.ScreenOrientationTypePortraitPrimary` and the others. `chromedp.EmulateOrientation` takes the new type.
- `page.CaptureScreenshotFormatPng` and `page.CaptureScreenshotFormatJpeg` keep their names and now have the type `page.CaptureScreenshotFormat`.
- `browser.DownloadProgressStateCompleted` and `browser.SetDownloadBehaviorBehaviorAllowAndName` keep their names and have the types `browser.DownloadProgressState` and `browser.SetDownloadBehaviorBehavior`.
- `runtime.ExceptionDetails` has no `Error` method. The actions `Evaluate`, `CallFunctionOn` and the query actions return `*chromedp.ExceptionError`, which wraps the details and has an `Error` method.
- The time types of `cdp` and `network` are plain `float64` values, so a method such as `ResponseTime.Time()` is gone. Convert the seconds yourself with `time.Unix`.
- `cdproto` uses `encoding/json/v2` and `encoding/json/jsontext` from the standard library. Use them in place of `github.com/go-json-experiment/json` and its `jsontext` package. The module needs Go 1.27.
- `EventExecutionContextDestroyed` names a context by `ExecutionContextUniqueID`. `chromedp` tracks the unique identifier for you.

# Migrate to the typed cdproto

This part lists the public API changes that come with the typed `cdproto`.
In the typed `cdproto`, a command is a value and not a function. The value
has the type `cdp.Command[P, R]`, where `P` is the parameter struct and `R` is
the result struct. A command with no parameters or no result uses `cdp.Empty`.
The generated parameter types have no `Do` method, no `With...` methods and no
constructors. The old name comes first, then the new name.

## Run a command

- `page.Navigate(url).Do(ctx)` becomes `chromedp.Call(ctx, page.Navigate, page.NavigateParams{URL: url})`. `Call` returns the result struct and an error. The old `Do` returned the result fields one by one, so `frameID, loaderID, errorText, _, err := ...` becomes `res, err := ...` and `res.FrameID`, `res.LoaderID` and `res.ErrorText`.
- `Call` sends the command to the target of the context. As `Run` does, it starts the browser and opens the target when the context has none yet. It returns `chromedp.ErrInvalidContext` when the context is not a chromedp context.
- `chromedp.CallBrowser` is the same for the browser. Use it for the commands of the `target` and `browser` domains. It replaces `cmd.Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Browser))`.
- `cdp.WithExecutor`, `cdp.ExecutorFromContext` and `cdp.Execute` are gone. The context holds no executor. Use `Call` and `CallBrowser`, or call `cdp.Call(ctx, session, cmd, params)` with the session that you want. `chromedp.FromContext(ctx).Target` and `chromedp.FromContext(ctx).Browser` are sessions.
- A command is no longer an `Action`. `chromedp.Run(ctx, page.Navigate(url))` becomes a `chromedp.ActionFunc` that calls `chromedp.Call`:

```go
chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
	_, err := chromedp.Call(ctx, page.Navigate, page.NavigateParams{URL: url})
	return err
}))
```

## Sessions and events

- `chromedp.Browser` and `chromedp.Target` implement `cdp.Session`. The method `Execute` of both types becomes `Call`. It has the same arguments. It returns a `*cdproto.Error` when the browser answers with an error.
- `Browser.Call` still refuses `Browser.close`, and `Target.Call` still refuses `Target.closeTarget`. Use `chromedp.Cancel` to close.
- Both types have a new method `Subscribe(method string)`. It returns a channel of the raw event parameters and a func that cancels the subscription. The subscription buffers events from the moment that `Subscribe` returns, without a limit. `cdp.Events(ctx, session, page.LoadEventFired)` wraps it in an iterator of typed events, so a caller can subscribe, trigger the event and then range over the events.
- `ListenTarget` and `ListenBrowser` keep their signatures. They still call the func with a pointer to the event struct, for example `*page.EventLoadEventFired`.

## Options

The generated `With...` methods are gone, so the option types of `chromedp` change. An option now changes the parameter struct that it receives and returns nothing. A field for an optional boolean has the type `*bool`. Use `new(true)` and `new(false)`.

- `EvaluateOption` becomes `func(*runtime.EvaluateParams)`. The options `EvalObjectGroup`, `EvalWithCommandLineAPI`, `EvalIgnoreExceptions` and `EvalAsValue` keep their names.
- `CallOption` becomes `func(*runtime.CallFunctionOnParams)`.
- `MouseOption` becomes `func(*input.DispatchMouseEventParams)`. The options `Button`, `ButtonType`, `ButtonLeft`, `ButtonMiddle`, `ButtonRight`, `ButtonNone`, `ButtonModifiers` and `ClickCount` keep their names.
- `KeyOption` becomes `func(*input.DispatchKeyEventParams)`. `KeyModifiers` keeps its name.
- `CreateBrowserContextOption` becomes `func(*target.CreateBrowserContextParams)`.
- A custom option such as `func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithSilent(true) }` becomes `func(p *runtime.EvaluateParams) { p.Silent = new(true) }`.

## Other changes

- `chromedp.MouseEvent` returns an action that runs `input.DispatchMouseEvent`. It no longer returns the parameters. The type `MouseAction` is unchanged.
- `MatchedStyle` takes a `**css.GetMatchedStylesForNodeResult`. The old name was `css.GetMatchedStylesForNodeReturns`.
- The result of a command that returns binary data, such as `page.CaptureScreenshot` and `page.PrintToPDF`, has a `Data []byte` field.
- `chromedp` no longer sends `focus: true` when it creates a target. It still sends `newWindow: true`. Without it, a new tab in a shared window is hidden when another tab is active. A hidden page gets no animation frames, so `Poll` never returns.
