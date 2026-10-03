# Stop relying on helpers that cdproto no longer generates

Status: Decided.

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

## The choice

The maintainer chose on 2026-10-03 that `chromedp` defines its own types. It has
a `Node` type and a `Frame` type that embed the protocol types and add the tree
state and the methods that the generated types used to have, such as
`Attribute`, `AttributeValue`, `Dump` and the XPath helpers. A user changes
`*cdp.Node` to `*chromedp.Node`, and the method calls stay the same. `chromedp`
also defines the key and mouse constants and the modifier type that it used
from `input`.

The other choices were to keep `*cdp.Node` and turn the methods into
functions, which breaks every method call in user code, and to copy the old
generated types into a sub-package, which keeps a second copy of the protocol
structs that must follow every change.
