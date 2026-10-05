// Package inbox owns the merchant inbox read side (contracts/live-console-v1.md §3.2/§3.6/§3.7, §11 A8-A11/A13/A14):
// the payload keyring that opens the inbound message bodies (A9), the frozen-classifier replay that extracts text and
// attachments, and the definer callers for conversation listing, thread reads, read/unread, takeover/release and the
// customer link. It never returns a driver message, never logs a message body or key material, and never trusts a
// request tenant/store/principal: scope comes from platform.WithScope and is re-checked inside each SECURITY DEFINER
// function against identity.store_grants.
// Purpose: own the Keyring and LoadKeyring: load the payload keyring pair (active key id + key map) and open/decrypt
// one sealed inbound message body (A9) via Keyring.open and eventContext.
// Depends on: livecommerce/internal/command (ValidID), livecommerce/internal/integrations/meta (ParseStrict), and the
// COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID + COMMERCE_META_PAYLOAD_KEYS_JSON environment pair shared with the consumer.
// Used by: cmd/api (newInbox builds the service), internal/httpapi/inbox.go (A8-A11/A13/A14 routes), internal/inbox methods.
package inbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/meta"
)

// Keyring is the meta payload keyring (the same keys the consumer uses). meta.PayloadKeyring.open and its fields are
// unexported, so the read side holds its own copy that opens only class "event" bodies. It reads the same environment
// variables COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID and COMMERCE_META_PAYLOAD_KEYS_JSON.
type Keyring struct {
	activeID string
	keys     map[string][32]byte
}

const (
	maxPayload   = 4 << 20
	payloadTag   = 16
	payloadNonce = 12
)

var (
	// ErrConfig is every keyring/config failure; it never carries key material.
	ErrConfig = errors.New("inbox: invalid configuration")
	// ErrPayload is every open failure (wrong key, tampered AAD, bad body); no detail on purpose.
	ErrPayload = errors.New("inbox: invalid payload")
)

func (Keyring) String() string { return "inbox.Keyring{redacted}" }
func (k Keyring) GoString() string {
	return k.String()
}
func (Keyring) MarshalJSON() ([]byte, error) { return []byte(`"inbox.Keyring{redacted}"`), nil }

func validKeyID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		ch := id[i]
		if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}

func validPayloadHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !(s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'f') {
			return false
		}
	}
	return true
}

