// Purpose: seal the private OA1 PREPARED notification into native send artifacts.
// Depends on: existing inbox payload AEAD, frozen classifier and Page HPKE public ring.
// Used by: the order-attempt executor after SQL preparation, with parent-minted IDs/job.
// Invariants: no DB/provider IO; exact scoped inbound, fixed template and dormant
// order-only fragment URL pinned to its observed origin/version; display copy has
// no bearer, recipient stays HPKE sealed.
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
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// Protocol and crypto failures retain retryable custody. Recipient/content
// refusals require authenticated plaintext that passes the message invariants.
var ErrOrderAttemptNotificationFormat = errors.New("inbox: order attempt notification format invariant")
var ErrOrderAttemptNotificationRecipient = errors.New("inbox: order attempt notification recipient invariant")
var ErrOrderAttemptNotificationContent = errors.New("inbox: order attempt notification rendered content invalid")

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
	OriginVersion      int64                     `json:"origin_version"`
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
	Origin             string `json:"origin"`
	OriginVersion      int64  `json:"origin_version"`
}

// PrepareOrderAttemptNotification transforms only the internal SQL PREPARED DTO.
// IDs/job are minted by the caller in its outer transaction; this method cannot
// create a job, obtain authority, redeem a credential or contact a provider.
func (s *Service) PrepareOrderAttemptNotification(prepared json.RawMessage, buyerLink, operationID, outboundID string, jobID int64) (json.RawMessage, error) {
	if !command.ValidID(operationID) || !command.ValidID(outboundID) || operationID == outboundID || jobID <= 0 {
		return nil, ErrOrderAttemptNotificationFormat
	}
	p, view, err := s.openOrderAttemptNotification(prepared, buyerLink)
	if err != nil {
		return nil, err
	}
	text, err := orderAttemptNotificationText(p, buyerLink)
	if err != nil {
		return nil, err
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
	artifact, err := json.Marshal(attemptNotificationArtifact{State: "READY", OperationID: operationID, OutboundID: outboundID, JobID: jobID, BodyHMAC: hex.EncodeToString(digest), KeyID: keyID, Nonce: hex.EncodeToString(displayNonce), Ciphertext: hex.EncodeToString(displayCiphertext), DispatchEnc: hex.EncodeToString(enc), DispatchCiphertext: hex.EncodeToString(sealed), TemplateID: p.TemplateID, TemplateVersion: p.TemplateVersion, Origin: p.Origin, OriginVersion: p.OriginVersion})
	if err != nil {
		return nil, ErrPayload
	}
	return artifact, nil
}

// CheckOrderAttemptNotificationRecipient runs before the caller creates a River
// operation job. It performs no writes and never returns the decrypted recipient.
func (s *Service) CheckOrderAttemptNotificationRecipient(prepared json.RawMessage, buyerLink string) error {
	_, _, err := s.openOrderAttemptNotification(prepared, buyerLink)
	return err
}

func (s *Service) openOrderAttemptNotification(prepared json.RawMessage, buyerLink string) (attemptNotificationPrepared, messageView, error) {
	var p attemptNotificationPrepared
	var view messageView
	if s == nil || s.keys == nil || s.seal == nil {
		return p, view, ErrSendUnavailable
	}
	if len(prepared) == 0 || len(prepared) > 1048576 {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	decoder := json.NewDecoder(bytes.NewReader(prepared))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	if decoder.Decode(new(any)) != io.EOF || p.State != "PREPARED" || !command.ValidID(p.TenantID) || !command.ValidID(p.StoreID) || !command.ValidID(p.OrderID) || !command.ValidID(p.ConversationID) || !command.ValidID(p.BindingID) || p.LinkVersion != 1 || p.TakeoverGeneration < 0 || p.OriginVersion <= 0 || (p.Platform != "facebook" && p.Platform != "instagram") || p.TemplateID != msgtemplates.FixedOrderPayLink || p.TemplateVersion != 1 || strings.Count(p.TemplateBody, linkPlaceholder) != 1 {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	_, contentErr := orderAttemptNotificationText(p, buyerLink)
	if contentErr != nil && !errors.Is(contentErr, ErrOrderAttemptNotificationContent) {
		return p, view, contentErr
	}
	r := p.Thread
	if r.TenantID != p.TenantID || r.StoreID != p.StoreID || !command.ValidID(r.EventID) || !command.ValidID(r.RouteID) || r.RouteEpoch <= 0 || r.ServerSeq <= 0 || r.Direction != "in" || r.AssetID == "" || (p.Platform == "facebook" && r.Object != "page") || (p.Platform == "instagram" && r.Object != "instagram") {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	nonce, err := hex.DecodeString(r.Nonce)
	if err != nil || len(nonce) != 12 {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	ciphertext, err := hex.DecodeString(r.Ciphertext)
	if err != nil || len(ciphertext) < 17 || len(ciphertext) > maxPayload+16 {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	plain, err := s.keys.open(eventContext{EventID: r.EventID, AppID: r.AppID, Object: r.Object, EventKey: r.EventKey, PayloadHash: r.PayloadHash, TenantID: r.TenantID, StoreID: r.StoreID, RouteID: r.RouteID, RouteEpoch: r.RouteEpoch}, r.KeyID, nonce, ciphertext)
	if err != nil {
		return p, view, ErrPayload
	}
	defer clear(plain)
	// Missing identities are deterministic recipient refusals only within an
	// otherwise valid message. Malformed JSON, message IDs and echo markers do
	// not establish a business refusal, even when sender is also missing.
	unit, err := meta.ParseStrict(plain)
	if err != nil {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	message, ok := unit["message"].(map[string]any)
	if !ok || !messageID(stringField(message, "mid")) {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	if echo, present := message["is_echo"]; present && echo != false {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	sender, senderOK := unit["sender"].(map[string]any)
	recipient, recipientOK := unit["recipient"].(map[string]any)
	if !senderOK || !recipientOK || !digits(stringField(sender, "id")) || !digits(stringField(recipient, "id")) || stringField(sender, "id") == r.AssetID || stringField(recipient, "id") != r.AssetID {
		return p, view, ErrOrderAttemptNotificationRecipient
	}
	view, err = replayMessage(plain, r.AssetID)
	if err != nil {
		return p, view, ErrOrderAttemptNotificationFormat
	}
	// A rendered-content refusal cannot mask malformed authenticated inbound or
	// an AEAD dependency fault. Those checks must finish before classification.
	return p, view, contentErr
}

func orderAttemptNotificationText(p attemptNotificationPrepared, buyerLink string) (string, error) {
	// Validate the complete URL/template before any recipient-only refusal.
	// The dormant fragment credential is never stored in the display copy.
	u, err := url.Parse(buyerLink)
	origin, originErr := url.Parse(p.Origin)
	if err != nil || originErr != nil || len(buyerLink) > 2048 || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || u.Scheme != "https" || u.Host != origin.Host || u.User != nil || u.RawQuery != "" || u.RawPath != "" || u.Path != "/"+p.Locale+"/pay-entry" || (p.Locale != "en" && p.Locale != "zh-TW" && p.Locale != "zh-CN") {
		return "", ErrOrderAttemptNotificationFormat
	}
	fragment, err := url.ParseQuery(u.Fragment)
	if err != nil || len(fragment) != 2 || len(fragment["o"]) != 1 || len(fragment["t"]) != 1 || fragment.Get("o") != p.OrderID {
		return "", ErrOrderAttemptNotificationFormat
	}
	token, err := base64.RawURLEncoding.DecodeString(fragment.Get("t"))
	if err != nil || len(token) != 32 || base64.RawURLEncoding.EncodeToString(token) != fragment.Get("t") {
		clear(token)
		return "", ErrOrderAttemptNotificationFormat
	}
	clear(token)
	text, err := checkText(KindDM, p.Platform, strings.Replace(p.TemplateBody, linkPlaceholder, buyerLink, 1))
	if err != nil {
		return "", ErrOrderAttemptNotificationContent
	}
	return text, nil
}
