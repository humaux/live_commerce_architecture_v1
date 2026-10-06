// Purpose: W3-03B merchant-triggered checkout reminders (live-console-v1 Amendment W3-03B): one scan of the session's unpaid buyers, then ONE
// transaction PER BUYER that issues the link that really completes the purchase and plans the DM in the same transaction — an unpaid
// merchant-created order gets a re-issued order link (fulfillment.regenerate_order_link, the LC-B6 / RegenerateLink rules, template
// order-pay-link/v1), a claim nobody opened gets a re-issued claim link (claims.issue_link, template checkout-reminder/v1). A refusal of one buyer
// (window closed since the scan, takeover, rate cap, state changed) rolls back only that buyer's link issue and is reported per buyer; the batch goes on.
// Depends on: internal/inbox (ScanCheckoutReminders, PlanCheckoutReminder), internal/claims (IssueLink), ManualOrders (capability derivation and the
//   order-link regenerate call, manual.go), internal/platform (WithScope), SQL 0144 + fulfillment.regenerate_order_link (0104).
// Used by: internal/httpapi/reminders.go (POST …/reminders[/{bundle_id}]); cmd/api builds it beside the ManualOrders pipeline and the inbox service.
// Invariants: the bearer link exists only in memory, in the sealed dispatch copy and as its own hash in the claims/order link tables (never in a display
//   copy, a log or a receipt); no send outside the 24 h window (the planner and Check refuse); one reminder per buyer per session; no new bearer type.
// Status: MOCK (REAL_PG + fake Graph).

package merchanttools

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"regexp"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

// ReminderInbox is the part of inbox.Service the reminders use (a seam for DB-free tests).
type ReminderInbox interface {
	ScanCheckoutReminders(ctx context.Context, tx pgx.Tx, sessionID, bundleID string) (inbox.ReminderScan, error)
	PlanCheckoutReminder(ctx context.Context, tx pgx.Tx, scope platform.Scope, sessionID string, c inbox.ReminderCandidate, link string) error
}

// CheckoutReminders runs the reminder pass. manual may be nil (no order-link re-issue: unpaid orders become link_unavailable refusals).
type CheckoutReminders struct {
	pool   *pgxpool.Pool
	manual *ManualOrders
	inbox  ReminderInbox
}

// NewCheckoutReminders wires the pass; pool is the commerce_runtime pool, in the inbox service the send side must be enabled.
func NewCheckoutReminders(pool *pgxpool.Pool, manual *ManualOrders, in ReminderInbox) (*CheckoutReminders, error) {
	if pool == nil || in == nil {
		return nil, command.ErrInvalid
	}
	return &CheckoutReminders{pool: pool, manual: manual, inbox: in}, nil
}

// ReminderOutcome is one buyer's result: queued | followup (code = the follow-up reason) | restricted | refused (code = the planner / link refusal).
type ReminderOutcome struct {
	BundleID string `json:"bundle_id"`
	Outcome  string `json:"outcome"`
	Code     string `json:"code,omitempty"`
}

// ReminderResult is the POST …/reminders response. Results lists every buyer handled this pass except the already-reminded ones (counted only).
type ReminderResult struct {
	Queued          int `json:"queued"`
	AlreadyReminded int `json:"already_reminded"`
	Followup        int `json:"followup"`
	// Restricted counts buyers skipped because the merchant restricted them (W3-05B blocklist): no link is issued and no DM planned. Not part of Followup.
	Restricted int               `json:"restricted"`
	Refused    int               `json:"refused"`
	Truncated  bool              `json:"truncated"`
	Results    []ReminderOutcome `json:"results"`
}

