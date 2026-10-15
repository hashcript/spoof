package format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// jpegCommentTag starts the payload of spoof's COM segment, so altered
// files can be recognised (T-04) and re-runs replace the old segment.
const jpegCommentTag = "spoof:"

const (
	markerSOI = 0xD8
	markerEOI = 0xD9
	markerSOS = 0xDA
	markerCOM = 0xFE
	markerTEM = 0x01
)

// JPEG places the nonce in a COM segment immediately before EOI.
type JPEG struct{}

func (JPEG) Name() string { return "jpeg" }

func (JPEG) Detect(data []byte) bool { return bytes.HasPrefix(data, []byte{0xFF, markerSOI, 0xFF}) }

func (JPEG) Parse(data []byte) (Slot, error) {
	if len(data) < 2 || data[0] != 0xFF || data[1] != markerSOI {
		return nil, errors.New("jpeg: missing SOI marker")
	}
	head := make([]byte, 0, len(data)+32)
	head = append(head, data[:2]...)
	pos := 2
	for {
		// Find the next marker, skipping fill bytes (FF FF ...).
		if pos >= len(data) || data[pos] != 0xFF {
			return nil, fmt.Errorf("jpeg: expected marker at offset %d", pos)
		}
		start := pos
		for pos < len(data) && data[pos] == 0xFF {
			pos++
		}
		if pos >= len(data) {
			return nil, errors.New("jpeg: truncated before EOI")
		}
		m := data[pos]
		pos++
		switch {
		case m == markerEOI:
			if pos != len(data) {
				return nil, fmt.Errorf("jpeg: %d bytes of trailing data after EOI", len(data)-pos)
			}
			return newJPEGSlot(head), nil
		case m == 0x00 || m == markerSOI:
			return nil, fmt.Errorf("jpeg: unexpected marker FF%02X at offset %d", m, start)
		case m == markerTEM || 0xD0 <= m && m <= 0xD7:
			// Standalone markers carry no length.
			head = append(head, data[start:pos]...)
			continue
		}
		if len(data)-pos < 2 {
			return nil, fmt.Errorf("jpeg: truncated segment FF%02X at offset %d", m, start)
		}
		n := int(binary.BigEndian.Uint16(data[pos:]))
		if n < 2 || n > len(data)-pos {
			return nil, fmt.Errorf("jpeg: segment FF%02X at offset %d has bad length %d", m, start, n)
		}
		payload := data[pos+2 : pos+n]
		pos += n
		if m == markerCOM && isSpoofComment(payload) {
			// A spoof segment from an earlier run: drop it so blocks do not stack.
			continue
		}
		if m == markerSOS {
			end, err := scanEntropy(data, pos)
			if err != nil {
				return nil, err
			}
			pos = end
		}
		head = append(head, data[start:pos]...)
	}
}

// scanEntropy returns the offset of the first marker after the
// entropy-coded data that starts at pos. Stuffed bytes (FF 00), restart
// markers (FF D0-D7) and fill bytes belong to the scan.
func scanEntropy(data []byte, pos int) (int, error) {
	for {
		i := bytes.IndexByte(data[pos:], 0xFF)
		if i < 0 {
			return 0, errors.New("jpeg: entropy-coded data runs to end of file without EOI")
		}
		pos += i
		j := pos + 1
		for j < len(data) && data[j] == 0xFF {
			j++
		}
		if j >= len(data) {
			return 0, errors.New("jpeg: truncated in entropy-coded data")
		}
		if m := data[j]; m == 0x00 || 0xD0 <= m && m <= 0xD7 {
			pos = j + 1
			continue
		}
		return pos, nil
	}
}

func isSpoofComment(payload []byte) bool {
	rest, ok := bytes.CutPrefix(payload, []byte(jpegCommentTag))
	return ok && isNonce(rest)
}

type jpegSlot struct{ head []byte }

func newJPEGSlot(body []byte) *jpegSlot {
	head := append(body, 0xFF, markerCOM)
	head = binary.BigEndian.AppendUint16(head, uint16(2+len(jpegCommentTag)+NonceLen))
	return &jpegSlot{head: append(head, jpegCommentTag...)}
}

func (s *jpegSlot) Head() []byte { return s.head }

func (s *jpegSlot) AppendTail(dst []byte, nonce uint64) []byte {
	return append(appendNonce(dst, nonce), 0xFF, markerEOI)
}
