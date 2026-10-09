// Purpose: live-console-v1 §2.6 / §2.5 / §7.4 — the console comment read-through service (unit LC-B2,
// API side). It resolves the session's one active claim source, then reads comments either from the
// claims-worker bridge (Facebook: process-memory ring buffer, no text in PG) or from the encrypted IG
// webhook copy via social.read_comment_events (Instagram: decrypted here with the payload keyring), and
// joins each comment to its claims/prints/replies facts through live.console_marks. Comment text and
// names exist only in these in-memory values and the HTTP response; nothing here persists or logs them.
// Depends on: draft.go (authorize/mapError/mapReadError), command.Run (ops.command_results receipts),
//
//	metabridge.BridgeClient (client.go), meta.PayloadKeyring.OpenComment (comment_read.go),
//	live.console_source/read_comment_events/console_marks/comment_print (0123).
//
// Used by: internal/httpapi/live_stream.go (routes A2/A3); cmd/api (NewCommentStream).
package live

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/metabridge"
	"livecommerce/internal/platform"
)

// Console error sentinels (mapped by httpapi.liveStreamClassify to fixed safe codes).
var (
	ErrNoSource          = errors.New("live console source unavailable")  // 409 no_source
	ErrStreamUnavailable = errors.New("live console stream unavailable")  // 503 stream_unavailable
	ErrInvalidCursor     = errors.New("live console invalid cursor")      // 400 invalid_cursor
	ErrInvalidRef        = errors.New("live console invalid comment ref") // 422 invalid_ref
)

var consoleRef = regexp.MustCompile(`^[0-9_]{1,80}$`)

// CommentStream reads console comments. bridge serves Facebook sources; payload (optional) decrypts the
// IG webhook fallback — a nil payload makes the IG path answer stream_unavailable (ig_fallback_unavailable).
type CommentStream struct {
	bridge  *metabridge.BridgeClient
	payload *meta.PayloadKeyring
}

func NewCommentStream(bridge *metabridge.BridgeClient, payload *meta.PayloadKeyring) (*CommentStream, error) {
	if bridge == nil {
		return nil, command.ErrInvalid
	}
	return &CommentStream{bridge: bridge, payload: payload}, nil
}

// ConsolePageQuery is the A2 query (parsed by httpapi): after_epoch/after_seq page forward, before_cursor
// pages older (Facebook only), limit 1..100 (default 50).
type ConsolePageQuery struct {
	AfterEpoch   *int64
	AfterSeq     *int64
	BeforeCursor *string
	Limit        int
}

// ConsoleStreamPage is the A2 response envelope (§2.6).
type ConsoleStreamPage struct {
	Epoch         int64              `json:"epoch"`
	Reset         bool               `json:"reset"`
	ScanExhausted bool               `json:"scan_exhausted"` // §2.6: false means the producer has no EOF proof.
	Items         []ConsoleComment   `json:"items"`
	Next          ConsoleCursor      `json:"next"`
	OlderCursor   *string            `json:"older_cursor"`
	Stream        ConsoleStreamState `json:"stream"`
}

type ConsoleCursor struct {
	Epoch int64 `json:"epoch"`
	Seq   int64 `json:"seq"`
}

// ConsoleStreamState is §2.4 StreamState; Reason carries the fixed code for non-live states.
type ConsoleStreamState struct {
	State           string     `json:"state"`
	PollIntervalMs  int        `json:"poll_interval_ms"`
	LastOKAt        *time.Time `json:"last_ok_at"`
	LagMs           *int64     `json:"lag_ms"`
	SourcePlatform  string     `json:"source_platform"`
	VideoEmbeddable bool       `json:"video_embeddable"`
	Reason          string     `json:"reason,omitempty"`
}

