package catalog

// images.go owns merchant product photos (docs/delivery/units/catalog-media.md CM1-CM3): validation of an
// uploaded file at the trust boundary and the upload/list/read/delete/reorder commands on catalog.product_images
// (migrations/0082). Original bytes stay exactly as validated; S1 image_sizes.go generates separate children (0111). Never serves buyers
// (the buyer side reads through the catalog.buyer_* definers in internal/storefront and internal/buyerhttp), and
// never trusts a client-supplied filename or Content-Type: the type is derived from the magic bytes.
//
// Depends on: internal/command (idempotent Run, Audit), internal/platform.Scope (tenant/store/principal from server
// auth). Tables: catalog.product_images (role commerce_runtime, FORCE RLS scope_access) and catalog.products
// (row lock via lockProduct, the same lock order as archive and SKU mutations: product first).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	_ "image/jpeg" // registers the JPEG decoder for image.DecodeConfig (dimensions only, never a full decode)
	_ "image/png"  // registers the PNG decoder for image.DecodeConfig

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const (
	// MaxImageBytes is the per-photo cap (CM1); the SQL CHECK octet_length(bytes) repeats it.
	MaxImageBytes = 2 << 20
	// MaxImagesPerProduct matches the position CHECK 0..11 in migrations/0082 (widened by 0109, product-editor §f).
	MaxImagesPerProduct = 12
	maxImageDimension   = 20000
)

// Image is one photo's metadata. Bytes are served only by GetImage.
type Image struct {
	ID          string `json:"id"`
	ProductID   string `json:"product_id"`
	Position    int    `json:"position"`
	ContentType string `json:"content_type"`
	SizeBytes   int    `json:"size_bytes"`
	Width       *int   `json:"width"`
	Height      *int   `json:"height"`
	Version     int64  `json:"version"`
}

// ImageList is the response of list, delete and reorder: the product's photos in display order.
type ImageList struct {
	Items []Image `json:"items"`
}

// ImageBytes is the merchant preview payload of GetImage.
type ImageBytes struct {
	ContentType string
	SHA256      []byte
	Bytes       []byte
}

// ReorderInput is POST .../images/order: the complete new order, a permutation of the current ids.
type ReorderInput struct {
	IDs []string `json:"ids"`
}

// SniffImage validates an uploaded file (CM2): size 1..2 MiB, a JPEG, PNG or WebP magic signature, and for
// JPEG/PNG a decodable header with sane dimensions (image.DecodeConfig; WebP width/height stay nil because the
// stdlib has no WebP decoder and a new dependency is not allowed). SVG, GIF and everything else is refused:
// scriptable or animated formats have no business on a buyer page.
func SniffImage(data []byte) (contentType string, width, height *int, err error) {
	if len(data) == 0 || len(data) > MaxImageBytes {
		return "", nil, nil, command.ErrInvalid
	}
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		contentType = "image/jpeg"
	case bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		contentType = "image/png"
	case validWebP(data):
		return "image/webp", nil, nil, nil
	default:
		return "", nil, nil, command.ErrInvalid
	}
	cfg, format, derr := image.DecodeConfig(bytes.NewReader(data))
	if derr != nil || (contentType == "image/jpeg") != (format == "jpeg") || (contentType == "image/png") != (format == "png") ||
		cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxImageDimension || cfg.Height > maxImageDimension {
		return "", nil, nil, command.ErrInvalid
	}
	w, h := cfg.Width, cfg.Height
	return contentType, &w, &h, nil
}

// validWebP checks the RIFF container (RIFF <size> WEBP) and that the first chunk is a WebP bitstream chunk
// (lossy "VP8 ", lossless "VP8L" or extended "VP8X"). It does not decode the bitstream.
func validWebP(data []byte) bool {
	if len(data) < 20 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return false
	}
	size := int64(binary.LittleEndian.Uint32(data[4:8]))
	switch string(data[12:16]) {
	case "VP8 ", "VP8L", "VP8X":
	default:
		return false
	}
	return size+8 <= int64(len(data)) && size >= 12
}

