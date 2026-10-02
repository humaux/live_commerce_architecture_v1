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

const maxDecodedImagePixels = 20_000_000

// Each active decode can allocate at most ~80 MiB for RGBA plus bounded output. Waiting respects request cancellation.
var imageDecodeSlots = make(chan struct{}, 2)

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
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxImageDimension || cfg.Height > maxImageDimension || int64(cfg.Width)*int64(cfg.Height) > maxDecodedImagePixels {
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
	source = orientPhoto(source, jpegOrientation(data))
	out := make([]ImageSize, 0, 3)
	for _, bucket := range []int{360, 720, 1080} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		width := min(bucket, source.Bounds().Dx())
		height := max(1, source.Bounds().Dy()*width/source.Bounds().Dx())
		target := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.Draw(target, target.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.CatmullRom.Scale(target, target.Bounds(), source, source.Bounds(), draw.Over, nil)
		var b bytes.Buffer
		if err := jpeg.Encode(&b, target, &jpeg.Options{Quality: 82}); err != nil {
			return nil, err
		}
		if b.Len() > MaxImageBytes {
			return nil, command.ErrInvalid
		}
		out = append(out, ImageSize{Width: bucket, PixelWidth: width, Bytes: b.Bytes(), SHA256: sha256.Sum256(b.Bytes())})
	}
	return out, nil
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
