// Purpose: the inbox Service and the A8-A11/A13/A14 request/response models (live-console-v1 §11). NewService requires
// the payload keyring (nil means the inbox is not mounted); the service holds no pool and every method runs inside the
// scoped transaction platform.WithScope provides.
// Depends on: livecommerce/internal/command (ErrNotFound), internal/inbox/keyring.go (Keyring).
// Used by: cmd/api (newInbox), internal/httpapi/inbox.go (route handlers), internal/inbox read.go/write.go.

package inbox

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/meta/pagetoken"
	"livecommerce/internal/msgtemplates"
)

// Service is the inbox read/write side. It holds the payload keyring (for A9 decryption) but no pool: every method
// runs inside the scoped transaction platform.WithScope provides, and every SECURITY DEFINER call re-checks the
// server-resolved tenant/store/principal.
type Service struct {
	keys *Keyring
	// Send side (live-console-v1 §3.3-3.5, §4; nil until EnableSend): the HPKE public ring that seals the dispatch copy, the River
	// client that inserts the external_operation_v1 job in the planning transaction, and the template resolver.
	seal      *pagetoken.SealKeys
	jobs      *river.Client[pgx.Tx]
	templates *msgtemplates.Service
}

// EnableSend turns the manual-send planners (A4/A5/A6/A12) on. seal is the Page HPKE PUBLIC ring (the API never holds the private
// half), jobs inserts the operation job in the planning transaction, templates resolves published template references (may be nil:
// then only literal text is accepted).
func (s *Service) EnableSend(seal *pagetoken.SealKeys, jobs *river.Client[pgx.Tx], templates *msgtemplates.Service) {
	s.seal, s.jobs, s.templates = seal, jobs, templates
}

// SendEnabled reports whether the send planners are configured.
func (s *Service) SendEnabled() bool { return s != nil && s.seal != nil && s.jobs != nil }

// NewService requires the payload keyring (the one A9 uses to open message bodies). cmd/api loads it with LoadKeyring
// and passes it here; a nil keyring means the inbox is not mounted (like every other nil service in httpapi.Options).
func NewService(keys *Keyring) (*Service, error) {
	if keys == nil {
		return nil, ErrConfig
	}
	return &Service{keys: keys}, nil
}

// ConversationItem is one A8 row (live-console-v1 §11). DisplayName (Amendment 1 P2-1) is derived in the API from the newest
// inbound envelope and is null when unreadable; linked_customer_id is null until the merchant links it (A14). A bundle-only item
// (P2-2) carries BundleID/SessionID, link_pending_manual=true and null conversation_id/mode/assignee/window_open_until.
type ConversationItem struct {
	ConversationID    string    `json:"conversation_id"`
	Platform          string    `json:"platform"`
	DisplayName       *string   `json:"display_name,omitempty"`
	LastAt            time.Time `json:"last_at"`
	Unread            bool      `json:"unread"`
	Unreplied         bool      `json:"unreplied"`
	Mode              string    `json:"mode"`
	Assignee          *string   `json:"assignee"`
	WindowOpenUntil   time.Time `json:"window_open_until"`
	LinkedCustomerID  *string   `json:"linked_customer_id"`
	BundleID          *string   `json:"bundle_id,omitempty"`
	SessionID         *string   `json:"session_id,omitempty"`
	LinkPendingManual bool      `json:"link_pending_manual,omitempty"`
	BundleOnly        bool      `json:"-"`
}

// MarshalJSON renders a bundle-only item with the explicit nulls Amendment 1 P2-2 fixes; conversation items use the plain shape.
func (c ConversationItem) MarshalJSON() ([]byte, error) {
	type plain ConversationItem
	if !c.BundleOnly {
		return json.Marshal(plain(c))
	}
	return json.Marshal(struct {
		ConversationID    *string   `json:"conversation_id"`
		BundleID          *string   `json:"bundle_id"`
		SessionID         *string   `json:"session_id"`
		Platform          string    `json:"platform"`
		LastAt            time.Time `json:"last_at"`
		Unread            bool      `json:"unread"`
		Unreplied         bool      `json:"unreplied"`
		Mode              *string   `json:"mode"`
		Assignee          *string   `json:"assignee"`
		WindowOpenUntil   *string   `json:"window_open_until"`
		LinkedCustomerID  *string   `json:"linked_customer_id"`
		LinkPendingManual bool      `json:"link_pending_manual"`
	}{nil, c.BundleID, c.SessionID, c.Platform, c.LastAt, false, true, nil, nil, nil, nil, true})
}

