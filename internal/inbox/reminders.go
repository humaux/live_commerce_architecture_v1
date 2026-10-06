// Purpose: W3-03B checkout reminders on the inbox side: ScanCheckoutReminders (the SQL candidate scan: eligibility, follow-up rows, audit),
// PlanCheckoutReminder (plan ONE 24 h-window DM for ONE scanned buyer through the existing LC-B4 send path) and ReminderReport (what was sent /
// is queued / failed / needs a manual follow-up). It decides no rule: inbox.checkout_reminder_candidates classifies, inbox.plan_checkout_reminder
// re-validates and plans; this file renders the fixed template of the buyer's state, seals both copies and calls the planner. The orchestration
// (one transaction per buyer: issue the link, then plan) lives in internal/merchanttools/checkout_reminders.go. No network call.
// Depends on: internal/command, internal/msgtemplates (Resolve checkout-reminder/v1 | order-pay-link/v1), internal/inbox send.go (prepare,
// latestSender), send_text.go (checkText), keyring.go (bodyHMAC); SQL migration 0144 (inbox.checkout_reminder_candidates,
// inbox.plan_checkout_reminder, inbox.reminder_report) and inbox.store_origins (0128).
// Used by: internal/merchanttools/checkout_reminders.go (scan + plan), internal/httpapi/reminders.go (report); cmd/api through the inbox service.
// Invariants: no message tag / no send outside the 24 h window (planner + Check refuse, never retried); one reminder per buyer per session; the
// bearer link only exists in memory and inside the sealed dispatch copy (the display copy keeps {{連結}}); PSID only in memory and the sealed copy.
// Status: MOCK (REAL_PG + fake Graph).

package inbox

