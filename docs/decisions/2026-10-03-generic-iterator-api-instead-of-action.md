# Replace the Action interface with a generic and iterator API

Status: Proposed.

The maintainer asked on 2026-10-03 whether an API built on generics and iterators must
replace the `chromedp.Action` interface. This is a question, and nobody has
chosen.

The shape under discussion:

- A typed `Command[P, R]` descriptor for each protocol command, run by a free
  `Execute` function.
- An explicit `Session` value in place of an executor that hides in the context.
- Typed events delivered as an `iter.Seq2`.
- An `Action[T]` that returns its value.

The reasons for it:

- A positional return tuple breaks when the protocol adds a field. A new
  return value changes the signature, so every caller fails to compile.
- The executor hides in `ctx`. A call without it fails at runtime with
  "invalid context", not at compile time.
- A result comes back through an out pointer, such as `*string`, which the
  caller must declare first.
- Events are untyped. A listener receives `any` and switches on the type.

The cost is a break with every program that uses `Action`, `Run` and
`ActionFunc`. The maintainer must weigh that. See the open questions in `../PLAN.md`.
