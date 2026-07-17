package chromedp

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"sync"

	"github.com/chromedp/cdproto"
	jsonv2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// PipeConn implements Transport for pipe-based CDP communication.
// It is used when Chrome is started with the --remote-debugging-pipe
// which expose FD 3 (stdin) and FD 4 (stdout) for bidirectional CDP messaging.
// Unlike the WebSocket transport, this mode uses an ASCIIZ encoding:
// each JSON message is followed by a null byte (0x00) as the terminator.
type PipeConn struct {
	reader  *bufio.Reader
	writer  *os.File
	readFd  *os.File
	writeFd *os.File

	writeMu sync.Mutex

	decoder jsontext.Decoder
	encoder jsontext.Encoder

	debugf func(string, ...any)
}

// Read reads a CDP message from Chrome (from FD 4).
// Messages are null-byte terminated in ASCIIZ mode.
func (p *PipeConn) Read(_ context.Context, msg *cdproto.Message) error {
	data, err := p.reader.ReadBytes(0)
	if err != nil {
		return err
	}

	// Strip the null terminator
	if len(data) > 0 {
		data = data[:len(data)-1]
	}

	if p.debugf != nil {
		p.debugf("<- %s", data)
	}

	p.decoder.Reset(bytes.NewReader(data), DefaultUnmarshalOptions)
	return jsonv2.UnmarshalDecode(&p.decoder, msg, DefaultUnmarshalOptions)
}

// Write writes a CDP message to Chrome (to FD 3).
// Messages are null-byte terminated in ASCIIZ mode.
func (p *PipeConn) Write(_ context.Context, msg *cdproto.Message) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	var b bytes.Buffer
	p.encoder.Reset(&b, DefaultMarshalOptions)
	if err := jsonv2.MarshalEncode(&p.encoder, msg, DefaultMarshalOptions); err != nil {
		return err
	}

	if p.debugf != nil {
		p.debugf("-> %s", b.Bytes())
	}

	if _, err := p.writer.Write(b.Bytes()); err != nil {
		return err
	}
	_, err := p.writer.Write([]byte{0})
	return err
}

func (p *PipeConn) Close() error {
	err1 := p.readFd.Close()
	err2 := p.writeFd.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
