package websocket

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeConn is the network side of a Conn in a unit test. It reads the bytes of
// in, and it records the bytes that the client writes and the number of Write
// calls.
type fakeConn struct {
	in     io.Reader
	mu     sync.Mutex
	out    bytes.Buffer
	writes int
	closed bool
}

func (f *fakeConn) Read(p []byte) (int, error) { return f.in.Read(p) }

func (f *fakeConn) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	return f.out.Write(p)
}

func (f *fakeConn) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeConn) written() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return bytes.Clone(f.out.Bytes())
}

// newTestConn makes a Conn that reads the given bytes.
func newTestConn(t testing.TB, in []byte) (*Conn, *fakeConn) {
	t.Helper()
	f := &fakeConn{in: bytes.NewReader(in)}
	c, err := newConn(f, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c, f
}

// clientFrame is a frame that the client wrote.
type clientFrame struct {
	fin     bool
	op      Opcode
	payload []byte
}

// parseClientFrames parses the bytes that the client wrote. It does so with
// its own code, and it fails when a frame is not masked.
func parseClientFrames(t testing.TB, b []byte) []clientFrame {
	t.Helper()
	var frames []clientFrame
	for len(b) > 0 {
		if len(b) < 2 {
			t.Fatalf("a client frame is cut: %x", b)
		}
		f := clientFrame{fin: b[0]&0x80 != 0, op: Opcode(b[0] & 0xf)}
		if b[0]&0x70 != 0 {
			t.Fatalf("the client set a reserved bit: %x", b[0])
		}
		if b[1]&0x80 == 0 {
			t.Fatalf("the client did not mask a frame")
		}
		n, b2 := int(b[1]&0x7f), b[2:]
		switch n {
		case 126:
			n, b2 = int(b2[0])<<8|int(b2[1]), b2[2:]
		case 127:
			n = 0
			for _, v := range b2[:8] {
				n = n<<8 | int(v)
			}
			b2 = b2[8:]
		}
		key, rest := b2[:4], b2[4:]
		f.payload = make([]byte, n)
		for i := range f.payload {
			f.payload[i] = rest[i] ^ key[i%4]
		}
		frames = append(frames, f)
		b = rest[n:]
	}
	return frames
}

// frame builds an unmasked server frame by hand.
func frame(fin bool, op Opcode, payload string) []byte {
	b0 := byte(op)
	if fin {
		b0 |= 0x80
	}
	n := len(payload)
	var b []byte
	switch {
	case n <= 125:
		b = []byte{b0, byte(n)}
	case n <= 0xffff:
		b = []byte{b0, 126, byte(n >> 8), byte(n)}
	default:
		b = []byte{b0, 127, 0, 0, 0, 0, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	}
	return append(b, payload...)
}

func cat(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

func closePayload(code uint16, reason string) string {
	return string([]byte{byte(code >> 8), byte(code)}) + reason
}

// TestReadMessage reads the valid examples of RFC 6455 section 5.7 and some
// more.
func TestReadMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       []byte
		wantOp   Opcode
		wantData string
		// pongs are the payloads of the pong frames that the client sends.
		pongs []string
	}{
		{"unmasked Hello", mustHex(t, "81 05 48656c6c6f"), OpText, "Hello", nil},
		{"empty text", mustHex(t, "81 00"), OpText, "", nil},
		{"fragmented Hel and lo", mustHex(t, "01 03 48656c 80 02 6c6f"), OpText, "Hello", nil},
		{"three fragments", cat(frame(false, OpText, "a"), frame(false, OpContinuation, "b"), frame(true, OpContinuation, "c")), OpText, "abc", nil},
		{"empty fragments", cat(frame(false, OpText, ""), frame(false, OpContinuation, ""), frame(true, OpContinuation, "x")), OpText, "x", nil},
		{"rune split over fragments", cat(frame(false, OpText, "\xe2\x82"), frame(true, OpContinuation, "\xac")), OpText, "€", nil},
		{"ping then message", cat(mustHex(t, "89 05 48656c6c6f"), frame(true, OpText, "x")), OpText, "x", []string{"Hello"}},
		{"empty ping", cat(frame(true, OpPing, ""), frame(true, OpText, "x")), OpText, "x", []string{""}},
		{"ping inside a fragmented message", cat(frame(false, OpText, "Hel"), frame(true, OpPing, "p"), frame(true, OpContinuation, "lo")), OpText, "Hello", []string{"p"}},
		{"pong is ignored", cat(mustHex(t, "8a 05 48656c6c6f"), frame(true, OpText, "x")), OpText, "x", nil},
		{"pong inside a fragmented message", cat(frame(false, OpText, "a"), frame(true, OpPong, "z"), frame(true, OpContinuation, "b")), OpText, "ab", nil},
		{"binary 256 bytes", cat(mustHex(t, "82 7e 0100"), bytes.Repeat([]byte{7}, 256)), OpBinary, strings.Repeat("\x07", 256), nil},
		{"binary 64 KiB", cat(mustHex(t, "82 7f 0000000000010000"), bytes.Repeat([]byte{9}, 65536)), OpBinary, strings.Repeat("\x09", 65536), nil},
		{"long encoding of a short length", mustHex(t, "81 7e 0002 6869"), OpText, "hi", nil},
		{"text of 70000 bytes", frame(true, OpText, strings.Repeat("a", 70000)), OpText, strings.Repeat("a", 70000), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConn(t, tt.in)
			op, data, err := c.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			if op != tt.wantOp || string(data) != tt.wantData {
				t.Fatalf("got %s %.20q, want %s %.20q", op, data, tt.wantOp, tt.wantData)
			}
			frames := parseClientFrames(t, f.written())
			if len(frames) != len(tt.pongs) {
				t.Fatalf("got %d frames from the client, want %d", len(frames), len(tt.pongs))
			}
			for i, p := range tt.pongs {
				if frames[i].op != OpPong || !frames[i].fin || string(frames[i].payload) != p {
					t.Errorf("frame %d is %+v, want a pong with %q", i, frames[i], p)
				}
			}
		})
	}
}

