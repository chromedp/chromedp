package chromedp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto"
	jsonv2 "github.com/chromedp/cdproto/cdp/jsonv2"
	"github.com/chromedp/cdproto/target"
)

func TestUnmarshalWithDefaultOptions(t *testing.T) {
	tests := []struct {
		Name      string
		Input     []byte
		WantError bool
	}{
		{Name: "simple json", Input: []byte(`{"id":1, "result": {"bar":"foo"}}`), WantError: false},
		{Name: "invalid utf-8 1", Input: []byte(`{"id":2, "result": "\udebe\\u018e8"}`), WantError: false},
		{Name: "invalid utf-8 2", Input: []byte(`{"id":2, "result": "y7vzPw=T6\u001a\u053a{\ud861=\u001b"}`), WantError: false},
		{Name: "empty json", Input: []byte(`{}`), WantError: false},
		{Name: "null result", Input: []byte(`{"id":4, "result": null}`), WantError: false},
		{Name: "nested json", Input: []byte(`{"id":5, "result": {"nested": {"foo": "bar"}}}`), WantError: false},
		{Name: "array in result", Input: []byte(`{"id":6, "result": ["foo", "bar"]}`), WantError: false},
		{Name: "boolean in result", Input: []byte(`{"id":7, "result": true}`), WantError: false},
		{Name: "number in result", Input: []byte(`{"id":8, "result": 12345}`), WantError: false},
		{Name: "string in result", Input: []byte(`{"id":9, "result": "foobar"}`), WantError: false},
		{Name: "extra fields", Input: []byte(`{"id":10, "result": {"bar":"foo"}, "extra": "field"}`), WantError: false},
		{Name: "invalid json", Input: []byte(`{"id":11, "result": {"bar":"foo"`), WantError: true},
		{Name: "empty input", Input: []byte(``), WantError: true},
		{Name: "whitespace input", Input: []byte(`   `), WantError: true},
		{Name: "null input", Input: nil, WantError: true},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var decoder jsonv2.Decoder
			var b bytes.Buffer
			b.Write(test.Input)
			var msg cdproto.Message
			decoder.Reset(&b, DefaultUnmarshalOptions)
			err := jsonv2.UnmarshalDecode(&decoder, &msg, DefaultUnmarshalOptions)
			if test.WantError && err == nil {
				t.Error("expected error in unmarshal decode, but got none")
			} else if !test.WantError && err != nil {
				t.Errorf("expected no error in unmarshal decode, but got %s", err)
			}
			err = jsonv2.Unmarshal(test.Input, &msg, DefaultUnmarshalOptions)
			if test.WantError && err == nil {
				t.Error("expected error, but got none")
			} else if !test.WantError && err != nil {
				t.Errorf("expected no error, but got %s", err)
			}
		})
	}
}

func TestMarshalWithDefaultOptions(t *testing.T) {
	tests := []struct {
		Name      string
		Input     cdproto.Message
		WantError bool
	}{
		{Name: "simple json", Input: cdproto.Message{Result: []byte(`{"bar":"foo"}`)}, WantError: false},
		{Name: "invalid utf-8 1", Input: cdproto.Message{Result: []byte(`"\udebe\\u018e8"`)}, WantError: false},
		{Name: "invalid utf-8 2", Input: cdproto.Message{Result: []byte(`"y7vzPw=T6\u001a\u053a{\ud861=\u001b"`)}, WantError: false},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var encoder jsonv2.Encoder
			var b bytes.Buffer
			encoder.Reset(&b, DefaultMarshalOptions)
			err := jsonv2.MarshalEncode(&encoder, &test.Input, DefaultMarshalOptions)
			if test.WantError && err == nil {
				t.Error("expected error in marshal encode, but got none")
			} else if !test.WantError && err != nil {
				t.Errorf("expected no error in marshal encode, but got %s", err)
			}
			_, err = jsonv2.Marshal(test.Input, DefaultMarshalOptions)
			if test.WantError && err == nil {
				t.Error("expected error, but got none")
			} else if !test.WantError && err != nil {
				t.Errorf("expected no error, but got %s", err)
			}
		})
	}
}

// scriptedTransport is a [Transport] that plays the browser. Read returns the
// messages that the test puts in reads. Write calls reply, which can put
// messages in reads.
type scriptedTransport struct {
	reads chan *cdproto.Message
	reply func(cmd *cdproto.Message)
	// idle gets a value when Read finds no message and starts to wait.
	idle chan struct{}

	closeOnce sync.Once
	closed    chan struct{}
}

func newScriptedTransport() *scriptedTransport {
	return &scriptedTransport{
		reads:  make(chan *cdproto.Message, 16),
		reply:  func(*cdproto.Message) {},
		idle:   make(chan struct{}, 1),
		closed: make(chan struct{}),
	}
}

