package format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/hashcript/spoof/internal/testimg"
	"github.com/hashcript/spoof/internal/verify"
)

func TestDetect(t *testing.T) {
	if a, err := Detect(testimg.PNG(t, 4, 4)); err != nil || a.Name() != "png" {
		t.Errorf("PNG detected as %v, %v", a, err)
	}
	if a, err := Detect(testimg.JPEG(t, 4, 4)); err != nil || a.Name() != "jpeg" {
		t.Errorf("JPEG detected as %v, %v", a, err)
	}
	for _, data := range [][]byte{nil, []byte("GIF89a...."), []byte("\x89PNG"), {0xFF, 0xD8}} {
		if _, err := Detect(data); !errors.Is(err, ErrUnsupported) {
			t.Errorf("Detect(%q) = %v, want ErrUnsupported", data, err)
		}
	}
}

func TestAppendNonce(t *testing.T) {
	if got := string(appendNonce(nil, 0x1234abcd)); got != "000000001234abcd" {
		t.Errorf("appendNonce = %q", got)
	}
	if got := string(appendNonce([]byte("x"), ^uint64(0))); got != "xffffffffffffffff" {
		t.Errorf("appendNonce = %q", got)
	}
}

type decoder func([]byte) (image.Image, error)

func decodePNG(b []byte) (image.Image, error)  { return png.Decode(bytes.NewReader(b)) }
func decodeJPEG(b []byte) (image.Image, error) { return jpeg.Decode(bytes.NewReader(b)) }

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		adapter Adapter
		src     []byte
		decode  decoder
		endLen  int // length of the original end marker
	}{
		{"png", PNG{}, testimg.PNG(t, 37, 23), decodePNG, len(pngIEND)},
		{"jpeg", JPEG{}, testimg.JPEG(t, 37, 23), decodeJPEG, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot, err := tt.adapter.Parse(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			out := Assemble(slot, 0xdeadbeef)
			if !bytes.HasPrefix(out, tt.src[:len(tt.src)-tt.endLen]) {
				t.Error("output does not keep the original bytes before the end marker")
			}
			if !bytes.HasSuffix(out, tt.src[len(tt.src)-tt.endLen:]) {
				t.Error("output does not end with the original end marker")
			}
			if grow := len(out) - len(tt.src); grow <= 0 || grow >= 64 {
				t.Errorf("output grew by %d bytes", grow)
			}
			if !bytes.Contains(out, []byte("00000000deadbeef")) {
				t.Error("nonce not found in output")
			}
			a, err := tt.decode(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			b, err := tt.decode(out)
			if err != nil {
				t.Fatalf("decoding output: %v", err)
			}
			if !verify.SamePixels(a, b) {
				t.Error("pixels differ")
			}

			// Re-running on the output replaces the spoof block instead of stacking.
			again, err := tt.adapter.Parse(out)
			if err != nil {
				t.Fatalf("parsing output: %v", err)
			}
			if !bytes.Equal(again.Head(), slot.Head()) {
				t.Error("re-parsed output has a different head; spoof blocks stack")
			}
			if !bytes.Equal(Assemble(again, 7), Assemble(slot, 7)) {
				t.Error("re-assembled output differs")
			}
		})
	}
}

func TestPNGChunkCRC(t *testing.T) {
	slot, err := PNG{}.Parse(testimg.PNG(t, 3, 3))
	if err != nil {
		t.Fatal(err)
	}
	out := Assemble(slot, 42)
	// Parse validates every CRC, including the spOf chunk's.
	if _, err := (PNG{}).Parse(out); err != nil {
		t.Fatal(err)
	}
	chunk := out[len(out)-len(pngIEND)-12-NonceLen : len(out)-len(pngIEND)]
	if string(chunk[4:8]) != pngChunkType || binary.BigEndian.Uint32(chunk) != NonceLen {
		t.Fatalf("unexpected chunk %q", chunk)
	}
	if got, want := binary.BigEndian.Uint32(chunk[len(chunk)-4:]), crc32.ChecksumIEEE(chunk[4:len(chunk)-4]); got != want {
		t.Errorf("CRC = %08x, want %08x", got, want)
	}
}

// pngChunk builds one PNG chunk with a correct CRC.
func pngChunk(typ string, data []byte) []byte {
	c := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	c = append(c, typ...)
	c = append(c, data...)
	return binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(c[4:]))
}