// ConversationList is the A8 response envelope. NextCursor is empty on the last page.
type ConversationList struct {
	Items       []ConversationItem `json:"items"`
	NextCursor  string             `json:"next_cursor"`
	UnreadTotal int64              `json:"unread_total"`
}

// MessageItem is one A9 row. Inbound rows carry direction "in" and a seq; outbound rows (the merchant's own sends, LC-B4) carry
// direction "out", kind, send_state (§4.4: queued | sent | failed | blocked | unknown), principal_id and no seq. Unreadable is set only
// when the frozen-classifier replay or the keyring open fails (never dropped silently).
type MessageItem struct {
	Direction   string           `json:"direction"`
	Seq         int64            `json:"seq,omitempty"`
	At          *time.Time       `json:"at"`
	Text        string           `json:"text"`
	Attachments []attachmentView `json:"attachments"`
	Kind        *string          `json:"kind,omitempty"`
	SendState   *string          `json:"send_state,omitempty"`
	SendCode    *string          `json:"send_code,omitempty"`
	PrincipalID *string          `json:"principal_id,omitempty"`
	Unreadable  *bool            `json:"unreadable,omitempty"`
}

// ThreadView is the A9 response envelope.
type ThreadView struct {
	Items              []MessageItem `json:"items"`
	WindowOpenUntil    time.Time     `json:"window_open_until"`
	Mode               string        `json:"mode"`
	TakeoverGeneration int64         `json:"takeover_generation"`
	HumanUntil         *time.Time    `json:"human_until"`
}

// ClaimRef is one A13 claim; LC-B3 always returns them empty (bundles/claims are LC-B4). The shape is fixed here so
// the response contract stays stable for the integrator's reader.
type ClaimRef struct {
	SessionID string `json:"session_id"`
	OfferID   string `json:"offer_id"`
	Keyword   string `json:"keyword"`
	Quantity  int    `json:"quantity"`
}

// OrderRef is one A13 order; LC-B3 always returns them empty (orders:read is LC-B4).
type OrderRef struct {
	OrderID    string    `json:"order_id"`
	Number     string    `json:"number"`
	State      string    `json:"state"`
	TotalMinor int64     `json:"total_minor"`
	CreatedAt  time.Time `json:"created_at"`
}

// BuyerPanel is the A13 response (LC-B3 implements the conversation-scoped fields only; claims/orders/auto_reply are
// LC-B4 and always empty/omitted).
type BuyerPanel struct {
	DisplayName      *string    `json:"display_name,omitempty"`
	Platform         string     `json:"platform"`
	PurchaseOrdinal  int        `json:"purchase_ordinal"`
	Claims           []ClaimRef `json:"claims"`
	ClaimTotalMinor  int64      `json:"claim_total_minor"`
	Orders           []OrderRef `json:"orders"`
	LinkedCustomerID *string    `json:"linked_customer_id,omitempty"`
	WindowOpenUntil  *time.Time `json:"window_open_until,omitempty"`
	// LinkPendingManual (Amendment 1 A1.1): the bundle's claim link could not be sent because the comment's private reply was used.
	LinkPendingManual bool `json:"link_pending_manual"`
}

// ReadInput is A10 `{read_seq}`.
type ReadInput struct {
	ReadSeq int64 `json:"read_seq"`
}

// ReadOutput is A10 `{read_seq}` (the value actually applied, unchanged for a reader without inbox:reply).
type ReadOutput struct {
	ReadSeq int64 `json:"read_seq"`
}

// TakeoverInput is A11 `{expected_generation}`.
type TakeoverInput struct {
	ExpectedGeneration int64 `json:"expected_generation"`
}

// TakeoverOutput is A11 `{mode, assignee, takeover_generation}`.
type TakeoverOutput struct {
	Mode               string  `json:"mode"`
	Assignee           *string `json:"assignee"`
	TakeoverGeneration int64   `json:"takeover_generation"`
}

// CustomerLinkInput is A14 `{customer_id | null, expected_version}`.
type CustomerLinkInput struct {
	CustomerID      *string `json:"customer_id"`
	ExpectedVersion int64   `json:"expected_version"`
}

// CustomerLinkOutput is A14 `{customer_id, version}`.
type CustomerLinkOutput struct {
	CustomerID *string `json:"customer_id"`
	Version    int64   `json:"version"`
}

// IsNotFound reports the fixed not-found class (used by the httpapi classifier's fallback).
func IsNotFound(err error) bool { return errors.Is(err, command.ErrNotFound) }