const imageColumns = `id::text,product_id::text,position,content_type,octet_length(bytes),width,height,version`

func scanImage(row pgx.Row, out *Image) error {
	return row.Scan(&out.ID, &out.ProductID, &out.Position, &out.ContentType, &out.SizeBytes, &out.Width, &out.Height, &out.Version)
}

// UploadImage stores one validated photo at the next free position of a draft or active product (CM3). The command
// request is the file's SHA-256 + size, not its bytes (command.Run caps the request at 64 KiB), so a retry of the
// same file under the same key replays the first Image and the same key with another file is ErrConflict. sizes are
// MakeImageSizes(data), computed by the caller BEFORE the transaction opens (decoding must not hold a pool connection).
func UploadImage(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, productID string, data []byte, sizes []ImageSize) (out Image, err error) {
	if !command.ValidID(productID) {
		return out, command.ErrInvalid
	}
	contentType, width, height, err := SniffImage(data)
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(data)
	request := struct {
		ProductID   string `json:"product_id"`
		SHA256      string `json:"sha256"`
		Size        int    `json:"size"`
		ContentType string `json:"content_type"`
	}{productID, hex.EncodeToString(digest[:]), len(data), contentType}
	err = command.Run(ctx, tx, scope, "catalog.image.upload", key, request, &out, func() error {
		// editableProduct locks the product row: every image mutation of this product serializes here, so the count
		// below cannot race another upload, delete or reorder.
		if _, err := editableProduct(ctx, tx, scope, productID); err != nil {
			return err
		}
		var count int
		// catalog.product_images (commerce_runtime): positions are contiguous 0..n-1, so n is the next free one.
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM catalog.product_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3`,
			scope.TenantID, scope.StoreID, productID).Scan(&count); err != nil {
			return err
		}
		if count >= MaxImagesPerProduct {
			return command.ErrConflict
		}
		if err := scanImage(tx.QueryRow(ctx, `INSERT INTO catalog.product_images(tenant_id,store_id,product_id,position,content_type,bytes,sha256,width,height)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+imageColumns,
			scope.TenantID, scope.StoreID, productID, count, contentType, data, digest[:], width, height), &out); err != nil {
			return err
		}
		if err := storeImageSizes(ctx, tx, scope, out.ID, sizes); err != nil {
			return err
		}
		return command.Audit(ctx, tx, scope, "catalog.product.image_added")
	})
	return out, mapError(err)
}

