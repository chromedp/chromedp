# A visible window is an opt-in

Status: Proposed.

Headless mode stays the default. A program that wants a visible window, or a
window that stays open after the program ends, asks for it. The change is on
the local branch `typed-api`. The maintainer fixed the names. Nobody has
approved the change as a whole, and it is not on GitHub.

## Why headless stays the default

- CI and containers have no display, so a default window fails there.
- Scripts run unattended, so a window has nobody to watch it.
- A visible window takes the focus from the person who works at the machine.

## The names

- `chromedp.WithVisibleWindow()` is a `ContextOption`. It makes `NewContext` build the default allocator without `Headless` and with `VisibleWindow`.
- `chromedp.VisibleWindow` is an `ExecAllocatorOption`. It edits the flags of the allocator. `NewExecAllocator(ctx, append(DefaultExecAllocatorOptions[:], VisibleWindow)...)` works too, and `WithVisibleWindow` uses it.
- The variable `CHROMEDP_VISIBLEWINDOW` has the same effect as `WithVisibleWindow`, with no change in the code. Any value other than the empty string, `false` and `0` turns it on. It applies where `NewContext` builds the default allocator.
- `NewContext` builds its option list in one func, from `DefaultExecAllocatorOptions`, so the context option and the allocator option cannot drift.
- The test helper reads `CHROMEDP_VISIBLEWINDOW`. It still reads the old name `CHROMEDP_NO_HEADLESS`.

## An allocator that the caller made

`WithVisibleWindow` and `WithKeepOpen` have no effect on an allocator that the
caller made with `NewExecAllocator` or `NewRemoteAllocator`. `NewContext` does
not build that allocator, so it does not change it. The doc comments say so. A
caller who makes an allocator adds `VisibleWindow` or `KeepOpen` to its options.
The environment variable has no effect there either.

## The flags

`VisibleWindow` removes `--headless`, `--hide-scrollbars`, `--mute-audio`,
`--enable-automation` and `--disable-extensions`. The infobar that
`--enable-automation` shows covers part of the page. It keeps every other flag of
the default list, and it adds `--start-maximized`. It does not open the
developer tools. A user adds `Flag("auto-open-devtools-for-tabs", true)` for
that. `Headless` is not changed.

## No display

On Linux, a visible window needs `DISPLAY` or `WAYLAND_DISPLAY`. When both are
empty, the first `Run` returns `ErrNoDisplay`. Its message names
`CHROMEDP_VISIBLEWINDOW` as the variable to unset. The allocator never falls
back to headless mode in silence, because the person asked to see a window.
macOS and Windows have no check.

## Leave the browser open

`WithKeepOpen()` and the allocator option `KeepOpen` leave the browser open when
the program ends or the context is canceled. They work with headless mode too.
`KeptOpen(ctx)` returns the websocket address and the user data directory, so
that a program can print them. The library prints nothing. A later program
attaches with `NewRemoteAllocator`. `WaitClosed(ctx)` blocks until the browser
process exits or the context ends, for a program that must stay alive while the
window is open.

The traps, and what the code does about each one:

- The pipe cannot survive the program. It closes when the Go process exits, and Chrome then exits. So `KeepOpen` always uses the websocket, with `--remote-debugging-port=0`. The allocator reads the file `DevToolsActivePort` in the user data directory, and not the output of Chrome, because nothing reads that output after the program ends.
- The process must not die with the program. The allocator starts it in a new session on Unix (`Setsid`) and detached with a new process group on Windows. It does not use `exec.CommandContext` and it does not set `Pdeathsig`. `Cancel` does not close the browser.
- The profile directory stays. Without `UserDataDir`, the directory is `keepopen-PID` in the `chromedp` directory of `os.UserCacheDir`, and the allocator never removes it. The user must delete it. Two kept browsers of one program share that name, so a program that keeps more than one open must set `UserDataDir`.
- `killLeftovers` on Linux must skip a kept browser, so a later run does not kill it. It skips the default kept directories and every directory that the program started with `KeepOpen`.
- A child process that nobody waits for becomes a zombie. A goroutine calls `cmd.Wait`, so no zombie stays while the program runs. After the program ends, the init process reaps the browser.
- The allocator deletes the old `DevToolsActivePort` file before it starts Chrome, because a file of an earlier run has an address that does not work.
- The output of Chrome goes nowhere, so `CombinedOutput` has no effect.

## What remains

The maintainer must approve the names and the flags. No Windows machine ran the
tests, so the Windows detach flags are not tested. A test for the visible
window needs a display, and it skips on Linux without one.