// ConsoleComment is §2.6's Comment plus marks; text/author_name exist only in memory and this response.
// Seq is the FB buffer or IG webhook-envelope item position; direct Graph history emits null.
type ConsoleComment struct {
	Ref           string       `json:"ref"`
	Seq           *int64       `json:"seq"`
	ParentRef     *string      `json:"parent_ref"`
	CreatedAt     time.Time    `json:"created_at"`
	AuthorName    *string      `json:"author_name"`
	Text          string       `json:"text"`
	IsPage        bool         `json:"is_page"`
	HasAttachment bool         `json:"has_attachment"`
	Marks         ConsoleMarks `json:"marks"`
}

// ConsoleMarks is §2.5's per-ref join result; every field is nullable except public_replies and
// private_reply_available.
type ConsoleMarks struct {
	Intake                        *IntakeMark       `json:"intake"`
	Claim                         *ClaimMark        `json:"claim"`
	PrivateReply                  *PrivateReplyMark `json:"private_reply"`
	PublicReplies                 int64             `json:"public_replies"`
	Printed                       *PrintedMark      `json:"printed"`
	PrivateReplyAvailable         bool              `json:"private_reply_available"`
	PrivateReplyUnavailableReason string            `json:"private_reply_unavailable_reason,omitempty"`
}

type IntakeMark struct {
	State      string  `json:"state"`
	DropReason *string `json:"drop_reason"`
}

type ClaimMark struct {
	Status   string  `json:"status"`
	Reason   *string `json:"reason"`
	OfferID  *string `json:"offer_id"`
	Keyword  *string `json:"keyword"`
	Quantity *int64  `json:"quantity"`
	BundleID *string `json:"bundle_id"`
}

type PrivateReplyMark struct {
	Kind          string  `json:"kind"`
	State         string  `json:"state"`
	BlockedReason *string `json:"blocked_reason"`
}

type PrintedMark struct {
	Count  int64      `json:"count"`
	LastAt *time.Time `json:"last_at"`
}

// CommentPrint is the A3 response (§7.4): the idempotent print fact, no label content.
type CommentPrint struct {
	PrintCount    int64      `json:"print_count"`
	LastPrintedAt *time.Time `json:"last_printed_at"`
}

type consoleSource struct {
	ID, Platform, Object, AssetID, SourceObjectID string
}

// Comments reads one comment page (A2). The scope transaction is the one platform.WithScope opened with
// live:read; the SQL definers re-authenticate live:read through identity.principal_holds.
func (cs *CommentStream) Comments(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string, q ConsolePageQuery) (ConsoleStreamPage, error) {
	if !command.ValidID(sessionID) || q.Limit < 1 || q.Limit > 100 {
		return ConsoleStreamPage{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return ConsoleStreamPage{}, err
	}
	src, err := cs.resolveSource(ctx, tx, scope, sessionID)
	if err != nil {
		return ConsoleStreamPage{}, err
	}
	switch src.Object {
	case "page":
		return cs.facebookPage(ctx, tx, scope, token, sessionID, src, q)
	case "instagram":
		return cs.instagramPage(ctx, tx, scope, token, sessionID, src, q)
	default:
		return ConsoleStreamPage{}, ErrStreamUnavailable
	}
}

// PrintComment records one label print (A3): the browser renders the label from the comment it already
// holds; only the fact is stored (live.comment_print), wrapped in the command.Run receipt so the route is
// idempotent per Idempotency-Key (§7.4): a same-key replay returns the stored {print_count,last_printed_at}
// without a new increment, and the same key with a different canonical request {session_id,comment_ref}
// conflicts. live:manage re-checked by the definer; a foreign/cross-store session id raises 23503 → 404.
func (cs *CommentStream) PrintComment(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, sessionID, commentRef string) (CommentPrint, error) {
	if !command.ValidID(sessionID) || !consoleRef.MatchString(commentRef) {
		return CommentPrint{}, ErrInvalidRef
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return CommentPrint{}, err
	}
	request := struct {
		SessionID  string `json:"session_id"`
		CommentRef string `json:"comment_ref"`
	}{sessionID, commentRef}
	var out CommentPrint
	err := command.Run(ctx, tx, scope, "live.comment.print", key, request, &out, func() error {
		err := tx.QueryRow(ctx, `SELECT print_count,last_printed_at FROM live.comment_print($1::uuid,$2::text,$3::uuid)`,
			sessionID, commentRef, scope.PrincipalID).Scan(&out.PrintCount, &out.LastPrintedAt)
		if err != nil {
			return mapReadError(err)
		}
		return nil
	})
	if err != nil {
		return CommentPrint{}, mapError(err)
	}
	// Run may replay a saved receipt after the advisory-lock wait; revocation still denies it.
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return CommentPrint{}, err
	}
	return out, nil
}