// TestReadMessageSequence reads two messages in a row. The first buffer can be
// reused for the second.
func TestReadMessageSequence(t *testing.T) {
	t.Parallel()

	c, _ := newTestConn(t, cat(frame(true, OpText, "one"), frame(true, OpText, "second")))
	for _, want := range []string{"one", "second"} {
		_, data, err := c.ReadMessage()
		if err != nil || string(data) != want {
			t.Fatalf("got %q and %v, want %q", data, err, want)
		}
	}
	if _, _, err := c.ReadMessage(); err != io.EOF {
		t.Fatalf("got %v at the end of the stream, want io.EOF", err)
	}
}

// TestReadMessageErrors covers each class of protocol error. The reader must
// send a close frame with the matching code.
func TestReadMessageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       []byte
		wantErr  error
		wantCode uint16
	}{
		{"masked server frame", mustHex(t, "81 85 37fa213d 7f9f4d5158"), ErrProtocol, 1002},
		{"RSV1 set", mustHex(t, "c1 02 6869"), ErrProtocol, 1002},
		{"RSV2 set", mustHex(t, "a1 02 6869"), ErrProtocol, 1002},
		{"RSV3 set", mustHex(t, "91 02 6869"), ErrProtocol, 1002},
		{"reserved opcode", mustHex(t, "83 00"), ErrProtocol, 1002},
		{"reserved control opcode", mustHex(t, "8f 00"), ErrProtocol, 1002},
		{"oversized ping", cat(mustHex(t, "89 7e 007e"), bytes.Repeat([]byte{1}, 126)), ErrProtocol, 1002},
		{"fragmented ping", mustHex(t, "09 00"), ErrProtocol, 1002},
		{"length with the top bit", mustHex(t, "82 7f 8000000000000000"), ErrProtocol, 1002},
		{"continuation without a start", frame(true, OpContinuation, "x"), ErrProtocol, 1002},
		{"new text frame in an open message", cat(frame(false, OpText, "a"), frame(true, OpText, "b")), ErrProtocol, 1002},
		{"new binary frame in an open message", cat(frame(false, OpText, "a"), frame(true, OpBinary, "b")), ErrProtocol, 1002},
		{"invalid UTF-8", frame(true, OpText, "\xff\xfe"), ErrInvalidPayload, 1007},
		{"truncated UTF-8 at the end", frame(true, OpText, "\xe2\x82"), ErrInvalidPayload, 1007},
		{"invalid UTF-8 over fragments", cat(frame(false, OpText, "\xe2"), frame(true, OpContinuation, "x")), ErrInvalidPayload, 1007},
		{"close with one byte", frame(true, OpClose, "\x03"), ErrProtocol, 1002},
		{"close with code 1005", frame(true, OpClose, closePayload(1005, "")), ErrProtocol, 1002},
		{"close with code 1006", frame(true, OpClose, closePayload(1006, "")), ErrProtocol, 1002},
		{"close with code 1004", frame(true, OpClose, closePayload(1004, "")), ErrProtocol, 1002},
		{"close with code 1015", frame(true, OpClose, closePayload(1015, "")), ErrProtocol, 1002},
		{"close with code 999", frame(true, OpClose, closePayload(999, "")), ErrProtocol, 1002},
		{"close with code 2000", frame(true, OpClose, closePayload(2000, "")), ErrProtocol, 1002},
		{"close with code 5000", frame(true, OpClose, closePayload(5000, "")), ErrProtocol, 1002},
		{"close with a reason that is not UTF-8", frame(true, OpClose, closePayload(1000, "\xff")), ErrInvalidPayload, 1007},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConn(t, tt.in)
			var err error
			for err == nil {
				_, _, err = c.ReadMessage()
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got the error %v, want %v", err, tt.wantErr)
			}
			frames := parseClientFrames(t, f.written())
			if len(frames) != 1 || frames[0].op != OpClose || len(frames[0].payload) < 2 {
				t.Fatalf("want one close frame, got %+v", frames)
			}
			p := frames[0].payload
			if code := uint16(p[0])<<8 | uint16(p[1]); code != tt.wantCode {
				t.Fatalf("got the close code %d, want %d", code, tt.wantCode)
			}
			if !f.closed {
				t.Error("the connection stays open after the failure")
			}
			// The error is sticky.
			if _, _, err2 := c.ReadMessage(); err2 != err {
				t.Errorf("a second read gives %v, want %v", err2, err)
			}
		})
	}
}

