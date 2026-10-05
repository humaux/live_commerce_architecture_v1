package inbox

import (
	"errors"
	"time"

	"livecommerce/internal/command"
)

// Service is the inbox read/write side. It holds the payload keyring (for A9 decryption) but no pool: every method
// runs inside the scoped transaction platform.WithScope provides, and every SECURITY DEFINER call re-checks the
// server-resolved tenant/store/principal.
type Service struct {
	keys *Keyring
}

// NewService requires the payload keyring (the one A9 uses to open message bodies). cmd/api loads it with LoadKeyring
// and passes it here; a nil keyring means the inbox is not mounted (like every other nil service in httpapi.Options).
func NewService(keys *Keyring) (*Service, error) {
	if keys == nil {
		return nil, ErrConfig
	}
	return &Service{keys: keys}, nil
}

// ConversationItem is one A8 row (live-console-v1 §11). bundle_id and display_name are LC-B4 (bundles / comment peers)
// and never set here; linked_customer_id is null until the merchant links it (A14).
type ConversationItem struct {
	ConversationID   string    `json:"conversation_id"`
	Platform         string    `json:"platform"`
	LastAt           time.Time `json:"last_at"`
	Unread           bool      `json:"unread"`
	Unreplied        bool      `json:"unreplied"`
	Mode             string    `json:"mode"`
	Assignee         *string   `json:"assignee"`
	WindowOpenUntil  time.Time `json:"window_open_until"`
	LinkedCustomerID *string   `json:"linked_customer_id"`
}

// ConversationList is the A8 response envelope. NextCursor is empty on the last page.
type ConversationList struct {
	Items       []ConversationItem `json:"items"`
	NextCursor  string             `json:"next_cursor"`
	UnreadTotal int64              `json:"unread_total"`
}

// MessageItem is one A9 row. The read side only ever returns inbound rows (direction "in"); kind, send_state and
// principal_id belong to outbound rows (LC-B4) and are omitted. Unreadable is set only when the frozen-classifier
// replay or the keyring open fails (never dropped silently).
type MessageItem struct {
	Direction   string           `json:"direction"`
	Seq         int64            `json:"seq"`
	At          *time.Time       `json:"at"`
	Text        string           `json:"text"`
	Attachments []attachmentView `json:"attachments"`
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