// resolveSource pins the session to its one active claim source; 404 when the session is not in the
// store, 409 no_source when the session has no active source.
func (cs *CommentStream) resolveSource(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string) (consoleSource, error) {
	var one int
	err := tx.QueryRow(ctx, `SELECT 1 FROM live.sessions WHERE tenant_id=$1 AND store_id=$2 AND id=$3`,
		scope.TenantID, scope.StoreID, sessionID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return consoleSource{}, command.ErrNotFound
	}
	if err != nil {
		return consoleSource{}, mapError(err)
	}
	var s consoleSource
	err = tx.QueryRow(ctx, `SELECT id::text,platform,object,asset_id,source_object_id FROM live.claim_sources
		WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND active ORDER BY updated_at DESC,id LIMIT 1`,
		scope.TenantID, scope.StoreID, sessionID).Scan(&s.ID, &s.Platform, &s.Object, &s.AssetID, &s.SourceObjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return consoleSource{}, ErrNoSource
	}
	if err != nil {
		return consoleSource{}, mapError(err)
	}
	return s, nil
}

// facebookPage reads through the bridge and joins the returned refs to marks (both inside the scope tx;
// the bridge call is a short backend-network read and holds no token).
func (cs *CommentStream) facebookPage(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string, src consoleSource, q ConsolePageQuery) (ConsoleStreamPage, error) {
	req := metabridge.BridgePageRequest{
		TenantID: scope.TenantID, StoreID: scope.StoreID, SessionID: sessionID, SourceID: src.ID,
		Limit: q.Limit,
	}
	if q.AfterEpoch != nil && q.AfterSeq != nil {
		req.After = &metabridge.Cursor{Epoch: *q.AfterEpoch, Seq: *q.AfterSeq}
	}
	req.BeforeCursor = q.BeforeCursor
	page, err := cs.bridge.CommentPage(ctx, req)
	if err != nil {
		return ConsoleStreamPage{}, bridgeError(err)
	}
	items := make([]ConsoleComment, len(page.Items))
	for i, it := range page.Items {
		items[i] = ConsoleComment{Ref: it.Ref, Seq: it.Seq, ParentRef: it.ParentRef, CreatedAt: it.CreatedAt,
			AuthorName: it.AuthorName, Text: it.Text, IsPage: it.IsPage, HasAttachment: it.HasAttachment}
	}
	items, err = cs.attachMarks(ctx, tx, scope, token, sessionID, items)
	if err != nil {
		return ConsoleStreamPage{}, err
	}
	reset := false
	if q.AfterEpoch != nil && *q.AfterEpoch != page.Epoch {
		reset = true
	}
	return ConsoleStreamPage{
		Epoch:       page.Epoch,
		Reset:       reset,
		Items:       items,
		Next:        ConsoleCursor{Epoch: page.Epoch, Seq: page.NextSeq},
		OlderCursor: page.OlderCursor,
		Stream:      bridgeStream(page.Stream),
	}, nil
}

