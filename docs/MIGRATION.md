# Migrate to the new API

This document lists the public API changes from `chromedp` v0.16.0 to v0.18.0.
Version v0.18.0 uses `cdproto` v0.157.4. The release v0.157.3 is the first one of
the typed API, and v0.157.4 makes some optional numbers pointers. The versions v0.157.0, v0.157.1 and v0.157.2 of `cdproto` have the old API. The
document has eight parts, in the order of the changes: the move to `cdproto`
v0.157.1, the move to the typed `cdproto`, the move to the generic action API,
the move to typed selectors, the pipe transport, the options for a visible
window, and the move of the websocket code to the module `remote`. Apply the parts in this order. A later part can replace a rule of an
earlier part.

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
- `Nodes(sel, &nodes, opts...)` becomes `Nodes(sel, opts...)`, an `Action[[]*Node]`. `NodeIDs` is now a selector type. The action that returns `[]cdp.NodeID` is `QueryNodeIDs`.
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

The query options `NodeVisible`, `AtLeast`, `RetryInterval`, `FromNode` and the others keep their names and their use. The lookup options `ByQuery`, `ByID` and the others change in the next part. Four option types receive the target, because the options send commands.

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

## Migrate to typed selectors

This part lists the changes to the selector of a query action. The type of the selector now chooses the lookup, so the `By...` lookup options are gone. The old name comes first, then the new name.

### Selector types

A query action takes a value of the constraint `Selectable`. A `Selectable` is a string type or a `[]cdp.NodeID` type. The first argument of every query action changed from `any` to `Selectable`.

- `ByQuery` becomes the type `CSS`. It selects the first match of `DOM.querySelector`. `Click("#a", ByQuery)` becomes `Click(CSS("#a"))`.
- `ByQueryAll` becomes the type `CSSAll`. It selects every match of `DOM.querySelectorAll`. `Nodes("a", ByQueryAll)` becomes `Nodes(CSSAll("a"))`.
- `ByID` becomes the type `ID`. It selects the element with this id, and a leading `#` is optional. `Click("#a", ByID)` becomes `Click(ID("a"))`.
- `BySearch` becomes the type `Search`. It uses `DOM.performSearch` and takes a CSS selector, an XPath query or text. It was the default, so a plain string is still a `Search`. `Click("//a", BySearch)` becomes `Click("//a")` or `Click(Search("//a"))`.
- `ByJSPath` becomes the type `JSPath`. It selects the node that a JavaScript expression gives. `Nodes("document", ByJSPath)` becomes `Nodes(JSPath("document"))`.
- `ByNodeID` becomes the type `NodeIDs`, which is `[]cdp.NodeID`. `Value(ids, ByNodeID)` becomes `Value(NodeIDs(ids))`. A plain `[]cdp.NodeID` also selects by node ids.

### Other changes

- A string constant or a string variable is a `Search`. A string type that you define, such as `type MySel string`, is also a `Search`. A slice type that you define with the elements `cdp.NodeID` counts as `NodeIDs`.
- A type outside the `Selectable` set, for example an int, a `*Node` or a `[]string`, does not compile. The panic of `ByNodeID` for a wrong type is gone.
- `ByFunc` stays. It replaces the lookup of the selector. The selector is then only a label in error messages, so pass `""` or any string.
- The action `NodeIDs(sel, opts...)` becomes `QueryNodeIDs(sel, opts...)`, because `NodeIDs` is now a selector type.
- The query actions are generic in the selector type: `Click[S Selectable](sel S, opts ...QueryOption)`. The compiler infers `S` from the argument. `QueryAfter[T, S]` and `JavascriptAttribute[T, S]` take the result type `T` first. Write `JavascriptAttribute[int](sel, "scrollTop")` and the compiler infers `S`.
- `FromNode` works with `CSS` and `CSSAll`. A `Search` and a `JSPath` selector ignore it, as before.

## Migrate to the pipe transport

The `ExecAllocator` now talks to the browser that it starts through a pipe, and
not through a websocket. This changes how the program and Chrome connect. It
does not change the actions or the events.

