// Purpose: the inbox read side's definer callers and projections: A8 ListConversations (metadata only, no ciphertext),
// A9 ReadThread (header + opened/decrypted inbound messages), A13 BuyerPanel (conversation-scoped fields), and the
// shared databaseError mapper that passes only the fixed PT400/PT403/PT404/PT409/PT422 codes through.
// Depends on: the SECURITY DEFINER functions social.list_conversations / social.read_thread / social.conversation_meta /
// social.unread_conversation_count / inbox.thread_opened (migration 0119), internal/inbox/keyring.go + classifier.go.
// Used by: internal/httpapi/inbox.go (A8/A9/A13 handlers), internal/inbox tests.

package inbox

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"livecommerce/internal/command"
)

// ListRequest is the A8 filter and keyset position. The httpapi layer decodes the opaque cursor into LastAt/CursorID;
// the definer re-scopes every read to the transaction's tenant/store, so a cursor never widens authority.
type ListRequest struct {
	Filter   string
	Limit    int
	LastAt   *time.Time
	CursorID *string
}

// ListConversations is A8: conversation metadata only (no ciphertext), ordered by last_at DESC, conversation_id DESC.
func (s *Service) ListConversations(ctx context.Context, tx pgx.Tx, req ListRequest) (ConversationList, error) {
	out := ConversationList{Items: []ConversationItem{}}
	rows, err := tx.Query(ctx, `SELECT conversation_id::text, platform, last_at, unread, unreplied, mode, assignee::text, window_open_until, linked_customer_id::text
		FROM social.list_conversations($1::text, $2::timestamptz, $3::uuid, $4::integer)`,
		req.Filter, req.LastAt, req.CursorID, req.Limit)
	if err != nil {
		return out, databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item ConversationItem
		// A conversation row without an inbound message has no window (NULL or -infinity last_at / window_open_until); it renders with zero times
		// instead of failing the whole page.
		var lastAt, windowUntil pgtype.Timestamptz
		var unread, unreplied *bool
		if err := rows.Scan(&item.ConversationID, &item.Platform, &lastAt, &unread, &unreplied,
			&item.Mode, &item.Assignee, &windowUntil, &item.LinkedCustomerID); err != nil {
			return out, databaseError(err)
		}
		item.Unread, item.Unreplied = unread != nil && *unread, unreplied != nil && *unreplied
		if lastAt.Valid && lastAt.InfinityModifier == pgtype.Finite {
			item.LastAt = lastAt.Time
		}
		if windowUntil.Valid && windowUntil.InfinityModifier == pgtype.Finite {
			item.WindowOpenUntil = windowUntil.Time
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return out, databaseError(err)
	}
	rows.Close()
	if err := s.fillDisplayNames(ctx, tx, out.Items); err != nil {
		return out, err
	}
	// Amendment 1 P2-2: bundles whose claim link could not be sent appear as bundle-only items on the first page (copy-link only).
	if req.LastAt == nil && req.Filter != "messenger" && req.Filter != "instagram" {
		bundles, err := s.linkPendingItems(ctx, tx, 10)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, bundles...)
	}
	if err := tx.QueryRow(ctx, `SELECT social.unread_conversation_count()`).Scan(&out.UnreadTotal); err != nil {
		return out, databaseError(err)
	}
	return out, nil
}

// fillDisplayNames sets display_name from the newest inbound envelope of each listed conversation (Amendment 1 P2-1); null when the
// sender carries no name or the envelope is unreadable. Calls social.conversation_heads (migration 0128), <= 50 ids.
func (s *Service) fillDisplayNames(ctx context.Context, tx pgx.Tx, items []ConversationItem) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	index := make(map[string]int, len(items))
	for i, it := range items {
		ids[i] = it.ConversationID
		index[it.ConversationID] = i
	}
	rows, err := tx.Query(ctx, `SELECT conversation_id::text, event_id::text, key_id, nonce, ciphertext, app_id, object, asset_id, event_key, payload_hash,
			tenant_id::text, store_id::text, route_id::text, route_epoch, server_seq, occurred_at
		FROM social.conversation_heads($1::uuid[])`, ids)
	if err != nil {
		return databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var conv string
		var r threadRow
		if err := rows.Scan(&conv, &r.eventID, &r.keyID, &r.nonce, &r.ciphertext, &r.appID, &r.object, &r.assetID,
			&r.eventKey, &r.payloadHash, &r.tenantID, &r.storeID, &r.routeID, &r.routeEpoch, &r.serverSeq, &r.occurredAt); err != nil {
			return databaseError(err)
		}
		plain, err := s.keys.open(eventContext{EventID: r.eventID, AppID: r.appID, Object: r.object, EventKey: r.eventKey, PayloadHash: r.payloadHash,
			TenantID: r.tenantID, StoreID: r.storeID, RouteID: r.routeID, RouteEpoch: r.routeEpoch}, r.keyID, r.nonce, r.ciphertext)
		if err != nil {
			continue
		}
		if name := senderName(plain); name != nil {
			if i, ok := index[conv]; ok {
				items[i].DisplayName = name
			}
		}
	}
	return databaseErrorOrNil(rows.Err())
}

