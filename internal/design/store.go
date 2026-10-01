package design

// store.go is the draft/publish/rollback lifecycle on design.documents and design.published_versions (migration 0087).
// Every function runs inside platform.WithScope as commerce_runtime: the tenant/store/principal come from the server's
// authentication, never from a request, and RLS (policy scope_access) repeats the store filter. Concurrency: the draft
// row is the per-store lock. SaveDraft is a compare-and-set UPDATE on version; Publish and Rollback take the same row
// FOR UPDATE, so the per-store published version counter cannot fork. No retry loop anywhere: a lost race is ErrConflict
// and the merchant re-reads.

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// maxVersionsListed bounds GET design/versions (ponytail: newest 100; a cursor when a store publishes more than that).
const maxVersionsListed = 100

// Draft is the editable document plus the live (highest published) version, if any. Version 0 means "no draft saved
// yet": Document is then the derived default and the first PUT must send expected_version 0.
type Draft struct {
	Version          int64           `json:"version"`
	Document         json.RawMessage `json:"document"`
	UpdatedAt        *time.Time      `json:"updated_at"`
	PublishedVersion *int64          `json:"published_version"`
}

// SaveInput is PUT design/draft. ExpectedVersion is a pointer so that a missing field is refused, not read as 0.
type SaveInput struct {
	ExpectedVersion *int64          `json:"expected_version"`
	Document        json.RawMessage `json:"document"`
}

// PublishInput is POST design/publish: the draft version the merchant looked at.
type PublishInput struct {
	ExpectedDraftVersion *int64 `json:"expected_draft_version"`
}

// RollbackInput is POST design/rollback: the published version to copy forward.
type RollbackInput struct {
	Version *int64 `json:"version"`
}

// VersionInfo is one row of published history (metadata only, never the document).
type VersionInfo struct {
	Version       int64     `json:"version"`
	Kind          string    `json:"kind"`
	SourceVersion int64     `json:"source_version"`
	PublishedAt   time.Time `json:"published_at"`
	PublishedBy   string    `json:"published_by"`
}

// VersionList is the response of GET design/versions, newest first. LiveVersion is the highest version (nil: none).
type VersionList struct {
	Items       []VersionInfo `json:"items"`
	LiveVersion *int64        `json:"live_version"`
}

func validScope(tx pgx.Tx, s platform.Scope) bool {
	return tx != nil && command.ValidID(s.TenantID) && command.ValidID(s.StoreID) && command.ValidID(s.PrincipalID)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return command.ErrConflict
		case "23503":
			return command.ErrNotFound
		case "22001", "22P02", "23514":
			return command.ErrInvalid
		}
	}
	return err
}

func liveVersion(ctx context.Context, tx pgx.Tx, s platform.Scope) (*int64, error) {
	var live *int64
	// design.published_versions (commerce_runtime): the highest version is the one buyers see.
	err := tx.QueryRow(ctx, `SELECT max(version) FROM design.published_versions WHERE tenant_id=$1 AND store_id=$2`, s.TenantID, s.StoreID).Scan(&live)
	return live, err
}

// ReadDraft returns the store's draft, or the derived default at version 0 when none was ever saved.
func ReadDraft(ctx context.Context, tx pgx.Tx, s platform.Scope) (Draft, error) {
	var out Draft
	if !validScope(tx, s) {
		return out, command.ErrInvalid
	}
	// design.documents (commerce_runtime, RLS scope_access): the single draft row.
	var document []byte // scanned as bytes: jsonb -> []byte is raw JSON in pgx
	err := tx.QueryRow(ctx, `SELECT version,document,updated_at FROM design.documents WHERE tenant_id=$1 AND store_id=$2`, s.TenantID, s.StoreID).
		Scan(&out.Version, &document, &out.UpdatedAt)
	out.Document = document
	if errors.Is(err, pgx.ErrNoRows) {
		var name string
		// control.stores (commerce_runtime, stores_scoped): the default document is derived from the store name.
		if err = tx.QueryRow(ctx, `SELECT name FROM control.stores WHERE tenant_id=$1 AND id=$2`, s.TenantID, s.StoreID).Scan(&name); err != nil {
			return out, mapError(err)
		}
		out = Draft{Version: 0, Document: DefaultDocument(name)}
	} else if err != nil {
		return out, mapError(err)
	}
	live, err := liveVersion(ctx, tx, s)
	out.PublishedVersion = live
	return out, mapError(err)
}

