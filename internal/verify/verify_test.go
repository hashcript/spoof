package verify

import (
	"bytes"
	"crypto/sha512"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/hashcript/spoof/internal/prefix"
	"github.com/hashcript/spoof/internal/testimg"
)

func TestOutputAcceptsIdenticalPixels(t *testing.T) {
	src := testimg.PNG(t, 6, 6)
	// Bytes after IEND are ignored by the decoder, so pixels stay identical.
	out := append(append([]byte{}, src...), "x"...)
	d := sha512.Sum512(out)
	p, _ := prefix.Parse(strings.ToUpper(string(hexDigit(d[0] >> 4))))
	if err := Output("png", src, out, p, d); err != nil {
		t.Fatal(err)
	}
}

func hexDigit(n byte) byte { return "0123456789abcdef"[n] }

func TestOutputRejects(t *testing.T) {
	src := testimg.PNG(t, 6, 6)
	img := testimg.Pattern(6, 6)
	img.Pix[0] ^= 1
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	changed := buf.Bytes()
	d := sha512.Sum512(changed)
	match, _ := prefix.Parse(string(hexDigit(d[0] >> 4)))
	miss, _ := prefix.Parse(string(hexDigit((d[0] >> 4) ^ 1)))

	if err := Output("png", src, changed, match, d); err == nil || !strings.Contains(err.Error(), "pixels differ") {
		t.Errorf("changed pixels: err = %v", err)
	}
	if err := Output("png", src, changed, miss, d); err == nil {
		t.Error("wrong prefix: want error")
	}
	if err := Output("png", src, changed, match, sha512.Sum512(src)); err == nil {
		t.Error("wrong digest: want error")
	}
	if err := Output("png", src, changed[:len(changed)/2], match, sha512.Sum512(changed[:len(changed)/2])); err == nil {
		t.Error("undecodable output: want error")
	}
}

func TestCheckDimensions(t *testing.T) {
	if err := CheckDimensions("png", testimg.PNG(t, 3, 3)); err != nil {
		t.Error(err)
	}
	if err := CheckDimensions("jpeg", testimg.JPEG(t, 3, 3)); err != nil {
		t.Error(err)
	}
	// A PNG header that declares 100000×100000 pixels.
	var buf bytes.Buffer
	png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1, 1)))
	big := buf.Bytes()
	copy(big[16:24], []byte{0, 1, 0x86, 0xa0, 0, 1, 0x86, 0xa0})
	if err := CheckDimensions("png", big); err == nil {
		t.Error("want error for huge dimensions")
	}
}

func TestSamePixelsTypes(t *testing.T) {
	a := image.NewGray(image.Rect(0, 0, 2, 2))
	b := image.NewGray(image.Rect(0, 0, 2, 2))
	if !SamePixels(a, b) {
		t.Error("equal Gray images differ")
	}
	b.Pix[3] = 1
	if SamePixels(a, b) {
		t.Error("different Gray images are equal")
	}
	if SamePixels(a, image.NewGray16(a.Rect)) {
		t.Error("different types are equal")
	}
	if SamePixels(a, image.NewGray(image.Rect(0, 0, 2, 3))) {
		t.Error("different bounds are equal")
	}
	// *image.Alpha is not special-cased, so it uses the per-pixel fallback.
	x, y := image.NewAlpha(a.Rect), image.NewAlpha(a.Rect)
	if !SamePixels(x, y) {
		t.Error("fallback: equal images differ")
	}
	y.Pix[1] = 9
	if SamePixels(x, y) {
		t.Error("fallback: different images are equal")
	}
}