// TestReadMessageTruncated checks the streams that end too soon.
func TestReadMessageTruncated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"nothing", nil, io.EOF},
		{"cut payload", mustHex(t, "81 05 6869"), io.ErrUnexpectedEOF},
		{"cut header", mustHex(t, "81"), io.ErrUnexpectedEOF},
		{"cut control payload", mustHex(t, "89 05 68"), io.ErrUnexpectedEOF},
		{"between fragments", frame(false, OpText, "a"), io.EOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConn(t, tt.in)
			_, _, err := c.ReadMessage()
			if tt.name == "between fragments" {
				// The stream ends at a frame boundary in the middle of a
				// message.
				if err != io.EOF {
					t.Fatalf("got %v, want io.EOF", err)
				}
				return
			}
			if err != tt.want {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if len(f.written()) != 0 {
				t.Errorf("the client wrote after a network error: %x", f.written())
			}
		})
	}
}

// TestMessageSizeLimit checks that the reader rejects a large message with the
// code 1009 before it reads or allocates the payload.
func TestMessageSizeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		limit uint64
		in    []byte
		ok    bool
	}{
		{"at the limit", 5, frame(true, OpText, "Hello"), true},
		{"over the limit", 4, frame(true, OpText, "Hello"), false},
		{"fragments over the limit", 5, cat(frame(false, OpText, "abc"), frame(true, OpContinuation, "def")), false},
		{"header only, over the default limit", MaxMessageSize, mustHex(t, "82 7f 0000000010000001"), false},
		{"header only, huge length", MaxMessageSize, mustHex(t, "82 7f 7fffffffffffffff"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConn(t, tt.in)
			c.maxMessage = tt.limit
			_, _, err := c.ReadMessage()
			if tt.ok {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, ErrMessageTooBig) {
				t.Fatalf("got %v, want ErrMessageTooBig", err)
			}
			frames := parseClientFrames(t, f.written())
			if len(frames) != 1 || frames[0].op != OpClose || frames[0].payload[1] != 0xf1 {
				t.Fatalf("want a close frame with the code 1009, got %+v", frames)
			}
			// The reader did not allocate for the payload.
			if cap(c.msg) > 1<<10 {
				t.Errorf("the buffer has %d bytes before any payload came", cap(c.msg))
			}
		})
	}
}