// checkImages proves every referenced image id is a design.store_media row of THIS store (a foreign or unknown id is a
// validation error at the path of its first use, indistinguishable from each other).
func checkImages(ctx context.Context, tx pgx.Tx, s platform.Scope, refs Refs) error {
	if len(refs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(refs))
	for id := range refs {
		ids = append(ids, id)
	}
	// design.store_media (commerce_runtime, RLS scope_access): ids of other stores simply do not appear.
	rows, err := tx.Query(ctx, `SELECT id::text FROM design.store_media WHERE tenant_id=$1 AND store_id=$2 AND id=ANY($3::uuid[])`, s.TenantID, s.StoreID, ids)
	if err != nil {
		return mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		delete(refs, id)
	}
	if err := rows.Err(); err != nil {
		return mapError(err)
	}
	var path string
	for _, p := range refs { // anything left is missing; report deterministically the lexicographically smallest path
		if path == "" || p < path {
			path = p
		}
	}
	if path != "" {
		return &ValidationError{Path: path, Reason: "image not found in this store's media"}
	}
	return nil
}

// SaveDraft validates, normalises and stores the document with compare-and-set on the draft version. expected_version 0
// creates the draft (a concurrent creator makes the loser ErrConflict). Not idempotency-keyed: the CAS makes a replay of
// the same request a conflict instead of a second write.
func SaveDraft(ctx context.Context, tx pgx.Tx, s platform.Scope, in SaveInput) (Draft, error) {
	var out Draft
	if !validScope(tx, s) || in.ExpectedVersion == nil || *in.ExpectedVersion < 0 {
		return out, command.ErrInvalid
	}
	doc, refs, err := Normalize(in.Document)
	if err != nil {
		return out, err
	}
	// D1: lock first, check references second, inside this same transaction (see lockStore).
	if err = lockStore(ctx, tx, s); err != nil {
		return out, err
	}
	if _, _, err = lockDraft(ctx, tx, s); err != nil && !errors.Is(err, command.ErrNotFound) {
		return out, err
	}
	if err = checkImages(ctx, tx, s, refs); err != nil {
		return out, err
	}
	var saved []byte
	if *in.ExpectedVersion == 0 {
		// design.documents (commerce_runtime): first save of the store's draft; ON CONFLICT means someone else created it.
		err = tx.QueryRow(ctx, `INSERT INTO design.documents(tenant_id,store_id,document,updated_by) VALUES($1,$2,$3,$4)
			ON CONFLICT (tenant_id,store_id) DO NOTHING RETURNING version,document,updated_at`,
			s.TenantID, s.StoreID, doc, s.PrincipalID).Scan(&out.Version, &saved, &out.UpdatedAt)
	} else {
		// design.documents: compare-and-set on version; zero rows = stale expected_version.
		err = tx.QueryRow(ctx, `UPDATE design.documents SET version=version+1,document=$4,updated_by=$5,updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND version=$3 RETURNING version,document,updated_at`,
			s.TenantID, s.StoreID, *in.ExpectedVersion, doc, s.PrincipalID).Scan(&out.Version, &saved, &out.UpdatedAt)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, command.ErrConflict
	}
	if err != nil {
		return out, mapError(err)
	}
	out.Document = saved
	out.PublishedVersion, err = liveVersion(ctx, tx, s)
	return out, mapError(err)
}

