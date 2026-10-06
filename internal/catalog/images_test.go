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
	if _, err := UploadImage(ctx, nil, scope, "key-12345678", "not-a-uuid", RoleMain, "", pngBytes(t, 2, 2), nil); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("upload bad product: %v", err)
	}
	if _, err := UploadImage(ctx, nil, scope, "key-12345678", good, RoleMain, "", []byte("GIF89a"), nil); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("upload non-image: %v", err)
	}
	if _, err := DeleteImage(ctx, nil, scope, "key-12345678", good, "x"); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("delete bad image: %v", err)
	}
	for name, ids := range map[string][]string{"empty": nil, "bad id": {"x"}, "five main": make([]string, 5)} {
		if _, err := ReorderImages(ctx, nil, scope, "key-12345678", good, ReorderInput{IDs: ids}); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("reorder %s: %v", name, err)
		}
	}
	// product-media-v2: role/option_value pairing and the detail ratio are refused before any SQL.
	for name, c := range map[string]struct {
		role, value string
		data        []byte
	}{
		"unknown role":        {"gallery", "", pngBytes(t, 2, 2)},
		"sku without a value": {RoleSKU, "", pngBytes(t, 2, 2)},
		"main with a value":   {RoleMain, "Red", pngBytes(t, 2, 2)},
		"detail over 6x":      {RoleDetail, "", pngBytes(t, 10, 61)},
	} {
		if _, err := UploadImage(ctx, nil, scope, "key-12345678", good, c.role, c.value, c.data, nil); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("upload %s: %v", name, err)
		}
	}
	if _, err := ReorderImages(ctx, nil, scope, "key-12345678", good, ReorderInput{Role: RoleSKU, IDs: []string{good}}); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("reorder of the sku role: %v", err)
	}
	for _, role := range []string{"", RoleSKU, "x"} {
		if _, err := MoveImage(ctx, nil, scope, "key-12345678", good, good, MoveInput{Role: role}); !errors.Is(err, command.ErrInvalid) {
			t.Errorf("move to %q: %v", role, err)
		}
	}
	if _, err := GetImage(ctx, nil, scope, good, good); !errors.Is(err, command.ErrInvalid) {
		t.Errorf("get with nil tx: %v", err)
	}
}

// Parity: the per-role caps are part of the contract (migration 0149 CHECKs and the frontend parsers repeat them).
func TestRoleCaps(t *testing.T) {
	if roleCap(RoleMain) != 4 || roleCap(RoleDetail) != 20 || roleCap(RoleSKU) != 50 || roleCap("x") != 0 {
		t.Fatalf("caps %d %d %d", roleCap(RoleMain), roleCap(RoleDetail), roleCap(RoleSKU))
	}
	w, h := 750, 4000
	if tooTall(&w, &h) || tooTall(nil, nil) {
		t.Error("750x4000 and WebP (no dimensions) must pass the detail ratio")
	}
	h = 4501
	if !tooTall(&w, &h) {
		t.Error("750x4501 is taller than 6x")
	}
}

func TestEffectiveAxis(t *testing.T) {
	axes := []OptionAxis{{Name: "Color", Values: []string{"Red", "Blue"}}, {Name: "Size", Values: []string{"S"}}}
	size, gone := "Size", "Material"
	for name, c := range map[string]struct {
		stored *string
		want   string
		ok     bool
	}{"default is the first axis": {nil, "Color", true}, "stored axis": {&size, "Size", true}, "stale stored axis falls back": {&gone, "Color", true}} {
		if got, ok := effectiveAxis(axes, c.stored); got != c.want || ok != c.ok {
			t.Errorf("%s: %q %v", name, got, ok)
		}
	}
	if _, ok := effectiveAxis(nil, nil); ok {
		t.Error("a product without options has no image axis")
	}
	if !axisHasValue(axes, "Color", "Blue") || axisHasValue(axes, "Color", "S") || axisHasValue(axes, "Material", "Red") {
		t.Error("axisHasValue")
	}
}
