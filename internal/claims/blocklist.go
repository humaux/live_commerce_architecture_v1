// Purpose: the merchant side of the restricted-buyer list (W3-05B): list a social actor as restricted for one store, remove an entry, page the list and
// answer "is this bundle's actor restricted". The effect on a claim (no link, no automatic reply) is decided in SQL (integration.claim_reply_plannable).
// Depends on: SQL claims.block_actor / unblock_actor / list_blocked_actors / actor_restricted_for_bundle (migration 0154; live:manage / live:read re-checked by the
//   definers), command (receipt + audit), pagination ("claim-blocklist" cursors), platform (scope).
// Used by: internal/httpapi/blocklist.go and tests/foundation/blocklist_test.go.
// Invariants: the client never supplies an actor_key (the definer resolves a server-known reference inside the caller's store); the note is merchant-only: it
//   is never in a receipt, an audit row, an error or a log; audit actions are claims.actor_blocked / claims.actor_unblocked.

package claims

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// Blocklist errors; everything else maps through mapError (422 invalid, 404 not found, 403 forbidden).
var (
	// ErrBlocklistFull is the 5000-entries-per-store bound (I23).
	ErrBlocklistFull = errors.New("blocklist is full")
	// ErrAmbiguousActor means a conversation maps to several actors, so the merchant must choose a bundle.
	ErrAmbiguousActor = errors.New("conversation maps to several actors")
)

// BlockInput names the actor to restrict by exactly one server-known reference. Note is the optional merchant-only reason (<= 200 characters).
type BlockInput struct {
	CommentRef     string `json:"comment_ref,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	BundleID       string `json:"bundle_id,omitempty"`
	Note           string `json:"note,omitempty"`
}

// BlockResult is the answer to a block: the entry id (the handle of a later unblock), never the actor key or the note.
type BlockResult struct {
	ID        string    `json:"id"`
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"created_at"`
	Created   bool      `json:"created"`
}

// UnblockResult is the answer to an unblock (Removed is true: an unknown id is a 404, not a false).
type UnblockResult struct {
	Removed bool `json:"removed"`
}

