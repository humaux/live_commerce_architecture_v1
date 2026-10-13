// Purpose: issue and replay encrypted, order-only random pay-entry credentials.
// Depends: standard-library crypto/JSON, command UUID validation and plan key validation.
// Used by: OA1 local C2 preparation; SQL owns issuance version, lifetime and validity.
// Invariants: dedicated immutable keyring, authenticated order/version/digest, no buyer
// capability, plaintext persistence, provider call or credential-bearing error.
package orderattempt

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"livecommerce/internal/command"
)

// PayEntryBinding comes from the held order fence, never from an untrusted bearer.
type PayEntryBinding struct {
	TenantID string `json:"tenant_id"`
	StoreID  string `json:"store_id"`
	OrderID  string `json:"order_id"`
	Version  int64  `json:"version"`
}

// PayEntryMaterial is the only persisted credential envelope. Ciphertext is
// nonce(12) || AES-256-GCM ciphertext || tag(16); Digest hashes the canonical token.
type PayEntryMaterial struct {
	KeyVersion string   `json:"key_version"`
	Digest     [32]byte `json:"digest"`
	Ciphertext []byte   `json:"ciphertext"`
}

// String and GoString omit even encrypted material from incidental formatting.
func (PayEntryMaterial) String() string     { return "[order pay-entry material]" }
func (m PayEntryMaterial) GoString() string { return m.String() }

// PayEntryError exposes only a fixed classification, never key or token details.
type PayEntryError struct {
	code string
}

func (e *PayEntryError) Error() string { return e.code }
func (e *PayEntryError) Code() string  { return e.code }

var (
	ErrPayEntryInvalid        = &PayEntryError{code: "order_pay_entry_invalid"}
	ErrPayEntryKeyUnavailable = &PayEntryError{code: "order_pay_entry_key_unavailable"}
	ErrPayEntrySealFailed     = &PayEntryError{code: "order_pay_entry_seal_failed"}
)

// PayEntryKeys is separately provisioned from PlanKeys and buyer authentication.
// Keep every referenced version during rotation; no mutable key material escapes.
type PayEntryKeys struct {
	activeVersion string
	keys          map[string]cipher.AEAD
}

func (PayEntryKeys) String() string     { return "[order pay-entry keys]" }
func (k PayEntryKeys) GoString() string { return k.String() }

// NewPayEntryKeys reuses AES key validation and defensive copying, not plan
// credentials or encryption. Every call allocates a separate immutable ring.
func NewPayEntryKeys(activeVersion string, keys map[string][]byte) (*PayEntryKeys, error) {
	validated, err := NewPlanKeys(activeVersion, keys)
	if err != nil {
		return nil, ErrPayEntryKeyUnavailable
	}
	return &PayEntryKeys{activeVersion: validated.activeVersion, keys: validated.keys}, nil
}

// Issue creates a new random credential. Callers must persist this envelope once
// under SQL's issuance fence and use Open for replay instead of issuing again.
func (k *PayEntryKeys) Issue(binding PayEntryBinding) (PayEntryMaterial, error) {
	if k == nil || k.keys[k.activeVersion] == nil {
		return PayEntryMaterial{}, ErrPayEntryKeyUnavailable
	}
	if !validPayEntryBinding(binding) {
		return PayEntryMaterial{}, ErrPayEntryInvalid
	}
	raw := make([]byte, 32)
	defer clear(raw)
	if _, err := rand.Read(raw); err != nil {
		return PayEntryMaterial{}, ErrPayEntrySealFailed
	}
	plain := make([]byte, base64.RawURLEncoding.EncodedLen(len(raw)))
	defer clear(plain)
	base64.RawURLEncoding.Encode(plain, raw)
	m := PayEntryMaterial{KeyVersion: k.activeVersion, Digest: sha256.Sum256(plain)}
	aad, err := payEntryAAD(binding, m.KeyVersion, m.Digest)
	if err != nil {
		return PayEntryMaterial{}, ErrPayEntryInvalid
	}
	aead := k.keys[k.activeVersion]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return PayEntryMaterial{}, ErrPayEntrySealFailed
	}
	m.Ciphertext = aead.Seal(nonce, nonce, plain, aad)
	return m, nil
}

// Open returns the same canonical token only after authenticating its exact
// binding and digest. It does not establish SQL expiry, revocation or payability.
func (k *PayEntryKeys) Open(binding PayEntryBinding, m PayEntryMaterial) (string, error) {
	if k == nil || len(k.keys) == 0 {
		return "", ErrPayEntryKeyUnavailable
	}
	if !validPayEntryBinding(binding) || !validPlanKeyVersion(m.KeyVersion) {
		return "", ErrPayEntryInvalid
	}
	aead := k.keys[m.KeyVersion]
	if aead == nil {
		return "", ErrPayEntryKeyUnavailable
	}
	nonceSize := aead.NonceSize()
	if len(m.Ciphertext) != nonceSize+43+aead.Overhead() {
		return "", ErrPayEntryInvalid
	}
	aad, err := payEntryAAD(binding, m.KeyVersion, m.Digest)
	if err != nil {
		return "", ErrPayEntryInvalid
	}
	plain, err := aead.Open(nil, m.Ciphertext[:nonceSize], m.Ciphertext[nonceSize:], aad)
	defer clear(plain)
	if err != nil || sha256.Sum256(plain) != m.Digest {
		return "", ErrPayEntryInvalid
	}
	raw := make([]byte, base64.RawURLEncoding.DecodedLen(len(plain)))
	defer clear(raw)
	n, err := base64.RawURLEncoding.Strict().Decode(raw, plain)
	if err != nil || n != 32 || base64.RawURLEncoding.EncodeToString(raw[:n]) != string(plain) {
		return "", ErrPayEntryInvalid
	}
	return string(plain), nil
}

func validPayEntryBinding(b PayEntryBinding) bool {
	return command.ValidID(b.TenantID) && command.ValidID(b.StoreID) && command.ValidID(b.OrderID) && b.Version > 0
}

func payEntryAAD(binding PayEntryBinding, version string, digest [32]byte) ([]byte, error) {
	return json.Marshal(struct {
		Domain     string          `json:"domain"`
		KeyVersion string          `json:"key_version"`
		Binding    PayEntryBinding `json:"binding"`
		Digest     [32]byte        `json:"digest"`
	}{"order-pay-entry-v1", version, binding, digest})
}