- The allocator starts Chrome with `--remote-debugging-pipe` and passes two pipes. On Unix they are the file descriptors 3 and 4. On Windows they are two handles that the switch `--remote-debugging-io-pipes` names. It does not add `--remote-debugging-port=0`. Chrome opens no port and writes no `DevTools listening on` line.
- `WSURLReadTimeout` only applies to the websocket mode.
- The file `DevToolsActivePort` in the user data directory does not exist in the pipe mode, because Chrome opens no port.
- Add `remote.WebSocket` to the options of `NewExecAllocator` to get the old behavior. The flags `remote-debugging-port` and `remote-debugging-address` also select the websocket mode, and they need `remote.WebSocket` too. The old name was `chromedp.WebSocket`. See the part "Move the websocket code to the remote module".
- On Windows the allocator uses the pipe too, with handles. A program for Windows needs no `remote.WebSocket`. See `docs/decisions/2026-10-04-the-pipe-works-on-windows.md`.
- The remote allocator connects to a browser that `chromedp` did not start, so it uses the websocket. It moved to the module `remote`.
- `NewBrowserTransport` creates a `Browser` from a `Transport` that is already open. `NewPipeConn` makes a `Transport` for the two pipes of a browser. Use `remote.DialContext` and then `NewBrowserTransport` for a websocket.
- A start that fails now gives an error that starts with `chrome failed to start:` and has the output of Chrome, in the pipe mode and in the websocket mode.

## New options for a visible window

These names are new, and no old code needs a change. Headless mode is still the default. See `docs/decisions/2026-10-03-a-visible-window-is-an-opt-in.md`.

- `WithVisibleWindow` and the allocator option `VisibleWindow` open a visible, maximized window. They replace `Flag("headless", false)`, which left the flags `--hide-scrollbars`, `--mute-audio`, `--enable-automation` and `--disable-extensions` in place.
- The variable `CHROMEDP_VISIBLEWINDOW` does the same with no change in the code. The tests also read the old variable `CHROMEDP_NO_HEADLESS`.
- `ErrNoDisplay` is the error on Linux when a visible window has no display.
- `remote.WithKeepOpen` and the allocator option `KeepOpen` leave the browser open. `KeepOpen` also needs `remote.WebSocket`. `KeptOpen` returns its address and profile directory. `WaitClosed` waits until the browser exits.

## Changes in v0.18.0

These changes need no change in old code, unless a bullet says so.

- `KeyEvent`, `SendKeys` and `KeyEventNode` send no char event for a key with the modifier Ctrl, Alt or Meta. Before, `KeyEvent("a", KeyModifiers(ModifierCtrl))` selected the text on Windows and then typed "a" over it. Alt and Meta shortcuts typed the letter on Linux too. A key with only the modifier Shift keeps its char event. Code that relied on the typed character must send the key without the modifier.
- The default exec allocator options no longer pass `site-per-process` in `--disable-features`. That is a switch name and not a feature name, so Chrome ignored it and the browser behaves as before. See `docs/decisions/2026-10-04-keep-site-isolation-on.md`.
- The target now handles the events `DOM.adRelatedStateUpdated`, `DOM.adoptedStyleSheetsModified`, `DOM.affectedByStartingStylesFlagUpdated` and `DOM.scrollableFlagUpdated`, and the nodes of a frame keep `AdProvenance`, `AdoptedStyleSheets`, `AffectedByStartingStyles` and `IsScrollable` up to date. Before, the first three events made the target log `unhandled node event`, and the last one changed nothing. `DOM.topLayerElementsUpdated` is ignored.
- The websocket connection answers a ping frame of the server with a pong frame, and it reads frames that arrive in the same packet as the handshake. Before, a ping broke the connection, and `DialContext` panicked in the second case.
- On Linux, the allocator starts Chrome from a goroutine that stays on its operating system thread until Chrome exits. Before, Chrome died in some programs when the thread that started it ended.
- `remote.WithDialHTTPHeader` and `remote.WithConnHTTPHeader` set HTTP headers on the websocket request to a remote browser. See example 15 in `docs/API.md`.
- `Run`, `Do`, `Call` and `CallBrowser` now return the exit error of the browser process when the process dies while nobody asked it to stop. The error wraps the error of the context, so `errors.Is(err, context.Canceled)` still works. It also wraps an `*exec.ExitError`, so `errors.As` gives the signal or the exit status. A program that compared the error with `==` to `context.Canceled` must use `errors.Is`. A browser that the program stops with `Cancel` or with a canceled context gives no exit error.
- `WithNewWindow` is new. A context that creates a tab opens it in a new window by default, because a hidden tab gets no animation frames and `Poll` waits for ever on it. `WithNewWindow(false)` opens a real tab in the window of the browser. Make a tab active with `target.ActivateTarget` before you run an action that waits for a frame on it. A child context inherits the choice.
- `LoadError` and `ErrPageLoad` are new. `Navigate`, `NavigateResponse` and `RunResponse` return a `*LoadError` when the page does not load. Its text is the same as before, `page load error` and the text of the browser. Use `errors.Is(err, chromedp.ErrPageLoad)` or `errors.As` with a `*LoadError` and read `ErrorText`. Code that searched the text of the error still works.