// ListImages returns the product's photos (metadata only) in display order. A product of another store is
// ErrNotFound through the scoped lookup, never an empty list.
func ListImages(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string) (ImageList, error) {
	list := ImageList{Items: make([]Image, 0)}
	if !validScope(tx, scope) || !command.ValidID(productID) {
		return list, command.ErrInvalid
	}
	// A read-only visibility check holds no row lock (same rule as ListSKUsPage).
	var visible bool
	if err := tx.QueryRow(ctx, `SELECT true FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, productID).Scan(&visible); err != nil {
		return list, mapError(err)
	}
	return readImages(ctx, tx, scope, productID)
}

func readImages(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID string) (ImageList, error) {
	list := ImageList{Items: make([]Image, 0)}
	// catalog.product_images (commerce_runtime): metadata projection, octet_length instead of the bytes themselves.
	rows, err := tx.Query(ctx, `SELECT `+imageColumns+` FROM catalog.product_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 ORDER BY position`,
		scope.TenantID, scope.StoreID, productID)
	if err != nil {
		return list, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var img Image
		if err := scanImage(rows, &img); err != nil {
			return list, err
		}
		list.Items = append(list.Items, img)
	}
	return list, mapError(rows.Err())
}

// GetImage reads one photo's bytes for the merchant preview. The product id in the path must own the image.
func GetImage(ctx context.Context, tx pgx.Tx, scope platform.Scope, productID, imageID string) (ImageBytes, error) {
	var out ImageBytes
	if !validScope(tx, scope) || !command.ValidID(productID) || !command.ValidID(imageID) {
		return out, command.ErrInvalid
	}
	// catalog.product_images (commerce_runtime): the only merchant read of bytes.
	err := tx.QueryRow(ctx, `SELECT content_type,sha256,bytes FROM catalog.product_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND id=$4`,
		scope.TenantID, scope.StoreID, productID, imageID).Scan(&out.ContentType, &out.SHA256, &out.Bytes)
	return out, mapError(err)
}

// DeleteImage removes one photo and closes the gap so positions stay contiguous (position 0 stays the cover).
// Deleting is allowed on an archived product. Returns the remaining list.
func DeleteImage(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, productID, imageID string) (out ImageList, err error) {
	if !command.ValidID(productID) || !command.ValidID(imageID) {
		return out, command.ErrInvalid
	}
	request := struct {
		ProductID string `json:"product_id"`
		ImageID   string `json:"image_id"`
	}{productID, imageID}
	err = command.Run(ctx, tx, scope, "catalog.image.delete", key, request, &out, func() error {
		if err := lockProduct(ctx, tx, scope, productID); err != nil {
			return err
		}
		var removed int
		// catalog.product_images (commerce_runtime): DELETE then renumber; positions are UNIQUE DEFERRABLE, so the
		// transient state inside this statement pair is legal and checked at commit.
		if err := tx.QueryRow(ctx, `DELETE FROM catalog.product_images WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND id=$4 RETURNING position`,
			scope.TenantID, scope.StoreID, productID, imageID).Scan(&removed); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE catalog.product_images SET position=position-1,version=version+1
			WHERE tenant_id=$1 AND store_id=$2 AND product_id=$3 AND position>$4`, scope.TenantID, scope.StoreID, productID, removed); err != nil {
			return err
		}
		if err := command.Audit(ctx, tx, scope, "catalog.product.image_deleted"); err != nil {
			return err
		}
		out, err = readImages(ctx, tx, scope, productID)
		return err
	})
	return out, mapError(err)
}

// ReorderImages applies a complete new order. ids must be exactly the product's current image ids (a permutation:
// no missing, extra or repeated id) so a stale client can never drop or resurrect a photo; otherwise ErrConflict.
func ReorderImages(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, productID string, in ReorderInput) (out ImageList, err error) {
	if !command.ValidID(productID) || len(in.IDs) < 1 || len(in.IDs) > MaxImagesPerProduct {
		return out, command.ErrInvalid
	}
	for _, id := range in.IDs {
		if !command.ValidID(id) {
			return out, command.ErrInvalid
		}
	}
	request := struct {
		ProductID string   `json:"product_id"`
		IDs       []string `json:"ids"`
	}{productID, in.IDs}
	err = command.Run(ctx, tx, scope, "catalog.image.reorder", key, request, &out, func() error {
		if err := lockProduct(ctx, tx, scope, productID); err != nil {
			return err
		}
		current, err := readImages(ctx, tx, scope, productID)
		if err != nil {
			return err
		}
		have := make(map[string]bool, len(current.Items))
		for _, img := range current.Items {
			have[img.ID] = true
		}
		if len(have) != len(in.IDs) {
			return command.ErrConflict
		}
		seen := make(map[string]bool, len(in.IDs))
		for _, id := range in.IDs {
			if !have[id] || seen[id] {
				return command.ErrConflict
			}
			seen[id] = true
		}
		// catalog.product_images (commerce_runtime): only rows whose slot changes are touched; the deferred UNIQUE
		// lets two rows swap inside this one statement.
		if _, err := tx.Exec(ctx, `UPDATE catalog.product_images i SET position=(v.ord-1)::smallint,version=i.version+1
			FROM unnest($4::uuid[]) WITH ORDINALITY AS v(id,ord)
			WHERE i.tenant_id=$1 AND i.store_id=$2 AND i.product_id=$3 AND i.id=v.id AND i.position<>(v.ord-1)::smallint`,
			scope.TenantID, scope.StoreID, productID, in.IDs); err != nil {
			return err
		}
		if err := command.Audit(ctx, tx, scope, "catalog.product.images_reordered"); err != nil {
			return err
		}
		out, err = readImages(ctx, tx, scope, productID)
		return err
	})
	return out, mapError(err)
}