func (s *scriptedTransport) Read(ctx context.Context, msg *cdproto.Message) error {
	select {
	case m := <-s.reads:
		*msg = *m
		return nil
	default:
	}
	select {
	case s.idle <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.closed:
		return io.EOF
	case m := <-s.reads:
		*msg = *m
		return nil
	}
}

func (s *scriptedTransport) Write(ctx context.Context, msg *cdproto.Message) error {
	s.reply(msg)
	return nil
}

func (s *scriptedTransport) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

// detachMessage returns the event of the browser that detaches the session.
func detachMessage(sessionID target.SessionID) *cdproto.Message {
	return &cdproto.Message{
		Method: cdproto.EventTargetDetachedFromTarget,
		Params: fmt.Appendf(nil, `{"sessionId":%q}`, sessionID),
	}
}

// TestDetachAfterSessionMessages makes the browser read a message of a session
// and then the detach of that session. The browser must route the message to
// the target before the detach. Before, it took the two from separate queues in
// a random order, so it sometimes dropped the message.
func TestDetachAfterSessionMessages(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tr := newScriptedTransport()
	cmds := make(chan *cdproto.Message, 1)
	holding := make(chan struct{})
	release := make(chan struct{})
	tr.reply = func(cmd *cdproto.Message) {
		if cmd.Method != "Test.hold" {
			cmds <- cmd
			return
		}
		// Keep the routing loop busy, so that the messages that the test
		// sends wait in the queues.
		holding <- struct{}{}
		<-release
		tr.reads <- &cdproto.Message{ID: cmd.ID}
	}
	b, err := NewBrowserTransport(ctx, tr)
	if err != nil {
		t.Fatal(err)
	}

	// send sends the messages while the routing loop is busy. Then it waits
	// until the browser reads them all, and lets the loop go on.
	send := func(msgs ...*cdproto.Message) {
		held := make(chan error, 1)
		go func() { held <- b.Call(ctx, "Test.hold", nil, nil) }()
		<-holding
		select {
		case <-tr.idle:
		default:
		}
		for _, m := range msgs {
			tr.reads <- m
		}
		// Without the fix, the reader puts the last message in its own queue
		// and waits for the next one. With the fix, it waits for room in the
		// queue, so the wait ends with the timer.
		select {
		case <-tr.idle:
		case <-time.After(10 * time.Millisecond):
		}
		release <- struct{}{}
		if err := <-held; err != nil {
			t.Fatal(err)
		}
	}

	// Each run loses the race about half of the time without the fix.
	const runs = 40
	var lostReplies, lostEvents int
	for i := range runs {
		tgt, err := b.newTarget(ctx, target.ID(fmt.Sprint("target", i)), target.SessionID(fmt.Sprint("session", i)))
		if err != nil {
			t.Fatal(err)
		}
		go tgt.run(ctx)
		var r recorder
		tgt.listen(ctx, eventsOnly(r.add))

		if i%2 == 0 {
			// A reply, then the detach. Call must return the reply.
			type result struct {
				value int
				err   error
			}
			done := make(chan result, 1)
			go func() {
				var res struct {
					Value int `json:"value"`
				}
				err := tgt.Call(ctx, "Test.command", nil, &res)
				done <- result{res.Value, err}
			}()
			cmd := <-cmds
			send(&cdproto.Message{ID: cmd.ID, SessionID: cmd.SessionID, Result: []byte(`{"value":7}`)},
				detachMessage(cmd.SessionID))
			var res result
			select {
			case res = <-done:
			case <-time.After(10 * time.Second):
				t.Fatalf("run %d: the command did not return", i)
			}
			switch {
			case errors.Is(res.err, ErrTargetDetached):
				lostReplies++
			case res.err != nil:
				t.Fatalf("run %d: %v", i, res.err)
			case res.value != 7:
				t.Fatalf("run %d: got the value %d, want 7", i, res.value)
			}
			continue
		}

		// An event, then the detach. The listener must see the event before
		// the target is detached.
		send(&cdproto.Message{
			SessionID: tgt.SessionID,
			Method:    cdproto.EventNetworkDataReceived,
			Params:    []byte(`{"requestId":"r","timestamp":1,"dataLength":1,"encodedDataLength":1}`),
		}, detachMessage(tgt.SessionID))
		select {
		case <-tgt.detached:
		case <-time.After(10 * time.Second):
			t.Fatalf("run %d: the target is not detached", i)
		}
		if len(r.snapshot()) != 1 {
			lostEvents++
		}
	}
	if lostReplies > 0 || lostEvents > 0 {
		t.Fatalf("the detach overtook %d of %d replies and %d of %d events", lostReplies, runs/2, lostEvents, runs/2)
	}
}
