package catalog

// collections.go owns merchant collections (contracts/storefront-v2.md section A, migrations/0086): create, read,
// patch, delete, the ordered membership, and the one collection image. Reads for buyers are NOT here (the
// catalog.buyer_v2_* definers, internal/buyerhttp catalogv2.go). Tables: catalog.collections,
// catalog.collection_products, catalog.collection_images (role commerce_runtime, FORCE RLS scope_access).
// Every write holds the collection row lock and checks the optimistic version (an image change is the exception:
// it takes no version, a replaced image has a new immutable id anyway). Images reuse SniffImage and MaxImageBytes
// from images.go (CM1/CM2), never a second validator.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

const (
	maxCollectionsPerStore = 200 // ponytail: soft cap (count then insert, no lock); the buyer list is unpaginated
	maxCollectionProducts  = 500 // mirrors the position CHECK 0..499
)

// Collection sort modes and statuses (migrations/0086 CHECKs).
var (
	collectionSortModes = map[string]bool{"manual": true, "newest": true, "price_asc": true, "price_desc": true}
	collectionStatuses  = map[string]bool{"active": true, "hidden": true}
)

type Collection struct {
	ID           string  `json:"id"`
	Slug         string  `json:"slug"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	SortMode     string  `json:"sort_mode"`
	Status       string  `json:"status"`
	ImageID      *string `json:"image_id"`
	ProductCount int     `json:"product_count"`
	Version      int64   `json:"version"`
}

// CollectionProduct is one member of a collection, in manual order.
type CollectionProduct struct {
	ProductID    string  `json:"product_id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Status       string  `json:"status"`
	Position     int     `json:"position"`
	CoverImageID *string `json:"cover_image_id"`
}

// CollectionDetail is a collection plus its members (the admin editor's read).
type CollectionDetail struct {
	Collection
	Products []CollectionProduct `json:"products"`
}