// linkPendingItems lists the bundle-only A8 items (inbox.link_pending_bundles, migration 0128).
func (s *Service) linkPendingItems(ctx context.Context, tx pgx.Tx, limit int) ([]ConversationItem, error) {
	rows, err := tx.Query(ctx, `SELECT bundle_id::text, session_id::text, created_at FROM inbox.link_pending_bundles($1::integer)`, limit)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	var out []ConversationItem
	for rows.Next() {
		var bundle, session string
		var at time.Time
		if err := rows.Scan(&bundle, &session, &at); err != nil {
			return nil, databaseError(err)
		}
		b, sid := bundle, session
		out = append(out, ConversationItem{BundleOnly: true, BundleID: &b, SessionID: &sid, Platform: "messenger", LastAt: at, Unreplied: true, LinkPendingManual: true})
	}
	return out, databaseErrorOrNil(rows.Err())
}

func databaseErrorOrNil(err error) error {
	if err == nil {
		return nil
	}
	return databaseError(err)
}

// threadRow is one social.read_thread output: the envelope plus the immutable AAD context of the source event.
type threadRow struct {
	eventID, keyID, appID, object, assetID, eventKey, payloadHash string
	tenantID, storeID, routeID                                    string
	nonce, ciphertext                                             []byte
	routeEpoch, serverSeq                                         int64
	occurredAt                                                    *time.Time
	direction                                                     string
}

// ReadThread is A9: the thread header (conversation_meta), the coalesced thread_opened audit, and the decrypted
// inbound messages. A message that fails the keyring open or the frozen-classifier replay renders unreadable (never
// dropped); plaintext exists only in the returned items.
func (s *Service) ReadThread(ctx context.Context, tx pgx.Tx, conversationID string, beforeSeq *int64, limit int) (ThreadView, error) {
	out := ThreadView{Items: []MessageItem{}}
	// The audit mark also proves the conversation exists in the caller's scope (PT404 otherwise).
	if _, err := tx.Exec(ctx, `SELECT inbox.thread_opened($1::uuid)`, conversationID); err != nil {
		return out, databaseError(err)
	}
	meta, err := s.conversationMeta(ctx, tx, conversationID)
	if err != nil {
		return out, err
	}
	out.WindowOpenUntil = meta.windowOpenUntil
	out.Mode = meta.mode
	out.TakeoverGeneration = meta.generation
	out.HumanUntil = meta.humanUntil

	rows, err := tx.Query(ctx, `SELECT event_id::text, key_id, nonce, ciphertext, app_id, object, asset_id, event_key, payload_hash,
			tenant_id::text, store_id::text, route_id::text, route_epoch, server_seq, occurred_at, direction
		FROM social.read_thread($1::uuid, $2::bigint, $3::integer)`, conversationID, beforeSeq, limit)
	if err != nil {
		return out, databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var r threadRow
		if err := rows.Scan(&r.eventID, &r.keyID, &r.nonce, &r.ciphertext, &r.appID, &r.object, &r.assetID,
			&r.eventKey, &r.payloadHash, &r.tenantID, &r.storeID, &r.routeID, &r.routeEpoch, &r.serverSeq,
			&r.occurredAt, &r.direction); err != nil {
			return out, databaseError(err)
		}
		out.Items = append(out.Items, s.openMessage(r))
	}
	if err := rows.Err(); err != nil {
		return out, databaseError(err)
	}
	rows.Close()
	if beforeSeq == nil {
		// Outbound rows (the merchant's own sends) are merged by time on the first page only.
		// ponytail: older pages show inbound rows only; the UI pages outbound through the first page's window. Upgrade path: a
		// (conversation, created_at) keyset on inbox.read_outbound when threads outgrow one inbound page.
		outbound, err := s.outboundItems(ctx, tx, conversationID, limit, out.Items)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, outbound...)
		sortThread(out.Items)
	}
	return out, nil
}

