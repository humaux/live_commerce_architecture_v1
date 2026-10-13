// Purpose: seal the private OA1 PREPARED notification into native send artifacts.
// Depends on: existing inbox payload AEAD, frozen classifier and Page HPKE public ring.
// Used by: the order-attempt executor after SQL preparation, with parent-minted IDs/job.
// Invariants: no DB/provider IO; exact scoped inbound, fixed template and dormant
// order-only fragment URL; display copy has no bearer, recipient stays HPKE sealed.
// OA2 owns the order-attempt-notification/v1 dispatcher and public URL consumer.
package inbox

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"livecommerce/internal/command"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// These private invariant failures do not authorize a business VOID. The SQL
// helper alone can rederive a refusal; callers retain retryable error custody.
var ErrOrderAttemptNotificationFormat = errors.New("inbox: order attempt notification format invariant")
var ErrOrderAttemptNotificationRecipient = errors.New("inbox: order attempt notification recipient invariant")

type attemptNotificationThread struct {
	EventID     string     `json:"event_id"`
	KeyID       string     `json:"key_id"`
	Nonce       string     `json:"nonce_hex"`
	Ciphertext  string     `json:"ciphertext_hex"`
	AppID       string     `json:"app_id"`
	Object      string     `json:"object"`
	AssetID     string     `json:"asset_id"`
	EventKey    string     `json:"event_key"`
	PayloadHash string     `json:"payload_hash"`
	TenantID    string     `json:"tenant_id"`
	StoreID     string     `json:"store_id"`
	RouteID     string     `json:"route_id"`
	RouteEpoch  int64      `json:"route_epoch"`
	ServerSeq   int64      `json:"server_seq"`
	OccurredAt  *time.Time `json:"occurred_at"`
	Direction   string     `json:"direction"`
}

type attemptNotificationPrepared struct {
	State              string                    `json:"state"`
	TenantID           string                    `json:"tenant_id"`
	StoreID            string                    `json:"store_id"`
	OrderID            string                    `json:"order_id"`
	LinkVersion        int64                     `json:"link_version"`
	ConversationID     string                    `json:"conversation_id"`
	BindingID          string                    `json:"binding_id"`
	TakeoverGeneration int64                     `json:"takeover_generation"`
	Platform           string                    `json:"platform"`
	TemplateID         string                    `json:"template_id"`
	TemplateVersion    int64                     `json:"template_version"`
	TemplateBody       string                    `json:"template_body"`
	Origin             string                    `json:"origin"`
	Locale             string                    `json:"locale"`
	Thread             attemptNotificationThread `json:"thread_row"`
}

type attemptNotificationArtifact struct {
	State              string `json:"state"`
	OperationID        string `json:"operation_id"`
	OutboundID         string `json:"outbound_id"`
	JobID              int64  `json:"job_id"`
	BodyHMAC           string `json:"body_hmac_hex"`
	KeyID              string `json:"key_id"`
	Nonce              string `json:"nonce_hex"`
	Ciphertext         string `json:"ciphertext_hex"`
	DispatchEnc        string `json:"dispatch_enc_hex"`
	DispatchCiphertext string `json:"dispatch_ciphertext_hex"`
	TemplateID         string `json:"template_id"`
	TemplateVersion    int64  `json:"template_version"`
}

