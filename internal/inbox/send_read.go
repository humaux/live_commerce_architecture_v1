// Purpose: the A9 merge of the merchant's own sent messages (live-console-v1 §3.4/§4.4): opens the display copies of
// inbox.outbound_messages and maps the operation state to the visible send_state (queued | sent | failed | blocked | unknown).
// Depends on: SQL inbox.read_outbound (migration 0128), internal/inbox/keyring.go (openOutbound).
// Used by: internal/inbox/read.go (ReadThread).
// Invariants: a display copy that cannot be opened renders unreadable, never dropped; no link ever persists in a display copy (send.go).
// UNKNOWN authority uses unfiltered operation states; incomplete scans and missing/unrecognized states never authorize sending.

package inbox

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// sendState maps an operation state to §4.4's visible state; ok=false when the operation row is gone.
func sendState(state *string) (string, bool) {
	if state == nil {
		return "", false
	}
	switch *state {
	case "READY", "DISPATCHING":
		return "queued", true
	case "SUCCEEDED", "ACKNOWLEDGED":
		return "sent", true
	case "FAILED_FINAL":
		return "failed", true
	case "BLOCKED_POLICY", "STALE_BINDING", "CANCELLED":
		return "blocked", true
	case "UNKNOWN":
		return "unknown", true
	}
	return "", false
}

// outboundItems scans the newest 50 sends for A9 authority on every page. Only the first page renders the first limit rows inside
// the existing inbound time window. Authority is independent of that display filtering and of display-copy decryption.
func (s *Service) outboundItems(ctx context.Context, tx pgx.Tx, conversationID string, limit int, inbound []MessageItem, display bool) ([]MessageItem, *bool, error) {
	var floor *time.Time
	if len(inbound) >= limit {
		for _, it := range inbound {
			if it.At != nil && (floor == nil || it.At.Before(*floor)) {
				t := *it.At
				floor = &t
			}
		}
	}
	// Calls inbox.read_outbound (live-console-v1 §11 A9); its scoped SECURITY DEFINER caps reads at 50.
	rows, err := tx.Query(ctx, `SELECT outbound_id::text, kind, principal_id::text, key_id, nonce, ciphertext, created_at, send_state, result_code,
			tenant_id::text, store_id::text
		FROM inbox.read_outbound($1::uuid, $2::integer)`, conversationID, 50)
	if err != nil {
		return nil, nil, databaseError(err)
	}
	defer rows.Close()
	var out []MessageItem
	count, allKnown, hasUnknown := 0, true, false
	for rows.Next() {
		var id, kind, principal, keyID, tenant, store string
		var nonce, ct []byte
		var at time.Time
		var state *string
		var code *string
		if err := rows.Scan(&id, &kind, &principal, &keyID, &nonce, &ct, &at, &state, &code, &tenant, &store); err != nil {
			return nil, nil, databaseError(err)
		}
		count++
		st, known := sendState(state)
		allKnown = allKnown && known
		hasUnknown = hasUnknown || st == "unknown"
		if !display || count > limit || (floor != nil && at.Before(*floor)) {
			continue
		}
		when := at
		k, p := kind, principal
		item := MessageItem{Direction: "out", At: &when, Attachments: []attachmentView{}, Kind: &k, PrincipalID: &p}
		if known {
			item.SendState = &st
			if (st == "failed" || st == "blocked") && code != nil && *code != "" {
				item.SendCode = code
			}
		}
		text, err := s.keys.openOutbound(tenant, store, id, kind, keyID, nonce, ct)
		if err != nil {
			markUnreadable(&item)
		} else {
			item.Text = text
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, databaseError(err)
	}
	// ponytail: the frozen definer's 50-row cap cannot exclude an older UNKNOWN. Upgrade to an unbounded scoped DB fact
	// when its contract/migration is authorized; until then a full or unrecognized scan deliberately fails closed.
	if hasUnknown || (count < 50 && allKnown) {
		return out, &hasUnknown, nil
	}
	return out, nil, nil
}

// sortThread orders a merged page newest first (inbound by time then seq; outbound by time).
func sortThread(items []MessageItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].At, items[j].At
		switch {
		case a == nil && b == nil:
			return items[i].Seq > items[j].Seq
		case a == nil:
			return false
		case b == nil:
			return true
		case a.Equal(*b):
			return items[i].Seq > items[j].Seq
		}
		return a.After(*b)
	})
}
