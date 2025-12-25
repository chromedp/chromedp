package chromedp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/chromedp/cdproto"
)

func TestPipeConnReadWrite(t *testing.T) {
	t.Parallel()

	// Create pipe pair to simulate Chrome connection
	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readFd.Close()
	defer writeFd.Close()

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	// Test Write
	msg := &cdproto.Message{
		ID:     1,
		Method: "Test.method",
	}

	// Write in goroutine since pipe blocks
	go func() {
		if err := conn.Write(context.Background(), msg); err != nil {
			t.Error(err)
		}
	}()

	// Read raw bytes to verify format
	var buf bytes.Buffer
	for {
		b := make([]byte, 1)
		_, err := readFd.Read(b)
		if err != nil {
			t.Fatal(err)
		}
		if b[0] == 0 {
			break // null terminator
		}
		buf.WriteByte(b[0])
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte(`"id":1`)) {
		t.Fatalf("expected id:1 in output, got %s", got)
	}
	if !bytes.Contains([]byte(got), []byte(`"method":"Test.method"`)) {
		t.Fatalf("expected method in output, got %s", got)
	}
}

func TestPipeConnRead(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readFd.Close()
	defer writeFd.Close()

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	// Simulate Chrome sending a message (null-terminated JSON)
	go func() {
		data := []byte(`{"id":42,"result":{"foo":"bar"}}`)
		writeFd.Write(data)
		writeFd.Write([]byte{0}) // null terminator
	}()

	var msg cdproto.Message
	if err := conn.Read(context.Background(), &msg); err != nil {
		t.Fatal(err)
	}

	if msg.ID != 42 {
		t.Fatalf("want id 42, got %d", msg.ID)
	}
}

func TestPipeConnClose(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = readFd.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("expected an error after closing readFd, got nil")
	}

	if !errors.Is(err, os.ErrClosed) && err != io.EOF && !strings.Contains(err.Error(), "closed") {
		t.Fatalf("expected closed error, got %v", err)
	}
}

func TestPipeConnReadEOF(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readFd.Close()

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	// Close write end to simulate Chrome disconnect
	writeFd.Close()

	var msg cdproto.Message
	err = conn.Read(context.Background(), &msg)
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestPipeConnDebugf(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readFd.Close()
	defer writeFd.Close()

	var debugOutput []string
	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
		debugf: func(format string, args ...any) {
			debugOutput = append(debugOutput, format)
		},
	}

	// Test write debug
	go func() {
		conn.Write(context.Background(), &cdproto.Message{ID: 1})
	}()

	// Drain the pipe
	buf := make([]byte, 1024)
	readFd.Read(buf)

	if len(debugOutput) != 1 || debugOutput[0] != "-> %s" {
		t.Fatalf("expected write debug output, got %v", debugOutput)
	}
}

func TestPipeConnReadMultiple(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readFd.Close()
	defer writeFd.Close()

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	go func() {
		for i := 1; i <= 3; i++ {
			data := []byte(fmt.Sprintf(`{"id":%d}`, i))
			writeFd.Write(data)
			writeFd.Write([]byte{0})
		}
	}()

	for i := 1; i <= 3; i++ {
		var msg cdproto.Message
		if err := conn.Read(context.Background(), &msg); err != nil {
			t.Fatal(err)
		}
		if msg.ID != int64(i) {
			t.Fatalf("want id %d, got %d", i, msg.ID)
		}
	}
}

func TestPipeConnConcurrentWrites(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readFd.Close()
	defer writeFd.Close()

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	// Drain pipe in background
	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := readFd.Read(buf); err != nil {
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn.Write(context.Background(), &cdproto.Message{ID: int64(id)})
		}(i)
	}
	wg.Wait()
}

func TestPipeConnReadMalformedJSON(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readFd.Close()
	defer writeFd.Close()

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	go func() {
		writeFd.Write([]byte(`{not valid json`))
		writeFd.Write([]byte{0})
	}()

	var msg cdproto.Message
	err = conn.Read(context.Background(), &msg)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestPipeConnDoubleClose(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	// Second close will error, just verify no panic
	_ = conn.Close()
}

func TestPipeConnBrokenPipe(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	// Close read end to simulate broken pipe
	readFd.Close()

	err = conn.Write(context.Background(), &cdproto.Message{ID: 1})
	if err == nil {
		t.Fatal("expected error on broken pipe")
	}
}

func TestPipeConnLargeMessage(t *testing.T) {
	t.Parallel()

	readFd, writeFd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readFd.Close()
	defer writeFd.Close()

	conn := &PipeConn{
		reader:  bufio.NewReader(readFd),
		writer:  writeFd,
		readFd:  readFd,
		writeFd: writeFd,
	}

	// 1MB payload
	largeData := bytes.Repeat([]byte("x"), 1024*1024)

	go func() {
		msg := []byte(`{"id":1,"result":"` + string(largeData) + `"}`)
		writeFd.Write(msg)
		writeFd.Write([]byte{0})
	}()

	var msg cdproto.Message
	if err := conn.Read(context.Background(), &msg); err != nil {
		t.Fatal(err)
	}

	if msg.ID != 1 {
		t.Fatalf("want id 1, got %d", msg.ID)
	}
}