// PrepareOrderAttemptNotification transforms only the internal SQL PREPARED DTO.
// IDs/job are minted by the caller in its outer transaction; this method cannot
// create a job, obtain authority, redeem a credential or contact a provider.
func (s *Service) PrepareOrderAttemptNotification(prepared json.RawMessage, buyerLink, operationID, outboundID string, jobID int64) (json.RawMessage, error) {
	if s == nil || s.keys == nil || s.seal == nil {
		return nil, ErrSendUnavailable
	}
	if len(prepared) == 0 || len(prepared) > 1048576 || !command.ValidID(operationID) || !command.ValidID(outboundID) || operationID == outboundID || jobID <= 0 {
		return nil, ErrOrderAttemptNotificationFormat
	}
	var p attemptNotificationPrepared
	decoder := json.NewDecoder(bytes.NewReader(prepared))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return nil, ErrOrderAttemptNotificationFormat
	}
	if decoder.Decode(new(any)) != io.EOF || p.State != "PREPARED" || !command.ValidID(p.TenantID) || !command.ValidID(p.StoreID) || !command.ValidID(p.OrderID) || !command.ValidID(p.ConversationID) || !command.ValidID(p.BindingID) || p.LinkVersion != 1 || p.TakeoverGeneration < 0 || (p.Platform != "facebook" && p.Platform != "instagram") || p.TemplateID != msgtemplates.FixedOrderPayLink || p.TemplateVersion != 1 || strings.Count(p.TemplateBody, linkPlaceholder) != 1 {
		return nil, ErrOrderAttemptNotificationFormat
	}
	// The dormant order-only URL is bounded by SQL's current origin and C1 locale.
	// Its token is never sent to a URL consumer during OA1 or stored as display text.
	u, err := url.Parse(buyerLink)
	origin, originErr := url.Parse(p.Origin)
	if err != nil || originErr != nil || len(buyerLink) > 2048 || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || u.Scheme != "https" || u.Host != origin.Host || u.User != nil || u.RawQuery != "" || u.RawPath != "" || u.Path != "/"+p.Locale+"/pay-entry" || (p.Locale != "en" && p.Locale != "zh-TW" && p.Locale != "zh-CN") {
		return nil, ErrOrderAttemptNotificationFormat
	}
	fragment, err := url.ParseQuery(u.Fragment)
	if err != nil || len(fragment) != 2 || len(fragment["o"]) != 1 || len(fragment["t"]) != 1 || fragment.Get("o") != p.OrderID {
		return nil, ErrOrderAttemptNotificationFormat
	}
	token, err := base64.RawURLEncoding.DecodeString(fragment.Get("t"))
	if err != nil || len(token) != 32 || base64.RawURLEncoding.EncodeToString(token) != fragment.Get("t") {
		clear(token)
		return nil, ErrOrderAttemptNotificationFormat
	}
	clear(token)
	r := p.Thread
	if r.TenantID != p.TenantID || r.StoreID != p.StoreID || !command.ValidID(r.EventID) || !command.ValidID(r.RouteID) || r.RouteEpoch <= 0 || r.ServerSeq <= 0 || r.Direction != "in" || r.AssetID == "" || (p.Platform == "facebook" && r.Object != "page") || (p.Platform == "instagram" && r.Object != "instagram") {
		return nil, ErrOrderAttemptNotificationFormat
	}
	nonce, err := hex.DecodeString(r.Nonce)
	if err != nil || len(nonce) != 12 {
		return nil, ErrOrderAttemptNotificationFormat
	}
	ciphertext, err := hex.DecodeString(r.Ciphertext)
	if err != nil || len(ciphertext) < 17 || len(ciphertext) > maxPayload+16 {
		return nil, ErrOrderAttemptNotificationFormat
	}
	plain, err := s.keys.open(eventContext{EventID: r.EventID, AppID: r.AppID, Object: r.Object, EventKey: r.EventKey, PayloadHash: r.PayloadHash, TenantID: r.TenantID, StoreID: r.StoreID, RouteID: r.RouteID, RouteEpoch: r.RouteEpoch}, r.KeyID, nonce, ciphertext)
	if err != nil {
		return nil, ErrPayload
	}
	defer clear(plain)
	view, err := replayMessage(plain, r.AssetID)
	if err != nil || view.senderID == "" {
		return nil, ErrOrderAttemptNotificationRecipient
	}
	text, err := checkText(KindDM, p.Platform, strings.Replace(p.TemplateBody, linkPlaceholder, buyerLink, 1))
	if err != nil {
		return nil, ErrOrderAttemptNotificationFormat
	}
	digest, err := s.keys.bodyHMAC(p.TenantID, "order-pay-link|"+p.OrderID+"|"+p.TemplateBody)
	if err != nil {
		return nil, err
	}
	keyID, displayNonce, displayCiphertext, err := s.keys.sealOutbound(p.TenantID, p.StoreID, outboundID, KindDM, scrubLinks(text))
	if err != nil {
		return nil, err
	}
	secret, err := json.Marshal(sendSecret{PSID: view.senderID, Text: text})
	if err != nil {
		return nil, ErrPayload
	}
	defer clear(secret)
	enc, sealed, err := s.sealDispatch(platform.Scope{TenantID: p.TenantID, StoreID: p.StoreID}, operationID, secret)
	if err != nil {
		return nil, err
	}
	artifact, err := json.Marshal(attemptNotificationArtifact{State: "READY", OperationID: operationID, OutboundID: outboundID, JobID: jobID, BodyHMAC: hex.EncodeToString(digest), KeyID: keyID, Nonce: hex.EncodeToString(displayNonce), Ciphertext: hex.EncodeToString(displayCiphertext), DispatchEnc: hex.EncodeToString(enc), DispatchCiphertext: hex.EncodeToString(sealed), TemplateID: p.TemplateID, TemplateVersion: p.TemplateVersion})
	if err != nil {
		return nil, ErrPayload
	}
	return artifact, nil
}