func TestPNGKeepsMetadata(t *testing.T) {
	src := testimg.PNG(t, 5, 5)
	text := pngChunk("tEXt", []byte("Comment\x00hello"))
	// Insert a text chunk right after IHDR (8-byte signature + 25-byte IHDR).
	withText := append(append(append([]byte{}, src[:33]...), text...), src[33:]...)
	slot, err := PNG{}.Parse(withText)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(Assemble(slot, 1), text) {
		t.Error("tEXt chunk was not preserved")
	}
}

func TestPNGMalformed(t *testing.T) {
	src := testimg.PNG(t, 4, 4)
	badCRC := append([]byte{}, src...)
	badCRC[30] ^= 0xff // inside IHDR's CRC-covered data
	hugeLen := append([]byte{}, src...)
	binary.BigEndian.PutUint32(hugeLen[8:], 0xfffffff0)
	noIHDR := append(append([]byte{}, pngSignature...), pngIEND...)
	badType := append([]byte{}, src...)
	badType[12] = '1'
	cases := map[string][]byte{
		"truncated":     src[:len(src)-5],
		"no IEND":       src[:len(src)-len(pngIEND)],
		"trailing data": append(append([]byte{}, src...), "junk"...),
		"bad CRC":       badCRC,
		"huge length":   hugeLen,
		"no IHDR":       noIHDR,
		"bad type":      badType,
		"signature":     pngSignature,
	}
	for name, data := range cases {
		if _, err := (PNG{}).Parse(data); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestJPEGKeepsComment(t *testing.T) {
	src := testimg.JPEG(t, 8, 8)
	com := []byte{0xFF, markerCOM, 0x00, 0x07, 'h', 'e', 'l', 'l', 'o'}
	withCom := append(append(append([]byte{}, src[:2]...), com...), src[2:]...)
	slot, err := JPEG{}.Parse(withCom)
	if err != nil {
		t.Fatal(err)
	}
	out := Assemble(slot, 1)
	if !bytes.HasPrefix(out, withCom[:len(withCom)-2]) {
		t.Error("user COM segment was not preserved")
	}
}

func TestJPEGEntropyScan(t *testing.T) {
	// A synthetic stream: the parser only needs segment framing.
	data := []byte{
		0xFF, markerSOI,
		0xFF, 0xE0, 0x00, 0x04, 'a', 'b', // APP0
		0xFF, 0xFF, markerSOS, 0x00, 0x03, 0x01, // fill byte, then SOS header
		0x12, 0xFF, 0x00, 0x34, // stuffed byte
		0xFF, 0xD3, 0x56, // restart marker
		0xFF, 0xFF, 0x00, 0x78, // fill before stuffed byte
		0xFF, markerEOI,
	}
	slot, err := JPEG{}.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(slot.Head(), data[:len(data)-2]) {
		t.Errorf("head = % x", slot.Head())
	}
}

func TestJPEGMalformed(t *testing.T) {
	src := testimg.JPEG(t, 8, 8)
	badLen := append([]byte{}, src...)
	badLen[4], badLen[5] = 0xFF, 0xFF // first segment's length
	zeroLen := append([]byte{}, src...)
	zeroLen[4], zeroLen[5] = 0x00, 0x01
	cases := map[string][]byte{
		"truncated":     src[:len(src)/2],
		"no EOI":        src[:len(src)-2],
		"trailing data": append(append([]byte{}, src...), 0, 1, 2),
		"bad length":    badLen,
		"short length":  zeroLen,
		"SOI only":      {0xFF, markerSOI},
		"not a marker":  {0xFF, markerSOI, 0x00, 0x01},
		"second SOI":    {0xFF, markerSOI, 0xFF, markerSOI, 0xFF, markerEOI},
	}
	for name, data := range cases {
		if _, err := (JPEG{}).Parse(data); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func fuzzParse(f *testing.F, a Adapter, seeds ...[]byte) {
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		slot, err := a.Parse(data)
		if err != nil {
			return
		}
		out := Assemble(slot, 99)
		// The output of a successful parse must itself parse to the same slot.
		again, err := a.Parse(out)
		if err != nil {
			t.Fatalf("output does not parse: %v", err)
		}
		if !bytes.Equal(again.Head(), slot.Head()) {
			t.Fatal("re-parse changed the head")
		}
	})
}

func FuzzParsePNG(f *testing.F) {
	fuzzParse(f, PNG{}, testimg.PNG(f, 2, 2), append(append([]byte{}, pngSignature...), pngIEND...))
}

func FuzzParseJPEG(f *testing.F) {
	fuzzParse(f, JPEG{}, testimg.JPEG(f, 2, 2), []byte{0xFF, markerSOI, 0xFF, markerEOI})
}
