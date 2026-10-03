package chromedp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/chromedp/cdproto"
)

// nopWriteCloser adds a Close method to a writer.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// fakePipeConn returns a PipeConn that reads from in and writes to out.
func fakePipeConn(in io.Reader, out io.Writer) *PipeConn {
	return NewPipeConn(io.NopCloser(in), nopWriteCloser{out})
}

func TestPipeConnRead(t *testing.T) {
	t.Parallel()

	const (
		msg1 = `{"id":1,"result":{"a":"b"}}`
		msg2 = `{"method":"Page.frameNavigated","params":{"x":[1,2,3]}}`
	)
	big := `{"id":3,"result":{"data":"` + strings.Repeat("x", 5<<20) + `"}}`

	tests := []struct {
		name   string
		input  string
		reader func(io.Reader) io.Reader
		ids    []int64
	}{
		{name: "one message", input: msg1 + "\x00", ids: []int64{1}},
		{
			name:   "split over several reads",
			input:  msg1 + "\x00",
			reader: iotest.OneByteReader,
			ids:    []int64{1},
		},
		{
			name:   "several messages in one read",
			input:  msg1 + "\x00" + msg2 + "\x00" + msg1 + "\x00",
			reader: func(r io.Reader) io.Reader { return r },
			ids:    []int64{1, 0, 1},
		},
		{name: "large message", input: big + "\x00", ids: []int64{3}},
		{
			name:   "large message in small reads",
			input:  big + "\x00" + msg1 + "\x00",
			reader: func(r io.Reader) io.Reader { return iotest.HalfReader(r) },
			ids:    []int64{3, 1},
		},
		{name: "empty messages are skipped", input: "\x00" + msg1 + "\x00\x00", ids: []int64{1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var r io.Reader = strings.NewReader(test.input)
			if test.reader != nil {
				r = test.reader(r)
			}
			c := fakePipeConn(r, io.Discard)
			for i, id := range test.ids {
				var msg cdproto.Message
				if err := c.Read(context.Background(), &msg); err != nil {
					t.Fatalf("message %d: %v", i, err)
				}
				if msg.ID != id {
					t.Fatalf("message %d: got id %d, want %d", i, msg.ID, id)
				}
			}
			var msg cdproto.Message
			if err := c.Read(context.Background(), &msg); err != io.EOF {
				t.Fatalf("got %v at the end, want io.EOF", err)
			}
		})
	}
}

func TestPipeConnReadEOF(t *testing.T) {
	t.Parallel()

	t.Run("closed pipe", func(t *testing.T) {
		pr, pw := io.Pipe()
		c := NewPipeConn(pr, nopWriteCloser{io.Discard})
		go pw.Close()
		var msg cdproto.Message
		if err := c.Read(context.Background(), &msg); err != io.EOF {
			t.Fatalf("got %v, want io.EOF", err)
		}
	})
	t.Run("message without the zero byte", func(t *testing.T) {
		c := fakePipeConn(strings.NewReader(`{"id":1}`), io.Discard)
		var msg cdproto.Message
		if err := c.Read(context.Background(), &msg); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("got %v, want io.ErrUnexpectedEOF", err)
		}
	})
	t.Run("bad JSON", func(t *testing.T) {
		c := fakePipeConn(strings.NewReader("{\"id\":\x00"), io.Discard)
		var msg cdproto.Message
		if err := c.Read(context.Background(), &msg); err == nil || err == io.EOF {
			t.Fatalf("got %v, want a decoding error", err)
		}
	})
}

func TestPipeConnWrite(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var debug []string
	c := fakePipeConn(strings.NewReader(""), &out)
	c.setDebugf(func(format string, args ...any) { debug = append(debug, fmt.Sprintf(format, args...)) })

	msg := &cdproto.Message{ID: 7, Method: "Browser.getVersion"}
	if err := c.Write(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	want := `{"id":7,"method":"Browser.getVersion"}` + "\x00"
	if got := out.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if len(debug) != 1 || debug[0] != "-> "+strings.TrimSuffix(want, "\x00") {
		t.Fatalf("got debug output %q", debug)
	}
}

// TestPipeConnWriteConcurrent checks that the messages of several writers
// do not mix. A large message needs more than one write to a real pipe.
func TestPipeConnWriteConcurrent(t *testing.T) {
	t.Parallel()

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	c := NewPipeConn(io.NopCloser(strings.NewReader("")), pw)

	const writers = 8
	payload := fmt.Sprintf(`"%s"`, strings.Repeat("y", 300<<10))
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			msg := &cdproto.Message{ID: int64(i + 1), Method: "Test.big", Params: []byte(payload)}
			if err := c.Write(context.Background(), msg); err != nil {
				t.Error(err)
			}
		})
	}
	go func() {
		wg.Wait()
		pw.Close()
	}()

	rc := fakePipeConn(pr, io.Discard)
	seen := make(map[int64]bool)
	for {
		var msg cdproto.Message
		err := rc.Read(context.Background(), &msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen[msg.ID] = true
	}
	if len(seen) != writers {
		t.Fatalf("read %d different messages, want %d", len(seen), writers)
	}
}

