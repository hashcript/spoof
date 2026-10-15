// Package testimg generates small PNG and JPEG files for tests.
package testimg

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// Pattern returns a deterministic w×h image with varied pixel values.
func Pattern(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 7), uint8(y * 13), uint8(x ^ y), uint8(255 - x)})
		}
	}
	return img
}

// PNG encodes Pattern(w, h) as PNG.
func PNG(t testing.TB, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, Pattern(w, h)); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// JPEG encodes Pattern(w, h) as JPEG.
func JPEG(t testing.TB, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, Pattern(w, h), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
