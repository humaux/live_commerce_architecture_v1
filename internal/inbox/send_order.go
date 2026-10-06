// Purpose: the §5.1 step-4 DM of an order made for a buyer (live-console-v1, LC-B6): plans ONE meta.dm_send with the system template
// order-pay-link/v1 whose {{連結}} placeholder is replaced by the buyer's pay link ONLY in the sealed dispatch copy (the display copy keeps the
// placeholder, §3.4/LCN13). The semantic key is derived from (conversation, order) in inbox.plan_dm (migration 0129), so no replay can plan two.
// Depends on: internal/command (Run receipt "inbox.dm.order_pay_link"), internal/msgtemplates (Resolve of the fixed template), SQL inbox.plan_dm
//   (0128 + 0129 order key) and social.conversation_meta/read_thread (0119) through send.go helpers, internal/integrations/core (job insert).
// Used by: internal/merchanttools/order_for_buyer.go (through its PayLinkPlanner interface); cmd/api wires the inbox.Service in.
// Invariants: A1.3 (the planner takes the lcn-dup advisory lock with target = conversation first; body_hmac carries the order id so two different
//   orders to one buyer are not "the same message"), LCN13 (no link in any stored display copy), §3.3 (window re-checked inside inbox.plan_dm).

package inbox

import (
	"context"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// payLinkOperation is the command receipt namespace of the for-buyer DM (the Idempotency-Key is the for-buyer request's).
const payLinkOperation = "inbox.dm.order_pay_link"

// ErrPayLinkNotPlanned is the replay probe's answer (link == ""): no receipt of this key exists yet, nothing was written.
var ErrPayLinkNotPlanned = errors.New("inbox: pay link not planned")

// PlanOrderPayLink plans the pay-link DM of orderID in conversationID (inbox:reply, enforced by inbox.plan_dm). With link == "" it only probes:
// it returns the saved SendOutput of an earlier run of the same key, a *SendError window_closed when the 24 h window is closed (so the caller
// does not re-issue a link for nothing), or ErrPayLinkNotPlanned. With a link it runs the planner: receipt + River job + inbox.plan_dm (implicit
// takeover, operation, outbound row, sealed secret, audit) in the caller's merchant transaction; refusals of the planner pass through as the
// PT409/PT429 database errors (window_closed, capability, takeover_changed, conversation_gone, duplicate_recent, rate_limited).
func (s *Service) PlanOrderPayLink(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, conversationID, orderID, link string) (SendOutput, error) {
	var out SendOutput
	if !s.SendEnabled() {
		return out, ErrSendUnavailable
	}
	if !command.ValidID(conversationID) || !command.ValidID(orderID) {
		return out, command.ErrInvalid
	}
	meta, err := s.conversationMeta(ctx, tx, conversationID)
	if err != nil {
		return out, err
	}
	if s.templates == nil {
		return out, invalidText(0, "template_unavailable")
	}
	tpl, err := s.templates.Resolve(ctx, tx, msgtemplates.FixedOrderPayLink, 1)
	if err != nil {
		if msgtemplates.IsNotFound(err) {
			return out, invalidText(0, "template_not_found")
		}
		return out, err
	}
	if strings.Count(tpl.Body, linkPlaceholder) != 1 || !slices.Contains(tpl.Kinds, KindDM) {
		return out, invalidText(0, "template_kind")
	}
	// The order id is part of the digest, so a second order to the same buyer inside 30 s is not "the same message" (A1.3 duplicate_recent).
	hmac, err := s.keys.bodyHMAC(scope.TenantID, "order-pay-link|"+orderID+"|"+tpl.Body)
	if err != nil {
		return out, err
	}
	tplID, tplVer := tpl.TemplateID, tpl.Version
	request := struct {
		Conversation string `json:"conversation_id"`
		Order        string `json:"order_id"`
		BodyHMAC     string `json:"body_hmac"`
		TemplateID   string `json:"template_id"`
		TemplateVer  int64  `json:"template_version"`
	}{conversationID, orderID, hex.EncodeToString(hmac), tplID, tplVer}
	err = command.Run(ctx, tx, scope, payLinkOperation, key, request, &out, func() error {
		if !meta.windowOpenUntil.After(time.Now()) {
			return &SendError{Status: 409, Code: "window_closed"}
		}
		if link == "" {
			return ErrPayLinkNotPlanned
		}
		text, err := checkText(KindDM, meta.platform, strings.Replace(tpl.Body, linkPlaceholder, link, 1))
		if err != nil {
			return err
		}
		psid, err := s.latestSender(ctx, tx, conversationID)
		if err != nil {
			return err
		}
		p, err := s.prepare(ctx, tx, scope, KindDM, psid, text, hmac)
		if err != nil {
			return err
		}
		// Calls inbox.plan_dm (0128, order key 0129): definer commerce_integration_writer; takeover, operation, outbound row, secret, audit.
		var op string
		if err := tx.QueryRow(ctx, `SELECT inbox.plan_dm($1::uuid,$2::bigint,$3::uuid,$4::uuid,$5::bigint,$6::uuid,$7::bytea,$8,$9::bytea,$10::bytea,$11::bytea,$12::bytea,$13,$14::bigint)::text`,
			conversationID, meta.generation, orderID, p.operation, p.job, p.outbound, p.hmac, p.keyID, p.nonce, p.ciphertext,
			p.enc, p.sealed, tplID, tplVer).Scan(&op); err != nil {
			return databaseError(err)
		}
		out = SendOutput{OperationID: op, OutboundID: p.outbound, SendState: "queued", TakeoverGeneration: &meta.generation}
		return nil
	})
	return out, err
}
