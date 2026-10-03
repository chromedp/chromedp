# Use a pipe to the browser by default

Status: Decided.

The `ExecAllocator` starts Chrome with `--remote-debugging-pipe` and talks to it
through two pipes, and not through a websocket. The maintainer approved the
change. The work was done on a branch and merged on 2026-10-04.

## Why

Puppeteer and the other DevTools packages use the pipe. A pipe needs no port, so
there is no port to find, to secure or to clash with another process. The program
does not wait for the `DevTools listening on` line and does not parse it. When
the program exits, the pipe closes and Chrome exits.

## The protocol

Chrome reads the commands from its file descriptor 3 and writes the responses
and the events to its file descriptor 4. Each JSON message ends with one zero
byte, and not with a newline, in both directions. In Go, `os/exec` passes the
files in `Cmd.ExtraFiles`. Index 0 becomes file descriptor 3 and index 1 becomes
4. Both Chrome and `headless-shell` support the flag.

## What the change does

- `pipe.go` has `PipeConn`, a `Transport` for the two pipe ends. It reads up to the zero byte, so a message can arrive in several reads, and it can be large. A write is one call under a lock.
- `NewBrowserTransport` makes a `Browser` from an open `Transport`. `NewBrowser` dials and then calls it.
- The pipe is the default of `ExecAllocator`. The new option `WebSocket` restores the websocket. A `remote-debugging-port` or `remote-debugging-address` flag also selects it, and so does the option `KeepOpen`. On Windows the allocator first used the websocket, because `os/exec` does not support `ExtraFiles` there. The decision `2026-10-04-the-pipe-works-on-windows.md` replaced that part.
- The allocator keeps the output of Chrome in a buffer of 64 KiB and in the combined output writer. It sends `Browser.getVersion` as the first command. If the pipe closes before the answer, the allocator waits for Chrome to exit and returns `chrome failed to start:` with the output.
- A killed Chrome closes the pipe, and `Browser.LostConnection` closes. A graceful close sends `Browser.close`, and Chrome exits and closes the pipe.
- `RemoteAllocator` still uses the websocket.

## What this changes for users

- `WSURLReadTimeout` only applies to the websocket mode.
- Chrome writes no `DevToolsActivePort` file in the pipe mode.
- A program that connects a second tool to the browser through the debugging port must use `WebSocket`.

## What remains

The maintainer decided that the pipe is the default. The decision
`2026-10-04-the-pipe-works-on-windows.md` made the pipe work on Windows, and the
tests ran there.

## Later change

The decision `2026-10-04-the-core-uses-only-the-standard-library.md` moved the
websocket code to the module `remote`. The option `WebSocket` is now
`remote.WebSocket`, the allocator needs a dialer for the websocket mode,
`NewBrowser` is gone, and `RemoteAllocator` is `remote.Allocator`.
