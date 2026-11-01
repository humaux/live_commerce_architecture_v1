// Purpose: validate and encrypt website contact messages and open authorized merchant details.
// Depends on: the shared payload-key environment, AES-GCM and migration 0211's restricted definers.
// Used by: buyerhttp contact admission and httpapi merchant contact routes.
// Invariants: no plaintext PII in persistence, receipts or logs; AAD binds store/message/key; no mail transport here.
package storefront

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"livecommerce/internal/command"
)

// ContactInput is literal text, never HTML or a mail header supplied by a buyer.
type ContactInput struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	OrderNumber string `json:"order_number,omitempty"`
	Message     string `json:"message"`
	Website     string `json:"website,omitempty"`
}

func contactText(v string, max int, multiline bool) bool {
	if !utf8.ValidString(v) || utf8.RuneCountInString(v) > max {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t')) {
			return false
		}
	}
	return true
}

// ValidContactInput repeats the public BFF bounds at the authoritative Go boundary.
func ValidContactInput(in ContactInput) bool {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Message) == "" || in.Website != "" ||
		!contactText(in.Name, 120, false) || !contactText(in.Email, 254, false) || len(in.Email) > 254 ||
		!contactText(in.OrderNumber, 64, false) || !contactText(in.Message, 5000, true) || strings.ContainsAny(in.Email, " <>\"\\") {
		return false
	}
	a, err := mail.ParseAddress(in.Email)
	return err == nil && a.Name == "" && a.Address == in.Email
}

// ContactEnvelope contains only encrypted PII plus opaque identity and transition metadata.
type ContactEnvelope struct {
	ID, TenantID, StoreID, KeyID string
	Nonce, Ciphertext            []byte
	Status                       string
	Version                      int64
}

// ContactDetail is exposed only after a current merchant inbox permission check.
type ContactDetail struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	OrderNumber string `json:"order_number"`
	Message     string `json:"message"`
	Status      string `json:"status"`
	Version     int64  `json:"version"`
}

// ContactKeys uses the existing payload-key custody; no second credential or external dependency.
type ContactKeys struct {
	active string
	keys   map[string][]byte
}

var errContactEnvelope = errors.New("storefront: contact encryption unavailable")
var contactKeyID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func (ContactKeys) String() string               { return "ContactKeys{redacted}" }
func (ContactKeys) GoString() string             { return "ContactKeys{redacted}" }
func (ContactKeys) MarshalJSON() ([]byte, error) { return []byte(`"redacted"`), nil }

// LoadContactKeys fails closed. Environment is read per request so all entry points share the same configuration.
func LoadContactKeys() (*ContactKeys, error) {
	active, raw := os.Getenv("COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID"), os.Getenv("COMMERCE_META_PAYLOAD_KEYS_JSON")
	if !contactKeyID.MatchString(active) || len(raw) > 8192 {
		return nil, errContactEnvelope
	}
	var doc struct {
		Keys []struct {
			ID  string `json:"id"`
			Key string `json:"key_base64"`
		} `json:"keys"`
	}
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&doc) != nil || len(doc.Keys) < 1 || len(doc.Keys) > 16 {
		return nil, errContactEnvelope
	}
	k := &ContactKeys{active: active, keys: map[string][]byte{}}
	for _, item := range doc.Keys {
		key, err := base64.StdEncoding.Strict().DecodeString(item.Key)
		if err != nil || len(key) != 32 || bytes.Equal(key, make([]byte, 32)) || !contactKeyID.MatchString(item.ID) || k.keys[item.ID] != nil {
			return nil, errContactEnvelope
		}
		k.keys[item.ID] = key
	}
	if k.keys[active] == nil {
		return nil, errContactEnvelope
	}
	return k, nil
}

func contactAAD(e ContactEnvelope) []byte {
	b, _ := json.Marshal([]string{"livecommerce/store-contact/v1", e.TenantID, e.StoreID, e.ID, e.KeyID})
	return b
}

func (k *ContactKeys) aead(id string) (cipher.AEAD, error) {
	if k == nil || len(k.keys[id]) != 32 {
		return nil, errContactEnvelope
	}
	b, err := aes.NewCipher(k.keys[id])
	if err != nil {
		return nil, errContactEnvelope
	}
	return cipher.NewGCM(b)
}

// Seal generates an opaque UUID and an independent random nonce. Replay compares decrypted original input,
// so neither unsalted PII fingerprints nor active-key rotation change idempotency semantics.
func (k *ContactKeys) Seal(tenant, store string, in ContactInput) (ContactEnvelope, error) {
	if !command.ValidID(tenant) || !command.ValidID(store) || !ValidContactInput(in) {
		return ContactEnvelope{}, command.ErrInvalid
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return ContactEnvelope{}, errContactEnvelope
	}
	id[6] = (id[6] & 15) | 64
	id[8] = (id[8] & 63) | 128
	e := ContactEnvelope{ID: fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), TenantID: tenant, StoreID: store, KeyID: k.active}
	a, err := k.aead(e.KeyID)
	if err != nil {
		return e, err
	}
	e.Nonce = make([]byte, a.NonceSize())
	if _, err = rand.Read(e.Nonce); err != nil {
		return e, errContactEnvelope
	}
	plain, _ := json.Marshal(in)
	defer clear(plain)
	e.Ciphertext = a.Seal(nil, e.Nonce, plain, contactAAD(e))
	return e, nil
}

// Open authenticates the whole envelope before returning any buyer text.
func (k *ContactKeys) Open(e ContactEnvelope) (ContactDetail, error) {
	a, err := k.aead(e.KeyID)
	if err != nil || !command.ValidID(e.ID) || !command.ValidID(e.TenantID) || !command.ValidID(e.StoreID) || len(e.Nonce) != 12 || len(e.Ciphertext) > 65552 {
		return ContactDetail{}, errContactEnvelope
	}
	plain, err := a.Open(nil, e.Nonce, e.Ciphertext, contactAAD(e))
	if err != nil {
		return ContactDetail{}, errContactEnvelope
	}
	defer clear(plain)
	var in ContactInput
	if json.Unmarshal(plain, &in) != nil || !ValidContactInput(in) {
		return ContactDetail{}, errContactEnvelope
	}
	return ContactDetail{e.ID, in.Name, in.Email, in.OrderNumber, in.Message, e.Status, e.Version}, nil
}

// MatchesContact compares canonical field values without storing any plaintext request hash.
func MatchesContact(d ContactDetail, in ContactInput) bool {
	return d.Name == in.Name && d.Email == in.Email && d.OrderNumber == in.OrderNumber && d.Message == in.Message
}
