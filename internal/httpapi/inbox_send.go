// Purpose: the manual-send HTTP adapter of live-console-v1 §11: A12 DM send, A4 manual private reply, A5 public reply and A6 recommend
// (with the optional recommend comment). It decodes strict bodies, resolves the comment facts through live.CommentStream for A4, runs the
// inbox.Service planners in one scoped merchant transaction and maps every refusal to the fixed transport codes. It decides no rule (the
// planner definers do), never returns a driver message and never logs a body or token.
// Depends on: internal/inbox (send planners, SendError), internal/live (CommentStream.Facts, RecordOfferFeatured), internal/command,
// internal/platform (WithScope via scopedAs).
// Used by: internal/httpapi/handler.go (registerInboxSendRoutes, gated on Options.Inbox with the send side enabled); cmd/api wires it.
// Invariants: LCN03 (cross-tenant/store → 404 by the scoped definers), LCN06/07/08 (deny codes pass through verbatim from the planners).

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/command"
	"livecommerce/internal/inbox"
	"livecommerce/internal/live"
	"livecommerce/internal/platform"
)

// sendDenyCodes are the planner deny codes (SQLSTATE PT409, message = code) that pass through to the transport verbatim.
var sendDenyCodes = map[string]bool{
	"window_closed": true, "takeover_changed": true, "capability": true, "conversation_gone": true, "duplicate_recent": true,
	"used": true, "auto_pending": true, "auto_pending_confirm": true, "expired_7d": true, "ig_live_ended": true,
	"offer_unavailable": true, "version_conflict": true, "ig_live_unsupported": true,
	"human_takeover": true, "already_reminded": true, "not_remindable": true, // W3-03B reminder planner (0131)
}

// registerInboxSendRoutes mounts A12 (conversation DM), A4/A5 (comment replies; need the comment stream for the facts lookup) and A6.
// Nothing is mounted unless the inbox service has the send side enabled.
func registerInboxSendRoutes(mux *http.ServeMux, pool *pgxpool.Pool, svc *inbox.Service, cs *live.CommentStream) {
	if svc == nil || !svc.SendEnabled() {
		return
	}
	const inboxBase = "/v1/admin/stores/{store_id}/inbox/conversations"
	const liveBase = "/v1/admin/stores/{store_id}/live-sessions"

	// A12: DM RESPONSE in the 24 h window; implicit takeover (§3.6). expected_generation is required.
	mux.HandleFunc("POST "+inboxBase+"/{conversation_id}/messages", inboxRoute(http.MethodPost, true, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[inbox.DMInput](w, r, []string{"expected_generation"}, []string{"text", "template_id"}, "template_version")
		if !ok {
			return
		}
		inboxSendScoped(pool, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.SendDM(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("conversation_id"), in, nil)
		})(w, r)
	}))

	if cs != nil {
		// A4: manual private reply. The comment facts (found / is_page / is_reply / created_at) come from the bridge or the IG webhook copy.
		mux.HandleFunc("POST "+liveBase+"/{session_id}/comments/{comment_ref}/private-reply", claimsRoute(http.MethodPost, false, func(w http.ResponseWriter, r *http.Request) {
			in, ok := claimsBody[inbox.ReplyInput](w, r, nil, []string{"text", "template_id"}, "template_version", "confirm_preempt_auto")
			if !ok {
				return
			}
			inboxSendScoped(pool, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
				session, ref := r.PathValue("session_id"), r.PathValue("comment_ref")
				facts := privateReplyFacts(cs, tx, s, bearerToken(r), session, ref)
				return svc.SendPrivateReply(ctx, tx, s, r.Header.Get("Idempotency-Key"), session, ref, in, facts)
			})(w, r)
		}))
	}

	// A5: public comment reply (§3.5 content rule runs in the service before anything is sealed).
	mux.HandleFunc("POST "+liveBase+"/{session_id}/comments/{comment_ref}/public-reply", claimsRoute(http.MethodPost, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[inbox.TextInput](w, r, nil, []string{"text", "template_id"}, "template_version")
		if !ok {
			return
		}
		inboxSendScoped(pool, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.SendPublicReply(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), r.PathValue("comment_ref"), in)
		})(w, r)
	}))

	// A6: recommend an offer (live:manage; post_comment additionally needs inbox:reply, checked by the planner). Records the
	// 'featured' timeline event and, when asked, plans the FB recommend comment, in one idempotent command.
	mux.HandleFunc("POST "+liveBase+"/{session_id}/claims/offers/{offer_id}/recommend", claimsRoute(http.MethodPost, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[recommendInput](w, r, []string{"expected_version"}, nil, "post_comment")
		if !ok {
			return
		}
		inboxSendScoped(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return recommendOffer(ctx, tx, s, svc, r, in)
		})(w, r)
	}))

	// Methodless fallbacks keep wrong-method answers inside the same private response boundary.
	for _, path := range []string{liveBase + "/{session_id}/comments/{comment_ref}/private-reply", liveBase + "/{session_id}/comments/{comment_ref}/public-reply",
		liveBase + "/{session_id}/claims/offers/{offer_id}/recommend"} {
		mux.HandleFunc(path, studioRoute("", false, nil))
	}
}