## Move the websocket code to the remote module

The core module `github.com/chromedp/chromedp` now uses only the standard library and `cdproto`. The code that needs a websocket moved to the module `github.com/chromedp/chromedp/remote`, which has the package `remote`. A program that attaches to a remote browser, uses a websocket exec allocator, or keeps the browser open runs `go get github.com/chromedp/chromedp/remote` and imports `github.com/chromedp/chromedp/remote`. A program that uses only the default allocator needs no change, on Windows too. See `docs/decisions/2026-10-04-the-core-uses-only-the-standard-library.md`.

The first column has the old name and the second column has the new name.

| Old name | New name |
| --- | --- |
| `chromedp.NewRemoteAllocator(parent, url, opts...)` | `remote.NewAllocator(parent, url, opts...)` |
| `chromedp.RemoteAllocator` | `remote.Allocator` |
| `chromedp.RemoteAllocatorOption` | `remote.Option` |
| `chromedp.WithRemoteDialHTTPHeader(h)` | `remote.WithDialHTTPHeader(h)` |
| `chromedp.NoModifyURL` | `remote.NoModifyURL` |
| `chromedp.WebSocket` | `remote.WebSocket` |
| `chromedp.WithKeepOpen()` | `remote.WithKeepOpen()` |
| `chromedp.Conn` | `remote.Conn` |
| `chromedp.DialContext(ctx, url, opts...)` | `remote.DialContext(ctx, url, opts...)` |
| `chromedp.DialOption` | `remote.DialOption` |
| `chromedp.WithConnHTTPHeader(h)` | `remote.WithConnHTTPHeader(h)` |
| `chromedp.WithConnDebugf(f)` | `remote.WithConnDebugf(f)` |
| `chromedp.ErrInvalidWebsocketMessage` | `remote.ErrInvalidMessage`, a variable made with `errors.New` |
| `chromedp.WithDialHTTPHeader(h)`, a `BrowserOption` | `remote.WithDialHTTPHeader(h)`, an option of the remote allocator |
| `chromedp.WithDialTimeout(d)`, a `BrowserOption` | `remote.WithDialTimeout(d)`, an option of the remote allocator |
| `chromedp.NewBrowser(ctx, url, opts...)` | `remote.DialContext` and then `chromedp.NewBrowserTransport` |
| `(*PipeConn).setDebugf` | `(*PipeConn).SetDebugf`, and `(*remote.Conn).SetDebugf` |

These names stay in the core: `KeepOpen`, `KeptOpen`, `WaitClosed`, `WithVisibleWindow`, `WithNewWindow`, `VisibleWindow`, `WSURLReadTimeout`, `Transport`, `NewBrowserTransport`, `PipeConn` and `NewPipeConn`.

The core has these new names, which the module `remote` uses:

- `Dialer` is the type of a func that connects to a websocket address and returns a `Transport`.
- `WithDialer(d)` is an allocator option. It makes the exec allocator start the browser with a debugging port and connect through `d`.
- `WithAllocatorOptions(opts...)` is a context option. It adds allocator options to the default allocator that `NewContext` builds.
- `Attacher` is an interface that an allocator implements when it attaches to a browser that runs already.
- `NewAllocatorContext(parent, a)` makes the context of an allocator that another module implements.
- `ErrNoDialer` is the error of an exec allocator that needs a websocket and has no dialer.

How to change old code:

- A program that used `chromedp.WithKeepOpen()` writes `remote.WithKeepOpen()`.
- A program that gave `chromedp.KeepOpen` to `NewExecAllocator` adds `remote.WebSocket` to the same options. Without it, the first `Run` returns `ErrNoDialer`.
- A program that gave the flag `remote-debugging-port` or `remote-debugging-address` adds `remote.WebSocket`.
- A program that used `chromedp.WithDialHTTPHeader` or `chromedp.WithDialTimeout` as a `BrowserOption` gives `remote.WithDialHTTPHeader` or `remote.WithDialTimeout` to `remote.NewAllocator`.
- A program for Windows needs no change, because the pipe transport works on Windows.

## Migrate to cdproto v0.157.4

In `cdproto` v0.157.3, a number that is not set was left out of the request, and
there was no way to send the value zero. In v0.157.4, 71 optional number fields
in the parameters and types of commands are pointers, where zero is a different
value from "not set". A nil pointer leaves the field out. A pointer to zero sends
zero, the same as the optional boolean fields. Use `new(0.5)`, or the address of
a variable, to set one.

