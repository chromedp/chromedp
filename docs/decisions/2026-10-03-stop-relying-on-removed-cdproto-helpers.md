# Stop relying on helpers that cdproto no longer generates

Status: Decided.

The maintainer proposed on 2026-10-03 that `chromedp` stops relying on helpers
that `cdproto` no longer generates.

`pdlgen` drops its fixups. Only the fix for stuttering names stays. `cdproto`
will be tagged `v0.<chromium major>.<patch>` automatically. A helper that a
fixup added then disappears from the generated code. The proposal names these
helpers:

- The `cdp.Node` methods, such as `Attribute` and the XPath helpers.
- `cdp.FrameState`, `input.Modifier` and the DOM `NodeType`.
- The `Timestamp` types.
- The `Error` method of the Runtime `ExceptionDetails`.

## The uses

A search of this repository found these uses before the port.

In the package code:

- `cdp.FrameState` with `cdp.FrameAttached` and `cdp.FrameLoading`, in
  `util.go`.
- `cdp.NodeTypeElement` and `cdp.NodeTypeText`, in `query.go`.
- `input.Modifier`, in the signatures of `ButtonModifiers` and `KeyModifiers`
  in `input.go`.
- The embedded `RLock` and `RUnlock` of `cdp.Node`, which `query.go` and
  `target.go` use.

In the tests and examples only:

- `Node.Dump` in `util_test.go` and `example_test.go`.
- `Node.FullXPath` in `input_test.go` and `query_test.go`.
- `Node.AttributeValue` in `query_test.go`.
- `ExceptionDetails.Error` in `example_test.go`.
- `ResponseTime.Time`, a timestamp type, in `chromedp_test.go`.

The package code used no other helper from the proposal. `Node.Attribute` and
the partial XPath helpers did not appear.

## The choice

The maintainer chose on 2026-10-03 that `chromedp` defines its own types. It
has a `Node` type and a `Frame` type. They embed the protocol types. They add
the tree state and the methods that the generated types had, such as
`Attribute`, `AttributeValue`, `Dump` and the XPath helpers. A user changes
`*cdp.Node` to `*chromedp.Node`, and the method calls stay the same.
`chromedp` also defines the key and mouse constants and the modifier type that
it took from `input`. `../MIGRATION.md` lists the renamed names.

The maintainer rejected two other choices:

- Keep `*cdp.Node` and turn the methods into functions. This breaks every
  method call in user code.
- Copy the old generated types into a sub-package. This keeps a second copy of
  the protocol structs, and that copy must follow every change.
