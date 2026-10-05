// Package pageopen owns the OPEN half of meta-page-token-v2 custody: the HPKE private-key ring and Open. It is imported only by the
// claims-worker's private-reply loader (internal/integrations/metareply); cmd/api must never import it or mount the private ring
// (checked by TestMetaConnectAPIHoldsNoPagePrivateKey).
//
// It never seals, never reads a database and never logs a key or a token. Seal-side counterpart: package pagetoken, whose Info
// function this package calls so a drift in either half fails the round-trip test instead of producing unreadable tokens.
package pageopen

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/meta/pagetoken"
	metaads "livecommerce/internal/integrations/meta_ads"
)

// envPrivateKeys names the secret (contents via lcentry expansion, or NAME_FILE read directly, see metaads.SecretFromEnv); the
// documented variable is COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS_FILE.
const envPrivateKeys = "COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS"

var (
	// ErrConfig is every key-file failure; it never carries key material or the file contents.
	ErrConfig = errors.New("pageopen: invalid configuration")
	// ErrOpen is every open failure (unknown key id, wrong scope, tampered ciphertext, bad token).
	ErrOpen = errors.New("pageopen: token unavailable")

	keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// Keyring holds the HPKE private keys; every formatter is redacted.
type Keyring struct {
	keys map[string]hpke.PrivateKey
}

func (Keyring) String() string               { return "[redacted]" }
func (Keyring) GoString() string             { return "[redacted]" }
func (Keyring) Format(f fmt.State, _ rune)   { _, _ = f.Write([]byte("[redacted]")) }
func (Keyring) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }

// LoadKeyring reads the ring named by COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS_FILE (or the lcentry-expanded
// COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS, not both). Format `{"keys":[{"id":"…","private_key_base64":"<44-char std base64 of 32
// bytes>"}]}`; 1..16 keys, no unknown or duplicate members, no duplicate ids, no all-zero key. Any problem is ErrConfig.
func LoadKeyring(getenv func(string) string) (*Keyring, error) {
	if getenv == nil {
		return nil, ErrConfig
	}
	raw, err := metaads.SecretFromEnv(getenv, envPrivateKeys, 8192)
	if err != nil {
		return nil, ErrConfig
	}
	defer clear(raw)
	if _, err := meta.ParseStrict(raw); err != nil {
		return nil, ErrConfig
	}
	var doc struct {
		Keys []struct {
			ID  string `json:"id"`
			Key string `json:"private_key_base64"`
		} `json:"keys"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var extra json.RawMessage
	if err := dec.Decode(&doc); err != nil || dec.Decode(&extra) == nil || len(doc.Keys) < 1 || len(doc.Keys) > 16 {
		return nil, ErrConfig
	}
	ring := &Keyring{keys: make(map[string]hpke.PrivateKey, len(doc.Keys))}
	kem := hpke.DHKEM(ecdh.X25519())
	for _, item := range doc.Keys {
		decoded, err := base64.StdEncoding.DecodeString(item.Key)
		if err != nil || len(item.Key) != 44 || len(decoded) != 32 || !keyIDPattern.MatchString(item.ID) || bytes.Equal(decoded, make([]byte, 32)) {
			return nil, ErrConfig
		}
		if _, dup := ring.keys[item.ID]; dup {
			clear(decoded)
			return nil, ErrConfig
		}
		pk, err := kem.NewPrivateKey(decoded)
		clear(decoded)
		if err != nil {
			return nil, ErrConfig
		}
		ring.keys[item.ID] = pk
	}
	return ring, nil
}

// Open decrypts one stored token. enc is the 32-byte encapsulated key (column nonce), ciphertext the AEAD output (17..8192 bytes). The
// caller must clear the returned slice after use. Any mismatch (key id, scope, tampering, token shape) is ErrOpen with no detail.
func (k *Keyring) Open(s pagetoken.Scope, keyID string, enc, ciphertext []byte) ([]byte, error) {
	if k == nil || len(enc) != pagetoken.EncSize || len(ciphertext) < 17 || len(ciphertext) > 8192 {
		return nil, ErrOpen
	}
	priv, ok := k.keys[keyID]
	if !ok {
		return nil, ErrOpen
	}
	info, err := pagetoken.Info(s, keyID)
	if err != nil {
		return nil, ErrOpen
	}
	joined := append(append(make([]byte, 0, len(enc)+len(ciphertext)), enc...), ciphertext...)
	plain, err := hpke.Open(priv, hpke.HKDFSHA256(), hpke.AES256GCM(), info, joined)
	if err != nil {
		return nil, ErrOpen
	}
	if !pagetoken.ValidToken(plain) {
		clear(plain)
		return nil, ErrOpen
	}
	return plain, nil
}

// OpenSend opens one stored send dispatch copy (inbox.send_secrets). The row carries no key id (live-console-v1 §3.4), so each key of
// the ring is tried; the info binds tenant/store/operation, so only the sealing key and the right operation succeed. The caller must
// clear the returned slice after use. Any failure is ErrOpen with no detail.
func (k *Keyring) OpenSend(tenant, store, operation string, enc, ciphertext []byte) ([]byte, error) {
	if k == nil || len(enc) != pagetoken.EncSize || len(ciphertext) < 17 || len(ciphertext) > 16384 {
		return nil, ErrOpen
	}
	info, err := pagetoken.SendInfo(tenant, store, operation)
	if err != nil {
		return nil, ErrOpen
	}
	joined := append(append(make([]byte, 0, len(enc)+len(ciphertext)), enc...), ciphertext...)
	for _, priv := range k.keys {
		if plain, err := hpke.Open(priv, hpke.HKDFSHA256(), hpke.AES256GCM(), info, joined); err == nil {
			return plain, nil
		}
	}
	return nil, ErrOpen
}
