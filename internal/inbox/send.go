// Purpose: the four manual-send planners of live-console-v1 §3.3/§4 as service methods: SendDM (A12), SendPrivateReply (A4),
// SendPublicReply (A5) and PlanRecommend (A6 post_comment). Each runs inside the caller's scoped merchant transaction through
// command.Run (Idempotency-Key replay, request hash over body_hmac, never the text), validates the text, seals the display copy (payload
// keyring) and the dispatch copy (Page HPKE public ring), inserts the external_operation_v1 job and calls ONE SECURITY DEFINER planner
// (inbox.plan_dm / inbox.plan_manual_private_reply / inbox.plan_public_reply / live.plan_offer_recommend, migration 0128). No network call.
// Depends on: internal/command, internal/integrations/core (InsertOperationJob), internal/integrations/meta/pagetoken (SealSend),
// internal/inbox keyring/classifier/send_text, internal/msgtemplates, SQL migration 0128 and social.read_thread (0119).
// Used by: internal/httpapi/inbox_send.go; cmd/api (EnableSend).
// Invariants: LCN06/07/08 (planner refusals pass through as PT409/PT422/PT429 with the deny code as message); LCN13 (display copy carries no
// link; the PSID exists only in API memory and inside the sealed dispatch copy).

package inbox

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

// ErrSendUnavailable means the send side is not configured (no seal ring / job client): the routes are not mounted then.
var ErrSendUnavailable = errors.New("inbox: send unavailable")

// SendOutput is the A4/A5/A12 response. send_state is always "queued" at planning (visible states: §4.4).
type SendOutput struct {
	OperationID        string `json:"operation_id"`
	OutboundID         string `json:"outbound_id"`
	SendState          string `json:"send_state"`
	TakeoverGeneration *int64 `json:"takeover_generation,omitempty"`
}

// DMInput is A12 `{text | template ref, expected_generation}`.
type DMInput struct {
	TextInput
	ExpectedGeneration *int64 `json:"expected_generation"`
}

// ReplyInput is A4 `{text | template ref, confirm_preempt_auto?}` (A1.1: only true or absent).
type ReplyInput struct {
	TextInput
	ConfirmPreemptAuto *bool `json:"confirm_preempt_auto"`
}

// sendSecret is the dispatch copy sealed to the Page HPKE public ring (§3.4); opened only by claims-worker.
type sendSecret struct {
	PSID string `json:"recipient_psid,omitempty"`
	Text string `json:"text"`
}

// prepared is everything the planner definers take after the scope arguments (the ‹E› tail of Amendment 1 P2-4).
type prepared struct {
	operation, outbound string
	job                 int64
	hmac                []byte
	keyID               string
	nonce, ciphertext   []byte
	enc, sealed         []byte
}