func digits(s string) bool {
	if len(s) < 1 || len(s) > 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func newKeyring(activeID string, keys map[string][]byte) (*Keyring, error) {
	if len(keys) < 1 || len(keys) > 16 || !validKeyID(activeID) {
		return nil, ErrConfig
	}
	out := &Keyring{activeID: activeID, keys: make(map[string][32]byte, len(keys))}
	seen := make(map[[32]byte]bool, len(keys))
	for id, source := range keys {
		if !validKeyID(id) || len(source) != 32 {
			return nil, ErrConfig
		}
		var key [32]byte
		copy(key[:], source)
		if key == ([32]byte{}) || seen[key] {
			return nil, ErrConfig
		}
		seen[key] = true
		out.keys[id] = key
	}
	if _, ok := out.keys[activeID]; !ok {
		return nil, ErrConfig
	}
	return out, nil
}

// LoadKeyring reads the shared meta payload keyring (same env vars as the consumer), so the read side opens exactly the
// bodies the consumer sealed. It never reads a credential file and never logs key material.
func LoadKeyring(getenv func(string) string) (*Keyring, error) {
	if getenv == nil {
		return nil, ErrConfig
	}
	activeID := getenv("COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID")
	raw := getenv("COMMERCE_META_PAYLOAD_KEYS_JSON")
	if !validKeyID(activeID) || len(raw) < 1 || len(raw) > 8192 {
		return nil, ErrConfig
	}
	root, err := meta.ParseStrict([]byte(raw))
	if err != nil || len(root) != 1 {
		return nil, ErrConfig
	}
	items, ok := root["keys"].([]any)
	if !ok || len(items) < 1 || len(items) > 16 {
		return nil, ErrConfig
	}
	keys := make(map[string][]byte, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok || len(m) != 2 {
			return nil, ErrConfig
		}
		id, idOK := m["id"].(string)
		encoded, keyOK := m["key_base64"].(string)
		if !idOK || !keyOK || !validKeyID(id) || len(encoded) != 44 {
			return nil, ErrConfig
		}
		if _, duplicate := keys[id]; duplicate {
			return nil, ErrConfig
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != encoded {
			return nil, ErrConfig
		}
		keys[id] = decoded
	}
	return newKeyring(activeID, keys)
}

// eventContext is the frozen AAD context of one stored inbound message (class "event"; BodyHash is always empty). The
// values come from the read_thread definer and were sealed by the consumer with exactly this context.
type eventContext struct {
	EventID, AppID, Object, EventKey, PayloadHash string
	TenantID, StoreID, RouteID                    string
	RouteEpoch                                    int64
}

func (c eventContext) valid() bool {
	return command.ValidID(c.EventID) && digits(c.AppID) && (c.Object == "page" || c.Object == "instagram") &&
		validPayloadHash(c.EventKey) && validPayloadHash(c.PayloadHash) &&
		command.ValidID(c.TenantID) && command.ValidID(c.StoreID) && command.ValidID(c.RouteID) && c.RouteEpoch > 0
}

func (c eventContext) aad(keyID string) []byte {
	// Positional JSON is the frozen wire format; keep empty fields and zero.
	b, _ := json.Marshal([]any{"livecommerce/meta-payload/v1", "event", c.EventID, c.AppID,
		c.Object, "", c.EventKey, c.PayloadHash, c.TenantID, c.StoreID,
		c.RouteID, c.RouteEpoch, keyID})
	return b
}

// open decrypts one stored message body and re-checks its digest. Any mismatch (key id, AAD, tamper, shape) is ErrPayload.
func (k *Keyring) open(c eventContext, keyID string, nonce, ciphertext []byte) ([]byte, error) {
	if k == nil || !c.valid() || !validKeyID(keyID) || len(nonce) != payloadNonce ||
		len(ciphertext) < payloadTag+1 || len(ciphertext) > maxPayload+payloadTag {
		return nil, ErrPayload
	}
	key, ok := k.keys[keyID]
	if !ok || key == ([32]byte{}) {
		return nil, ErrPayload
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, ErrPayload
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrPayload
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, c.aad(keyID))
	if err != nil || len(plaintext) < 1 || len(plaintext) > maxPayload || digest(plaintext) != c.PayloadHash {
		return nil, ErrPayload
	}
	return plaintext, nil
}

// ErrDatabase is the fixed wrap for any definer/database failure; it never carries a driver message.
var ErrDatabase = errors.New("inbox: database unavailable")

// outboundAAD is the frozen AAD of a display copy (class "outbound", live-console-v1 §3.4).
func outboundAAD(tenant, store, id, kind, keyID string) []byte {
	b, _ := json.Marshal([]string{"livecommerce/meta-outbound/v1", tenant, store, id, kind, keyID})
	return b
}

// sealOutbound seals one display copy (the text the merchant sent, links already replaced by the caller) with the ACTIVE payload key
// and a random nonce. Used by the send planners; opened again only by openOutbound for the A9 thread.
func (k *Keyring) sealOutbound(tenant, store, id, kind, text string) (keyID string, nonce, ciphertext []byte, err error) {
	if k == nil || !command.ValidID(tenant) || !command.ValidID(store) || !command.ValidID(id) || text == "" {
		return "", nil, nil, ErrPayload
	}
	key, ok := k.keys[k.activeID]
	if !ok {
		return "", nil, nil, ErrPayload
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", nil, nil, ErrPayload
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", nil, nil, ErrPayload
	}
	nonce = make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, nil, ErrPayload
	}
	return k.activeID, nonce, aead.Seal(nil, nonce, []byte(text), outboundAAD(tenant, store, id, kind, k.activeID)), nil
}

// openOutbound opens one display copy; any mismatch (key id, AAD, tamper) is ErrPayload.
func (k *Keyring) openOutbound(tenant, store, id, kind, keyID string, nonce, ciphertext []byte) (string, error) {
	if k == nil || !validKeyID(keyID) || len(nonce) != payloadNonce || len(ciphertext) < payloadTag+1 {
		return "", ErrPayload
	}
	key, ok := k.keys[keyID]
	if !ok {
		return "", ErrPayload
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", ErrPayload
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrPayload
	}
	plain, err := aead.Open(nil, nonce, ciphertext, outboundAAD(tenant, store, id, kind, keyID))
	if err != nil {
		return "", ErrPayload
	}
	return string(plain), nil
}

// bodyHMAC is HMAC-SHA256(key derived from the ACTIVE payload key with info "livecommerce/meta-outbound-hmac/v1" || tenant, text):
// a keyed, tenant-separated digest, never an unsalted hash (a low-entropy 「謝謝」 would be a dictionary oracle).
func (k *Keyring) bodyHMAC(tenant, text string) ([]byte, error) {
	if k == nil || !command.ValidID(tenant) {
		return nil, ErrPayload
	}
	key, ok := k.keys[k.activeID]
	if !ok {
		return nil, ErrPayload
	}
	derive := hmac.New(sha256.New, key[:])
	derive.Write([]byte("livecommerce/meta-outbound-hmac/v1" + tenant))
	mac := hmac.New(sha256.New, derive.Sum(nil))
	mac.Write([]byte(text))
	return mac.Sum(nil), nil
}
