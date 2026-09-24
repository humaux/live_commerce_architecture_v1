package accounts

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

var errInvalidSecret = errors.New("invalid credentials")

// Credentials only exists at the input boundary. Its formatting and JSON
// representation deliberately cannot reveal either provider secret.
type Credentials struct {
	HashKey string
	HashIV  string
}

func (Credentials) String() string               { return "[redacted credentials]" }
func (c Credentials) GoString() string           { return c.String() }
func (Credentials) MarshalJSON() ([]byte, error) { return []byte(`"[redacted credentials]"`), nil }

type Keyring struct {
	activeID  string
	keys      map[string][]byte
	replayKey []byte
}

// NewKeyring owns copies of every key; changing the caller's map or slices
// cannot alter encryption or decryption in a live service.
func NewKeyring(activeID string, keys map[string][]byte, replayKey []byte) (*Keyring, error) {
	if len(keys) < 1 || len(keys) > 16 || !keyIDPattern.MatchString(activeID) || len(replayKey) != 32 {
		return nil, errInvalidSecret
	}
	copyKeys := make(map[string][]byte, len(keys))
	for id, key := range keys {
		if !keyIDPattern.MatchString(id) || len(key) != 32 || bytes.Equal(key, replayKey) {
			return nil, errInvalidSecret
		}
		copyKeys[id] = append([]byte(nil), key...)
	}
	if _, ok := copyKeys[activeID]; !ok {
		return nil, errInvalidSecret
	}
	return &Keyring{activeID: activeID, keys: copyKeys, replayKey: append([]byte(nil), replayKey...)}, nil
}

type credentialAAD struct {
	FormatVersion     int    `json:"format_version"`
	TenantID          string `json:"tenant_id"`
	StoreID           string `json:"store_id"`
	ConnectionID      string `json:"connection_id"`
	Provider          string `json:"provider"`
	Environment       string `json:"environment"`
	AccountID         string `json:"account_id"`
	CredentialVersion int64  `json:"credential_version"`
}

func (k *Keyring) seal(aad credentialAAD, credentials Credentials) (keyID string, nonce, ciphertext []byte, err error) {
	if k == nil || !validCredentials(credentials) || !validAAD(aad) {
		return "", nil, nil, errInvalidSecret
	}
	key, ok := k.keys[k.activeID]
	if !ok {
		return "", nil, nil, errInvalidSecret
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", nil, nil, errInvalidSecret
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", nil, nil, errInvalidSecret
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", nil, nil, fmt.Errorf("generate credential nonce: %w", err)
	}
	associated, _ := json.Marshal(aad)
	plain, _ := json.Marshal(struct {
		HashKey string `json:"hash_key"`
		HashIV  string `json:"hash_iv"`
	}{credentials.HashKey, credentials.HashIV})
	return k.activeID, nonce, gcm.Seal(nil, nonce, plain, associated), nil
}

func (k *Keyring) open(aad credentialAAD, keyID string, nonce, ciphertext []byte) (Credentials, error) {
	if k == nil || !validAAD(aad) || len(nonce) != 12 || len(ciphertext) < 17 {
		return Credentials{}, errInvalidSecret
	}
	key, ok := k.keys[keyID]
	if !ok {
		return Credentials{}, errInvalidSecret
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Credentials{}, errInvalidSecret
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Credentials{}, errInvalidSecret
	}
	associated, _ := json.Marshal(aad)
	plain, err := gcm.Open(nil, nonce, ciphertext, associated)
	if err != nil {
		return Credentials{}, errInvalidSecret
	}
	var credential struct {
		HashKey string `json:"hash_key"`
		HashIV  string `json:"hash_iv"`
	}
	if err := json.Unmarshal(plain, &credential); err != nil {
		return Credentials{}, errInvalidSecret
	}
	result := Credentials{HashKey: credential.HashKey, HashIV: credential.HashIV}
	if !validCredentials(result) {
		return Credentials{}, errInvalidSecret
	}
	return result, nil
}

func validAAD(a credentialAAD) bool {
	return a.FormatVersion == 1 && a.TenantID != "" && a.StoreID != "" && a.ConnectionID != "" &&
		a.Provider == "payuni" && validEnvironment(a.Environment) && validAccountID(a.AccountID) && a.CredentialVersion > 0
}

func validCredentials(c Credentials) bool {
	return validSecretPart(c.HashKey) && validSecretPart(c.HashIV)
}

func validSecretPart(value string) bool {
	if len(value) < 1 || len(value) > 512 || value[0] == ' ' || value[len(value)-1] == ' ' {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] > 0x7e {
			return false
		}
	}
	return true
}
