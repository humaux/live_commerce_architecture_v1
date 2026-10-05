// Purpose: live-console-v1 Amendment 1 A1.2 — the API-side comment-facts lookup (created_at / is_page /
// is_reply of ONE comment) and the facts_unavailable mark. Order: (a) bridge comment-facts (ring buffer, else
// one platform-branched Graph read in claims-worker), (b) only for an IG source and only after (a) answered
// found:false or was unreachable: the encrypted IG webhook copy via social.read_comment_facts, decrypted HERE
// with the payload keyring (claims-worker never holds it), (c) neither → manual private reply is disabled for
// that comment. The author id (from.id) is collapsed to is_page and never leaves memory.
// Depends on: stream.go (resolveSource/marks/bridgeError), metabridge.BridgeClient.CommentFacts,
//
//	meta.PayloadKeyring.OpenComment, SQL social.read_comment_facts (0123).
//
// Used by: the LC-B4 manual private-reply planner (A4) and the LC-B3/B4 inbox comment marks (A8/A13).
// Invariants: I11 (no comment text/author persisted), I01 (scope from the authenticated transaction).
// Status: MOCK (IG Graph field set pending probe R3 / LC-U12; the webhook copy is real stored data).
package live

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/metabridge"
	"livecommerce/internal/platform"
)

// ErrFactsUnavailable means neither the Graph read nor the IG webhook copy can confirm a comment's time and
// author (A1.2 1c): A4 answers 409 comment_facts_unavailable and the marks carry reason facts_unavailable.
var ErrFactsUnavailable = errors.New("live console comment facts unavailable")

// FactsUnavailableReason is the §2.5 private_reply_unavailable_reason for ErrFactsUnavailable.
const FactsUnavailableReason = "facts_unavailable"

// CommentFacts are the three facts the manual private-reply rules need (§3.3). Found=false means the
// source's platform does not know the comment (A4: comment_unknown); IG without any source is
// ErrFactsUnavailable instead, never a guessed timestamp.
type CommentFacts struct {
	Found     bool
	CreatedAt time.Time
	IsPage    bool
	IsReply   bool
}

// WithFactsUnavailable disables manual private reply for a comment whose facts cannot be confirmed. An
// existing used/auto_pending reason wins (same precedence as live.console_marks).
func (m ConsoleMarks) WithFactsUnavailable() ConsoleMarks {
	if m.PrivateReplyAvailable {
		m.PrivateReplyAvailable = false
		m.PrivateReplyUnavailableReason = FactsUnavailableReason
	}
	return m
}

// Facts looks one comment up (live:read re-checked here and by the SQL definer). Side effects: one bridge
// call (which may issue one Graph read) and, for IG, one read-only social.read_comment_facts call.
func (cs *CommentStream) Facts(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID, commentRef string) (CommentFacts, error) {
	if !command.ValidID(sessionID) || !consoleRef.MatchString(commentRef) {
		return CommentFacts{}, ErrInvalidRef
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return CommentFacts{}, err
	}
	src, err := cs.resolveSource(ctx, tx, scope, sessionID)
	if err != nil {
		return CommentFacts{}, err
	}
	return cs.facts(ctx, tx, scope, sessionID, src, commentRef)
}

// PrivateReplyMarks is the marks of one comment with the facts check folded in: the §2.5 join, and when
// private reply would otherwise be available but the facts cannot be confirmed, reason facts_unavailable.
func (cs *CommentStream) PrivateReplyMarks(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID, commentRef string) (ConsoleMarks, error) {
	if !command.ValidID(sessionID) || !consoleRef.MatchString(commentRef) {
		return ConsoleMarks{}, ErrInvalidRef
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return ConsoleMarks{}, err
	}
	src, err := cs.resolveSource(ctx, tx, scope, sessionID)
	if err != nil {
		return ConsoleMarks{}, err
	}
	marks, err := cs.marks(ctx, tx, scope, token, sessionID, []string{commentRef})
	if err != nil {
		return ConsoleMarks{}, err
	}
	m, ok := marks[commentRef]
	if !ok {
		return ConsoleMarks{}, ErrNoSource
	}
	if !m.PrivateReplyAvailable {
		return m, nil
	}
	if _, err := cs.facts(ctx, tx, scope, sessionID, src, commentRef); errors.Is(err, ErrFactsUnavailable) {
		return m.WithFactsUnavailable(), nil
	} else if err != nil {
		return ConsoleMarks{}, err
	}
	return m, nil
}

