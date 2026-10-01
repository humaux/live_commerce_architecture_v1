package design

// preview.go issues the short-lived preview token of storefront-v2 section B: "the storefront renders the draft when
// ?preview=<token> is present". The token is 32 random bytes (base64url, the buyer capability pattern of
// internal/buyer: random, stored only as SHA-256), valid 15 minutes, bound to this store and to the draft version at
// issue time, and grants nothing but design.buyer_preview (the draft read). Verification is in SQL (migration 0087), not
// here: the buyer pool never needs a signing key, and a token can be revoked by deleting its row.
// Deviation from the unit brief ("HMAC with an existing server secret"): no key distribution to cmd/api is needed.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// maxLiveTokens bounds unexpired tokens per store; issuing another revokes the oldest.
const maxLiveTokens = 20

var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// PreviewToken is the one-time answer of POST design/preview-token. The token is never retrievable again.
type PreviewToken struct {
	Token        string    `json:"token"`
	DraftVersion int64     `json:"draft_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// TokenHash returns the SHA-256 the database stores, or false when token is not a well-formed preview token.
func TokenHash(token string) ([]byte, bool) {
	if !tokenPattern.MatchString(token) {
		return nil, false
	}
	sum := sha256.Sum256([]byte(token))
	return sum[:], true
}

// IssuePreviewToken binds a new token to the store's current draft version. ErrNotFound when no draft was saved yet.
func IssuePreviewToken(ctx context.Context, tx pgx.Tx, s platform.Scope) (PreviewToken, error) {
	var out PreviewToken
	if !validScope(tx, s) {
		return out, command.ErrInvalid
	}
	// design.documents: the row lock orders issue against a concurrent draft save, so the bound version is the current one.
	version, _, err := lockDraft(ctx, tx, s)
	if err != nil {
		return out, err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return out, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash, _ := TokenHash(token)
	// design.preview_tokens (commerce_runtime): drop expired rows, and the oldest live ones beyond the per-store cap.
	if _, err = tx.Exec(ctx, `DELETE FROM design.preview_tokens WHERE tenant_id=$1 AND store_id=$2 AND (expires_at<=clock_timestamp() OR token_hash IN
		(SELECT token_hash FROM design.preview_tokens WHERE tenant_id=$1 AND store_id=$2 ORDER BY created_at DESC OFFSET $3))`,
		s.TenantID, s.StoreID, maxLiveTokens-1); err != nil {
		return out, mapError(err)
	}
	err = tx.QueryRow(ctx, `INSERT INTO design.preview_tokens(tenant_id,store_id,token_hash,draft_version,created_by) VALUES($1,$2,$3,$4,$5) RETURNING expires_at`,
		s.TenantID, s.StoreID, hash, version, s.PrincipalID).Scan(&out.ExpiresAt)
	out.Token, out.DraftVersion = token, version
	return out, mapError(err)
}
