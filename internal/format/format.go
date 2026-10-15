// Package format detects image formats and parses them into a nonce slot:
// the place, just before the end marker, where spoof inserts an ignorable
// block holding a counter (ADR-0002, ADR-0003).
package format

import (
	"bytes"
	"errors"
)

// ErrUnsupported is returned when the input is neither PNG nor JPEG.
var ErrUnsupported = errors.New("unsupported image format (only PNG and JPEG are supported)")

// NonceLen is the width of the encoded nonce: a uint64 as lowercase hex.
const NonceLen = 16

// Adapter knows how to place a nonce slot in one image format.
type Adapter interface {
	// Name is a short format name, e.g. "png".
	Name() string
	// Detect reports whether data starts with the format's magic bytes.
	Detect(data []byte) bool
	// Parse walks the file with bounds checks and returns its nonce slot.
	// An existing spoof block is replaced rather than stacked.
	Parse(data []byte) (Slot, error)
}

// Slot splits an output file into a fixed head and a nonce-dependent tail.
// Implementations must be safe for concurrent use.
type Slot interface {
	// Head returns every output byte before the nonce. Callers must not modify it.
	Head() []byte
	// AppendTail appends the encoded nonce and all bytes after it to dst.
	AppendTail(dst []byte, nonce uint64) []byte
}

// Assemble returns the complete output file for nonce.
func Assemble(s Slot, nonce uint64) []byte {
	head := s.Head()
	out := make([]byte, len(head), len(head)+64)
	copy(out, head)
	return s.AppendTail(out, nonce)
}

var adapters = []Adapter{PNG{}, JPEG{}}

// Detect returns the adapter for data, or ErrUnsupported.
func Detect(data []byte) (Adapter, error) {
	for _, a := range adapters {
		if a.Detect(data) {
			return a, nil
		}
	}
	return nil, ErrUnsupported
}

// appendNonce appends nonce as NonceLen lowercase hex digits. The digits
// never include 0xFF, so they cannot be mistaken for a JPEG marker.
func appendNonce(dst []byte, nonce uint64) []byte {
	const hex = "0123456789abcdef"
	var b [NonceLen]byte
	for i := NonceLen - 1; i >= 0; i-- {
		b[i] = hex[nonce&0xf]
		nonce >>= 4
	}
	return append(dst, b[:]...)
}

// isNonce reports whether b looks like a nonce written by appendNonce.
func isNonce(b []byte) bool {
	if len(b) != NonceLen {
		return false
	}
	return len(bytes.Trim(b, "0123456789abcdef")) == 0
}