func (cs *CommentStream) facts(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string, src consoleSource, ref string) (CommentFacts, error) {
	// (a) Calls the claims-worker bridge comment-facts (live-console-v1 §2.3): ring buffer, else one Graph read.
	bf, bridgeErr := cs.bridge.CommentFacts(ctx, metabridge.BridgeFactsRequest{
		TenantID: scope.TenantID, StoreID: scope.StoreID, SessionID: sessionID, SourceID: src.ID, CommentRef: ref})
	if bridgeErr == nil && bf.Found && bf.CreatedAt != nil {
		return CommentFacts{Found: true, CreatedAt: *bf.CreatedAt, IsPage: bf.IsPage, IsReply: bf.IsReply}, nil
	}
	if src.Object != "instagram" {
		if bridgeErr != nil {
			return CommentFacts{}, bridgeError(bridgeErr)
		}
		return CommentFacts{Found: false}, nil
	}
	// (b) IG only: the encrypted webhook copy. A nil keyring (api without the payload keys) skips it.
	if cs.payload != nil {
		f, ok, err := cs.igWebhookFacts(ctx, tx, scope, sessionID, src, ref)
		if err != nil {
			return CommentFacts{}, err
		}
		if ok {
			return f, nil
		}
	}
	// A transient bridge failure is retryable (503); a clean "nobody has it" is (c): no guess, no default.
	if bridgeErr != nil && !errors.Is(bridgeErr, metabridge.ErrBridgeNotFound) {
		return CommentFacts{}, bridgeError(bridgeErr)
	}
	return CommentFacts{}, ErrFactsUnavailable
}

// igWebhookFacts reads the newest stored webhook copy of one IG comment and derives the facts from it
// (created_at = the copy's occurred_at, delivery time for IG — known limit U7). A copy whose media id is not
// the session source's, or that cannot be opened, is "not found" rather than an error.
func (cs *CommentStream) igWebhookFacts(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string, src consoleSource, ref string) (CommentFacts, bool, error) {
	var env meta.CommentEnvelope
	// Calls social.read_comment_facts (Amendment 1 A1.2 1b); envelope + AAD only, zero rows when none.
	err := tx.QueryRow(ctx, `SELECT event_id::text,kind,occurred_at,received_at,key_id,nonce,ciphertext,
		app_id,object,asset_id,event_key,payload_hash,route_id::text,route_epoch
		FROM social.read_comment_facts($1::uuid,$2::text)`, sessionID, ref).
		Scan(&env.EventID, &env.Kind, &env.OccurredAt, &env.ReceivedAt, &env.KeyID, &env.Nonce, &env.Ciphertext,
			&env.AppID, &env.Object, &env.AssetID, &env.EventKey, &env.PayloadHash, &env.RouteID, &env.RouteEpoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return CommentFacts{}, false, nil
	}
	if err != nil {
		return CommentFacts{}, false, mapReadError(err)
	}
	cr, err := cs.payload.OpenComment(scope.TenantID, scope.StoreID, env)
	if err != nil || cr.Ref != ref || cr.MediaID != src.SourceObjectID {
		return CommentFacts{}, false, nil
	}
	return CommentFacts{Found: true, CreatedAt: cr.CreatedAt, IsPage: cr.IsPage, IsReply: cr.ParentRef != ""}, true, nil
}
