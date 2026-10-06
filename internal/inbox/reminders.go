// Purpose: W3-03B checkout reminders as service methods: PlanCheckoutReminders (one merchant-triggered pass over a session's unpaid buyers
// that plans at most one 24 h-window DM per buyer through the existing LC-B4 send path), ReminderReport (what was sent / is queued / needs a
// manual follow-up) and the per-store reminder settings. It decides no rule: inbox.checkout_reminder_candidates classifies, inbox.plan_
// checkout_reminder re-validates and plans; this file renders the fixed checkout-reminder/v1 template, seals both copies and plans in
// ONE scoped merchant transaction. No network call.
// Depends on: internal/command, internal/msgtemplates (Resolve checkout-reminder/v1), internal/inbox send.go (prepare, latestSender),
// send_text.go (checkText), keyring.go (bodyHMAC); SQL migration 0131 (inbox.checkout_reminder_candidates, inbox.plan_checkout_reminder,
// inbox.reminder_report, live.get_reminder_settings, live.put_reminder_settings) and inbox.store_origins (0128).
// Used by: internal/httpapi/reminders.go; cmd/api (through the inbox service the send side was enabled on).
// Invariants: no message tag / no send outside the 24 h window (planner + Check refuse, never retried); one reminder per buyer per session;
// the reminder link is the store's non-bearer checkout URL (origin + /<locale>/checkout): it is sealed in the dispatch copy and scrubbed from
// the display copy like every link (send.go prepare); PSID only in memory and the sealed copy.
// Status: MOCK (REAL_PG + fake Graph).

package inbox

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// reminderTemplate is the fixed template (0131) every checkout reminder renders; reminderLimit bounds one trigger so the planning
// transaction stays inside the request budget (the merchant triggers again for the rest; reminded buyers are skipped).
const (
	reminderTemplate = "checkout-reminder/v1"
	reminderLimit    = 100
)

// ReminderOutput is the POST …/reminders response: how many DMs were planned and why the others were not.
type ReminderOutput struct {
	Queued          int  `json:"queued"`
	AlreadyReminded int  `json:"already_reminded"`
	Followup        int  `json:"followup"`
	Skipped         int  `json:"skipped"`
	Truncated       bool `json:"truncated"`
}

// reminderCandidate is one row of inbox.checkout_reminder_candidates.
type reminderCandidate struct {
	bundle, verdict, conversation, platform, locale string
	generation                                      *int64
}

// PlanCheckoutReminders runs one reminder pass for a session (inbox:reply): the batch over every eligible buyer, or the single buyer bundleID
// (non-empty; 409 not_remindable when she is no candidate): it records follow-up rows for buyers that cannot be reached
// in the window and plans one meta.dm_send (origin=auto) per reachable buyer. Replaying the same Idempotency-Key returns the first answer;
// a new key re-evaluates (already reminded buyers are never reminded twice). Side effects: command receipt, follow-up rows, and per sent
// buyer a River job + operation + outbound row + sealed dispatch copy (inbox.plan_checkout_reminder).
func (s *Service) PlanCheckoutReminders(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, sessionID, bundleID string) (ReminderOutput, error) {
	var out ReminderOutput
	if !s.SendEnabled() {
		return out, ErrSendUnavailable
	}
	if !command.ValidID(sessionID) || (bundleID != "" && !command.ValidID(bundleID)) {
		return out, command.ErrInvalid
	}
	request := struct {
		Session string `json:"session_id"`
		Bundle  string `json:"bundle_id"`
	}{sessionID, bundleID}
	err := command.Run(ctx, tx, scope, "inbox.checkout_reminder.trigger", key, request, &out, func() error {
		origin, err := s.storeOrigin(ctx, tx)
		if err != nil {
			return err
		}
		body, err := s.reminderBody(ctx, tx)
		if err != nil {
			return err
		}
		// Calls inbox.checkout_reminder_candidates (0131): classifies every candidate buyer, records the follow-up rows, takes the session lock.
		cands, truncated, err := s.reminderCandidates(ctx, tx, sessionID, bundleID)
		if err != nil {
			return err
		}
		out = ReminderOutput{Truncated: truncated}
		for _, c := range cands {
			switch c.verdict {
			case "send":
				skipped, err := s.planReminder(ctx, tx, scope, sessionID, c, origin, body)
				switch {
				case err != nil:
					return err
				case skipped:
					out.Skipped++ // the buyer's thread cannot be read (purged): nothing was written
				default:
					out.Queued++
				}
			case "already_reminded":
				out.AlreadyReminded++
			default:
				out.Followup++
			}
		}
		return nil
	})
	return out, err
}