// Trigger runs one pass for the session: the batch over every eligible buyer, or the single buyer bundleID. token is the merchant bearer (the link
// issuers re-verify it in the database), key the request's Idempotency-Key (per-buyer link keys derive from it). Needs inbox:reply and live:manage; the
// order-link re-issue also needs inventory:reserve and catalog:read (a buyer whose link step is refused for lack of them is a per-buyer refusal).
// Side effects: follow-up rows + one audit row (scan transaction), and per sent buyer a link issue, River job, operation, outbound row, sealed copy.
func (r *CheckoutReminders) Trigger(ctx context.Context, token, storeID, key, sessionID, bundleID string) (ReminderResult, error) {
	out := ReminderResult{Results: []ReminderOutcome{}}
	if !command.ValidID(sessionID) || (bundleID != "" && !command.ValidID(bundleID)) {
		return out, command.ErrInvalid
	}
	var scan inbox.ReminderScan
	err := platform.WithScope(ctx, r.pool, token, storeID, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
		var err error
		scan, err = r.inbox.ScanCheckoutReminders(ctx, tx, sessionID, bundleID)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Truncated = scan.Truncated
	needsLink := slices.ContainsFunc(scan.Candidates, func(c inbox.ReminderCandidate) bool { return c.Verdict == "send" })
	if needsLink && scan.Origin == "" {
		return out, &inbox.SendError{Status: http.StatusConflict, Code: "no_storefront"}
	}
	for _, c := range scan.Candidates {
		switch c.Verdict {
		case "already_reminded":
			out.AlreadyReminded++
		case "restricted":
			out.Restricted++
			out.Results = append(out.Results, ReminderOutcome{BundleID: c.BundleID, Outcome: "restricted"})
		case "send":
			code, err := r.remind(ctx, token, storeID, key, sessionID, scan.Origin, c)
			if err != nil {
				return out, err
			}
			if code == "" {
				out.Queued++
				out.Results = append(out.Results, ReminderOutcome{BundleID: c.BundleID, Outcome: "queued"})
			} else {
				out.Refused++
				out.Results = append(out.Results, ReminderOutcome{BundleID: c.BundleID, Outcome: "refused", Code: code})
			}
		default:
			out.Followup++
			out.Results = append(out.Results, ReminderOutcome{BundleID: c.BundleID, Outcome: "followup", Code: c.Verdict})
		}
	}
	return out, nil
}

// remind runs one buyer's transaction: issue the link, plan the DM. It returns the refusal code ("" = planned) for a per-buyer refusal (the
// transaction, link issue included, was rolled back) and an error only for failures that end the pass (database, authorization of the bearer).
func (r *CheckoutReminders) remind(ctx context.Context, token, storeID, key, sessionID, origin string, c inbox.ReminderCandidate) (string, error) {
	buyerKey := stepKey(key, "crm|"+c.BundleID) // 8..128 safe ASCII; one link key per (request, buyer)
	err := platform.WithScope(ctx, r.pool, token, storeID, "inbox:reply", func(tx pgx.Tx, scope platform.Scope) error {
		link, err := r.issue(ctx, tx, scope, token, buyerKey, sessionID, origin, c)
		if err != nil {
			return err
		}
		return r.inbox.PlanCheckoutReminder(ctx, tx, scope, sessionID, c, link)
	})
	if err == nil {
		return "", nil
	}
	if code := refusalCode(err); code != "" {
		return code, nil
	}
	return "", err
}

// issue creates the link of the buyer's state inside tx and returns its full URL (the only place the bearer exists besides its hash).
func (r *CheckoutReminders) issue(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, buyerKey, sessionID, origin string, c inbox.ReminderCandidate) (string, error) {
	switch c.State {
	case inbox.StateClaimed:
		// Calls claims.issue_link (0060) through claims.IssueLink: live:manage re-resolved in the database, CAS on the scanned generation, the old
		// token dies. The same mechanism and the same /claim URL as the original claim-link push; no new bearer type.
		issued, err := claims.IssueLink(ctx, tx, scope, token, buyerKey, sessionID, c.BundleID, claims.LinkInput{ExpectedGeneration: c.LinkGeneration})
		if err != nil {
			return "", err
		}
		if issued.Token == "" {
			return "", errLinkReplayed // a receipt of this key exists but its plan did not commit: nothing can be delivered from a replay
		}
		return origin + "/" + c.Locale + "/claim#t=" + string(issued.Token), nil
	case inbox.StateAwaitingPayment:
		if r.manual == nil || c.OrderID == "" {
			return "", errLinkUnavailable
		}
		// Same derivation and rules as ManualOrders.RegenerateLink (manual.go): a NEW single-use order link derived from the key, every unused old
		// link of the order dies, only merchant_manual orders qualify (fulfillment.regenerate_order_link re-checks the bearer + source).
		if err := requireCatalog(ctx, tx, scope, token); err != nil {
			return "", err
		}
		linkToken := r.manual.capability("relink", scope.StoreID, buyerKey)
		hash := sha256.Sum256([]byte(token))
		linkHash := sha256.Sum256([]byte(linkToken))
		if _, err := tx.Exec(ctx, `SELECT fulfillment.regenerate_order_link($1,$2::uuid,$3::uuid,$4)`, hash[:], scope.StoreID, c.OrderID, linkHash[:]); err != nil {
			return "", err
		}
		return origin + "/" + c.Locale + "/order-link#o=" + c.OrderID + "&t=" + linkToken, nil
	}
	return "", errLinkUnavailable
}

var refusalPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

var (
	errLinkReplayed    = errors.New("reminder link receipt replayed")
	errLinkUnavailable = errors.New("reminder link unavailable")
)

// refusalCode maps a per-buyer failure to its fixed code, or "" when the failure must end the pass (unknown errors never leak a driver message).
func refusalCode(err error) string {
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, errLinkReplayed):
		return "link_replayed"
	case errors.Is(err, errLinkUnavailable):
		return "link_unavailable"
	case errors.Is(err, command.ErrConflict):
		return "link_changed" // the claim-link generation moved since the scan (another issue or a release)
	case errors.Is(err, platform.ErrForbidden):
		return "forbidden"
	case errors.As(err, &pg) && pg.Code == "PT409":
		if !refusalPattern.MatchString(pg.Message) {
			return "conflict" // e.g. regenerate_order_link's "not a manual order": never pass free text through
		}
		return pg.Message // the planner refusal: window_closed, human_takeover, takeover_changed, capability, already_reminded, not_remindable, ...
	case errors.As(err, &pg) && pg.Code == "PT429":
		return "rate_limited"
	case errors.As(err, &pg) && pg.Code == "PT403":
		return "forbidden"
	}
	return ""
}
