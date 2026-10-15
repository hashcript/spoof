package format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// pngChunkType is spoof's private, ancillary, safe-to-copy chunk. Decoders
// must skip unknown ancillary chunks, so it never changes the pixels.
const pngChunkType = "spOf"

// pngIEND is the complete, fixed IEND chunk: length 0, type, CRC.
var pngIEND = []byte("\x00\x00\x00\x00IEND\xae\x42\x60\x82")

// PNG places the nonce in a spOf chunk immediately before IEND.
type PNG struct{}

func (PNG) Name() string { return "png" }

func (PNG) Detect(data []byte) bool { return bytes.HasPrefix(data, pngSignature) }

func (PNG) Parse(data []byte) (Slot, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, errors.New("png: missing signature")
	}
	head := make([]byte, 0, len(data)+12)
	head = append(head, pngSignature...)
	pos := len(pngSignature)
	first := true
	for {
		if len(data)-pos < 12 {
			return nil, fmt.Errorf("png: truncated chunk at offset %d", pos)
		}
		n := binary.BigEndian.Uint32(data[pos:])
		if n > 1<<31-1 || uint64(n) > uint64(len(data)-pos-12) {
			return nil, fmt.Errorf("png: chunk length %d at offset %d exceeds file size", n, pos)
		}
		end := pos + 12 + int(n)
		typ := data[pos+4 : pos+8]
		for _, c := range typ {
			if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z') {
				return nil, fmt.Errorf("png: invalid chunk type %q at offset %d", typ, pos)
			}
		}
		want := binary.BigEndian.Uint32(data[end-4:])
		if got := crc32.ChecksumIEEE(data[pos+4 : end-4]); got != want {
			return nil, fmt.Errorf("png: bad CRC in %s chunk at offset %d", typ, pos)
		}
		if first && string(typ) != "IHDR" {
			return nil, errors.New("png: first chunk is not IHDR")
		}
		first = false
		switch string(typ) {
		case "IEND":
			if end != len(data) {
				return nil, fmt.Errorf("png: %d bytes of trailing data after IEND", len(data)-end)
			}
			return newPNGSlot(head), nil
		case pngChunkType:
			// A spoof chunk from an earlier run: drop it so blocks do not stack.
		default:
			head = append(head, data[pos:end]...)
		}
		pos = end
	}
}

type pngSlot struct {
	head    []byte
	typeCRC uint32 // CRC-32 state after the chunk type
}

func newPNGSlot(body []byte) *pngSlot {
	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[:], NonceLen)
	copy(hdr[4:], pngChunkType)
	return &pngSlot{
		head:    append(body, hdr[:]...),
		typeCRC: crc32.ChecksumIEEE([]byte(pngChunkType)),
	}
}

func (s *pngSlot) Head() []byte { return s.head }

func (s *pngSlot) AppendTail(dst []byte, nonce uint64) []byte {
	start := len(dst)
	dst = appendNonce(dst, nonce)
	crc := crc32.Update(s.typeCRC, crc32.IEEETable, dst[start:])
	dst = binary.BigEndian.AppendUint32(dst, crc)
	return append(dst, pngIEND...)
}