// commentFacts is the part of live.CommentStream the A4 handler needs (a seam for the DB-free mapping test).
type commentFacts interface {
	Facts(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID, commentRef string) (live.CommentFacts, error)
}

// privateReplyFacts returns the comment-facts lookup of A4 (live-console-v1 §3.3 row 2, Amendment 1 A1.2): the comment's created_at when it
// can receive a manual private reply, else the fixed refusal — comment_facts_unavailable (neither the bridge nor the IG webhook copy can confirm
// time and author), comment_unknown (not a comment of this session's source), page_comment (the Page's own), reply_comment_unsupported (a reply
// to a comment). Other lookup errors (bridge down, no source) pass through to the live-stream classifier.
func privateReplyFacts(f commentFacts, tx pgx.Tx, s platform.Scope, token, session, ref string) func(context.Context) (time.Time, error) {
	return func(ctx context.Context) (time.Time, error) {
		facts, err := f.Facts(ctx, tx, s, token, session, ref)
		switch {
		case errors.Is(err, live.ErrFactsUnavailable):
			return time.Time{}, &inbox.SendError{Status: http.StatusConflict, Code: "comment_facts_unavailable"}
		case err != nil:
			return time.Time{}, err
		case !facts.Found:
			return time.Time{}, &inbox.SendError{Status: http.StatusConflict, Code: "comment_unknown"}
		case facts.IsPage:
			return time.Time{}, &inbox.SendError{Status: http.StatusConflict, Code: "page_comment"}
		case facts.IsReply:
			return time.Time{}, &inbox.SendError{Status: http.StatusConflict, Code: "reply_comment_unsupported"}
		}
		return facts.CreatedAt, nil
	}
}

// recommendInput is A6 `{expected_version, post_comment?}`.
type recommendInput struct {
	ExpectedVersion int64 `json:"expected_version"`
	PostComment     bool  `json:"post_comment"`
}

// recommendOutput is A6 `{recommended_at, operation_id?}`.
type recommendOutput struct {
	RecommendedAt time.Time `json:"recommended_at"`
	OperationID   string    `json:"operation_id,omitempty"`
}

// recommendOffer runs A6 in one command: CAS on the offer version, the featured timeline event (§7.3), and with post_comment the
// recommend comment planner. A refused plan rolls the timeline event back with it.
func recommendOffer(ctx context.Context, tx pgx.Tx, s platform.Scope, svc *inbox.Service, r *http.Request, in recommendInput) (any, error) {
	session, offer := r.PathValue("session_id"), r.PathValue("offer_id")
	if in.ExpectedVersion < 1 {
		return nil, command.ErrInvalid
	}
	var out recommendOutput
	request := struct {
		Session string `json:"session_id"`
		Offer   string `json:"offer_id"`
		Version int64  `json:"expected_version"`
		Post    bool   `json:"post_comment"`
	}{session, offer, in.ExpectedVersion, in.PostComment}
	err := command.Run(ctx, tx, s, "live.offer.recommend", r.Header.Get("Idempotency-Key"), request, &out, func() error {
		var version int64
		if err := tx.QueryRow(ctx, `SELECT version FROM live.offers WHERE id=$1::uuid AND session_id=$2::uuid`, offer, session).Scan(&version); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return command.ErrNotFound
			}
			return err
		}
		if version != in.ExpectedVersion {
			return &inbox.SendError{Status: http.StatusConflict, Code: "version_conflict"}
		}
		at, err := live.RecordOfferFeatured(ctx, tx, s, bearerToken(r), session, offer)
		if err != nil {
			return err
		}
		out = recommendOutput{RecommendedAt: at}
		if in.PostComment {
			if out.OperationID, err = svc.PlanRecommend(ctx, tx, s, session, offer, in.ExpectedVersion); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

func inboxSendScoped(pool *pgxpool.Pool, permission string, fn action) http.HandlerFunc {
	return scopedAs(pool, permission, inboxSendClassify, fn)
}

// inboxSendClassify maps send refusals to fixed transport codes: SendError as declared, planner deny codes (PT409 message) verbatim when
// listed, PT422 ig_live_unsupported, PT429 rate_limited; everything else falls back to the live-stream/claims tables (never a driver
// message, unknown errors are 503 unavailable).
func inboxSendClassify(err error) (int, string) {
	var se *inbox.SendError
	if errors.As(err, &se) {
		return se.Status, se.Code
	}
	if errors.Is(err, inbox.ErrSendUnavailable) {
		return http.StatusServiceUnavailable, "unavailable"
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT403":
			return http.StatusForbidden, "forbidden"
		case "PT404":
			return http.StatusNotFound, "not_found"
		case "PT409":
			if sendDenyCodes[pg.Message] {
				return http.StatusConflict, pg.Message
			}
			return http.StatusConflict, "conflict"
		case "PT422":
			if pg.Message == "ig_live_unsupported" {
				return http.StatusUnprocessableEntity, "ig_live_unsupported"
			}
			return http.StatusUnprocessableEntity, "invalid_request"
		case "PT429":
			return http.StatusTooManyRequests, "rate_limited"
		}
	}
	if inbox.IsNotFound(err) {
		return http.StatusNotFound, "not_found"
	}
	return liveStreamClassify(err)
}