// CollectionInput creates a collection. SortMode "" = manual, Status "" = active, Slug "" = generated.
type CollectionInput struct {
	Title       string `json:"title"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	SortMode    string `json:"sort_mode,omitempty"`
	Status      string `json:"status,omitempty"`
}

// CollectionPatch is a partial update; nil fields are unchanged. ExpectedVersion is mandatory.
type CollectionPatch struct {
	Title           *string `json:"title,omitempty"`
	Slug            *string `json:"slug,omitempty"`
	Description     *string `json:"description,omitempty"`
	SortMode        *string `json:"sort_mode,omitempty"`
	Status          *string `json:"status,omitempty"`
	ExpectedVersion int64   `json:"expected_version"`
}

// CollectionProductsInput replaces the whole ordered membership (add, remove and reorder in one command).
type CollectionProductsInput struct {
	ProductIDs      []string `json:"product_ids"`
	ExpectedVersion int64    `json:"expected_version"`
}

// CollectionDeleted is the answer of DeleteCollection.
type CollectionDeleted struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// CollectionImage is the metadata of the collection's one image (bytes only via GetCollectionImage).
type CollectionImage struct {
	ID           string `json:"id"`
	CollectionID string `json:"collection_id"`
	ContentType  string `json:"content_type"`
	SizeBytes    int    `json:"size_bytes"`
	Width        *int   `json:"width"`
	Height       *int   `json:"height"`
}

const collectionSelect = `SELECT c.id::text,c.slug,c.title,c.description,c.sort_mode,c.status,
	(SELECT i.id::text FROM catalog.collection_images i WHERE i.tenant_id=c.tenant_id AND i.store_id=c.store_id AND i.collection_id=c.id),
	(SELECT count(*) FROM catalog.collection_products m WHERE m.tenant_id=c.tenant_id AND m.store_id=c.store_id AND m.collection_id=c.id),
	c.version FROM catalog.collections c`

func collectionFields(c *Collection) []any {
	return []any{&c.ID, &c.Slug, &c.Title, &c.Description, &c.SortMode, &c.Status, &c.ImageID, &c.ProductCount, &c.Version}
}

func readCollection(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (Collection, error) {
	var c Collection
	err := tx.QueryRow(ctx, collectionSelect+` WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.id=$3`, scope.TenantID, scope.StoreID, id).Scan(collectionFields(&c)...)
	return c, mapError(err)
}

// lockCollection takes the collection row lock and returns its version; ErrNotFound for another store's id.
func lockCollection(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (int64, error) {
	var version int64
	err := tx.QueryRow(ctx, `SELECT version FROM catalog.collections WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR UPDATE`, scope.TenantID, scope.StoreID, id).Scan(&version)
	return version, mapError(err)
}

func validCollectionTitle(t string) bool { return utf8Count(t) >= 1 && utf8Count(t) <= 80 }

// CreateCollection inserts a collection. A generated slug that is taken gets a numeric suffix; a title with no ASCII
// letters or digits gets a random 12-hex slug (collections have no id-prefix trigger). An explicit taken slug is
// ErrConflict.
func CreateCollection(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string, in CollectionInput) (out Collection, err error) {
	sortMode, status := in.SortMode, in.Status
	if sortMode == "" {
		sortMode = "manual"
	}
	if status == "" {
		status = "active"
	}
	if !validCollectionTitle(in.Title) || utf8Count(in.Description) > 2000 || !collectionSortModes[sortMode] || !collectionStatuses[status] ||
		(in.Slug != "" && !validSlug(in.Slug)) {
		return out, command.ErrInvalid
	}
	err = command.Run(ctx, tx, scope, "catalog.collection.create", key, in, &out, func() error {
		if !validScope(tx, scope) {
			return command.ErrInvalid
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM catalog.collections WHERE tenant_id=$1 AND store_id=$2`, scope.TenantID, scope.StoreID).Scan(&count); err != nil {
			return err
		}
		if count >= maxCollectionsPerStore {
			return command.ErrConflict
		}
		slug := &in.Slug
		if in.Slug == "" {
			var err error
			if slug, err = freeSlug(ctx, tx, scope, "catalog.collections", Slugify(in.Title)); err != nil {
				return err
			}
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO catalog.collections(tenant_id,store_id,slug,title,description,sort_mode,status)
			VALUES($1,$2,coalesce($3,left(replace(gen_random_uuid()::text,'-',''),12)),$4,$5,$6,$7) RETURNING id::text`,
			scope.TenantID, scope.StoreID, slug, in.Title, in.Description, sortMode, status).Scan(&id); err != nil {
			return err
		}
		var rerr error
		if out, rerr = readCollection(ctx, tx, scope, id); rerr != nil {
			return rerr
		}
		return command.Audit(ctx, tx, scope, "catalog.collection.created")
	})
	return out, mapError(err)
}

// PatchCollection applies a partial update under the row lock and the optimistic version.
func PatchCollection(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, in CollectionPatch) (out Collection, err error) {
	if !command.ValidID(id) || in.ExpectedVersion < 1 || (in.Title == nil && in.Slug == nil && in.Description == nil && in.SortMode == nil && in.Status == nil) ||
		(in.Title != nil && !validCollectionTitle(*in.Title)) || (in.Description != nil && utf8Count(*in.Description) > 2000) ||
		(in.Slug != nil && !validSlug(*in.Slug)) || (in.SortMode != nil && !collectionSortModes[*in.SortMode]) ||
		(in.Status != nil && !collectionStatuses[*in.Status]) {
		return out, command.ErrInvalid
	}
	request := struct {
		ID string `json:"id"`
		CollectionPatch
	}{id, in}
	err = command.Run(ctx, tx, scope, "catalog.collection.update", key, request, &out, func() error {
		version, err := lockCollection(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if version != in.ExpectedVersion {
			return command.ErrConflict
		}
		cur, err := readCollection(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		for _, f := range []struct {
			set *string
			dst *string
		}{{in.Title, &cur.Title}, {in.Slug, &cur.Slug}, {in.Description, &cur.Description}, {in.SortMode, &cur.SortMode}, {in.Status, &cur.Status}} {
			if f.set != nil {
				*f.dst = *f.set
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE catalog.collections SET title=$4,slug=$5,description=$6,sort_mode=$7,status=$8,version=version+1,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, id, cur.Title, cur.Slug, cur.Description, cur.SortMode, cur.Status); err != nil {
			return err
		}
		if out, err = readCollection(ctx, tx, scope, id); err != nil {
			return err
		}
		return command.Audit(ctx, tx, scope, "catalog.collection.updated")
	})
	return out, mapError(err)
}

// DeleteCollection removes the collection with its membership and image (merchant content, not buyer data).
func DeleteCollection(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, expectedVersion int64) (out CollectionDeleted, err error) {
	if !command.ValidID(id) || expectedVersion < 1 {
		return out, command.ErrInvalid
	}
	request := struct {
		ID              string `json:"id"`
		ExpectedVersion int64  `json:"expected_version"`
	}{id, expectedVersion}
	err = command.Run(ctx, tx, scope, "catalog.collection.delete", key, request, &out, func() error {
		version, err := lockCollection(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if version != expectedVersion {
			return command.ErrConflict
		}
		for _, table := range []string{"catalog.collection_images", "catalog.collection_products"} {
			if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE tenant_id=$1 AND store_id=$2 AND collection_id=$3`, scope.TenantID, scope.StoreID, id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM catalog.collections WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, id); err != nil {
			return err
		}
		out = CollectionDeleted{ID: id, Deleted: true}
		return command.Audit(ctx, tx, scope, "catalog.collection.deleted")
	})
	return out, mapError(err)
}

