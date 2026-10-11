package websocket

import (
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// The close status codes that RFC 6455 section 7.4.1 defines, and the code 1012
// to 1014 that the IANA registry adds.
const (
	CloseNormal           uint16 = 1000
	CloseGoingAway        uint16 = 1001
	CloseProtocolError    uint16 = 1002
	CloseUnsupportedData  uint16 = 1003
	CloseInvalidPayload   uint16 = 1007
	ClosePolicyViolation  uint16 = 1008
	CloseMessageTooBig    uint16 = 1009
	CloseMandatoryExt     uint16 = 1010
	CloseInternalError    uint16 = 1011
	closeServiceRestart   uint16 = 1012
	closeTryAgainLater    uint16 = 1013
	closeBadGateway       uint16 = 1014
	closeMinApplication   uint16 = 3000
	closeMaxApplication   uint16 = 4999
	closeNoStatusReceived uint16 = 1005
)

// The errors of a connection that breaks the protocol. The reader sends a close
// frame with a matching status code before it returns one of them. Each error
// that the reader returns wraps one of these values.
var (
	// ErrProtocol means that the server broke a rule of RFC 6455. The close
	// code is 1002.
	ErrProtocol = errors.New("websocket protocol error")

	// ErrInvalidPayload means that a text message or a close reason is not
	// valid UTF-8. The close code is 1007.
	ErrInvalidPayload = errors.New("websocket invalid payload")

	// ErrMessageTooBig means that a message is larger than the limit of the
	// connection. The close code is 1009.
	ErrMessageTooBig = errors.New("websocket message too big")
)

// CloseError is the error of a connection that the server closed with a
// status code other than 1000 and 1001, as RFC 6455 section 7.1.5 describes.
// [Conn.ReadMessage] returns io.EOF for a normal close.
type CloseError struct {
	// Code is the status code of the close frame.
	Code uint16

	// Reason is the text that the close frame carries. It can be empty.
	Reason string
}

// Error returns the code and the reason.
func (e *CloseError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("websocket closed with the code %d", e.Code)
	}
	return fmt.Sprintf("websocket closed with the code %d: %s", e.Code, e.Reason)
}

// protocolError returns an error that wraps [ErrProtocol].
func protocolError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrProtocol, fmt.Sprintf(format, args...))
}

// failCode returns the close code that matches an error of the reader. It
// returns 0 for an error that needs no close frame, such as a network error.
func failCode(err error) uint16 {
	switch {
	case errors.Is(err, ErrProtocol):
		return CloseProtocolError
	case errors.Is(err, ErrInvalidPayload):
		return CloseInvalidPayload
	case errors.Is(err, ErrMessageTooBig):
		return CloseMessageTooBig
	}
	return 0
}

// validCloseCode reports whether a peer can send the code in a close frame.
// RFC 6455 section 7.4.1 defines 1000 to 1003 and 1007 to 1011. Codes 3000 to
// 4999 are for libraries and applications. The code 1012 to 1014 are in the
// IANA registry, so the check accepts them. The codes 1004 to 1006 and 1015
// must never go on the wire.
func validCloseCode(code uint16) bool {
	switch {
	case code >= CloseNormal && code <= CloseUnsupportedData:
		return true
	case code >= CloseInvalidPayload && code <= closeBadGateway:
		return true
	case code >= closeMinApplication && code <= closeMaxApplication:
		return true
	}
	return false
}

// parseClose parses the payload of a close frame. RFC 6455 section 5.5.1 says
// that the payload is empty or has a two byte status code and then a UTF-8
// reason. A payload of one byte, an invalid code and a reason that is not
// UTF-8 are errors. An empty payload gives the code 1005, which means that no
// code came.
func parseClose(p []byte) (code uint16, reason string, err error) {
	switch {
	case len(p) == 0:
		return closeNoStatusReceived, "", nil
	case len(p) == 1:
		return 0, "", protocolError("a close frame has a payload of one byte")
	}
	code = uint16(p[0])<<8 | uint16(p[1])
	if !validCloseCode(code) {
		return 0, "", protocolError("a close frame has the invalid code %d", code)
	}
	if !utf8.Valid(p[2:]) {
		return 0, "", fmt.Errorf("%w: the reason of a close frame is not UTF-8", ErrInvalidPayload)
	}
	return code, string(p[2:]), nil
}

// closeResult returns the error that the reader gives for a close frame. A
// normal close gives io.EOF, as does a close without a code.
func closeResult(code uint16, reason string) error {
	switch code {
	case CloseNormal, CloseGoingAway, closeNoStatusReceived:
		return io.EOF
	}
	return &CloseError{Code: code, Reason: reason}
}
