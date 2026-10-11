package websocket

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

// Opcode is the opcode of a frame. RFC 6455 section 5.2 defines the values.
type Opcode byte

// The opcodes that RFC 6455 section 5.2 defines. All other values are
// reserved, and a frame with a reserved opcode is a protocol error.
const (
	OpContinuation Opcode = 0x0
	OpText         Opcode = 0x1
	OpBinary       Opcode = 0x2
	OpClose        Opcode = 0x8
	OpPing         Opcode = 0x9
	OpPong         Opcode = 0xa
)

// String returns the name of the opcode.
func (o Opcode) String() string {
	switch o {
	case OpContinuation:
		return "continuation"
	case OpText:
		return "text"
	case OpBinary:
		return "binary"
	case OpClose:
		return "close"
	case OpPing:
		return "ping"
	case OpPong:
		return "pong"
	}
	return fmt.Sprintf("opcode 0x%x", byte(o))
}

// isControl reports whether the opcode is a control opcode. RFC 6455 section
// 5.5 says that the control opcodes are the ones with the highest bit set.
func (o Opcode) isControl() bool {
	return o&0x8 != 0
}

// isDefined reports whether RFC 6455 section 5.2 defines the opcode.
func (o Opcode) isDefined() bool {
	switch o {
	case OpContinuation, OpText, OpBinary, OpClose, OpPing, OpPong:
		return true
	}
	return false
}

// maxControlPayload is the largest payload of a control frame. RFC 6455
// section 5.5 sets the limit.
const maxControlPayload = 125

// maxHeaderLen is the length of the longest frame header of a client: two
// bytes, an 8 byte length and a 4 byte mask key.
const maxHeaderLen = 2 + 8 + 4

// frameHeader is a parsed frame header. It has no mask key, because a server
// must not mask a frame.
type frameHeader struct {
	fin    bool
	opcode Opcode
	length uint64
}

// readFrameHeader reads and checks the header of a frame that a server sent.
// RFC 6455 section 5.2 describes the format. It returns io.EOF when the
// stream ends before the first byte, and io.ErrUnexpectedEOF when it ends in
// the middle of the header. A frame that breaks a rule gives an error that
// wraps [ErrProtocol].
func readFrameHeader(br *bufio.Reader) (frameHeader, error) {
	var h frameHeader
	b0, err := br.ReadByte()
	if err != nil {
		return h, err
	}
	b1, err := br.ReadByte()
	if err != nil {
		return h, unexpectedEOF(err)
	}

	h.fin = b0&0x80 != 0
	// RSV1 to RSV3 must be 0 when no extension defines them. The client
	// negotiates no extension.
	if b0&0x70 != 0 {
		return h, protocolError("a frame has a reserved bit set")
	}
	h.opcode = Opcode(b0 & 0x0f)
	if !h.opcode.isDefined() {
		return h, protocolError("a frame has the reserved %s", h.opcode)
	}
	// RFC 6455 section 5.1: a server must not mask the frames that it sends.
	if b1&0x80 != 0 {
		return h, protocolError("a frame from the server is masked")
	}

	switch n := b1 & 0x7f; n {
	case 126:
		p, err := br.Peek(2)
		if err != nil {
			return h, unexpectedEOF(err)
		}
		h.length = uint64(binary.BigEndian.Uint16(p))
		br.Discard(2)
	case 127:
		p, err := br.Peek(8)
		if err != nil {
			return h, unexpectedEOF(err)
		}
		h.length = binary.BigEndian.Uint64(p)
		br.Discard(8)
		// The most significant bit of a 64 bit length must be 0.
		if h.length>>63 != 0 {
			return h, protocolError("a frame has a length with the highest bit set")
		}
	default:
		h.length = uint64(n)
	}

	// RFC 6455 section 5.5: a control frame must not be fragmented, and its
	// payload must be 125 bytes or less. A length encoding that is longer
	// than needed is legal, so the check uses the decoded value.
	if h.opcode.isControl() {
		if !h.fin {
			return h, protocolError("a %s frame is fragmented", h.opcode)
		}
		if h.length > maxControlPayload {
			return h, protocolError("a %s frame has a payload of %d bytes", h.opcode, h.length)
		}
	}
	return h, nil
}

// appendHeader appends the header of a client frame to b. The frame has the
// FIN bit set, the mask bit set and the given mask key. RFC 6455 section 5.2
// says that the shortest length encoding is the one to write.
func appendHeader(b []byte, op Opcode, n int, key [4]byte) []byte {
	b = append(b, 0x80|byte(op))
	switch {
	case n <= 125:
		b = append(b, 0x80|byte(n))
	case n <= 0xffff:
		b = append(b, 0x80|126)
		b = binary.BigEndian.AppendUint16(b, uint16(n))
	default:
		b = append(b, 0x80|127)
		b = binary.BigEndian.AppendUint64(b, uint64(n))
	}
	return append(b, key[:]...)
}

// maskBytes applies the mask key to b in place, as RFC 6455 section 5.3
// describes. Applying it twice gives the original bytes. The func works on
// eight bytes at a time.
func maskBytes(key [4]byte, b []byte) {
	k := uint64(binary.LittleEndian.Uint32(key[:]))
	k |= k << 32
	for len(b) >= 8 {
		binary.LittleEndian.PutUint64(b, binary.LittleEndian.Uint64(b)^k)
		b = b[8:]
	}
	// The loop above used a multiple of four bytes, so the key starts at
	// its first byte again.
	for i := range b {
		b[i] ^= key[i&3]
	}
}

// unexpectedEOF turns io.EOF into io.ErrUnexpectedEOF. A stream must not end
// in the middle of a frame.
func unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