// prepare allocates the operation/outbound ids, inserts the River job in this transaction (a service nobody enqueues is dead code),
// computes body_hmac and seals both copies. The display copy is the text with every link replaced (§3.4).
func (s *Service) prepare(ctx context.Context, tx pgx.Tx, scope platform.Scope, kind, psid, text string, hmac []byte) (prepared, error) {
	var p prepared
	if !s.SendEnabled() {
		return p, ErrSendUnavailable
	}
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text, gen_random_uuid()::text`).Scan(&p.operation, &p.outbound); err != nil {
		return p, ErrDatabase
	}
	job, err := core.InsertOperationJob(ctx, s.jobs, tx, p.operation)
	if err != nil {
		return p, ErrDatabase
	}
	p.job, p.hmac = job, hmac
	if p.keyID, p.nonce, p.ciphertext, err = s.keys.sealOutbound(scope.TenantID, scope.StoreID, p.outbound, kind, scrubLinks(text)); err != nil {
		return p, err
	}
	plain, err := json.Marshal(sendSecret{PSID: psid, Text: text})
	if err != nil {
		return p, ErrDatabase
	}
	p.enc, p.sealed, err = s.sealDispatch(scope, p.operation, plain)
	clear(plain)
	return p, err
}

func (s *Service) sealDispatch(scope platform.Scope, operation string, plain []byte) (enc, sealed []byte, err error) {
	enc, sealed, err = s.seal.SealSend(scope.TenantID, scope.StoreID, operation, plain)
	if err != nil {
		return nil, nil, fmt.Errorf("seal send copy: %w", ErrConfig)
	}
	return enc, sealed, nil
}

func templateArgs(t resolvedText) (*string, *int64) { return t.templateID, t.version }

// SendDM is A12: plan one DM RESPONSE in an open 24 h window (inbox:reply). The PSID is read from the newest inbound message of the
// conversation and exists only in this request and the sealed dispatch copy. orderID is the optional for-buyer order (LC-B6).
// Side effects (one transaction): command receipt, River job, inbox.plan_dm (implicit takeover, operation, outbound row, secret, audit).
func (s *Service) SendDM(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, conversationID string, in DMInput, orderID *string) (SendOutput, error) {
	var out SendOutput
	if !s.SendEnabled() {
		return out, ErrSendUnavailable
	}
	if !command.ValidID(conversationID) || in.ExpectedGeneration == nil || *in.ExpectedGeneration < 0 || (orderID != nil && !command.ValidID(*orderID)) {
		return out, command.ErrInvalid
	}
	meta, err := s.conversationMeta(ctx, tx, conversationID)
	if err != nil {
		return out, err
	}
	rt, err := s.resolveText(ctx, tx, KindDM, meta.platform, in.TextInput)
	if err != nil {
		return out, err
	}
	hmac, err := s.keys.bodyHMAC(scope.TenantID, rt.text)
	if err != nil {
		return out, err
	}
	tplID, tplVer := templateArgs(rt)
	request := struct {
		Conversation string  `json:"conversation_id"`
		Generation   int64   `json:"expected_generation"`
		BodyHMAC     string  `json:"body_hmac"`
		TemplateID   *string `json:"template_id"`
		TemplateVer  *int64  `json:"template_version"`
		Order        *string `json:"order_id"`
	}{conversationID, *in.ExpectedGeneration, hex.EncodeToString(hmac), tplID, tplVer, orderID}
	err = command.Run(ctx, tx, scope, "inbox.dm.send", key, request, &out, func() error {
		psid, err := s.latestSender(ctx, tx, conversationID)
		if err != nil {
			return err
		}
		p, err := s.prepare(ctx, tx, scope, KindDM, psid, rt.text, hmac)
		if err != nil {
			return err
		}
		// Calls inbox.plan_dm (0128): definer commerce_integration_writer; takeover, operation, outbound row, secret, audit.
		var op string
		if err := tx.QueryRow(ctx, `SELECT inbox.plan_dm($1::uuid,$2::bigint,$3::uuid,$4::uuid,$5::bigint,$6::uuid,$7::bytea,$8,$9::bytea,$10::bytea,$11::bytea,$12::bytea,$13,$14::bigint)::text`,
			conversationID, *in.ExpectedGeneration, orderID, p.operation, p.job, p.outbound, p.hmac, p.keyID, p.nonce, p.ciphertext,
			p.enc, p.sealed, tplID, tplVer).Scan(&op); err != nil {
			return databaseError(err)
		}
		after, err := s.conversationMeta(ctx, tx, conversationID)
		if err != nil {
			return err
		}
		out = SendOutput{OperationID: op, OutboundID: p.outbound, SendState: "queued", TakeoverGeneration: &after.generation}
		return nil
	})
	return out, err
}

// sourcePlatform is the platform of the session's active claim source (facebook | instagram); no source is a 404.
func (s *Service) sourcePlatform(ctx context.Context, tx pgx.Tx, sessionID string) (string, error) {
	var p string
	err := tx.QueryRow(ctx, `SELECT platform FROM live.claim_sources WHERE session_id=$1::uuid AND active ORDER BY updated_at DESC, id LIMIT 1`, sessionID).Scan(&p)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", command.ErrNotFound
	}
	if err != nil {
		return "", ErrDatabase
	}
	return p, nil
}

// SendPrivateReply is A4: one manual private reply to a comment (inbox:reply). createdAt is the comment-facts lookup (live-console-v1
// §2.3 / Amendment 1 A1.2), called inside the command so a replay never needs the bridge; it returns the *SendError of comment_unknown /
// page_comment / reply_comment_unsupported / comment_facts_unavailable. The planner freezes created_at in the request.
func (s *Service) SendPrivateReply(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, sessionID, commentRef string, in ReplyInput,
	createdAt func(context.Context) (time.Time, error)) (SendOutput, error) {
	var out SendOutput
	if !s.SendEnabled() {
		return out, ErrSendUnavailable
	}
	if !command.ValidID(sessionID) || !commentRefPattern(commentRef) || createdAt == nil || (in.ConfirmPreemptAuto != nil && !*in.ConfirmPreemptAuto) {
		return out, command.ErrInvalid
	}
	confirm := in.ConfirmPreemptAuto != nil
	src, err := s.sourcePlatform(ctx, tx, sessionID)
	if err != nil {
		return out, err
	}
	rt, err := s.resolveText(ctx, tx, KindPrivate, src, in.TextInput)
	if err != nil {
		return out, err
	}
	hmac, err := s.keys.bodyHMAC(scope.TenantID, rt.text)
	if err != nil {
		return out, err
	}
	tplID, tplVer := templateArgs(rt)
	request := struct {
		Session    string  `json:"session_id"`
		Comment    string  `json:"comment_ref"`
		BodyHMAC   string  `json:"body_hmac"`
		Confirm    bool    `json:"confirm_preempt_auto"`
		TemplateID *string `json:"template_id"`
		TemplateV  *int64  `json:"template_version"`
	}{sessionID, commentRef, hex.EncodeToString(hmac), confirm, tplID, tplVer}
	err = command.Run(ctx, tx, scope, "inbox.private_reply.send", key, request, &out, func() error {
		created, err := createdAt(ctx)
		if err != nil {
			return err
		}
		p, err := s.prepare(ctx, tx, scope, KindPrivate, "", rt.text, hmac)
		if err != nil {
			return err
		}
		// Calls inbox.plan_manual_private_reply (0128): the mpr: quota, the 120 s confirm gate, implicit takeover of a linked peer.
		var op string
		if err := tx.QueryRow(ctx, `SELECT inbox.plan_manual_private_reply($1::uuid,$2,$3::timestamptz,$4::boolean,$5::uuid,$6::bigint,$7::uuid,$8::bytea,$9,$10::bytea,$11::bytea,$12::bytea,$13::bytea,$14,$15::bigint)::text`,
			sessionID, commentRef, created, confirm, p.operation, p.job, p.outbound, p.hmac, p.keyID, p.nonce, p.ciphertext,
			p.enc, p.sealed, tplID, tplVer).Scan(&op); err != nil {
			return databaseError(err)
		}
		out = SendOutput{OperationID: op, OutboundID: p.outbound, SendState: "queued"}
		return nil
	})
	return out, err
}

// SendPublicReply is A5: one public comment reply (inbox:reply). The §3.5 content rule runs here (422 public_reply_forbidden_content)
// before anything is sealed; inbox.plan_public_reply refuses IG live (ig_live_unsupported) and a missing capability.
func (s *Service) SendPublicReply(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, sessionID, commentRef string, in TextInput) (SendOutput, error) {
	var out SendOutput
	if !s.SendEnabled() {
		return out, ErrSendUnavailable
	}
	if !command.ValidID(sessionID) || !commentRefPattern(commentRef) {
		return out, command.ErrInvalid
	}
	src, err := s.sourcePlatform(ctx, tx, sessionID)
	if err != nil {
		return out, err
	}
	rt, err := s.resolveText(ctx, tx, KindPublic, src, in)
	if err != nil {
		return out, err
	}
	hmac, err := s.keys.bodyHMAC(scope.TenantID, rt.text)
	if err != nil {
		return out, err
	}
	tplID, tplVer := templateArgs(rt)
	request := struct {
		Session    string  `json:"session_id"`
		Comment    string  `json:"comment_ref"`
		BodyHMAC   string  `json:"body_hmac"`
		TemplateID *string `json:"template_id"`
		TemplateV  *int64  `json:"template_version"`
	}{sessionID, commentRef, hex.EncodeToString(hmac), tplID, tplVer}
	err = command.Run(ctx, tx, scope, "inbox.public_reply.send", key, request, &out, func() error {
		p, err := s.prepare(ctx, tx, scope, KindPublic, "", rt.text, hmac)
		if err != nil {
			return err
		}
		// Calls inbox.plan_public_reply (0128): never a takeover; IG live refused.
		var op string
		if err := tx.QueryRow(ctx, `SELECT inbox.plan_public_reply($1::uuid,$2,$3::uuid,$4::bigint,$5::uuid,$6::bytea,$7,$8::bytea,$9::bytea,$10::bytea,$11::bytea,$12,$13::bigint)::text`,
			sessionID, commentRef, p.operation, p.job, p.outbound, p.hmac, p.keyID, p.nonce, p.ciphertext, p.enc, p.sealed, tplID, tplVer).Scan(&op); err != nil {
			return databaseError(err)
		}
		out = SendOutput{OperationID: op, OutboundID: p.outbound, SendState: "queued"}
		return nil
	})
	return out, err
}

// fixedRecommendBody is msgtemplates' offer-recommend/v1 body (migration 0121); used when no resolver is wired.
const fixedRecommendBody = "推薦商品：{{product.name}} {{variant}}，關鍵字「{{keyword}}」，直播價 {{live_price}}"

// PlanRecommend plans the offer recommend comment of A6 (FB only; the caller already holds live:manage + inbox:reply and runs this inside
// its own command). It renders the fixed offer-recommend/v1 template from catalog facts, runs the §3.5 rule, and calls
// live.plan_offer_recommend. Returns the operation id.
func (s *Service) PlanRecommend(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID, offerID string, expectedVersion int64) (string, error) {
	if !s.SendEnabled() {
		return "", ErrSendUnavailable
	}
	if !command.ValidID(sessionID) || !command.ValidID(offerID) || expectedVersion < 1 {
		return "", command.ErrInvalid
	}
	var keyword, name, code, currency string
	var price, version int64
	// Calls live.offer_recommend_facts (0128): keyword, product name, sku code, price; live:manage re-checked in SQL.
	err := tx.QueryRow(ctx, `SELECT keyword, product_name, sku_code, price_minor, currency, version FROM live.offer_recommend_facts($1::uuid,$2::uuid)`,
		sessionID, offerID).Scan(&keyword, &name, &code, &price, &currency, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &SendError{Status: 409, Code: "offer_unavailable"}
	}
	if err != nil {
		return "", databaseError(err)
	}
	body := fixedRecommendBody
	if s.templates != nil {
		if r, e := s.templates.Resolve(ctx, tx, "offer-recommend/v1", 1); e == nil {
			body = r.Body
		}
	}
	amount := fmt.Sprintf("%s %d", currency, price)
	if currency == "TWD" {
		amount = fmt.Sprintf("NT$%d", price)
	}
	text := strings.NewReplacer("{{product.name}}", name, "{{variant}}", code, "{{keyword}}", keyword, "{{live_price}}", amount).Replace(body)
	origins, err := s.originsFor(ctx, tx, KindRecommend)
	if err != nil {
		return "", err
	}
	text, err = checkText(KindRecommend, "facebook", text, origins...)
	if err != nil {
		return "", err
	}
	hmac, err := s.keys.bodyHMAC(scope.TenantID, text)
	if err != nil {
		return "", err
	}
	p, err := s.prepare(ctx, tx, scope, KindRecommend, "", text, hmac)
	if err != nil {
		return "", err
	}
	tplID, tplVer := "offer-recommend/v1", int64(1)
	var op string
	// Calls live.plan_offer_recommend (0128): lcn-rec advisory lock, 10 min rate, FB only.
	if err := tx.QueryRow(ctx, `SELECT live.plan_offer_recommend($1::uuid,$2::uuid,$3::bigint,$4::uuid,$5::bigint,$6::uuid,$7::bytea,$8,$9::bytea,$10::bytea,$11::bytea,$12::bytea,$13,$14::bigint)::text`,
		sessionID, offerID, expectedVersion, p.operation, p.job, p.outbound, p.hmac, p.keyID, p.nonce, p.ciphertext, p.enc, p.sealed, tplID, tplVer).Scan(&op); err != nil {
		return "", databaseError(err)
	}
	return op, nil
}

func commentRefPattern(ref string) bool {
	if len(ref) < 1 || len(ref) > 80 {
		return false
	}
	for i := 0; i < len(ref); i++ {
		if !(ref[i] >= '0' && ref[i] <= '9') && ref[i] != '_' {
			return false
		}
	}
	return true
}

// latestSender opens the newest inbound message of a conversation and returns its sender id (the PSID). A conversation without a
// readable inbound message cannot be answered: 409 conversation_gone.
func (s *Service) latestSender(ctx context.Context, tx pgx.Tx, conversationID string) (string, error) {
	// Calls social.read_thread (0119) for the single newest row; the plaintext stays in this function.
	rows, err := tx.Query(ctx, `SELECT event_id::text, key_id, nonce, ciphertext, app_id, object, asset_id, event_key, payload_hash,
			tenant_id::text, store_id::text, route_id::text, route_epoch, server_seq, occurred_at, direction
		FROM social.read_thread($1::uuid, NULL::bigint, 1)`, conversationID)
	if err != nil {
		return "", databaseError(err)
	}
	defer rows.Close()
	if !rows.Next() {
		if rows.Err() != nil {
			return "", databaseError(rows.Err())
		}
		return "", &SendError{Status: 409, Code: "conversation_gone"}
	}
	var r threadRow
	if err := rows.Scan(&r.eventID, &r.keyID, &r.nonce, &r.ciphertext, &r.appID, &r.object, &r.assetID, &r.eventKey, &r.payloadHash,
		&r.tenantID, &r.storeID, &r.routeID, &r.routeEpoch, &r.serverSeq, &r.occurredAt, &r.direction); err != nil {
		return "", databaseError(err)
	}
	plain, err := s.keys.open(eventContext{EventID: r.eventID, AppID: r.appID, Object: r.object, EventKey: r.eventKey, PayloadHash: r.payloadHash,
		TenantID: r.tenantID, StoreID: r.storeID, RouteID: r.routeID, RouteEpoch: r.routeEpoch}, r.keyID, r.nonce, r.ciphertext)
	if err != nil {
		return "", &SendError{Status: 409, Code: "conversation_gone"}
	}
	view, err := replayMessage(plain, r.assetID)
	if err != nil || view.senderID == "" {
		return "", &SendError{Status: 409, Code: "conversation_gone"}
	}
	return view.senderID, nil
}
