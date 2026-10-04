package catalog

// images_test.go: SniffImage trust-boundary cases (CM2) and the pure guards of the image commands that run before
// any SQL (a nil tx is never touched). Real-PG behavior (positions, RLS, replay) is the independent test phase.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func webpBytes(chunk string, payload int) []byte {
	b := make([]byte, 20+payload)
	copy(b, "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	copy(b[8:], "WEBP")
	copy(b[12:], chunk)
	return b
}

func TestSniffImageAccepts(t *testing.T) {
	for name, tc := range map[string]struct {
		data        []byte
		contentType string
		w, h        int // 0 = dimensions stay nil (WebP)
	}{
		"png":        {pngBytes(t, 7, 5), "image/png", 7, 5},
		"jpeg":       {jpegBytes(t, 9, 4), "image/jpeg", 9, 4},
		"webp lossy": {webpBytes("VP8 ", 10), "image/webp", 0, 0},
		"webp ll":    {webpBytes("VP8L", 10), "image/webp", 0, 0},
		"webp ext":   {webpBytes("VP8X", 10), "image/webp", 0, 0},
	} {
		ct, w, h, err := SniffImage(tc.data)
		if err != nil || ct != tc.contentType {
			t.Fatalf("%s: %q %v", name, ct, err)
		}
		if tc.w == 0 && (w != nil || h != nil) || tc.w != 0 && (w == nil || h == nil || *w != tc.w || *h != tc.h) {
			t.Errorf("%s: dimensions %v %v", name, w, h)
		}
	}
}

func TestSniffImageRejects(t *testing.T) {
	var g bytes.Buffer
	if err := gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	truncatedPNG := pngBytes(t, 4, 4)[:12] // magic present, header cut: DecodeConfig must fail
	badRIFF := webpBytes("VP8 ", 10)
	binary.LittleEndian.PutUint32(badRIFF[4:], uint32(len(badRIFF))) // declared size runs past the file
	for name, data := range map[string][]byte{
		"empty":           nil,
		"gif":             g.Bytes(),
		"svg":             []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":            []byte("<html><body>x</body></html>"),
		"png magic only":  truncatedPNG,
		"jpeg magic only": {0xFF, 0xD8, 0xFF, 0xE0},
		"riff not webp":   append([]byte("RIFF\x04\x00\x00\x00WAVEfmt "), make([]byte, 8)...),
		"webp bad chunk":  webpBytes("JUNK", 10),
		"webp size lies":  badRIFF,
		"too big":         append(pngBytes(t, 2, 2), make([]byte, MaxImageBytes)...),
	} {
		if ct, _, _, err := SniffImage(data); !errors.Is(err, command.ErrInvalid) || ct != "" {
			t.Errorf("%s: accepted %q, err %v", name, ct, err)
		}
	}
	// The boundary itself: exactly MaxImageBytes of otherwise valid PNG passes the size gate.
	exact := pngBytes(t, 2, 2)
	exact = append(exact, make([]byte, MaxImageBytes-len(exact))...)
	if _, _, _, err := SniffImage(exact); err != nil {
		t.Errorf("exactly %d bytes rejected: %v", MaxImageBytes, err)
	}
}

func TestImageCommandsRejectBeforeSQL(t *testing.T) {
	scope := platform.Scope{TenantID: "00000000-0000-0000-0000-000000000001", StoreID: "00000000-0000-0000-0000-000000000002", PrincipalID: "00000000-0000-0000-0000-000000000003"}
	ctx := context.Background()
	good := "00000000-0000-0000-0000-0000000000aa"
	if _, err := UploadImage(ctx, nil, scope, "key-12345678", "not-a-uuid", pngBytes(t, 2, 2), nil); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("upload bad product: %v", err)
	}
	if _, err := UploadImage(ctx, nil, scope, "key-12345678", good, []byte("GIF89a"), nil); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("upload non-image: %v", err)
	}
	if _, err := DeleteImage(ctx, nil, scope, "key-12345678", good, "x"); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("delete bad image: %v", err)
	}
	for name, ids := range map[string][]string{"empty": nil, "bad id": {"x"}, "nine": make([]string, 9)} {
		if _, err := ReorderImages(ctx, nil, scope, "key-12345678", good, ReorderInput{IDs: ids}); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("reorder %s: %v", name, err)
		}
	}
	if _, err := GetImage(ctx, nil, scope, good, good); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("get with nil tx: %v", err)
	}
}