func TestPipeConnClose(t *testing.T) {
	t.Parallel()

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cr, cw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer cr.Close()
	defer cw.Close()
	c := NewPipeConn(pr, pw)

	done := make(chan error, 1)
	go func() {
		var msg cdproto.Message
		done <- c.Read(context.Background(), &msg)
	}()
	time.Sleep(50 * time.Millisecond)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Read returned without an error after Close")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Read did not return after Close")
	}
	// A second Close returns the same result.
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Write(context.Background(), &cdproto.Message{ID: 1}); err == nil {
		t.Fatal("Write succeeded after Close")
	}
}

// TestBrowserTransportPipe runs a browser over fake pipes, with a goroutine
// that plays Chrome.
func TestBrowserTransportPipe(t *testing.T) {
	t.Parallel()

	cmdR, cmdW := io.Pipe() // commands: the browser writes, the fake reads
	resR, resW := io.Pipe() // responses: the fake writes, the browser reads

	tr := NewPipeConn(resR, cmdW)
	var debugMu sync.Mutex
	var debug []string
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	b, err := NewBrowserTransport(ctx, tr, WithBrowserDebugf(func(format string, args ...any) {
		debugMu.Lock()
		defer debugMu.Unlock()
		debug = append(debug, fmt.Sprintf(format, args...))
	}))
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		fake := NewPipeConn(cmdR, resW)
		for {
			var msg cdproto.Message
			if err := fake.Read(ctx, &msg); err != nil {
				resW.Close()
				return
			}
			fake.Write(ctx, &cdproto.Message{ID: msg.ID, Result: []byte(`{"product":"Fake"}`)})
		}
	}()

	var res struct {
		Product string `json:"product"`
	}
	if err := b.Call(ctx, "Browser.getVersion", nil, &res); err != nil {
		t.Fatal(err)
	}
	if res.Product != "Fake" {
		t.Fatalf("got product %q, want Fake", res.Product)
	}
	debugMu.Lock()
	got := strings.Join(debug, "\n")
	debugMu.Unlock()
	if !strings.Contains(got, "-> {") || !strings.Contains(got, "<- {") {
		t.Fatalf("got debug output %q, want a line for each direction", got)
	}

	// The fake ends its side, like a Chrome that exits.
	cmdW.Close()
	select {
	case <-b.LostConnection:
	case <-time.After(5 * time.Second):
		t.Fatal("LostConnection was not closed")
	}
}

// skipUnlessPipe skips a test that needs the pipe mode of ExecAllocator.
func skipUnlessPipe(t *testing.T) {
	t.Helper()
	if !usePipe() {
		t.Skip("the platform cannot pass extra file descriptors")
	}
}

func TestExecAllocatorPipe(t *testing.T) {
	t.Parallel()
	skipUnlessPipe(t)

	allocCtx, cancel := NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, _ := NewContext(allocCtx)

	var got string
	if err := Do(ctx,
		Navigate(testdataDir+"/form.html"),
		into(&got, Text(ID("foo"))),
	); err != nil {
		t.Fatal(err)
	}
	if want := "insert"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	b := FromContext(ctx).Browser
	if _, ok := b.conn.(*PipeConn); !ok {
		t.Fatalf("got transport %T, want *PipeConn", b.conn)
	}
	if err := Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(b.userDataDir); !os.IsNotExist(err) {
		t.Fatalf("temporary user data dir %q not deleted", b.userDataDir)
	}
	select {
	case <-b.LostConnection:
	default:
		t.Fatal("the pipe is still open after Cancel")
	}
}

