// Package verify checks an assembled output before it is written (R-15):
// its digest must have the target prefix and it must decode to exactly the
// same pixels as the source (R-04).
package verify

import (
	"bytes"
	"crypto/sha512"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/hashcript/spoof/internal/prefix"
)

// MaxPixels caps width × height before decoding, to refuse decompression
// bombs (T-07).
const MaxPixels = 1 << 27

// CheckDimensions returns an error if the image declares too many pixels.
func CheckDimensions(formatName string, data []byte) error {
	cfg, err := decodeConfig(formatName, data)
	if err != nil {
		return fmt.Errorf("reading %s header: %w", formatName, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return fmt.Errorf("image dimensions %d×%d exceed the %d pixel limit", cfg.Width, cfg.Height, MaxPixels)
	}
	return nil
}

// Output checks output against source for the given format and prefix,
// and that its SHA-512 equals wantDigest.
func Output(formatName string, source, output []byte, p prefix.Prefix, wantDigest [sha512.Size]byte) error {
	d := sha512.Sum512(output)
	if d != wantDigest {
		return errors.New("output digest differs from the one found by the search")
	}
	if !p.Matches(d[:]) {
		return fmt.Errorf("output digest does not start with %s", p)
	}
	if err := CheckDimensions(formatName, source); err != nil {
		return err
	}
	a, err := decode(formatName, source)
	if err != nil {
		return fmt.Errorf("decoding source: %w", err)
	}
	b, err := decode(formatName, output)
	if err != nil {
		return fmt.Errorf("decoding output: %w", err)
	}
	if !SamePixels(a, b) {
		return errors.New("output pixels differ from the source")
	}
	return nil
}

func decodeConfig(formatName string, data []byte) (image.Config, error) {
	switch formatName {
	case "png":
		return png.DecodeConfig(bytes.NewReader(data))
	case "jpeg":
		return jpeg.DecodeConfig(bytes.NewReader(data))
	}
	return image.Config{}, fmt.Errorf("no decoder for format %q", formatName)
}

func decode(formatName string, data []byte) (image.Image, error) {
	switch formatName {
	case "png":
		return png.Decode(bytes.NewReader(data))
	case "jpeg":
		return jpeg.Decode(bytes.NewReader(data))
	}
	return nil, fmt.Errorf("no decoder for format %q", formatName)
}

// SamePixels reports whether a and b have the same type, bounds and samples.
func SamePixels(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	switch a := a.(type) {
	case *image.RGBA:
		b, ok := b.(*image.RGBA)
		return ok && a.Stride == b.Stride && bytes.Equal(a.Pix, b.Pix)
	case *image.NRGBA:
		b, ok := b.(*image.NRGBA)
		return ok && a.Stride == b.Stride && bytes.Equal(a.Pix, b.Pix)
	case *image.RGBA64:
		b, ok := b.(*image.RGBA64)
		return ok && a.Stride == b.Stride && bytes.Equal(a.Pix, b.Pix)
	case *image.NRGBA64:
		b, ok := b.(*image.NRGBA64)
		return ok && a.Stride == b.Stride && bytes.Equal(a.Pix, b.Pix)
	case *image.Gray:
		b, ok := b.(*image.Gray)
		return ok && a.Stride == b.Stride && bytes.Equal(a.Pix, b.Pix)
	case *image.Gray16:
		b, ok := b.(*image.Gray16)
		return ok && a.Stride == b.Stride && bytes.Equal(a.Pix, b.Pix)
	case *image.CMYK:
		b, ok := b.(*image.CMYK)
		return ok && a.Stride == b.Stride && bytes.Equal(a.Pix, b.Pix)
	case *image.YCbCr:
		b, ok := b.(*image.YCbCr)
		return ok && a.SubsampleRatio == b.SubsampleRatio &&
			a.YStride == b.YStride && a.CStride == b.CStride &&
			bytes.Equal(a.Y, b.Y) && bytes.Equal(a.Cb, b.Cb) && bytes.Equal(a.Cr, b.Cr)
	case *image.Paletted:
		b, ok := b.(*image.Paletted)
		if !ok || a.Stride != b.Stride || !bytes.Equal(a.Pix, b.Pix) || len(a.Palette) != len(b.Palette) {
			return false
		}
		for i := range a.Palette {
			if a.Palette[i] != b.Palette[i] {
				return false
			}
		}
		return true
	}
	// Fallback for any other image type: compare every pixel.
	if fmt.Sprintf("%T", a) != fmt.Sprintf("%T", b) {
		return false
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}