// instagramPage reads the encrypted webhook copy (social.read_comment_events), decrypts each event here
// and drops events whose media id differs from the source's source_object_id (only inside the ciphertext).
func (cs *CommentStream) instagramPage(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string, src consoleSource, q ConsolePageQuery) (ConsoleStreamPage, error) {
	if q.BeforeCursor != nil {
		return ConsoleStreamPage{}, ErrInvalidCursor // IG has no Graph backfill
	}
	if cs.payload == nil {
		return ConsoleStreamPage{}, ErrStreamUnavailable // api process lacks the payload keyring
	}
	afterSeq := int64(0)
	if q.AfterSeq != nil {
		afterSeq = *q.AfterSeq
	}
	rows, err := tx.Query(ctx, `SELECT event_id::text,kind,occurred_at,received_at,key_id,nonce,ciphertext,
		app_id,object,asset_id,event_key,payload_hash,route_id::text,route_epoch,seq
		FROM social.read_comment_events($1::uuid,$2::bigint,$3::integer)`, sessionID, afterSeq, q.Limit)
	if err != nil {
		return ConsoleStreamPage{}, mapReadError(err)
	}
	defer rows.Close()
	comments := make([]ConsoleComment, 0, q.Limit)
	lastSeq := afterSeq
	scanned := 0
	var newest time.Time
	for rows.Next() {
		var env meta.CommentEnvelope
		if err := rows.Scan(&env.EventID, &env.Kind, &env.OccurredAt, &env.ReceivedAt, &env.KeyID, &env.Nonce,
			&env.Ciphertext, &env.AppID, &env.Object, &env.AssetID, &env.EventKey, &env.PayloadHash,
			&env.RouteID, &env.RouteEpoch, &env.Seq); err != nil {
			return ConsoleStreamPage{}, ErrStreamUnavailable
		}
		// §2.6: raw query progress survives decryption/media filtering; visible length cannot prove EOF.
		scanned++
		if env.Seq > lastSeq {
			lastSeq = env.Seq
		}
		cr, err := cs.payload.OpenComment(scope.TenantID, scope.StoreID, env)
		if err != nil || cr.MediaID != src.SourceObjectID {
			continue // not this source's media, or undecryptable: drop (never an error for the console)
		}
		parent := (*string)(nil)
		if cr.ParentRef != "" {
			p := cr.ParentRef
			parent = &p
		}
		name := (*string)(nil)
		if cr.AuthorName != "" {
			n := cr.AuthorName
			name = &n
		}
		seq := env.Seq // §2.6: preserve this decrypted envelope's position, not the page's last sequence.
		comments = append(comments, ConsoleComment{
			Ref: cr.Ref, Seq: &seq, ParentRef: parent, CreatedAt: cr.CreatedAt, AuthorName: name,
			Text: cr.Text, IsPage: cr.IsPage, HasAttachment: cr.HasAttachment,
		})
		if cr.CreatedAt.After(newest) {
			newest = cr.CreatedAt
		}
	}
	if err := rows.Err(); err != nil {
		return ConsoleStreamPage{}, ErrStreamUnavailable
	}
	items, err := cs.attachMarks(ctx, tx, scope, token, sessionID, comments)
	if err != nil {
		return ConsoleStreamPage{}, err
	}
	now := time.Now().UTC()
	var lag *int64
	if !newest.IsZero() {
		l := now.Sub(newest).Milliseconds()
		lag = &l
	}
	return ConsoleStreamPage{
		Epoch:         0,
		Reset:         q.AfterEpoch != nil && *q.AfterEpoch != 0,
		ScanExhausted: scanned < q.Limit,
		Items:         items,
		Next:          ConsoleCursor{Epoch: 0, Seq: lastSeq},
		Stream: ConsoleStreamState{
			State: "live", PollIntervalMs: 3000, LastOKAt: &now, LagMs: lag,
			SourcePlatform: "instagram", VideoEmbeddable: false,
		},
	}, nil
}