// storeOrigin is the store's first active storefront origin; no origin means no link to send (409 no_storefront).
func (s *Service) storeOrigin(ctx context.Context, tx pgx.Tx) (string, error) {
	var origins []string
	// Calls inbox.store_origins (0128): public data, scoped by the transaction GUCs.
	if err := tx.QueryRow(ctx, `SELECT inbox.store_origins()`).Scan(&origins); err != nil {
		return "", databaseError(err)
	}
	if len(origins) == 0 {
		return "", &SendError{Status: 409, Code: "no_storefront"}
	}
	return origins[0], nil
}

// reminderBody resolves the fixed template body (it must keep its {{連結}} placeholder and be a dm template).
func (s *Service) reminderBody(ctx context.Context, tx pgx.Tx) (string, error) {
	if s.templates == nil {
		return "", invalidText(0, "template_unavailable")
	}
	r, err := s.templates.Resolve(ctx, tx, reminderTemplate, 1)
	if err != nil {
		if msgtemplates.IsNotFound(err) {
			return "", invalidText(0, "template_not_found")
		}
		return "", err
	}
	if !slices.Contains(r.Kinds, KindDM) || !strings.Contains(r.Body, linkPlaceholder) {
		return "", invalidText(0, "template_kind")
	}
	return r.Body, nil
}