// TestLargeMessage reads a message of several steps.
func TestLargeMessage(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("0123456789abcdef", 3<<16) // 3 MiB
	c, _ := newTestConn(t, frame(true, OpText, payload))
	_, data, err := c.ReadMessage()
	if err != nil || string(data) != payload {
		t.Fatalf("got %d bytes and %v, want %d bytes", len(data), err, len(payload))
	}
}

// TestCloseFrame checks the answer to a close frame, and the error of the
// reader.
func TestCloseFrame(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		payload  string
		wantEOF  bool
		wantErr  *CloseError
		wantEcho string
	}{
		{"no code", "", true, nil, ""},
		{"normal", closePayload(1000, ""), true, nil, closePayload(1000, "")},
		{"normal with a reason", closePayload(1000, "bye"), true, nil, closePayload(1000, "")},
		{"going away", closePayload(1001, ""), true, nil, closePayload(1001, "")},
		{"unsupported data", closePayload(1003, "no"), false, &CloseError{1003, "no"}, closePayload(1003, "")},
		{"policy", closePayload(1008, "policy"), false, &CloseError{1008, "policy"}, closePayload(1008, "")},
		{"internal error", closePayload(1011, ""), false, &CloseError{1011, ""}, closePayload(1011, "")},
		{"application code", closePayload(4000, "app"), false, &CloseError{4000, "app"}, closePayload(4000, "")},
		{"library code", closePayload(3000, ""), false, &CloseError{3000, ""}, closePayload(3000, "")},
		{"service restart", closePayload(1012, ""), false, &CloseError{1012, ""}, closePayload(1012, "")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConn(t, cat(frame(true, OpClose, tt.payload), frame(true, OpText, "late")))
			_, _, err := c.ReadMessage()
			if tt.wantEOF {
				if err != io.EOF {
					t.Fatalf("got %v, want io.EOF", err)
				}
			} else {
				var ce *CloseError
				if !errors.As(err, &ce) || *ce != *tt.wantErr {
					t.Fatalf("got %v, want %+v", err, tt.wantErr)
				}
			}
			frames := parseClientFrames(t, f.written())
			if len(frames) != 1 || frames[0].op != OpClose || string(frames[0].payload) != tt.wantEcho {
				t.Fatalf("want one close frame with %q, got %+v", tt.wantEcho, frames)
			}
			// A later read gives the same result, and the client sends no
			// second close frame.
			if _, _, err2 := c.ReadMessage(); err2 != err {
				t.Errorf("a second read gives %v, want %v", err2, err)
			}
			c.Close()
			if got := parseClientFrames(t, f.written()); len(got) != 1 {
				t.Errorf("the client wrote %d frames, want one", len(got))
			}
		})
	}
}

