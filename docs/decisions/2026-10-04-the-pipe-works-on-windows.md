# The pipe works on Windows

Status: Amends 2026-10-03-use-a-pipe-to-the-browser-by-default.md and 2026-10-04-the-core-uses-only-the-standard-library.md.

The maintainer wanted plain `chromedp` code to work on Windows, with no
websocket and no `remote` module. The exec allocator now uses the pipe on
Windows, as it does on Linux and macOS. The earlier decisions said that Windows
always uses the websocket. That part is replaced.

## How it works

On Unix, the browser reads its commands from the file descriptor 3 and writes to
the file descriptor 4. A Windows process has no such descriptors. Chromium has a
switch for Windows instead, `--remote-debugging-io-pipes=<read>,<write>`. The
value is two handles as unsigned numbers. The first handle is the pipe that the
browser reads the commands from. The second is the pipe that it writes the
responses and the events to. Chromium takes each handle with `_open_osfhandle`
after it checks that the handle is a pipe. The switch goes together with
`--remote-debugging-pipe`. The source is in
`content/browser/devtools/devtools_agent_host_impl.cc`, and the switch is in
`content/public/common/content_switches.cc`. Google Chrome 154 and Microsoft
Edge 154 accept it.

The allocator does this on Windows:

1. It makes the two pipes with `os.Pipe`, which makes inheritable handles.
2. It clears the inherit flag on the ends that the parent keeps, so that no other process that the program starts can get them.
3. It puts the two ends of the browser in `SysProcAttr.AdditionalInheritedHandles`, and writes their values in the switch. A handle that a child inherits has the same value in the child.
4. After the process starts, it closes the two ends of the browser in the parent.

The messages are the same as on Unix, JSON that ends with a zero byte. The code
reuses `PipeConn`. Closing the connection closes the pipes, and the browser
exits. A killed browser closes the pipes, and the allocator sees the loss.

The function `usePipe` is gone. `pipe_unix.go` and `pipe_windows.go` each hold
one function, `setChildPipes`, which gives the ends of the browser to the
process in the way of the platform.

## What this changes for users

- A program for Windows needs no `remote.WebSocket`. The first `Run` starts the browser through the pipe.
- `ErrNoDialer` happens only when the program asks for the websocket, with `KeepOpen` or with a debugging port flag, and gives no dialer.
- A `ModifyCmdFunc` that replaces `cmd.SysProcAttr` on Windows removes the handles. Change the fields of the value that the allocator made instead.

## What this found on Windows

The tests ran on Windows 11 with Chrome 154 and Edge 154. The two browsers gave
the same results. These differences from Linux are not bugs of the library:

- Windows has no signals. A killed process ends with `exit status 1`, and the tests accept that text.
- The browser ignores the `TZ` variable on Windows, so `TestEnv` and `TestModifyCmdFunc` skip there.
- A press of the middle button on a page that scrolls starts autoscroll on Windows. Then the browser sends no `auxclick` event, so that case of `TestMouseClickNode` skips there.

Two things were library problems. `Allocate` now checks the context before it
starts the process, because `os/exec` reports a missing program on Windows before
it looks at the context. The call that fails after a browser died waited for the
exit error through a channel that also waits for the output of the browser. On
Windows the child processes of the browser hold the output open, so the wait
ended too late sometimes. A new channel closes when the process is reaped.