// BlockedActor is one list row. Note is the merchant-only reason; SourceBundleID is the bundle the actor was identified by (informational, may be nil).
type BlockedActor struct {
	ID             string    `json:"id"`
	Platform       string    `json:"platform"`
	Note           *string   `json:"note"`
	SourceBundleID *string   `json:"source_bundle_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// blocklistError separates the two codes mapError folds into ErrConflict (both PT409), then applies the shared mapping.
func blocklistError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "PT409" {
		switch pgErr.Message {
		case "limit_reached":
			return ErrBlocklistFull
		case "ambiguous_actor":
			return ErrAmbiguousActor
		}
	}
	return mapError(err)
}

func (in BlockInput) ref() (map[string]string, error) {
	out := map[string]string{}
	for k, v := range map[string]string{"comment_ref": in.CommentRef, "conversation_id": in.ConversationID, "bundle_id": in.BundleID} {
		if v != "" {
			out[k] = v
		}
	}
	if len(out) != 1 || len(in.Note) > 800 { // 200 characters is at most 800 bytes; the definer checks the character count
		return nil, command.ErrInvalid
	}
	return out, nil
}

// BlockActor restricts the actor named by in for the caller's store (live:manage). sessionID scopes the comment_ref lookup. It writes one blocklist row and one
// audit row claims.actor_blocked (only when the entry is new; re-blocking a listed actor is a no-op that keeps the first note). tx must be a platform.WithScope transaction.
func BlockActor(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, sessionID string, in BlockInput) (BlockResult, error) {
	ref, err := in.ref()
	if err != nil || !command.ValidID(sessionID) {
		return BlockResult{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return BlockResult{}, err
	}
	digest := sha256.Sum256([]byte(in.Note)) // the note joins the receipt request only as a digest
	request := struct {
		SessionID  string            `json:"session_id"`
		Ref        map[string]string `json:"ref"`
		NoteDigest string            `json:"note_digest"`
	}{sessionID, ref, hex.EncodeToString(digest[:])}
	var out BlockResult
	err = command.Run(ctx, tx, scope, "claims.actor.block", key, request, &out, func() error {
		raw, err := json.Marshal(ref)
		if err != nil {
			return command.ErrInvalid
		}
		var note any
		if in.Note != "" {
			note = in.Note
		}
		// claims.block_actor: definer commerce_claims_writer (migration 0154); resolves the actor_key from the reference inside this store.
		if err := tx.QueryRow(ctx, `SELECT id::text, platform, created_at, created FROM claims.block_actor($1::uuid, $2::jsonb, $3::text)`,
			sessionID, string(raw), note).Scan(&out.ID, &out.Platform, &out.CreatedAt, &out.Created); err != nil {
			return blocklistError(err)
		}
		if !out.Created {
			return nil
		}
		return command.Audit(ctx, tx, scope, "claims.actor_blocked") // no note, no actor key
	})
	return out, err
}

// UnblockActor removes one entry of the caller's store by id (live:manage); an unknown id is command.ErrNotFound. Writes one audit row claims.actor_unblocked.
func UnblockActor(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, entryID string) (UnblockResult, error) {
	if !command.ValidID(entryID) {
		return UnblockResult{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return UnblockResult{}, err
	}
	var out UnblockResult
	err := command.Run(ctx, tx, scope, "claims.actor.unblock", key, map[string]string{"entry_id": entryID}, &out, func() error {
		// claims.unblock_actor: definer commerce_claims_writer (migration 0154).
		if err := tx.QueryRow(ctx, `SELECT claims.unblock_actor($1::uuid)`, entryID).Scan(&out.Removed); err != nil {
			return blocklistError(err)
		}
		return command.Audit(ctx, tx, scope, "claims.actor_unblocked")
	})
	return out, err
}

// ListBlockedActors pages the store's entries newest first (live:read). Read only.
func ListBlockedActors(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, page pagination.Request) (pagination.Page[BlockedActor], error) {
	empty := pagination.Page[BlockedActor]{Items: []BlockedActor{}}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return empty, err
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "claim-blocklist"}
	limit, keys, err := pagination.Decode(page, binding, 2)
	if err != nil {
		return empty, err
	}
	var afterTime, afterID any
	if len(keys) == 2 {
		afterTime, afterID = keys[0], keys[1]
	}
	// claims.list_blocked_actors: definer commerce_claims_writer (migration 0154); one extra row tells whether a next page exists.
	rows, err := tx.Query(ctx, `SELECT id::text, platform, note, source_bundle_id::text, created_at
		FROM claims.list_blocked_actors($1::timestamptz, $2::uuid, $3::int)`, afterTime, afterID, limit+1)
	if err != nil {
		return empty, blocklistError(err)
	}
	defer rows.Close()
	out := pagination.Page[BlockedActor]{Items: []BlockedActor{}}
	for rows.Next() {
		var b BlockedActor
		if err := rows.Scan(&b.ID, &b.Platform, &b.Note, &b.SourceBundleID, &b.CreatedAt); err != nil {
			return empty, mapError(err)
		}
		out.Items = append(out.Items, b)
	}
	if err := rows.Err(); err != nil {
		return empty, blocklistError(err)
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		last := out.Items[len(out.Items)-1]
		if out.NextCursor, err = pagination.Encode(binding, []string{last.CreatedAt.UTC().Format(cursorTime), last.ID}); err != nil {
			return empty, err
		}
	}
	return out, nil
}

// BlockedForBundle reports whether the bundle's actor is restricted (live:read); an unknown or purged bundle is command.ErrNotFound. Boolean only: the note never leaves.
func BlockedForBundle(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, bundleID string) (bool, error) {
	if !command.ValidID(bundleID) {
		return false, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return false, err
	}
	var restricted bool
	// claims.actor_restricted_for_bundle: definer commerce_claims_writer (migration 0154).
	err := tx.QueryRow(ctx, `SELECT claims.actor_restricted_for_bundle($1::uuid)`, bundleID).Scan(&restricted)
	return restricted, blocklistError(err)
}