// TestWriteText checks the frame of a message, with the sizes where the
// length encoding changes.
func TestWriteText(t *testing.T) {
	t.Parallel()

	for _, n := range []int{0, 1, 5, 125, 126, 127, 65535, 65536, 100000} {
		c, f := newTestConn(t, nil)
		payload := bytes.Repeat([]byte("héllo"), n/6+1)[:n]
		orig := bytes.Clone(payload)
		if err := c.WriteText(payload); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(payload, orig) {
			t.Fatalf("size %d: WriteText changed the bytes of the caller", n)
		}
		if f.writes != 1 {
			t.Fatalf("size %d: %d Write calls, want one", n, f.writes)
		}
		frames := parseClientFrames(t, f.written())
		if len(frames) != 1 || frames[0].op != OpText || !frames[0].fin || !bytes.Equal(frames[0].payload, orig) {
			t.Fatalf("size %d: got %d frames", n, len(frames))
		}
	}
}

// TestMaskKeysDiffer checks that each frame has its own mask key.
func TestMaskKeysDiffer(t *testing.T) {
	t.Parallel()

	c, f := newTestConn(t, nil)
	const n = 64
	for range n {
		if err := c.WriteText([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	out := f.written()
	const size = 2 + 4 + 1
	keys := map[string]bool{}
	for i := range n {
		keys[string(out[i*size+2:i*size+6])] = true
	}
	if len(keys) < n-2 {
		t.Fatalf("only %d different mask keys in %d frames", len(keys), n)
	}
}

// TestCloseTwice checks that Close sends one close frame, and that a second
// call and a read after Close are safe.
func TestCloseTwice(t *testing.T) {
	t.Parallel()

	c, f := newTestConn(t, nil)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	frames := parseClientFrames(t, f.written())
	if len(frames) != 1 || frames[0].op != OpClose || string(frames[0].payload) != closePayload(1000, "") {
		t.Fatalf("want one close frame with the code 1000, got %+v", frames)
	}
	if _, _, err := c.ReadMessage(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("read after Close: got %v, want net.ErrClosed", err)
	}
	if err := c.WriteText([]byte("x")); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("write after Close: got %v, want net.ErrClosed", err)
	}
}

// TestCloseWhileReading checks that Close ends a blocked read, with an error
// that wraps net.ErrClosed. It uses net.Pipe, which supports deadlines.
func TestCloseWhileReading(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer server.Close()
	// Nobody reads the server end, so the close frame times out.
	c, err := newConn(client, client)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := c.ReadMessage()
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	for range 3 {
		go c.Close()
	}
	c.Close()
	select {
	case err := <-done:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("got %v, want net.ErrClosed", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not end the read")
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("Close took %v", d)
	}
}

// TestConcurrentWrites runs writers and a ping answer at the same time. The
// race detector must find nothing, and each frame must be whole.
func TestConcurrentWrites(t *testing.T) {
	t.Parallel()

	var in bytes.Buffer
	for range 100 {
		in.Write(frame(true, OpPing, "ping"))
	}
	in.Write(frame(true, OpText, "end"))
	c, f := newTestConn(t, in.Bytes())
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 50 {
				if err := c.WriteText([]byte("message")); err != nil {
					t.Error(err)
				}
			}
		})
	}
	if _, _, err := c.ReadMessage(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	var text, pong int
	for _, fr := range parseClientFrames(t, f.written()) {
		switch fr.op {
		case OpText:
			text++
		case OpPong:
			pong++
		}
	}
	if text != 200 || pong != 100 {
		t.Fatalf("got %d text and %d pong frames, want 200 and 100", text, pong)
	}
}

// TestCloseAfterServerClose checks that the client closes the network
// connection when it echoes the close frame, and that Close then returns nil
// every time.
func TestCloseAfterServerClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"normal close", frame(true, OpClose, closePayload(1000, "")), io.EOF},
		{"close without a code", frame(true, OpClose, ""), io.EOF},
		{"close and then the stream ends", cat(frame(true, OpClose, closePayload(1001, "")), []byte{0x81}), io.EOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConn(t, tt.in)
			if _, _, err := c.ReadMessage(); err != tt.want {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			f.mu.Lock()
			closed := f.closed
			f.mu.Unlock()
			if !closed {
				t.Fatal("the connection stays open after the echo of the close frame")
			}
			for i := range 2 {
				if err := c.Close(); err != nil {
					t.Fatalf("Close %d: %v", i+1, err)
				}
			}
			if got := parseClientFrames(t, f.written()); len(got) != 1 || got[0].op != OpClose {
				t.Fatalf("want one close frame, got %+v", got)
			}
			if _, _, err := c.ReadMessage(); err != tt.want {
				t.Fatalf("a read after the close gives %v, want %v", err, tt.want)
			}
		})
	}
}

