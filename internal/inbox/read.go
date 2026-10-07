// Purpose: the inbox read side's definer callers and projections: A8 ListConversations (metadata only, no ciphertext),
// A9 ReadThread (header + opened/decrypted inbound messages), A13 BuyerPanel / BuyerPanelByBundle (claims, orders, ordinal,
// auto_reply), and the shared databaseError mapper that passes only the fixed PT400/PT403/PT404/PT409/PT422 codes through.
// Depends on: the SECURITY DEFINER functions social.list_conversations / social.read_thread / social.conversation_meta /
// social.unread_conversation_count / inbox.thread_opened (migration 0119, the first two re-created by 0165),
// inbox.buyer_panel / inbox.live_comment_bundles / inbox.link_pending_bundles / inbox.conversation_binding (migration 0165, LC-B3b),
// internal/inbox/keyring.go + classifier.go.
// Used by: internal/httpapi/inbox.go (A8/A9/A13 handlers), internal/inbox tests.
// Invariants: I09 (the panel's identity link is inbox.bundle_peers only, decided in SQL), LCN03 (out of scope = 404).

package inbox

import (
	"context"
	"encoding/json"
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
	Filter    string
	SessionID *string // LC-B3b: keep conversations bundle_peers-linked to a bundle of this session
	Limit     int
	LastAt    *time.Time
	CursorID  *string
}

