package catalog

// S1 owns bounded product-photo decoding and immutable JPEG renditions. Original bytes stay in product_images;
// children share its RLS/FK custody (0111). No remote URLs, filesystem cache, buyer-triggered writes or image updates.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/jpeg"

	"github.com/jackc/pgx/v5"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // real WebP decoding; legacy SniffImage metadata remains backwards compatible
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const (
	maxDecodedImagePixels = 20_000_000
	// maxDecodedImageBytes bounds the decoded source (20 MP at 4 bytes/pixel), so a 16-bit source (8 bytes/pixel) is
	// capped at 10 MP. Estimated from DecodeConfig before any pixel allocation.
	maxDecodedImageBytes = 80 << 20
	// maxScaleScratchBytes bounds x/image/draw's kernel scratch, which is dst_width*src_height*32 bytes
	// ([][4]float64 in draw/scale.go) and is NOT bounded by the pixel cap: a 1000x20000 detail strip would need
	// 610 MiB. Fits CatmullRom for a 20 MP portrait phone photo at the 1080 bucket (1620*3648*32 = 180 MiB).
	maxScaleScratchBytes = 192 << 20
)

// One decode at a time: source ≤ 80 MiB + one scratch ≤ 192 MiB + the 1080 rendition (and its oriented copy). Measured
// live heap growth (TestImageSizesMemoryBudget): 20 MP EXIF-6 portrait JPEG ~216 MiB, 1000x20000 strip ~209 MiB, under the
// api's GOMEMLIMIT 410 MiB / mem_limit 512m (deploy/compose.yml). Waiting respects request cancellation; the upload path
// decodes before opening its DB transaction, so waiters do not hold pool connections.
var imageDecodeSlots = make(chan struct{}, 1)

type ImageSize struct {
	Width      int
	PixelWidth int
	Bytes      []byte
	SHA256     [32]byte
}

// MakeImageSizes decodes once, limits compressed bytes/pixels/concurrency, preserves aspect and composites alpha on white.
// Go's JPEG encoder is standard library; x/image supplies maintained resampling/WebP support, not another media service.
func MakeImageSizes(ctx context.Context, data []byte) ([]ImageSize, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, _, _, err := SniffImage(data); err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxImageDimension || cfg.Height > maxImageDimension || int64(cfg.Width)*int64(cfg.Height) > maxDecodedImagePixels ||
		int64(cfg.Width)*int64(cfg.Height)*decodedBytesPerPixel(cfg.ColorModel) > maxDecodedImageBytes {
		return nil, command.ErrInvalid
	}
	select {
	case imageDecodeSlots <- struct{}{}:
		defer func() { <-imageDecodeSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, command.ErrInvalid
	}
	if source.Bounds().Dx() != cfg.Width || source.Bounds().Dy() != cfg.Height {
		return nil, command.ErrInvalid
	}
	// Scale the raw (unrotated) pixels, then rotate only the small rendition: the oriented view reads through the
	// generic At() path, which on a full-size source cost seconds of CPU per upload.
	orientation := jpegOrientation(data)
	sw, sh := cfg.Width, cfg.Height
	transposed := orientation >= 5 && orientation <= 8
	if transposed {
		sw, sh = sh, sw // oriented width/height
	}
	// Largest first, each smaller rendition scaled from the previous one: only the first step reads the full-size source
	// (one big scratch, and the source becomes collectable), and 720/360 are short CatmullRom steps of 1.5x/2x.
	buckets := []int{1080, 720, 360}
	out := make([]ImageSize, len(buckets))
	var from image.Image = source
	for i, bucket := range buckets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		width := min(bucket, sw)
		height := max(1, sh*width/sw)
		rawW, rawH := width, height
		if transposed {
			rawW, rawH = height, width
		}
		raw := image.NewRGBA(image.Rect(0, 0, rawW, rawH))
		draw.Draw(raw, raw.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		var scaler draw.Scaler = draw.CatmullRom
		if int64(rawW)*int64(from.Bounds().Dy())*32 > maxScaleScratchBytes {
			// ponytail: ApproxBiLinear needs no scratch but aliases beyond ~2x; only tall strips/extreme aspects land
			// here. Scale in overlapping horizontal stripes if their rendition quality matters.
			scaler = draw.ApproxBiLinear
		}
		scaler.Scale(raw, raw.Bounds(), from, from.Bounds(), draw.Over, nil)
		from = raw
		target := raw
		if orientation >= 2 && orientation <= 8 {
			target = image.NewRGBA(image.Rect(0, 0, width, height))
			draw.Draw(target, target.Bounds(), orientPhoto(raw, orientation), image.Point{}, draw.Src)
		}
		var b bytes.Buffer
		if err := jpeg.Encode(&b, target, &jpeg.Options{Quality: 82}); err != nil {
			return nil, err
		}
		if b.Len() > MaxImageBytes {
			return nil, command.ErrInvalid
		}
		out[len(buckets)-1-i] = ImageSize{Width: bucket, PixelWidth: width, Bytes: b.Bytes(), SHA256: sha256.Sum256(b.Bytes())}
	}
	return out, nil
}

// decodedBytesPerPixel is the decoded-buffer cost per pixel: 16-bit models decode to 8 bytes, everything else to at most 4.
func decodedBytesPerPixel(m color.Model) int64 {
	switch m {
	case color.RGBA64Model, color.NRGBA64Model, color.Gray16Model, color.Alpha16Model:
		return 8
	}
	return 4
}

// storeImageSizes may insert only into the authenticated parent scope, never replace previously published bytes.
func storeImageSizes(ctx context.Context, tx pgx.Tx, scope platform.Scope, imageID string, sizes []ImageSize) error {
	for _, size := range sizes {
		if _, err := tx.Exec(ctx, `INSERT INTO catalog.product_image_sizes(tenant_id,store_id,image_id,width,pixel_width,bytes,sha256)
   VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant_id,store_id,image_id,width) DO NOTHING`, scope.TenantID, scope.StoreID, imageID, size.Width, size.PixelWidth, size.Bytes, size.SHA256[:]); err != nil {
			return err
		}
	}
	return nil
}

type ImageSizesResult struct {
	ImageID string `json:"image_id"`
	Widths  []int  `json:"widths"`
}

// BackfillImageSizes is the explicit per-image merchant task for historical photos. It is catalog:write and command-idempotent.
// Decode occurs before the product lock, then the immutable source identity is rechecked under that lock (delete uses it too).
func BackfillImageSizes(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, productID, imageID string) (out ImageSizesResult, err error) {
	original, err := GetImage(ctx, tx, scope, productID, imageID)
	if err != nil {
		return out, err
	}
	sizes, err := MakeImageSizes(ctx, original.Bytes)
	if err != nil {
		return out, err
	}
	request := struct{ ProductID, ImageID, SHA256 string }{productID, imageID, hex.EncodeToString(original.SHA256)}
	err = command.Run(ctx, tx, scope, "catalog.image.sizes", key, request, &out, func() error {
		if _, err := editableProduct(ctx, tx, scope, productID); err != nil {
			return err
		}
		current, err := GetImage(ctx, tx, scope, productID, imageID)
		if err != nil {
			return err
		}
		if !bytes.Equal(current.SHA256, original.SHA256) {
			return command.ErrConflict
		}
		if err := storeImageSizes(ctx, tx, scope, imageID, sizes); err != nil {
			return err
		}
		out = ImageSizesResult{ImageID: imageID, Widths: []int{360, 720, 1080}}
		return command.Audit(ctx, tx, scope, "catalog.product.image_sizes_added")
	})
	return out, mapError(err)
}