// attachMarks joins 1..100 refs to live.console_marks and zips them back onto the items by ref.
func (cs *CommentStream) attachMarks(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string, items []ConsoleComment) ([]ConsoleComment, error) {
	if len(items) == 0 {
		return items, nil
	}
	refs := make([]string, len(items))
	for i, it := range items {
		refs[i] = it.Ref
	}
	marks, err := cs.marks(ctx, tx, scope, token, sessionID, refs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Marks = marks[items[i].Ref]
	}
	return items, nil
}

// marks calls live.console_marks (≤100 refs, live:read re-checked by the definer) and folds its 17
// nullable columns into one ConsoleMarks per ref.
func (cs *CommentStream) marks(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string, refs []string) (map[string]ConsoleMarks, error) {
	out := make(map[string]ConsoleMarks, len(refs))
	rows, err := tx.Query(ctx, `SELECT ref,intake_state,intake_drop_reason,claim_outcome,claim_reason,
		offer_id::text,keyword,quantity,bundle_id::text,private_reply_kind,private_reply_state,
		private_reply_blocked_reason,public_replies,printed_count,printed_last_at,private_reply_available,
		private_reply_unavailable_reason
		FROM live.console_marks($1::uuid,$2::text[])`, sessionID, refs)
	if err != nil {
		return nil, mapReadError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var ref string
		var intakeState, intakeDrop, claimStatus, claimReason *string
		var offerID, keyword, bundleID *string
		var quantity *int64
		var replyKind, replyState, replyBlocked *string
		var publicReplies, printedCount int64
		var printedLastAt *time.Time
		var privateAvailable bool
		var privateUnavailable *string
		if err := rows.Scan(&ref, &intakeState, &intakeDrop, &claimStatus, &claimReason, &offerID,
			&keyword, &quantity, &bundleID, &replyKind, &replyState, &replyBlocked, &publicReplies,
			&printedCount, &printedLastAt, &privateAvailable, &privateUnavailable); err != nil {
			return nil, ErrStreamUnavailable
		}
		m := ConsoleMarks{PublicReplies: publicReplies, PrivateReplyAvailable: privateAvailable}
		if intakeState != nil {
			m.Intake = &IntakeMark{State: *intakeState, DropReason: intakeDrop}
		}
		if claimStatus != nil {
			m.Claim = &ClaimMark{Status: *claimStatus, Reason: claimReason, OfferID: offerID,
				Keyword: keyword, Quantity: quantity, BundleID: bundleID}
		}
		if replyState != nil {
			kind := "auto"
			if replyKind != nil {
				kind = *replyKind
			}
			m.PrivateReply = &PrivateReplyMark{Kind: kind, State: *replyState, BlockedReason: replyBlocked}
		}
		m.Printed = &PrintedMark{Count: printedCount, LastAt: printedLastAt}
		if privateUnavailable != nil {
			m.PrivateReplyUnavailableReason = *privateUnavailable
		}
		out[ref] = m
	}
	if err := rows.Err(); err != nil {
		return nil, ErrStreamUnavailable
	}
	return out, nil
}

// bridgeError maps the bridge's fixed safe errors to live sentinels.
func bridgeError(err error) error {
	switch {
	case errors.Is(err, metabridge.ErrBridgeNotFound):
		return command.ErrNotFound
	case errors.Is(err, metabridge.ErrBridgeInvalidCur):
		return ErrInvalidCursor
	case errors.Is(err, metabridge.ErrBridgeNotOwner), errors.Is(err, metabridge.ErrBridgeUnavailable):
		return ErrStreamUnavailable
	default:
		return ErrStreamUnavailable
	}
}

func bridgeStream(s metabridge.BridgeStreamState) ConsoleStreamState {
	return ConsoleStreamState{
		State: s.State, PollIntervalMs: s.PollIntervalMs, LastOKAt: s.LastOKAt, LagMs: s.LagMs,
		SourcePlatform: s.SourcePlatform, VideoEmbeddable: s.VideoEmbeddable, Reason: s.Reason,
	}
}
