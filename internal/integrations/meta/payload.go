package meta

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"

	"livecommerce/internal/command"
)

const (
	maxPayload   = 4 << 20
	payloadTag   = 16
	payloadNonce = 12
)

var ErrPayload = errors.New("meta: invalid payload")

// PayloadKeyring is an independent encryption domain for Meta inbox bodies.
type PayloadKeyring struct {
	activeID string
	keys     map[string][32]byte
}

type payloadContext struct {
	Class, ID, AppID, Object, BodyHash, EventKey, PayloadHash string
	TenantID, StoreID, RouteID                                string
	RouteEpoch                                                int64
}

type sealedPayload struct {
	KeyID             string
	Nonce, Ciphertext []byte
}

// These types can contain credentials, provider data or scope identifiers.
func (PayloadKeyring) String() string     { return "meta.PayloadKeyring{redacted}" }
func (k PayloadKeyring) GoString() string { return k.String() }
func (PayloadKeyring) MarshalJSON() ([]byte, error) {
	return []byte(`"meta.PayloadKeyring{redacted}"`), nil
}
func (payloadContext) String() string     { return "meta.payloadContext{redacted}" }
func (c payloadContext) GoString() string { return c.String() }
func (payloadContext) MarshalJSON() ([]byte, error) {
	return []byte(`"meta.payloadContext{redacted}"`), nil
}
func (sealedPayload) String() string     { return "meta.sealedPayload{redacted}" }
func (s sealedPayload) GoString() string { return s.String() }
func (sealedPayload) MarshalJSON() ([]byte, error) {
	return []byte(`"meta.sealedPayload{redacted}"`), nil
}

func NewPayloadKeyring(activeID string, keys map[string][]byte) (*PayloadKeyring, error) {
	if len(keys) < 1 || len(keys) > 16 || !validPayloadKeyID(activeID) {
		return nil, ErrConfig
	}
	out := &PayloadKeyring{activeID: activeID, keys: make(map[string][32]byte, len(keys))}
	seen := make(map[[32]byte]bool, len(keys))
	for id, source := range keys {
		if !validPayloadKeyID(id) || len(source) != 32 {
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

func validPayloadKeyID(id string) bool {
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

func (c payloadContext) valid() bool {
	if !command.ValidID(c.ID) || !digits(c.AppID) || (c.Object != "page" && c.Object != "instagram") {
		return false
	}
	switch c.Class {
	case "raw":
		return validPayloadHash(c.BodyHash) && c.EventKey == "" && c.PayloadHash == "" &&
			c.TenantID == "" && c.StoreID == "" && c.RouteID == "" && c.RouteEpoch == 0
	case "event":
		return c.BodyHash == "" && validPayloadHash(c.EventKey) && validPayloadHash(c.PayloadHash) &&
			command.ValidID(c.TenantID) && command.ValidID(c.StoreID) && command.ValidID(c.RouteID) && c.RouteEpoch > 0
	case "quarantine":
		return c.BodyHash == "" && validPayloadHash(c.EventKey) && validPayloadHash(c.PayloadHash) &&
			c.TenantID == "" && c.StoreID == "" && c.RouteID == "" && c.RouteEpoch == 0
	default:
		return false
	}
}

func (c payloadContext) hash() string {
	if c.Class == "raw" {
		return c.BodyHash
	}
	return c.PayloadHash
}

func (c payloadContext) aad(keyID string) []byte {
	// Positional JSON is the frozen wire format; keep empty fields and zero.
	b, _ := json.Marshal([]any{"livecommerce/meta-payload/v1", c.Class, c.ID, c.AppID,
		c.Object, c.BodyHash, c.EventKey, c.PayloadHash, c.TenantID, c.StoreID,
		c.RouteID, c.RouteEpoch, keyID})
	return b
}

func (k *PayloadKeyring) seal(c payloadContext, plaintext []byte) (sealedPayload, error) {
	if k == nil || !c.valid() || len(plaintext) < 1 || len(plaintext) > maxPayload ||
		(c.Class == "raw" && len(plaintext) > maxBody) || digest(plaintext) != c.hash() {
		return sealedPayload{}, ErrPayload
	}
	key, ok := k.keys[k.activeID]
	if !ok || !validPayloadKeyID(k.activeID) || key == ([32]byte{}) {
		return sealedPayload{}, ErrPayload
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return sealedPayload{}, ErrPayload
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return sealedPayload{}, ErrPayload
	}
	nonce := make([]byte, payloadNonce)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return sealedPayload{}, ErrPayload
	}
	return sealedPayload{k.activeID, nonce, aead.Seal(nil, nonce, plaintext, c.aad(k.activeID))}, nil
}

func (k *PayloadKeyring) open(c payloadContext, s sealedPayload) ([]byte, error) {
	if k == nil || !c.valid() || !validPayloadKeyID(s.KeyID) || len(s.Nonce) != payloadNonce ||
		len(s.Ciphertext) < payloadTag+1 || len(s.Ciphertext) > maxPayload+payloadTag ||
		(c.Class == "raw" && len(s.Ciphertext) > maxBody+payloadTag) {
		return nil, ErrPayload
	}
	key, ok := k.keys[s.KeyID]
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
	plaintext, err := aead.Open(nil, s.Nonce, s.Ciphertext, c.aad(s.KeyID))
	if err != nil || len(plaintext) < 1 || len(plaintext) > maxPayload ||
		(c.Class == "raw" && len(plaintext) > maxBody) || digest(plaintext) != c.hash() {
		return nil, ErrPayload
	}
	return plaintext, nil
}