// ListConversations is A8: metadata only (no ciphertext), keyset-ordered by last_at/conversation_id;
// live_comment uses the bundle's created_at/id instead, without changing the other filters.
func (s *Service) ListConversations(ctx context.Context, tx pgx.Tx, req ListRequest) (ConversationList, error) {
	out := ConversationList{Items: []ConversationItem{}}
	// Calls social.list_conversations (0165: session filter + link_version). filter=live_comment matches no conversation there.
	rows, err := tx.Query(ctx, `SELECT conversation_id::text, platform, last_at, unread, unreplied, mode, assignee::text, window_open_until, linked_customer_id::text, link_version
		FROM social.list_conversations($1::text, $2::uuid, $3::timestamptz, $4::uuid, $5::integer)`,
		req.Filter, req.SessionID, req.LastAt, req.CursorID, req.Limit)
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
			&item.Mode, &item.Assignee, &windowUntil, &item.LinkedCustomerID, &item.LinkVersion); err != nil {
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
	// Live comments have their own bundle keyset on every page. Other eligible filters retain
	// Amendment 1 P2-2's first-page-only pending-link append (copy-link only).
	if req.Filter == "live_comment" || (req.LastAt == nil && req.Filter != "messenger" && req.Filter != "instagram") {
		var bundles []ConversationItem
		var err error
		if req.Filter == "live_comment" {
			bundles, err = s.liveCommentItems(ctx, tx, req)
		} else {
			bundles, err = s.linkPendingItems(ctx, tx, req.SessionID)
		}
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

// linkPendingItems lists at most 10 bundle-only link-pending A8 items. The 0165 overload
// applies the optional session predicate before its SQL limit, so other sessions cannot consume the bound.
func (s *Service) linkPendingItems(ctx context.Context, tx pgx.Tx, session *string) ([]ConversationItem, error) {
	rows, err := tx.Query(ctx, `SELECT bundle_id::text, session_id::text, created_at FROM inbox.link_pending_bundles($1::integer, $2::uuid)`, 10, session)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	var out []ConversationItem
	for rows.Next() {
		var bundle, sess string
		var at time.Time
		if err := rows.Scan(&bundle, &sess, &at); err != nil {
			return nil, databaseError(err)
		}
		b, sid := bundle, sess
		out = append(out, ConversationItem{BundleOnly: true, BundleID: &b, SessionID: &sid, Platform: "messenger", LastAt: at, Unreplied: true, LinkPendingManual: true})
	}
	return out, databaseErrorOrNil(rows.Err())
}

// liveCommentItems is A8 filter=live_comment (LC-B3b): the bundle-only rows of the store's live comments (inbox.live_comment_bundles,
// migration 0165), newest first, at most req.Limit, with session and keyset predicates before LIMIT.
// Platform facebook renders as the console name messenger.
func (s *Service) liveCommentItems(ctx context.Context, tx pgx.Tx, req ListRequest) ([]ConversationItem, error) {
	rows, err := tx.Query(ctx, `SELECT bundle_id::text, session_id::text, platform, created_at, link_pending_manual
		FROM inbox.live_comment_bundles($1::uuid, $2::integer, $3::timestamptz, $4::uuid)`, req.SessionID, req.Limit, req.LastAt, req.CursorID)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	var out []ConversationItem
	for rows.Next() {
		var bundle, sess, platform string
		var at time.Time
		var pending bool
		if err := rows.Scan(&bundle, &sess, &platform, &at, &pending); err != nil {
			return nil, databaseError(err)
		}
		if platform == "facebook" {
			platform = "messenger"
		}
		b, sid := bundle, sess
		out = append(out, ConversationItem{BundleOnly: true, BundleID: &b, SessionID: &sid, Platform: platform, LastAt: at, Unreplied: pending, LinkPendingManual: pending})
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
	out.LinkVersion = meta.linkVersion
	// Calls inbox.conversation_binding (0165): the send binding the UI matches against the capability row; NULL = none.
	if err := tx.QueryRow(ctx, `SELECT inbox.conversation_binding($1::uuid)::text`, conversationID).Scan(&out.BindingID); err != nil {
		return out, databaseError(err)
	}

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
	linkVersion      int64
}

func (s *Service) conversationMeta(ctx context.Context, tx pgx.Tx, conversationID string) (conversationMeta, error) {
	var m conversationMeta
	err := tx.QueryRow(ctx, `SELECT platform, mode, assignee::text, takeover_generation, human_until, window_open_until, linked_customer_id::text, link_version
		FROM social.conversation_meta($1::uuid)`, conversationID).
		Scan(&m.platform, &m.mode, &m.assignee, &m.generation, &m.humanUntil, &m.windowOpenUntil, &m.linkedCustomerID, &m.linkVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, command.ErrNotFound
	}
	if err != nil {
		return m, databaseError(err)
	}
	return m, nil
}

// panelFacts is inbox.buyer_panel's jsonb. Orders is nil when the principal lacks orders:read (the key is absent);
// AutoReplyState is the raw operation state (§4.4 mapping stays here).
type panelFacts struct {
	Claims            []ClaimRef  `json:"claims"`
	ClaimTotalMinor   int64       `json:"claim_total_minor"`
	PurchaseOrdinal   int         `json:"purchase_ordinal"`
	LinkPendingManual bool        `json:"link_pending_manual"`
	Platform          string      `json:"platform"`
	Orders            *[]OrderRef `json:"orders"`
	AutoReplyState    *string     `json:"auto_reply_state"`
}

// panel calls inbox.buyer_panel (0165) for exactly one of conversation/bundle and fills the shared DTO fields. Side effect: none (STABLE).
func (s *Service) panel(ctx context.Context, tx pgx.Tx, conversationID, bundleID *string) (BuyerPanel, error) {
	out := BuyerPanel{Claims: []ClaimRef{}}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT inbox.buyer_panel($1::uuid, $2::uuid)::text`, conversationID, bundleID).Scan(&raw); err != nil {
		return out, databaseError(err)
	}
	var f panelFacts
	if err := json.Unmarshal(raw, &f); err != nil {
		return out, ErrDatabase
	}
	if f.Claims != nil {
		out.Claims = f.Claims
	}
	out.Platform, out.ClaimTotalMinor, out.PurchaseOrdinal, out.LinkPendingManual = f.Platform, f.ClaimTotalMinor, f.PurchaseOrdinal, f.LinkPendingManual
	out.Orders = f.Orders
	if state, ok := sendState(f.AutoReplyState); ok {
		out.AutoReply = &AutoReply{SendState: state}
	}
	return out, nil
}

// BuyerPanel is A13 by conversation (LC-B3b): the conversation header fields plus the claims/orders of the bundles that
// inbox.bundle_peers links to this conversation's peer (I09). display_name only from the conversation's own newest inbound envelope.
func (s *Service) BuyerPanel(ctx context.Context, tx pgx.Tx, conversationID string) (BuyerPanel, error) {
	meta, err := s.conversationMeta(ctx, tx, conversationID)
	if err != nil {
		return BuyerPanel{Claims: []ClaimRef{}}, err
	}
	out, err := s.panel(ctx, tx, &conversationID, nil)
	if err != nil {
		return out, err
	}
	out.LinkedCustomerID = meta.linkedCustomerID
	out.WindowOpenUntil = &meta.windowOpenUntil
	one := []ConversationItem{{ConversationID: conversationID}}
	if err := s.fillDisplayNames(ctx, tx, one); err != nil {
		return out, err
	}
	out.DisplayName = one[0].DisplayName
	return out, nil
}

// BuyerPanelByBundle is A13 with ?bundle_id= (Amendment 1 A1.1/P2-2 + LC-B3b): that bundle's claims/orders only, no conversation-scoped
// fields. A bundle outside the caller's tenant/store (or retention-purged) is a 404.
func (s *Service) BuyerPanelByBundle(ctx context.Context, tx pgx.Tx, bundleID string) (BuyerPanel, error) {
	return s.panel(ctx, tx, nil, &bundleID)
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
