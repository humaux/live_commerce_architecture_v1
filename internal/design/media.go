package design

// media.go owns store-media: the merchant's logo, favicon, hero and section images (design.store_media, migration
// 0087). Validation is catalog.SniffImage (the product-photo CM2 rules: <= 2 MiB, JPEG/PNG/WebP by magic bytes, never
// the client's filename or Content-Type, no SVG/GIF). At most MaxMediaPerStore per store, enforced under a per-store
// advisory lock; identical bytes are stored once (UNIQUE (store, sha256)) so a retried upload is naturally idempotent.
// Buyers never reach these functions: /media/s/{id} is served by internal/buyerhttp through design.buyer_media.

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/catalog"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// MaxMediaPerStore is the contract cap (storefront-v2 section B, store media).
const MaxMediaPerStore = 60

// Media is one image's metadata; bytes are served only by GetMedia (merchant preview) and design.buyer_media.
type Media struct {
	ID          string    `json:"id"`
	ContentType string    `json:"content_type"`
	SizeBytes   int       `json:"size_bytes"`
	Width       *int      `json:"width"`
	Height      *int      `json:"height"`
	CreatedAt   time.Time `json:"created_at"`
}

// MediaList is the response of list: newest first.
type MediaList struct {
	Items []Media `json:"items"`
}

// MediaBytes is the merchant preview payload of GetMedia.
type MediaBytes struct {
	ContentType string
	Bytes       []byte
}

const mediaColumns = `id::text,content_type,octet_length(bytes),width,height,created_at`

func scanMedia(row pgx.Row, m *Media) error {
	return row.Scan(&m.ID, &m.ContentType, &m.SizeBytes, &m.Width, &m.Height, &m.CreatedAt)
}

// UploadMedia validates and stores one image, or returns the existing row when the same bytes are already in the store.
func UploadMedia(ctx context.Context, tx pgx.Tx, s platform.Scope, data []byte) (Media, error) {
	var out Media
	if !validScope(tx, s) {
		return out, command.ErrInvalid
	}
	contentType, width, height, err := catalog.SniffImage(data)
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(data)
	// Per-store advisory lock: the count-then-insert below cannot race another upload of this store.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('design.store_media|'||$1::text,0))`, s.StoreID); err != nil {
		return out, err
	}
	// design.store_media (commerce_runtime): identical bytes already stored -> return that row (idempotent upload).
	err = scanMedia(tx.QueryRow(ctx, `SELECT `+mediaColumns+` FROM design.store_media WHERE tenant_id=$1 AND store_id=$2 AND sha256=$3`,
		s.TenantID, s.StoreID, digest[:]), &out)
	if err == nil {
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, mapError(err)
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM design.store_media WHERE tenant_id=$1 AND store_id=$2`, s.TenantID, s.StoreID).Scan(&count); err != nil {
		return out, err
	}
	if count >= MaxMediaPerStore {
		return out, command.ErrConflict
	}
	err = scanMedia(tx.QueryRow(ctx, `INSERT INTO design.store_media(tenant_id,store_id,content_type,bytes,sha256,width,height)
		VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+mediaColumns, s.TenantID, s.StoreID, contentType, data, digest[:], width, height), &out)
	return out, mapError(err)
}

// ListMedia returns the store's images (metadata), newest first.
func ListMedia(ctx context.Context, tx pgx.Tx, s platform.Scope) (MediaList, error) {
	list := MediaList{Items: []Media{}}
	if !validScope(tx, s) {
		return list, command.ErrInvalid
	}
	// design.store_media (commerce_runtime): metadata projection, octet_length instead of the bytes.
	rows, err := tx.Query(ctx, `SELECT `+mediaColumns+` FROM design.store_media WHERE tenant_id=$1 AND store_id=$2 ORDER BY created_at DESC,id LIMIT $3`,
		s.TenantID, s.StoreID, MaxMediaPerStore)
	if err != nil {
		return list, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var m Media
		if err := scanMedia(rows, &m); err != nil {
			return list, err
		}
		list.Items = append(list.Items, m)
	}
	return list, mapError(rows.Err())
}

// GetMedia reads one image's bytes for the merchant preview; another store's id is ErrNotFound (RLS + scope filter).
func GetMedia(ctx context.Context, tx pgx.Tx, s platform.Scope, id string) (MediaBytes, error) {
	var out MediaBytes
	if !validScope(tx, s) || !command.ValidID(id) {
		return out, command.ErrInvalid
	}
	err := tx.QueryRow(ctx, `SELECT content_type,bytes FROM design.store_media WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
		s.TenantID, s.StoreID, id).Scan(&out.ContentType, &out.Bytes)
	return out, mapError(err)
}

// DeleteMedia removes an image unless the draft or the live (latest published) document still references it
// (ErrConflict): a published page must never point at a deleted image. Older published versions may reference it; a
// rollback to such a version is refused later by Rollback's image check instead.
func DeleteMedia(ctx context.Context, tx pgx.Tx, s platform.Scope, id string) (MediaList, error) {
	if !validScope(tx, s) || !command.ValidID(id) {
		return MediaList{}, command.ErrInvalid
	}
	// Same lock order as SaveDraft (advisory -> draft row, see lockStore): a reference cannot appear between the check and
	// the delete, even on a store whose first draft is being created right now.
	if err := lockStore(ctx, tx, s); err != nil {
		return MediaList{}, err
	}
	if _, _, err := lockDraft(ctx, tx, s); err != nil && !errors.Is(err, command.ErrNotFound) {
		return MediaList{}, err
	}
	var used bool
	// An image id is a random UUID, so a substring match on the document text is exact enough (no false positives in practice).
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM design.documents WHERE tenant_id=$1 AND store_id=$2 AND document::text LIKE '%'||$3::text||'%')
		OR EXISTS(SELECT 1 FROM (SELECT document FROM design.published_versions WHERE tenant_id=$1 AND store_id=$2 ORDER BY version DESC LIMIT 1) live
		          WHERE live.document::text LIKE '%'||$3::text||'%')`, s.TenantID, s.StoreID, id).Scan(&used)
	if err != nil {
		return MediaList{}, mapError(err)
	}
	if used {
		return MediaList{}, command.ErrConflict
	}
	tag, err := tx.Exec(ctx, `DELETE FROM design.store_media WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, s.TenantID, s.StoreID, id)
	if err != nil {
		return MediaList{}, mapError(err)
	}
	if tag.RowsAffected() != 1 {
		return MediaList{}, command.ErrNotFound
	}
	return ListMedia(ctx, tx, s)
}
