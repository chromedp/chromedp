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
