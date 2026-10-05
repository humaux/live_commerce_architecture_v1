// Purpose: the frozen message-template shapes — the §3.4 outbound kind vocabulary, the two system-fixed template ids,
// and the publish/list/resolve input/output structs. The contract fixes {template_id, version, public_safe, kinds}
// for this unit; name/body and the rest are W2-05B-owned fields.
// Depends on: nothing (plain data shapes).
// Used by: publish.go/list.go/resolve.go, internal/httpapi/templates.go (strict decode), LC-B4 send path.

package msgtemplates

import "time"

// §3.4 outbound kind enum. A template is only usable for the kinds listed on it.
const (
	KindDM           = "dm"
	KindPrivateReply = "private_reply"
	KindPublicReply  = "public_reply"
	KindRecommend    = "recommend"
)

var validKind = map[string]bool{
	KindDM: true, KindPrivateReply: true, KindPublicReply: true, KindRecommend: true,
}

// System-fixed template ids (migration 0121 seeds these; merchants can never republish them — publish raises PT409).
const (
	FixedOrderPayLink   = "order-pay-link/v1"
	FixedOfferRecommend = "offer-recommend/v1"
)

// PublishInput is POST /message-templates. template_id, name, kinds, public_safe and body are all required; null is
// rejected at the transport layer (claimsBody), so public_safe never decodes from an absent/null boolean.
type PublishInput struct {
	TemplateID string   `json:"template_id"`
	Name       string   `json:"name"`
	Kinds      []string `json:"kinds"`
	PublicSafe bool     `json:"public_safe"`
	Body       string   `json:"body"`
}

// PublishOutput is the frozen publish receipt {template_id, version, public_safe, kinds}.
type PublishOutput struct {
	TemplateID string   `json:"template_id"`
	Version    int64    `json:"version"`
	PublicSafe bool     `json:"public_safe"`
	Kinds      []string `json:"kinds"`
}

// ListItem is one GET /message-templates row: the latest published version of each template_id.
type ListItem struct {
	TemplateID string    `json:"template_id"`
	Version    int64     `json:"version"`
	Name       string    `json:"name"`
	Kinds      []string  `json:"kinds"`
	PublicSafe bool      `json:"public_safe"`
	CreatedAt  time.Time `json:"created_at"`
}

// ListOutput is the GET /message-templates envelope.
type ListOutput struct {
	Items []ListItem `json:"items"`
}

// Resolved is one resolver row — a fixed or merchant-published template body (the LC-B4 send path renders it).
type Resolved struct {
	TemplateID string   `json:"template_id"`
	Version    int64    `json:"version"`
	Name       string   `json:"name"`
	Body       string   `json:"body"`
	Kinds      []string `json:"kinds"`
	PublicSafe bool     `json:"public_safe"`
	Fixed      bool     `json:"fixed"`
}

// publishRequest is the canonical idempotent-command request. Only its sha256 reaches ops.command_results; the
// plaintext body is never stored beside the receipt.
type publishRequest struct {
	TemplateID string   `json:"template_id"`
	Name       string   `json:"name"`
	Kinds      []string `json:"kinds"`
	PublicSafe bool     `json:"public_safe"`
	Body       string   `json:"body"`
}