// openMessage opens one stored body and replays the classifier; any failure renders the item unreadable.
func (s *Service) openMessage(r threadRow) MessageItem {
	item := MessageItem{Direction: r.direction, Seq: r.serverSeq, At: r.occurredAt, Attachments: []attachmentView{}}
	plain, err := s.keys.open(eventContext{
		EventID: r.eventID, AppID: r.appID, Object: r.object, EventKey: r.eventKey, PayloadHash: r.payloadHash,
		TenantID: r.tenantID, StoreID: r.storeID, RouteID: r.routeID, RouteEpoch: r.routeEpoch,
	}, r.keyID, r.nonce, r.ciphertext)
	if err != nil {
		markUnreadable(&item)
		return item
	}
	view, err := replayMessage(plain, r.assetID)
	if err != nil {
		markUnreadable(&item)
		return item
	}
	item.Text = view.Text
	item.Attachments = view.Attachments
	return item
}

func markUnreadable(item *MessageItem) {
	t := true
	item.Unreadable = &t
}

// conversationMeta is the A9 header / A13 conversation-scoped projection (no ciphertext). Zero rows means the
// conversation is not in the caller's scope.
type conversationMeta struct {
	platform         string
	mode             string
	assignee         *string
	generation       int64
	humanUntil       *time.Time
	windowOpenUntil  time.Time
	linkedCustomerID *string
}

func (s *Service) conversationMeta(ctx context.Context, tx pgx.Tx, conversationID string) (conversationMeta, error) {
	var m conversationMeta
	err := tx.QueryRow(ctx, `SELECT platform, mode, assignee::text, takeover_generation, human_until, window_open_until, linked_customer_id::text
		FROM social.conversation_meta($1::uuid)`, conversationID).
		Scan(&m.platform, &m.mode, &m.assignee, &m.generation, &m.humanUntil, &m.windowOpenUntil, &m.linkedCustomerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, command.ErrNotFound
	}
	if err != nil {
		return m, databaseError(err)
	}
	return m, nil
}

// BuyerPanel is A13. LC-B3 implements the conversation-scoped fields only; claims, orders, purchase_ordinal and
// auto_reply are LC-B4 and always empty/omitted here (documented in DELIVERY.md).
func (s *Service) BuyerPanel(ctx context.Context, tx pgx.Tx, conversationID string) (BuyerPanel, error) {
	out := BuyerPanel{PurchaseOrdinal: 0, Claims: []ClaimRef{}, ClaimTotalMinor: 0, Orders: []OrderRef{}}
	meta, err := s.conversationMeta(ctx, tx, conversationID)
	if err != nil {
		return out, err
	}
	out.Platform = meta.platform
	out.LinkedCustomerID = meta.linkedCustomerID
	out.WindowOpenUntil = &meta.windowOpenUntil
	// Calls inbox.link_pending_for (migration 0128): the flag of the bundles linked to this conversation's peer.
	if err := tx.QueryRow(ctx, `SELECT link_pending_manual FROM inbox.link_pending_for($1::uuid, NULL)`, conversationID).Scan(&out.LinkPendingManual); err != nil {
		return out, databaseError(err)
	}
	return out, nil
}

// BuyerPanelByBundle is A13 with ?bundle_id= (Amendment 1 A1.1/P2-2): the bundle-scoped panel. It carries the platform and the
// link_pending_manual flag; claims/orders stay empty here (their read model is a later unit) and a bundle outside the caller's
// scope is a 404.
func (s *Service) BuyerPanelByBundle(ctx context.Context, tx pgx.Tx, bundleID string) (BuyerPanel, error) {
	out := BuyerPanel{Claims: []ClaimRef{}, Orders: []OrderRef{}}
	err := tx.QueryRow(ctx, `SELECT platform, link_pending_manual FROM inbox.link_pending_for(NULL, $1::uuid)`, bundleID).Scan(&out.Platform, &out.LinkPendingManual)
	if err != nil {
		return out, databaseError(err)
	}
	return out, nil
}

// databaseError passes the fixed definer codes through unchanged (the httpapi inbox classifier maps them) and flattens
// every other database failure to a fixed sentinel that never carries a driver message.
func databaseError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400", "PT403", "PT404", "PT409", "PT422", "PT429":
			return err
		}
	}
	return ErrDatabase
}