// lockStore is the first lock of every path that checks image references against a write of the draft or of the media
// (SaveDraft, DeleteMedia): one transaction-scoped advisory lock per store, taken BEFORE the draft row lock and before any
// reference check, so "does the draft reference image X" and "delete image X" can never interleave (write skew, defect D1).
// It also covers the first save, when no draft row exists yet to lock. Lock order everywhere: advisory -> draft row.
// Publish/Rollback/preview take only the row lock and never wait on the advisory one, so no cycle is possible.
func lockStore(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('design.document|'||$1::text,0))`, s.StoreID)
	return err
}

// lockDraft takes the per-store lock. ErrNotFound when no draft was ever saved (nothing can be published or rolled back).
func lockDraft(ctx context.Context, tx pgx.Tx, s platform.Scope) (version int64, document []byte, err error) {
	// design.documents: FOR UPDATE serialises publish, rollback and draft saves of this store.
	err = tx.QueryRow(ctx, `SELECT version,document FROM design.documents WHERE tenant_id=$1 AND store_id=$2 FOR UPDATE`, s.TenantID, s.StoreID).Scan(&version, &document)
	return version, document, mapError(err)
}

func appendVersion(ctx context.Context, tx pgx.Tx, s platform.Scope, kind string, source int64, document []byte) (out VersionInfo, err error) {
	// design.published_versions (commerce_runtime, INSERT only): the next per-store version under the draft row lock.
	err = tx.QueryRow(ctx, `INSERT INTO design.published_versions(tenant_id,store_id,version,document,kind,source_version,published_by)
		SELECT $1::uuid,$2::uuid,coalesce(max(version),0)+1,$3::jsonb,$4::text,$5::bigint,$6::uuid FROM design.published_versions WHERE tenant_id=$1 AND store_id=$2
		RETURNING version,kind,source_version,published_at,published_by::text`,
		s.TenantID, s.StoreID, document, kind, source, s.PrincipalID).Scan(&out.Version, &out.Kind, &out.SourceVersion, &out.PublishedAt, &out.PublishedBy)
	return out, mapError(err)
}

// Publish copies the draft the merchant saw into a new published version. A stale expected_draft_version, or a draft
// that is already the latest published version, is ErrConflict (so a double click never creates two versions).
func Publish(ctx context.Context, tx pgx.Tx, s platform.Scope, in PublishInput) (VersionInfo, error) {
	var out VersionInfo
	if !validScope(tx, s) || in.ExpectedDraftVersion == nil || *in.ExpectedDraftVersion < 1 {
		return out, command.ErrInvalid
	}
	version, document, err := lockDraft(ctx, tx, s)
	if err != nil {
		return out, err
	}
	if version != *in.ExpectedDraftVersion {
		return out, command.ErrConflict
	}
	var latestKind *string
	var latestSource *int64
	// design.published_versions: the newest row tells whether this very draft version is already live.
	err = tx.QueryRow(ctx, `SELECT kind,source_version FROM design.published_versions WHERE tenant_id=$1 AND store_id=$2 ORDER BY version DESC LIMIT 1`,
		s.TenantID, s.StoreID).Scan(&latestKind, &latestSource)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, mapError(err)
	}
	if latestKind != nil && *latestKind == "publish" && *latestSource == version {
		return out, command.ErrConflict
	}
	// The stored draft already passed Normalize; media may have been removed since only if unreferenced, so re-check anyway.
	if _, refs, nerr := Normalize(document); nerr != nil {
		return out, nerr
	} else if err = checkImages(ctx, tx, s, refs); err != nil {
		return out, err
	}
	if out, err = appendVersion(ctx, tx, s, "publish", version, document); err != nil {
		return out, err
	}
	return out, command.Audit(ctx, tx, s, "design.published")
}

// Rollback publishes a copy of an older published version (history is never mutated; the draft is left as it is).
// Rolling back to the version that is already live is ErrConflict, an unknown version ErrNotFound.
func Rollback(ctx context.Context, tx pgx.Tx, s platform.Scope, in RollbackInput) (VersionInfo, error) {
	var out VersionInfo
	if !validScope(tx, s) || in.Version == nil || *in.Version < 1 {
		return out, command.ErrInvalid
	}
	if _, _, err := lockDraft(ctx, tx, s); err != nil {
		return out, err
	}
	var document []byte
	var live int64
	// design.published_versions: the target row, plus the live (max) version to refuse a no-op rollback.
	err := tx.QueryRow(ctx, `SELECT v.document,(SELECT max(version) FROM design.published_versions WHERE tenant_id=$1 AND store_id=$2)
		FROM design.published_versions v WHERE v.tenant_id=$1 AND v.store_id=$2 AND v.version=$3`, s.TenantID, s.StoreID, *in.Version).Scan(&document, &live)
	if err != nil {
		return out, mapError(err)
	}
	if live == *in.Version {
		return out, command.ErrConflict
	}
	if _, refs, nerr := Normalize(document); nerr != nil {
		return out, nerr
	} else if err = checkImages(ctx, tx, s, refs); err != nil {
		// An image of the old version was deleted since: rolling back would publish a broken page.
		var invalid *ValidationError
		if errors.As(err, &invalid) {
			return out, command.ErrConflict
		}
		return out, err
	}
	if out, err = appendVersion(ctx, tx, s, "rollback", *in.Version, document); err != nil {
		return out, err
	}
	return out, command.Audit(ctx, tx, s, "design.rolled_back")
}

// ListVersions returns the newest published versions (metadata) and which one is live.
func ListVersions(ctx context.Context, tx pgx.Tx, s platform.Scope) (VersionList, error) {
	list := VersionList{Items: []VersionInfo{}}
	if !validScope(tx, s) {
		return list, command.ErrInvalid
	}
	// design.published_versions: metadata projection, never the document column.
	rows, err := tx.Query(ctx, `SELECT version,kind,source_version,published_at,published_by::text FROM design.published_versions
		WHERE tenant_id=$1 AND store_id=$2 ORDER BY version DESC LIMIT $3`, s.TenantID, s.StoreID, maxVersionsListed)
	if err != nil {
		return list, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v VersionInfo
		if err := rows.Scan(&v.Version, &v.Kind, &v.SourceVersion, &v.PublishedAt, &v.PublishedBy); err != nil {
			return list, err
		}
		list.Items = append(list.Items, v)
	}
	if err := rows.Err(); err != nil {
		return list, mapError(err)
	}
	if len(list.Items) > 0 {
		list.LiveVersion = &list.Items[0].Version
	}
	return list, nil
}