// TestExecAllocatorPipeKillBrowser checks that a killed Chrome closes the pipe.
func TestExecAllocatorPipeKillBrowser(t *testing.T) {
	t.Parallel()
	skipUnlessPipe(t)

	ctx, _ := testAllocateSeparate(t)
	b := FromContext(ctx).Browser
	if _, ok := b.conn.(*PipeConn); !ok {
		t.Fatalf("got transport %T, want *PipeConn", b.conn)
	}
	if err := b.process.Signal(os.Kill); err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.LostConnection:
	case <-time.After(10 * time.Second):
		t.Fatal("LostConnection was not closed after Chrome was killed")
	}
	// The context is canceled and the directory is removed.
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the context was not cancelled")
	}
	if err := Do(ctx, Navigate(testdataDir+"/form.html")); err == nil {
		t.Fatal("expected an error after Chrome was killed")
	}
	// Wait for the allocator to remove the directory.
	<-FromContext(ctx).allocated
	if _, err := os.Lstat(b.userDataDir); !os.IsNotExist(err) {
		t.Fatalf("temporary user data dir %q not deleted", b.userDataDir)
	}
}

func TestExecAllocatorPipeStartFailure(t *testing.T) {
	t.Parallel()
	skipUnlessPipe(t)

	t.Run("pipe not open", func(t *testing.T) {
		t.Parallel()

		// Chrome exits at once when its file descriptors 3 and 4 are
		// not open.
		buf := new(syncBuffer)
		allocCtx, cancel := NewExecAllocator(context.Background(),
			append([]ExecAllocatorOption{
				CombinedOutput(buf),
				ModifyCmdFunc(func(cmd *exec.Cmd) { cmd.ExtraFiles = nil }),
			}, allocOpts...)...)
		defer cancel()
		ctx, cancel := NewContext(allocCtx)
		defer cancel()

		err := Do(ctx)
		if err == nil || !strings.Contains(err.Error(), "failed to start") {
			t.Fatalf("got %v, want an error about the start", err)
		}
		if !strings.Contains(err.Error(), "Remote debugging pipe file descriptors are not open") {
			t.Fatalf("the error does not have the output of Chrome: %v", err)
		}
		if !strings.Contains(buf.String(), "Remote debugging pipe file descriptors are not open") {
			t.Fatalf("the combined output does not have the output of Chrome: %q", buf.String())
		}
	})
	t.Run("program that exits", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("needs a shell script")
		}

		script := filepath.Join(t.TempDir(), "fake-chrome")
		if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'fake chrome says no' >&2\nexit 3\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		allocCtx, cancel := NewExecAllocator(context.Background(),
			append(allocOpts, ExecPath(script))...)
		defer cancel()
		ctx, cancel := NewContext(allocCtx)
		defer cancel()

		err := Do(ctx)
		if err == nil || !strings.Contains(err.Error(), "chrome failed to start") ||
			!strings.Contains(err.Error(), "fake chrome says no") {
			t.Fatalf("got %v, want the output of the program", err)
		}
	})
}

// TestExecAllocatorWebSocket checks that the websocket mode still works, and
// that the flags for a debugging port or address select it.
func TestExecAllocatorWebSocket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []ExecAllocatorOption
	}{
		{name: "WebSocket option", opts: []ExecAllocatorOption{WebSocket}},
		{name: "remote-debugging-port flag", opts: []ExecAllocatorOption{Flag("remote-debugging-port", "0")}},
		{name: "remote-debugging-address flag", opts: []ExecAllocatorOption{Flag("remote-debugging-address", "127.0.0.1")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			allocCtx, cancel := NewExecAllocator(context.Background(),
				append(append([]ExecAllocatorOption{}, test.opts...), allocOpts...)...)
			defer cancel()
			ctx, _ := NewContext(allocCtx)

			var got string
			if err := Do(ctx,
				Navigate(testdataDir+"/form.html"),
				into(&got, Text(ID("foo"))),
			); err != nil {
				t.Fatal(err)
			}
			if want := "insert"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
			b := FromContext(ctx).Browser
			if _, ok := b.conn.(*Conn); !ok {
				t.Fatalf("got transport %T, want *Conn", b.conn)
			}
			if err := Cancel(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(b.userDataDir); !os.IsNotExist(err) {
				t.Fatalf("temporary user data dir %q not deleted", b.userDataDir)
			}
		})
	}
}
