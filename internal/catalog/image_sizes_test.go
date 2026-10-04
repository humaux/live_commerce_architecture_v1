package catalog

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"
)

func TestImageSizesDimensionsAndOriginal(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1440, 1800))
	for y := 0; y < 1800; y++ {
		for x := 0; x < 1440; x++ {
			src.Set(x, y, color.RGBA{uint8(x), 64, uint8(y), 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, src, nil); err != nil {
		t.Fatal(err)
	}
	original := append([]byte(nil), b.Bytes()...)
	sizes, err := MakeImageSizes(context.Background(), b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 3 {
		t.Fatalf("got %d sizes", len(sizes))
	}
	for i, w := range []int{360, 720, 1080} {
		cfg, format, err := image.DecodeConfig(bytes.NewReader(sizes[i].Bytes))
		if err != nil || format != "jpeg" || cfg.Width != w || cfg.Height != w*5/4 {
			t.Fatalf("%d: %+v %s %v", w, cfg, format, err)
		}
	}
	if !bytes.Equal(b.Bytes(), original) {
		t.Fatal("source mutated")
	}
	if len(sizes[0].Bytes) > len(original)*3/10 {
		t.Fatal("360 derivative not 70% smaller")
	}
}

func TestImageSizesNoUpscaleAndAlpha(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	sizes, err := MakeImageSizes(context.Background(), b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range sizes {
		decoded, _, err := image.Decode(bytes.NewReader(size.Bytes))
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Bounds().Dx() != 4 || decoded.Bounds().Dy() != 3 {
			t.Fatal("small original upscaled")
		}
		if size.PixelWidth != decoded.Bounds().Dx() {
			t.Fatal("srcset metadata must be decoded width")
		}
		r, g, b, _ := decoded.At(0, 0).RGBA()
		if r < 64000 || g < 64000 || b < 64000 {
			t.Fatal("transparent background must be white")
		}
	}
}

func TestImageSizesRejectCorruptAndBudget(t *testing.T) {
	if _, err := MakeImageSizes(context.Background(), webpBytes("VP8L", 10)); err == nil {
		t.Fatal("corrupt WebP accepted")
	}
	if _, err := MakeImageSizes(context.Background(), []byte("not an image")); err == nil {
		t.Fatal("invalid image accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := MakeImageSizes(ctx, jpegBytes(t, 4, 3)); err == nil {
		t.Fatal("cancelled decode accepted")
	}
	// A valid PNG header describing 24 MP must be rejected before full decode/allocation.
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	binary.BigEndian.PutUint32(data[16:20], 6000)
	binary.BigEndian.PutUint32(data[20:24], 4000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil || cfg.Width != 6000 {
		t.Fatalf("budget fixture header invalid: %+v %v", cfg, err)
	}
	if _, err := MakeImageSizes(context.Background(), data); err == nil {
		t.Fatal("24MP accepted")
	}
}

func TestImageSizesWebP(t *testing.T) {
	data, _ := base64.StdEncoding.DecodeString("UklGRh4AAABXRUJQVlA4TBEAAAAvA4AAAAdQqIKUsf+BiOh/AAA=")
	sizes, err := MakeImageSizes(context.Background(), data)
	if err != nil || len(sizes) != 3 {
		t.Fatalf("WebP: %v", err)
	}
	for _, size := range sizes {
		cfg, format, err := image.DecodeConfig(bytes.NewReader(size.Bytes))
		if err != nil || format != "jpeg" || cfg.Width != 4 || cfg.Height != 3 {
			t.Fatalf("WebP size: %+v %s %v", cfg, format, err)
		}
	}
}

func TestImageSizesOrientation(t *testing.T) {
	// A 2x3 asymmetric source makes all eight transforms distinguishable by corner and dimensions.
	src := image.NewGray(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			src.SetGray(x, y, color.Gray{uint8(1 + x + 2*y)})
		}
	}
	for orientation, want := range map[uint16]uint8{1: 1, 2: 2, 3: 6, 4: 5, 5: 1, 6: 5, 7: 6, 8: 2} {
		got := orientPhoto(src, orientation)
		if color.GrayModel.Convert(got.At(0, 0)).(color.Gray).Y != want {
			t.Fatalf("orientation %d corner", orientation)
		}
		w, h := 2, 3
		if orientation >= 5 {
			w, h = h, w
		}
		if got.Bounds().Dx() != w || got.Bounds().Dy() != h {
			t.Fatalf("orientation %d bounds", orientation)
		}
	}
	// Standard little-endian IFD0 orientation=6; byte bounds are checked on every truncated prefix.
	segment := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	file := append([]byte{255, 216, 255, 225, 0, byte(len(segment) + 2)}, segment...)
	if jpegOrientation(file) != 6 {
		t.Fatal("EXIF orientation not read")
	}
	for end := 0; end < len(file); end++ {
		_ = jpegOrientation(file[:end])
	}
}

// peakHeapGrowth samples HeapInuse while fn runs and returns the peak growth over the pre-call baseline. GC runs at 5%
// growth meanwhile so HeapInuse tracks live memory (what an OOM kill depends on), not the default pacer's garbage.
func peakHeapGrowth(fn func()) uint64 {
	defer debug.SetGCPercent(debug.SetGCPercent(5))
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	base, peak := m.HeapInuse, m.HeapInuse
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			var s runtime.MemStats
			runtime.ReadMemStats(&s)
			peak = max(peak, s.HeapInuse)
			time.Sleep(time.Millisecond)
		}
	}()
	fn()
	close(done)
	wg.Wait()
	return peak - base
}

// A valid in-contract 1000x20000 detail strip made the kernel scaler allocate dst_width*src_height*32 bytes (711 MiB
// peak measured), over the api container's 512m. Each case must stay inside the documented one-slot budget; the 20 MP
// EXIF-6 portrait is the largest common photo (scratch for the 1080 bucket ≈ 180 MiB).
func TestImageSizesMemoryBudget(t *testing.T) {
	strip := func(w, h int) func() []byte { return func() []byte { return pngBytes(t, w, h) } }
	cases := map[string]func() []byte{
		"1000x20000 strip": strip(1000, 20000),
		"20000x1000 strip": strip(20000, 1000),
		"20MP EXIF-6 portrait": func() []byte {
			src := image.NewGray(image.Rect(0, 0, 5472, 3648))
			for i := range src.Pix {
				src.Pix[i] = uint8(i / 5472 / 16) // smooth rows keep the JPEG under MaxImageBytes
			}
			var b bytes.Buffer
			if err := jpeg.Encode(&b, src, &jpeg.Options{Quality: 60}); err != nil {
				t.Fatal(err)
			}
			return withOrientation6(b.Bytes())
		},
	}
	for name, fixture := range cases {
		data := fixture()
		var sizes []ImageSize
		var err error
		grew := peakHeapGrowth(func() { sizes, err = MakeImageSizes(context.Background(), data) })
		if err != nil || len(sizes) != 3 {
			t.Fatalf("%s: sizes=%d err=%v", name, len(sizes), err)
		}
		if grew > 320<<20 {
			t.Fatalf("%s: decode heap grew %d MiB, budget 320 MiB", name, grew>>20)
		}
		t.Logf("%s: peak heap growth %d MiB", name, grew>>20)
	}
}

// withOrientation6 inserts a little-endian EXIF APP1 (IFD0 orientation=6) right after the JPEG SOI.
func withOrientation6(jpg []byte) []byte {
	segment := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	file := append([]byte{255, 216, 255, 225, 0, byte(len(segment) + 2)}, segment...)
	return append(file, jpg[2:]...)
}

// A 16-bit source decodes at 8 bytes/pixel; above 10 MP it is refused from the header, before any pixel allocation.
// The fixture is a real, decodable 3300x3300 16-bit PNG (inside the 20 MP cap, over the 80 MiB decoded-bytes budget).
func TestImageSizesRejectsLarge16Bit(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA64(image.Rect(0, 0, 3300, 3300))); err != nil {
		t.Fatal(err)
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(b.Bytes())); err != nil || cfg.ColorModel != color.NRGBA64Model || b.Len() > MaxImageBytes {
		t.Fatalf("16-bit fixture invalid: %+v len=%d %v", cfg, b.Len(), err)
	}
	if _, err := MakeImageSizes(context.Background(), b.Bytes()); err == nil {
		t.Fatal("10.9 MP 16-bit PNG accepted")
	}
}

// Guard for the scale-then-rotate order: EXIF orientation 6 on a 40x30 raw JPEG yields a 30x40 rendition.
func TestImageSizesOrientationEndToEnd(t *testing.T) {
	sizes, err := MakeImageSizes(context.Background(), withOrientation6(jpegBytes(t, 40, 30)))
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range sizes {
		img, err := jpeg.Decode(bytes.NewReader(size.Bytes))
		if err != nil || size.PixelWidth != 30 || img.Bounds().Dx() != 30 || img.Bounds().Dy() != 40 {
			t.Fatalf("bucket %d: pixel_width=%d bounds=%v err=%v", size.Width, size.PixelWidth, img.Bounds(), err)
		}
	}
}
