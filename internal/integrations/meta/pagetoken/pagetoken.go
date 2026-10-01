// Package pagetoken owns the SEAL half of meta-page-token-v2 custody (contracts/meta-claims-intake-v1.md "Merchant connect (R4)"):
// the merchant connect in cmd/api seals the Page access token it just received to HPKE PUBLIC keys, so the API can encrypt a token
// but never read one back. The OPEN half lives in the subpackage pageopen, imported only by the claims-worker's reply loader
// (metareply) and guarded by the cmd/api custody test.
//
// It never opens a token, never holds a private key, never reads a database, and never replaces meta-page-token-v1 (AES-256-GCM,
// operator CLI path): v1 rows stay readable, a stored row is v2 exactly when its nonce column is the 32-byte HPKE encapsulated key.
// Suite: DHKEM(X25519, HKDF-SHA256) / HKDF-SHA256 / AES-256-GCM via stdlib crypto/hpke, no dependency. RFC 9180:
// https://www.rfc-editor.org/rfc/rfc9180.html (retrieved 2026-10-01). The info array binds a ciphertext to
// (tenant, store, binding, provider, asset, version, key id), exactly the v1 AAD fields, so a copied row fails to open.
package pagetoken

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/meta"
)

const (
	// InfoDomain is the first element of the HPKE info array.
	InfoDomain = "livecommerce/meta-page-token/v2"

	envPublicKeys  = "COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON"
	envActiveKeyID = "COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID"

	// EncSize is the X25519 encapsulated-key length stored in the nonce column (0095 widens the CHECK to 12 or 32).
	EncSize = 32
	// MaxTokenBytes bounds a plaintext token (visible ASCII); the ciphertext then fits 17..8192.
	MaxTokenBytes = 4096
)

var (
	// ErrConfig is every key-config failure; it never carries key material.
	ErrConfig = errors.New("pagetoken: invalid configuration")
	// ErrSeal is every seal failure; it never carries key or token material.
	ErrSeal = errors.New("pagetoken: token seal failed")

	keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	assetPattern = regexp.MustCompile(`^[0-9]{1,40}$`)
)

// Scope is everything the HPKE info binds a ciphertext to; Version is the credential version being written (head + 1).
type Scope struct {
	TenantID, StoreID, BindingID, Provider, AssetID string
	Version                                         int64
}

// Info is the HPKE `info` of one sealed token: the JSON array
// ["livecommerce/meta-page-token/v2", tenant, store, binding, provider, asset, version, key_id]. pageopen calls this same function
// so seal and open cannot drift.
func Info(s Scope, keyID string) ([]byte, error) {
	if !command.ValidID(s.TenantID) || !command.ValidID(s.StoreID) || !command.ValidID(s.BindingID) ||
		(s.Provider != "facebook" && s.Provider != "instagram") || !assetPattern.MatchString(s.AssetID) || s.Version < 1 ||
		!keyIDPattern.MatchString(keyID) {
		return nil, ErrSeal
	}
	return json.Marshal([]string{InfoDomain, s.TenantID, s.StoreID, s.BindingID, s.Provider, s.AssetID, strconv.FormatInt(s.Version, 10), keyID})
}

// ValidToken: 1..4096 visible ASCII bytes (Meta tokens are alphanumeric); no space or control.
func ValidToken(t []byte) bool {
	if len(t) < 1 || len(t) > MaxTokenBytes {
		return false
	}
	for _, b := range t {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}

// SealKeys holds the HPKE public keys and the active key id. Public keys are not secret, but the type is redacted in every
// formatter anyway (no key material in logs).
type SealKeys struct {
	activeID string
	keys     map[string]hpke.PublicKey
}

func (SealKeys) String() string               { return "[redacted]" }
func (SealKeys) GoString() string             { return "[redacted]" }
func (SealKeys) Format(f fmt.State, _ rune)   { _, _ = f.Write([]byte("[redacted]")) }
func (SealKeys) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }

// LoadSealKeys reads COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON `{"keys":[{"id":"…","public_key_base64":"<44-char std base64 of 32
// bytes>"}]}` and COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID. Unknown members, duplicate object members (meta.ParseStrict) and duplicate
// ids are rejected; 1..16 keys; the active id must be one of them.
func LoadSealKeys(getenv func(string) string) (*SealKeys, error) {
	if getenv == nil {
		return nil, ErrConfig
	}
	raw := getenv(envPublicKeys)
	if len(raw) < 1 || len(raw) > 8192 {
		return nil, ErrConfig
	}
	if _, err := meta.ParseStrict([]byte(raw)); err != nil {
		return nil, ErrConfig
	}
	var doc struct {
		Keys []struct {
			ID  string `json:"id"`
			Key string `json:"public_key_base64"`
		} `json:"keys"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var extra json.RawMessage
	if err := dec.Decode(&doc); err != nil || dec.Decode(&extra) == nil || len(doc.Keys) < 1 || len(doc.Keys) > 16 {
		return nil, ErrConfig
	}
	activeID := getenv(envActiveKeyID)
	if !keyIDPattern.MatchString(activeID) {
		return nil, ErrConfig
	}
	out := &SealKeys{activeID: activeID, keys: make(map[string]hpke.PublicKey, len(doc.Keys))}
	for _, item := range doc.Keys {
		decoded, err := base64.StdEncoding.DecodeString(item.Key)
		if err != nil || len(item.Key) != 44 || len(decoded) != 32 || !keyIDPattern.MatchString(item.ID) {
			return nil, ErrConfig
		}
		if _, dup := out.keys[item.ID]; dup {
			return nil, ErrConfig
		}
		pk, err := hpke.DHKEM(ecdh.X25519()).NewPublicKey(decoded)
		if err != nil {
			return nil, ErrConfig
		}
		out.keys[item.ID] = pk
	}
	if _, ok := out.keys[activeID]; !ok {
		return nil, ErrConfig
	}
	return out, nil
}

// Seal encrypts token for scope under the active key and returns (key id, encapsulated key = nonce column, AEAD ciphertext). The
// caller zeroes token afterwards; Seal keeps no reference to it.
func (k *SealKeys) Seal(s Scope, token []byte) (keyID string, enc, ciphertext []byte, err error) {
	if k == nil || !ValidToken(token) {
		return "", nil, nil, ErrSeal
	}
	info, err := Info(s, k.activeID)
	if err != nil {
		return "", nil, nil, err
	}
	out, err := hpke.Seal(k.keys[k.activeID], hpke.HKDFSHA256(), hpke.AES256GCM(), info, token)
	if err != nil || len(out) <= EncSize {
		return "", nil, nil, ErrSeal
	}
	return k.activeID, append([]byte(nil), out[:EncSize]...), append([]byte(nil), out[EncSize:]...), nil
}