import (
	"context"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// Reminder states (inbox.crm_bundle_facts) and the fixed template each one sends: a never-opened claim gets checkout-reminder/v1 (claim link),
// an unpaid merchant-created order gets the LC-B6 order-pay-link/v1 (order link).
const (
	StateClaimed         = "claimed"
	StateAwaitingPayment = "awaiting_payment"
	reminderLimit        = 100
)

// ReminderCandidate is one row of inbox.checkout_reminder_candidates. Verdict "send" rows carry what the planner needs (conversation, takeover
// generation, platform) and what the link issue needs (order id for an unpaid order, current claim-link generation for a claim).
type ReminderCandidate struct {
	BundleID       string
	Verdict        string // send | already_reminded | window_closed | human_takeover | no_peer | capability | link_unavailable
	State          string // claimed | awaiting_payment
	OrderID        string
	LinkGeneration int64
	ConversationID string
	Generation     int64
	Platform       string
	Locale         string
}

// ReminderScan is one scan: every classified candidate, whether more buyers remain (Truncated) and the store's checkout origin ("" = none).
type ReminderScan struct {
	Candidates []ReminderCandidate
	Truncated  bool
	Origin     string
}

// ScanCheckoutReminders classifies the session's candidate buyers (inbox:reply and live:manage; bundleID "" = the batch, else that buyer only,
// 409 not_remindable when she is no candidate). Side effects: follow-up rows for the buyers that cannot be reached and one audit row.
// Calls inbox.checkout_reminder_candidates (0144) and inbox.store_origins (0128).
func (s *Service) ScanCheckoutReminders(ctx context.Context, tx pgx.Tx, sessionID, bundleID string) (ReminderScan, error) {
	var out ReminderScan
	if !s.SendEnabled() {
		return out, ErrSendUnavailable
	}
	if !command.ValidID(sessionID) || (bundleID != "" && !command.ValidID(bundleID)) {
		return out, command.ErrInvalid
	}
	rows, err := tx.Query(ctx, `SELECT bundle_id::text, verdict, reminder_state, order_id::text, link_generation, conversation_id::text, takeover_generation,
		platform, locale, truncated FROM inbox.checkout_reminder_candidates($1::uuid,'manual',$2,nullif($3,'')::uuid)`, sessionID, reminderLimit, bundleID)
	if err != nil {
		return out, databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var c ReminderCandidate
		var bundle, state, order, conv, plat *string
		var linkGen, gen *int64
		var trunc bool
		if err := rows.Scan(&bundle, &c.Verdict, &state, &order, &linkGen, &conv, &gen, &plat, &c.Locale, &trunc); err != nil {
			return out, databaseError(err)
		}
		if trunc {
			out.Truncated = true
			continue
		}
		c.BundleID, c.State, c.OrderID, c.ConversationID, c.Platform = deref(bundle), deref(state), deref(order), deref(conv), deref(plat)
		if linkGen != nil {
			c.LinkGeneration = *linkGen
		}
		if gen != nil {
			c.Generation = *gen
		}
		out.Candidates = append(out.Candidates, c)
	}
	if err := rows.Err(); err != nil {
		return out, databaseError(err)
	}
	rows.Close()
	origins, err := s.storeOrigins(ctx, tx)
	if err != nil {
		return out, err
	}
	if len(origins) > 0 {
		out.Origin = origins[0]
	}
	return out, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Service) storeOrigins(ctx context.Context, tx pgx.Tx) ([]string, error) {
	var origins []string
	// Calls inbox.store_origins (0128): public data, scoped by the transaction GUCs.
	if err := tx.QueryRow(ctx, `SELECT inbox.store_origins()`).Scan(&origins); err != nil {
		return nil, databaseError(err)
	}
	return origins, nil
}

// reminderTemplate returns the fixed template id of a state (version 1).
func reminderTemplate(state string) (string, bool) {
	switch state {
	case StateClaimed:
		return msgtemplates.FixedCheckoutReminder, true
	case StateAwaitingPayment:
		return msgtemplates.FixedOrderPayLink, true
	}
	return "", false
}

// PlanCheckoutReminder plans ONE reminder DM for the scanned buyer c (inbox:reply) inside the caller's per-buyer transaction, where the caller has
// just issued link (a claim link or an order link) — a refusal of the planner (PT409 with the deny code: window_closed, human_takeover,
// takeover_changed, capability, already_reminded, not_remindable, conversation_gone, rate_limited) rolls that issue back with the transaction.
// The conversation named by c is verified by the planner to be a peer of the bundle (never re-picked). Side effects: River job, operation,
// outbound row, sealed dispatch copy, reminder row, audit (inbox.plan_checkout_reminder, 0144).
func (s *Service) PlanCheckoutReminder(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string, c ReminderCandidate, link string) error {
	if !s.SendEnabled() {
		return ErrSendUnavailable
	}
	tplID, ok := reminderTemplate(c.State)
	if !ok || !command.ValidID(sessionID) || !command.ValidID(c.BundleID) || !command.ValidID(c.ConversationID) || link == "" {
		return command.ErrInvalid
	}
	if s.templates == nil {
		return invalidText(0, "template_unavailable")
	}
	tpl, err := s.templates.Resolve(ctx, tx, tplID, 1)
	if err != nil {
		if msgtemplates.IsNotFound(err) {
			return invalidText(0, "template_not_found")
		}
		return err
	}
	if strings.Count(tpl.Body, linkPlaceholder) != 1 || !slices.Contains(tpl.Kinds, KindDM) {
		return invalidText(0, "template_kind")
	}
	text, err := checkText(KindDM, c.Platform, strings.Replace(tpl.Body, linkPlaceholder, link, 1))
	if err != nil {
		return err
	}
	// The digest never contains the link (a bearer secret that differs per issue): the template body and the bundle make it stable, so the
	// A1.3 lcn-dup lock key is meaningful and the stored hash is not a function of the bearer.
	hmac, err := s.keys.bodyHMAC(scope.TenantID, "crm|"+c.State+"|"+c.BundleID+"|"+tpl.Body)
	if err != nil {
		return err
	}
	psid, err := s.latestSender(ctx, tx, c.ConversationID)
	if err != nil {
		return err
	}
	p, err := s.prepare(ctx, tx, scope, KindDM, psid, text, hmac)
	if err != nil {
		return err
	}
	// Calls inbox.plan_checkout_reminder (0144): definer commerce_integration_writer; lcn-dup lock, window/takeover/capability/state re-check, the
	// once-per-buyer row, then inbox.lcn_emit (operation, outbound row, sealed secret, audit). Never a takeover (origin=auto).
	var op string
	if err := tx.QueryRow(ctx, `SELECT inbox.plan_checkout_reminder($1::uuid,$2::uuid,$3::uuid,'manual',$4::bigint,$5,$6::uuid,$7::bigint,$8::uuid,$9::bytea,$10,$11::bytea,$12::bytea,$13::bytea,$14::bytea,$15,$16::bigint)::text`,
		sessionID, c.BundleID, c.ConversationID, c.Generation, c.State, p.operation, p.job, p.outbound, p.hmac, p.keyID, p.nonce, p.ciphertext,
		p.enc, p.sealed, tplID, int64(1)).Scan(&op); err != nil {
		return databaseError(err)
	}
	return nil
}

// ReminderItem is one buyer of the report: the bundle, the manual label (null for Meta buyers: no names are stored) and, for a queued DM,
// its visible send state (§4.4) and code.
type ReminderItem struct {
	BundleID      string  `json:"bundle_id"`
	DisplayName   *string `json:"display_name"`
	ReminderState string  `json:"reminder_state"`
	OperationID   *string `json:"operation_id,omitempty"`
	SendState     string  `json:"send_state,omitempty"`
	Code          *string `json:"code,omitempty"`
}

// FollowupItem is a buyer the merchant must handle (reason window_closed | human_takeover | no_peer | capability | link_unavailable); copy the
// link from the report's link field (link_copy_allowed is false when the store has no active storefront domain). The link is the store's
// non-bearer checkout URL: it never carries a buyer secret.
type FollowupItem struct {
	BundleID        string  `json:"bundle_id"`
	DisplayName     *string `json:"display_name"`
	ReminderState   string  `json:"reminder_state"`
	Reason          string  `json:"reason"`
	LinkCopyAllowed bool    `json:"link_copy_allowed"`
}

// ReminderReport is GET …/reminders: Sent are DMs that reached Messenger/IG, Queued counts DMs still in the ledger, Failed carries the
// failed / blocked / unknown ones, Link is the non-bearer checkout URL to copy (null without a storefront domain).
type ReminderReport struct {
	Sent     []ReminderItem `json:"sent"`
	Queued   int            `json:"queued"`
	Failed   []ReminderItem `json:"failed"`
	Followup []FollowupItem `json:"followup"`
	Link     *string        `json:"link"`
}

// ReminderReport reads the session's reminder rows (inbox:read) and the copyable link. Calls inbox.reminder_report (0144).
func (s *Service) ReminderReport(ctx context.Context, tx pgx.Tx, sessionID string) (ReminderReport, error) {
	out := ReminderReport{Sent: []ReminderItem{}, Failed: []ReminderItem{}, Followup: []FollowupItem{}}
	if !command.ValidID(sessionID) {
		return out, command.ErrInvalid
	}
	rows, err := tx.Query(ctx, `SELECT bundle_id::text, label, outcome, reason, reminder_state, operation_id::text, op_state, result_code, locale
		FROM inbox.reminder_report($1::uuid)`, sessionID)
	if err != nil {
		return out, databaseError(err)
	}
	type row struct {
		bundle, outcome, state, locale         string
		label, reason, op, opState, resultCode *string
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.bundle, &r.label, &r.outcome, &r.reason, &r.state, &r.op, &r.opState, &r.resultCode, &r.locale); err != nil {
			rows.Close()
			return out, databaseError(err)
		}
		all = append(all, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, databaseError(err)
	}
	var origin string
	if origins, err := s.storeOrigins(ctx, tx); err != nil {
		return out, err
	} else if len(origins) > 0 {
		origin = origins[0]
	}
	if origin != "" {
		locale := "zh-TW" // the session's claim-source locale rides on every row; no rows = nothing to copy for yet
		if len(all) > 0 {
			locale = all[0].locale
		}
		link := origin + "/" + locale + "/checkout"
		out.Link = &link
	}
	for _, r := range all {
		if r.outcome == "followup" {
			out.Followup = append(out.Followup, FollowupItem{BundleID: r.bundle, DisplayName: r.label, ReminderState: r.state,
				Reason: deref(r.reason), LinkCopyAllowed: origin != ""})
			continue
		}
		it := ReminderItem{BundleID: r.bundle, DisplayName: r.label, ReminderState: r.state, OperationID: r.op, Code: r.resultCode}
		st, ok := sendState(r.opState)
		if !ok {
			st = "unknown" // operation row purged by retention: the DM's fate is no longer provable
		}
		it.SendState = st
		switch st {
		case "sent":
			out.Sent = append(out.Sent, it)
		case "queued":
			out.Queued++
		default:
			out.Failed = append(out.Failed, it)
		}
	}
	return out, nil
}
