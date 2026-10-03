# Wrap errors with %w

Status: Decided.

On 2021-04-29 the commit `254eeac` changed the error wrapping to the Go 1.13
`%w` verb. Callers can then use `errors.Is` and `errors.As` on an error that
`chromedp` returns.

Write `fmt.Errorf("context: %w", err)`. Do not use `%s` or `%v` for an error,
because that drops the chain.