- The fields are, for example, `Depth` of `dom.RequestChildNodesParams`,
  `dom.GetDocumentParams`, `dom.DescribeNodeParams` and
  `accessibility.GetFullAXTreeParams`, the four margins of `page.PrintToPDFParams`,
  `Quality` of `page.CaptureScreenshotParams`, the seven values of
  `emulation.SetGeolocationOverrideParams`, and `ID`, `RadiusX`, `RadiusY` and
  `Force` of `input.TouchPoint`. The decision
  `docs/decisions/2026-10-04-an-optional-number-can-be-a-pointer.md` of the
  `pdlgen` project lists all of them.
- `FullScreenshot(quality)` now sends the quality that you give,
  also when it is zero. A JPEG with quality 0 is the lowest quality. Before,
  quality 0 was left out and Chrome used its default.
- `network.CookiePartitionKey` decodes both the object that Chrome sends today and
  the plain string that older versions of Chrome send.
- `cdp.ErrInvalidContext` is removed from `cdproto`. `chromedp.ErrInvalidContext` is the
  error of this package.

## Changes since v0.18.0

These names are new, and no old code needs a change, unless a bullet says so.

- `Tap` and `TapXY` are new. They send a touch tap, a `touchStart` event and then a `touchEnd` event, and not mouse events. `Tap` takes a selector and the query options, like `Click`. Turn on touch emulation with `EmulateTouch`, or the page gets no touch events. See example 16 in `docs/API.md`.
- The exec allocator now passes the flags to the browser in the order of the `Flag` options, and not in a random order. A flag that you set again keeps its first place with the new value. The flags of `DefaultExecAllocatorOptions` come first in their listed order. Put `Flag("flag-switches-begin", true)` and `Flag("flag-switches-end", true)` around the switches that Chrome must take as the switches of `chrome://flags`. The names and the use of the options are the same.
- `cdp.Call` on a `Target` or a `Browser` returns an error when the connection to the browser is lost, for example when the browser process is killed. Before, a call with a context that never ends, such as `context.Background()`, waited for ever. The error says `lost the connection to the browser`, and it wraps the error of the connection and `context.Canceled`, as the exit error of `Run` does. A call with a context that the program cancels still returns the error of that context.
- `NoInheritEnv` is a new allocator option. By default, the browser gets the environment of the Go process, with the variables of `Env` added. With `NoInheritEnv`, the browser gets only the variables of `Env` and the variables that a `ModifyCmdFunc` func sets in `cmd.Env`. Add to `Env` each variable that Chrome needs, such as `HOME` or `DISPLAY`. The default is the same as before.
- A `JSPath` expression can give a node, an array or a `NodeList` of nodes, or null or undefined. Null, undefined and an empty list select no element. Before, they made the lookup fail and retry until the timeout, so `WaitNotPresent(JSPath(...))` never succeeded. It succeeds now, and `WaitVisible` and the other waits keep waiting until a node appears. An expression that gives any other value, such as a number or a string, returns an error at once and does not retry.
- `WithDetachOnCancel` is a new context option. By default, the cancellation of a context closes its tab, as before. With the option, it detaches from the tab and leaves the tab open, and it keeps a browser context that `WithNewBrowserContext` made. Use it with a remote browser that keeps its tabs. See example 17 in `docs/API.md`.
- `DragAndDrop` and `DragAndDropXY` are new. `DragAndDrop` takes two selectors, which can have different types, and the query options. It drags the first element of the first selector and drops it on the first element of the second selector. `DragAndDropXY` takes two points and an optional number of steps. Both work for a page that listens for mouse events and for a page with HTML5 drag and drop, because they use `input.SetInterceptDrags` and `input.DispatchDragEvent` when the browser starts a native drag. The old API had no such action. A program sent `MouseEvent` actions by hand. See example 24 in `docs/API.md`.
- `Console`, `ConsoleMessage` and `ConsoleType` are new, with the constants `ConsoleLog`, `ConsoleDebug`, `ConsoleInfo`, `ConsoleWarning`, `ConsoleError` and `ConsoleException`. `Console(ctx)` returns an `iter.Seq2[ConsoleMessage, error]` of the console API calls, the uncaught exceptions and the entries of the browser log, in the order that they arrive. It joins three events that the old code read with `ListenTarget`: `*runtime.EventConsoleAPICalled`, `*runtime.EventExceptionThrown` and `*log.EventEntryAdded`. `Events` gives one event type, as the protocol sends it, so use `Events` when you need the raw event, and use `Console` when you need the text. Like `Events`, `Console` subscribes when it returns, so a program can subscribe, run actions and then range. The old code needed a type switch in a func, and a mutex or a channel to hand the values to other code. The line and the column of a `ConsoleMessage` start at 0, as in the protocol. See example 25 in `docs/API.md`.
