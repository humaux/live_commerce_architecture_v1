//go:build browser

package foundation_test

// Purpose: synthetic image fixtures for the PM-U real-upload browser gate — a valid JPEG carrying an EXIF orientation-6
//   APP1 segment (the "phone photo" the admin UI must rotate and downsize before upload) and a deterministic > 2 MiB
//   noisy JPEG, plus the fitPhoto mirror used to predict stored dimensions.
// Depends on: stdlib image/jpeg only; the 2 MiB original cap and the 2000 px longest-side rule of
//   contracts/catalog-inventory-v1.md "Amendment — product-media-v2" and apps/admin/lib/photo-preprocess.ts.
// Used by: tests/foundation/browser_product_media_v2_test.go (TestBrowserProductMediaV2RealUpload) only.
// Invariants: bytes <= 2 MiB per original (catalog-inventory-v1.md amendment); EXIF orientation applied before sizing.

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// pmv2Fit mirrors apps/admin/lib/product-media-model.ts fitPhoto: longest side capped, floor, never upscale.
// Kept as an independent restatement so the gate catches a client rule that drifts from the documented 2000 px.
func pmv2Fit(w, h int) (int, int) {
	const longest = 2000
	factor := float64(longest) / float64(max(w, h))
	if factor > 1 {
		factor = 1
	}
	return max(1, int(float64(w)*factor)), max(1, int(float64(h)*factor))
}

// pmv2EXIF6 wraps baseline JPEG bytes with an EXIF APP1 segment declaring orientation 6 (the camera stored the
// sensor rows rotated; the displayed image is 90° CW, so display dimensions are height x width of the buffer).
// The segment is inserted directly after SOI, which is where EXIF belongs.
func pmv2EXIF6(t *testing.T, plain []byte) []byte {
	t.Helper()
	if len(plain) < 4 || plain[0] != 0xFF || plain[1] != 0xD8 {
		t.Fatal("pmv2EXIF6 needs baseline JPEG bytes starting with SOI")
	}
	// TIFF payload, little-endian ("II"): magic 42, IFD at offset 8, one SHORT entry tag 0x0112 (Orientation) = 6.
	tiff := []byte{
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, // header
		0x01, 0x00, // one IFD entry
		0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, // Orientation, SHORT, count 1, value 6
		0x00, 0x00, 0x00, 0x00, // no next IFD
	}
	app1 := append([]byte("Exif\x00\x00"), tiff...)
	segment := new(bytes.Buffer)
	segment.Write([]byte{0xFF, 0xE1, byte((len(app1) + 2) >> 8), byte(len(app1) + 2)})
	segment.Write(app1)
	out := new(bytes.Buffer)
	out.Write(plain[:2])
	out.Write(segment.Bytes())
	out.Write(plain[2:])
	return out.Bytes()
}

// pmv2NoisyPhoto encodes a w x h deterministic photo-like JPEG: a smooth gradient with a band of xorshift noise so
// the result exceeds the 2 MiB original cap (asserted here — a fixture that no longer trips the cap must fail loudly)
// while its 2000 px downscale lands well under it, so the browser-side normalization stops at the first fit.
func pmv2NoisyPhoto(t *testing.T, w, h int, seed uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	state := uint32(seed)*2654435761 + 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8((x + int(seed)) % 256)
			g := uint8((y*2 + int(seed)) % 256)
			b := uint8(((x + y) / 3) % 256)
			if (x+y)%7 == 0 { // ~14% of pixels: full-entropy noise
				state ^= state << 13
				state ^= state >> 17
				state ^= state << 5
				r, g, b = uint8(state), uint8(state>>8), uint8(state>>16)
			}
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() <= 2<<20 {
		t.Fatalf("noisy photo fixture is %d bytes, want > 2 MiB so the UI downsize path is the only way in", buf.Len())
	}
	return buf.Bytes()
}