// SetCollectionProducts replaces the ordered membership: product_ids is the complete new list (unique, <= 500, every
// id a product of this store of any status; a foreign or unknown id is ErrNotFound through the composite FK).
func SetCollectionProducts(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, in CollectionProductsInput) (out CollectionDetail, err error) {
	if !command.ValidID(id) || in.ExpectedVersion < 1 || len(in.ProductIDs) > maxCollectionProducts {
		return out, command.ErrInvalid
	}
	seen := make(map[string]bool, len(in.ProductIDs))
	for _, p := range in.ProductIDs {
		if !command.ValidID(p) || seen[p] {
			return out, command.ErrInvalid
		}
		seen[p] = true
	}
	ids := in.ProductIDs
	if ids == nil {
		ids = []string{}
	}
	request := struct {
		ID string `json:"id"`
		CollectionProductsInput
	}{id, in}
	err = command.Run(ctx, tx, scope, "catalog.collection.set_products", key, request, &out, func() error {
		version, err := lockCollection(ctx, tx, scope, id)
		if err != nil {
			return err
		}
		if version != in.ExpectedVersion {
			return command.ErrConflict
		}
		// Positions are UNIQUE DEFERRABLE, so delete + insert in one transaction is legal; FK catalog.products
		// (tenant, store, id) rejects a foreign product id.
		if _, err := tx.Exec(ctx, `DELETE FROM catalog.collection_products WHERE tenant_id=$1 AND store_id=$2 AND collection_id=$3`, scope.TenantID, scope.StoreID, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO catalog.collection_products(tenant_id,store_id,collection_id,product_id,position)
			SELECT $1,$2,$3,v.id,(v.ord-1)::integer FROM unnest($4::uuid[]) WITH ORDINALITY AS v(id,ord)`, scope.TenantID, scope.StoreID, id, ids); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE catalog.collections SET version=version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, scope.TenantID, scope.StoreID, id); err != nil {
			return err
		}
		if out, err = readCollectionDetail(ctx, tx, scope, id); err != nil {
			return err
		}
		return command.Audit(ctx, tx, scope, "catalog.collection.products_set")
	})
	return out, mapError(err)
}

func readCollectionDetail(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (CollectionDetail, error) {
	d := CollectionDetail{Products: []CollectionProduct{}}
	var err error
	if d.Collection, err = readCollection(ctx, tx, scope, id); err != nil {
		return d, err
	}
	rows, err := tx.Query(ctx, `SELECT p.id::text,p.name,p.slug,p.status,m.position,
		(SELECT i.id::text FROM catalog.product_images i WHERE i.tenant_id=p.tenant_id AND i.store_id=p.store_id AND i.product_id=p.id AND i.role='main' ORDER BY i.position LIMIT 1)
		FROM catalog.collection_products m JOIN catalog.products p ON p.tenant_id=m.tenant_id AND p.store_id=m.store_id AND p.id=m.product_id
		WHERE m.tenant_id=$1 AND m.store_id=$2 AND m.collection_id=$3 ORDER BY m.position`, scope.TenantID, scope.StoreID, id)
	if err != nil {
		return d, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var p CollectionProduct
		if err := rows.Scan(&p.ProductID, &p.Name, &p.Slug, &p.Status, &p.Position, &p.CoverImageID); err != nil {
			return d, err
		}
		d.Products = append(d.Products, p)
	}
	return d, mapError(rows.Err())
}

// GetCollection reads one collection with its members. Another store's id is ErrNotFound.
func GetCollection(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (CollectionDetail, error) {
	if !validScope(tx, scope) || !command.ValidID(id) {
		return CollectionDetail{Products: []CollectionProduct{}}, command.ErrInvalid
	}
	return readCollectionDetail(ctx, tx, scope, id)
}

const timeKey = `to_char(c.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')`

// ListCollectionsPage lists the store's collections oldest first (keyset on created_at, id).
func ListCollectionsPage(ctx context.Context, tx pgx.Tx, scope platform.Scope, request pagination.Request) (pagination.Page[Collection], error) {
	page := pagination.Page[Collection]{Items: make([]Collection, 0)}
	if !validScope(tx, scope) {
		return page, command.ErrInvalid
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "collections"}
	limit, after, err := pagination.Decode(request, binding, 2)
	if err != nil {
		return page, err
	}
	args := []any{scope.TenantID, scope.StoreID}
	filter := ""
	if len(after) == 2 {
		filter = ` AND (c.created_at,c.id) > ($3::timestamptz,$4::uuid)`
		args = append(args, after[0], after[1])
	}
	args = append(args, limit+1)
	rows, err := tx.Query(ctx, collectionSelect+` WHERE c.tenant_id=$1 AND c.store_id=$2`+filter+` ORDER BY c.created_at,c.id LIMIT $`+placeholder(len(args)), args...)
	if err != nil {
		return page, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var c Collection
		if err := rows.Scan(collectionFields(&c)...); err != nil {
			return page, err
		}
		page.Items = append(page.Items, c)
	}
	if err := rows.Err(); err != nil {
		return page, mapError(err)
	}
	if len(page.Items) <= limit {
		return page, nil
	}
	page.Items = page.Items[:limit]
	last := page.Items[limit-1]
	var stamp string
	rows.Close()
	if err := tx.QueryRow(ctx, `SELECT `+timeKey+` FROM catalog.collections c WHERE c.tenant_id=$1 AND c.store_id=$2 AND c.id=$3`, scope.TenantID, scope.StoreID, last.ID).Scan(&stamp); err != nil {
		return page, mapError(err)
	}
	page.NextCursor, err = pagination.Encode(binding, []string{stamp, last.ID})
	return page, err
}

// UploadCollectionImage sets the collection's image, replacing any previous one (new id, so the public URL is
// immutable-cacheable). Same trust boundary as UploadImage: SniffImage decides the type from the magic bytes. The
// command request is the file's SHA-256 + size, not its bytes.
func UploadCollectionImage(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string, data []byte) (out CollectionImage, err error) {
	if !command.ValidID(id) {
		return out, command.ErrInvalid
	}
	contentType, width, height, err := SniffImage(data)
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(data)
	request := struct {
		CollectionID string `json:"collection_id"`
		SHA256       string `json:"sha256"`
		Size         int    `json:"size"`
		ContentType  string `json:"content_type"`
	}{id, hex.EncodeToString(digest[:]), len(data), contentType}
	err = command.Run(ctx, tx, scope, "catalog.collection.image_upload", key, request, &out, func() error {
		if _, err := lockCollection(ctx, tx, scope, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM catalog.collection_images WHERE tenant_id=$1 AND store_id=$2 AND collection_id=$3`, scope.TenantID, scope.StoreID, id); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO catalog.collection_images(tenant_id,store_id,collection_id,content_type,bytes,sha256,width,height)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text,collection_id::text,content_type,octet_length(bytes),width,height`,
			scope.TenantID, scope.StoreID, id, contentType, data, digest[:], width, height).Scan(&out.ID, &out.CollectionID, &out.ContentType, &out.SizeBytes, &out.Width, &out.Height); err != nil {
			return err
		}
		return command.Audit(ctx, tx, scope, "catalog.collection.image_set")
	})
	return out, mapError(err)
}

// DeleteCollectionImage removes the collection's image; returns the collection afterwards.
func DeleteCollectionImage(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, id string) (out Collection, err error) {
	if !command.ValidID(id) {
		return out, command.ErrInvalid
	}
	request := struct {
		CollectionID string `json:"collection_id"`
	}{id}
	err = command.Run(ctx, tx, scope, "catalog.collection.image_delete", key, request, &out, func() error {
		if _, err := lockCollection(ctx, tx, scope, id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM catalog.collection_images WHERE tenant_id=$1 AND store_id=$2 AND collection_id=$3`, scope.TenantID, scope.StoreID, id); err != nil {
			return err
		}
		var rerr error
		if out, rerr = readCollection(ctx, tx, scope, id); rerr != nil {
			return rerr
		}
		return command.Audit(ctx, tx, scope, "catalog.collection.image_removed")
	})
	return out, mapError(err)
}

// GetCollectionImage reads the image bytes for the merchant preview. ErrNotFound when the collection has none.
func GetCollectionImage(ctx context.Context, tx pgx.Tx, scope platform.Scope, id string) (ImageBytes, error) {
	var out ImageBytes
	if !validScope(tx, scope) || !command.ValidID(id) {
		return out, command.ErrInvalid
	}
	err := tx.QueryRow(ctx, `SELECT content_type,sha256,bytes FROM catalog.collection_images WHERE tenant_id=$1 AND store_id=$2 AND collection_id=$3`,
		scope.TenantID, scope.StoreID, id).Scan(&out.ContentType, &out.SHA256, &out.Bytes)
	return out, mapError(err)
}