func (s *Service) reminderCandidates(ctx context.Context, tx pgx.Tx, sessionID, bundleID string) ([]reminderCandidate, bool, error) {
	rows, err := tx.Query(ctx, `SELECT bundle_id::text, verdict, conversation_id::text, takeover_generation, platform, locale, truncated
		FROM inbox.checkout_reminder_candidates($1::uuid,'manual',$2,nullif($3,'')::uuid)`, sessionID, reminderLimit, bundleID)
	if err != nil {
		return nil, false, databaseError(err)
	}
	defer rows.Close()
	var out []reminderCandidate
	truncated := false
	for rows.Next() {
		var c reminderCandidate
		var bundle, conv, plat *string
		var trunc bool
		if err := rows.Scan(&bundle, &c.verdict, &conv, &c.generation, &plat, &c.locale, &trunc); err != nil {
			return nil, false, databaseError(err)
		}
		if trunc {
			truncated = true
			continue
		}
		c.bundle = deref(bundle)
		c.conversation = deref(conv)
		c.platform = deref(plat)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, false, databaseError(err)
	}
	return out, truncated, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// planReminder plans one DM. It returns skipped=true (nothing written) when the buyer's thread cannot be read; every other refusal of the
// planner (a window / takeover / capability race after the candidate scan, PT409 with the deny code) aborts the whole trigger and rolls the
// transaction back, River jobs included: the merchant triggers again and the scan re-classifies. No savepoint: inbox.lcn_emit pins the job
// row to the top-level transaction id, which a subtransaction insert would not match.
func (s *Service) planReminder(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string, c reminderCandidate, origin, body string) (skipped bool, err error) {
	text := strings.ReplaceAll(body, linkPlaceholder, origin+"/"+c.locale+"/checkout")
	text, err = checkText(KindDM, c.platform, text)
	if err != nil {
		return false, err
	}
	hmac, err := s.keys.bodyHMAC(scope.TenantID, text)
	if err != nil {
		return false, err
	}
	psid, err := s.latestSender(ctx, tx, c.conversation)
	if err != nil {
		var se *SendError
		if errors.As(err, &se) && se.Code == "conversation_gone" {
			return true, nil
		}
		return false, err
	}
	p, err := s.prepare(ctx, tx, scope, KindDM, psid, text, hmac)
	if err != nil {
		return false, err
	}
	tplID, tplVer := reminderTemplate, int64(1)
	var gen int64
	if c.generation != nil {
		gen = *c.generation
	}
	// Calls inbox.plan_checkout_reminder (0131): definer commerce_integration_writer; window/takeover/capability re-check, the once-per-buyer
	// row, then inbox.lcn_emit (operation, outbound row, sealed secret, audit). Never a takeover (origin=auto).
	var op string
	if err := tx.QueryRow(ctx, `SELECT inbox.plan_checkout_reminder($1::uuid,$2::uuid,'manual',$3::bigint,$4::uuid,$5::bigint,$6::uuid,$7::bytea,$8,$9::bytea,$10::bytea,$11::bytea,$12::bytea,$13,$14::bigint)::text`,
		sessionID, c.bundle, gen, p.operation, p.job, p.outbound, p.hmac, p.keyID, p.nonce, p.ciphertext, p.enc, p.sealed, tplID, tplVer).Scan(&op); err != nil {
		return false, databaseError(err)
	}
	return false, nil
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

// FollowupItem is a buyer the merchant must handle (reason window_closed | human_takeover | no_peer | capability); copy the link from the
// report's link field (link_copy_allowed is false when the store has no active storefront domain).
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

// ReminderReport reads the session's reminder rows (inbox:read) and the copyable link. Calls inbox.reminder_report (0131).
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

func (s *Service) storeOrigins(ctx context.Context, tx pgx.Tx) ([]string, error) {
	var origins []string
	if err := tx.QueryRow(ctx, `SELECT inbox.store_origins()`).Scan(&origins); err != nil {
		return nil, databaseError(err)
	}
	return origins, nil
}

// ReminderSettings is the per-store 「場次結束後自動提醒」 setting (default off, 30 min, version 0 = never saved).
type ReminderSettings struct {
	Enabled      bool  `json:"enabled"`
	DelayMinutes int   `json:"delay_minutes"`
	Version      int64 `json:"version"`
}

// ReminderSettingsInput is PUT …/live-settings/reminder: delay 10..1440 minutes; expected_version 0 creates the row.
type ReminderSettingsInput struct {
	Enabled         bool  `json:"enabled"`
	DelayMinutes    int   `json:"delay_minutes"`
	ExpectedVersion int64 `json:"expected_version"`
}

// GetReminderSettings reads the store's setting (live:read). Calls live.get_reminder_settings (0131).
func (s *Service) GetReminderSettings(ctx context.Context, tx pgx.Tx) (ReminderSettings, error) {
	var out ReminderSettings
	if err := tx.QueryRow(ctx, `SELECT enabled, delay_minutes, version FROM live.get_reminder_settings()`).Scan(&out.Enabled, &out.DelayMinutes, &out.Version); err != nil {
		return out, databaseError(err)
	}
	return out, nil
}

// PutReminderSettings stores the setting with an expected_version CAS (live:manage; 409 version_conflict). Calls live.put_reminder_settings
// (0131); audit live.reminder_settings.updated. Nothing here sends: see DELIVERY.md for the automatic trigger.
func (s *Service) PutReminderSettings(ctx context.Context, tx pgx.Tx, in ReminderSettingsInput) (ReminderSettings, error) {
	var out ReminderSettings
	if in.DelayMinutes < 10 || in.DelayMinutes > 1440 || in.ExpectedVersion < 0 {
		return out, command.ErrInvalid
	}
	var v int64
	if err := tx.QueryRow(ctx, `SELECT live.put_reminder_settings($1::boolean,$2::integer,$3::bigint)`, in.Enabled, in.DelayMinutes, in.ExpectedVersion).Scan(&v); err != nil {
		return out, databaseError(err)
	}
	return ReminderSettings{Enabled: in.Enabled, DelayMinutes: in.DelayMinutes, Version: v}, nil
}
