# Stop relying on helpers that cdproto no longer generates

Status: Proposed.

The maintainer proposed on 2026-10-03 that `chromedp` stops relying on helpers that
`cdproto` no longer generates.

`pdlgen` is dropping its fixups. Only the fix for stuttering names stays.
`cdproto` will be tagged `v0.<chromium major>.<patch>` automatically. A helper
that a fixup added will then disappear from the generated code. The helpers
that the proposal names are the `cdp.Node` methods such as `Attribute` and the
XPath helpers, `cdp.FrameState`, `input.Modifier`, the `Timestamp` types, the
DOM `NodeType` and the `Error` method of the Runtime `ExceptionDetails`.

The uses that a search of this repository found are listed in `../PLAN.md`,
under Open questions. Package code uses `cdp.FrameState`,
`cdp.NodeTypeElement`, `cdp.NodeTypeText` and `input.Modifier`. Tests and
examples use a few more.

If the maintainer accepts this, `chromedp` must define its own version of each helper it
needs, or must change its API so that it does not expose one. A public
signature that takes `input.Modifier` makes the second choice a breaking
change. The maintainer must choose.