// TestPingInFragmentedMessageAtLimit checks that a control frame between two
// fragments does not count for the size of the message.
func TestPingInFragmentedMessageAtLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		second  string
		wantErr error
	}{
		{"at the limit", "fghij", nil},
		{"over the limit", "fghijk", ErrMessageTooBig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConn(t, cat(
				frame(false, OpText, "abcde"),
				frame(true, OpPing, "a ping of 12 b"),
				frame(true, OpContinuation, tt.second),
			))
			c.maxMessage = 10
			_, data, err := c.ReadMessage()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || string(data) != "abcdefghij" {
				t.Fatalf("got %q and %v", data, err)
			}
			if frames := parseClientFrames(t, f.written()); len(frames) != 1 || frames[0].op != OpPong {
				t.Fatalf("want one pong, got %+v", frames)
			}
		})
	}
}

// TestFrameFlood checks the limit on the frames of one message.
func TestFrameFlood(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
	}{
		{"empty fragments", cat(frame(false, OpText, ""), bytes.Repeat(frame(false, OpContinuation, ""), 200))},
		{"pings", bytes.Repeat(frame(true, OpPing, ""), 200)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, f := newTestConn(t, tt.in)
			c.maxFrames = 100
			_, _, err := c.ReadMessage()
			if !errors.Is(err, ErrMessageTooBig) {
				t.Fatalf("got %v, want ErrMessageTooBig", err)
			}
			frames := parseClientFrames(t, f.written())
			last := frames[len(frames)-1]
			if last.op != OpClose || last.payload[1] != 0xf1 {
				t.Fatalf("want a close frame with the code 1009, got %+v", last)
			}
		})
	}
}

// TestPingAfterCloseStarted checks that a ping after Close gives no pong and
// no write error.
func TestPingAfterCloseStarted(t *testing.T) {
	t.Parallel()

	c, f := newTestConn(t, cat(frame(true, OpPing, "late"), frame(true, OpText, "x")))
	c.Close()
	// The frames of the test stream are still there, and a read of a closed
	// connection must report the close and not a pong error.
	if _, _, err := c.ReadMessage(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("got %v, want net.ErrClosed", err)
	}
	if got := parseClientFrames(t, f.written()); len(got) != 1 || got[0].op != OpClose {
		t.Fatalf("want only the close frame, got %+v", got)
	}

	// A ping that the reader handles while Close waits is dropped.
	c, f = newTestConn(t, cat(frame(true, OpPing, "late"), frame(true, OpText, "x")))
	c.closing.Store(true)
	c.writeClose(nil)
	if _, data, err := c.ReadMessage(); err != nil || string(data) != "x" {
		t.Fatalf("got %q and %v", data, err)
	}
	if got := parseClientFrames(t, f.written()); len(got) != 1 {
		t.Fatalf("want only the close frame, got %+v", got)
	}
}
