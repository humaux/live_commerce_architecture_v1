// Purpose: live-console-v1 §2.3 — the internal API↔claims-worker bridge. The worker side is the
// Console HTTP handler (POST /internal/v1/comment-page and /comment-facts) served only on the backend
// Docker network with a shared 32-byte bearer token; the API side is BridgeClient. Comment text and
// names exist only inside these responses and are never logged or persisted (the bodies are never
// logged, and every error is a fixed safe code).
// Depends on: comment_poll.go (Console), live.console_source (0123), metaoauth.Graph, core.Secret.
//
//	internal/integrations/metabridge (API-side client + wire types, re-exported below).
//
// Used by: cmd/claims-worker (Console.Handler); the API reaches the bridge through internal/integrations/metabridge, not this package.
package metareply

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"livecommerce/internal/integrations/metabridge"
)

// Wire types, errors and the API-side BridgeClient live in internal/integrations/metabridge (cmd/api must not link this package: it
// contains the Page-token opener). They are re-exported here so the worker-side Console and its tests keep one definition.
type (
	Cursor             = metabridge.Cursor
	BridgeComment      = metabridge.BridgeComment
	BridgeStreamState  = metabridge.BridgeStreamState
	BridgePageRequest  = metabridge.BridgePageRequest
	BridgePage         = metabridge.BridgePage
	BridgeFactsRequest = metabridge.BridgeFactsRequest
	CommentFacts       = metabridge.CommentFacts
	BridgeClient       = metabridge.BridgeClient
)

var (
	ErrBridgeNotFound    = metabridge.ErrBridgeNotFound
	ErrBridgeNotOwner    = metabridge.ErrBridgeNotOwner
	ErrBridgeInvalidCur  = metabridge.ErrBridgeInvalidCur
	ErrBridgeUnavailable = metabridge.ErrBridgeUnavailable

	// NewBridgeClient builds the API-side client (re-export for worker-side tests; cmd/api imports metabridge directly).
	NewBridgeClient = metabridge.NewBridgeClient

	commentRefShape = metabridge.CommentRefShape
	// Go regexp caps repeat counts at 1000; the 1024 bound is enforced in openCursor by len().
	validCursorB64 = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,1000}$`)
	validGraphCur  = regexp.MustCompile(`^[A-Za-z0-9_=+./-]{1,512}$`)
)

// sealCursor builds older_cursor = base64url(JSON{tenant,store,session,source,graph_cursor,exp}‖HMAC) (§2.4).
func (c *Console) sealCursor(tenant, store, session, source, graphCursor string, exp time.Time) string {
	raw, _ := json.Marshal([]string{tenant, store, session, source, graphCursor, exp.UTC().Format(time.RFC3339Nano)})
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, c.cursorKey)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// openCursor verifies and decodes older_cursor; any scope/format/tamper/expiry problem is ErrBridgeInvalidCur.
func (c *Console) openCursor(cursor, tenant, store, session, source string, now time.Time) (graphCursor string, err error) {
	if len(cursor) > 1024 || !validCursorB64.MatchString(cursor) {
		return "", ErrBridgeInvalidCur
	}
	payload, sig, ok := strings.Cut(cursor, ".")
	if !ok {
		return "", ErrBridgeInvalidCur
	}
	mac := hmac.New(sha256.New, c.cursorKey)
	mac.Write([]byte(payload))
	want := mac.Sum(nil)
	got, decErr := base64.RawURLEncoding.DecodeString(sig)
	if decErr != nil || !hmac.Equal(got, want) {
		return "", ErrBridgeInvalidCur
	}
	raw, decErr := base64.RawURLEncoding.DecodeString(payload)
	if decErr != nil {
		return "", ErrBridgeInvalidCur
	}
	var parts []string
	if json.Unmarshal(raw, &parts) != nil || len(parts) != 6 ||
		parts[0] != tenant || parts[1] != store || parts[2] != session || parts[3] != source {
		return "", ErrBridgeInvalidCur
	}
	exp, err := time.Parse(time.RFC3339Nano, parts[5])
	if err != nil || !now.Before(exp) || !validGraphCur.MatchString(parts[4]) {
		return "", ErrBridgeInvalidCur
	}
	return parts[4], nil
}

// constantTokenEqual is the §2.3 constant-time bearer compare.
func constantTokenEqual(a, b []byte) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1
}
