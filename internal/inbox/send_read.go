// Purpose: the A9 merge of the merchant's own sent messages (live-console-v1 §3.4/§4.4): opens the display copies of
// inbox.outbound_messages and maps the operation state to the visible send_state (queued | sent | failed | blocked | unknown).
// Depends on: SQL inbox.read_outbound (migration 0128), internal/inbox/keyring.go (openOutbound).
// Used by: internal/inbox/read.go (ReadThread).
// Invariants: a display copy that cannot be opened renders unreadable, never dropped; no link ever persists in a display copy (send.go).

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

// outboundItems reads up to limit newest sends of the conversation (inbox:read) and returns those that fall inside the page's time
// window: all of them when the inbound page is not full, else only those not older than the oldest inbound row shown.
func (s *Service) outboundItems(ctx context.Context, tx pgx.Tx, conversationID string, limit int, inbound []MessageItem) ([]MessageItem, error) {
	var floor *time.Time
	if len(inbound) >= limit {
		for _, it := range inbound {
			if it.At != nil && (floor == nil || it.At.Before(*floor)) {
				t := *it.At
				floor = &t
			}
		}
	}
	rows, err := tx.Query(ctx, `SELECT outbound_id::text, kind, principal_id::text, key_id, nonce, ciphertext, created_at, send_state, result_code,
			tenant_id::text, store_id::text
		FROM inbox.read_outbound($1::uuid, $2::integer)`, conversationID, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	var out []MessageItem
	for rows.Next() {
		var id, kind, principal, keyID, tenant, store string
		var nonce, ct []byte
		var at time.Time
		var state *string
		var code *string
		if err := rows.Scan(&id, &kind, &principal, &keyID, &nonce, &ct, &at, &state, &code, &tenant, &store); err != nil {
			return nil, databaseError(err)
		}
		if floor != nil && at.Before(*floor) {
			continue
		}
		when := at
		k, p := kind, principal
		item := MessageItem{Direction: "out", At: &when, Attachments: []attachmentView{}, Kind: &k, PrincipalID: &p}
		if st, ok := sendState(state); ok {
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
	return out, databaseErrorOrNil(rows.Err())
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
