package accounts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

var errKeyringConfig = errors.New("invalid account keyring configuration")

// LoadKeyring is shared by the API and the separate payment worker. Its errors
// never contain key material or environment values.
func LoadKeyring(getenv func(string) string) (*Keyring, error) {
	if getenv == nil {
		return nil, errKeyringConfig
	}
	activeID := getenv("COMMERCE_ACCOUNT_ACTIVE_KEY_ID")
	rawKeys := getenv("COMMERCE_ACCOUNT_KEYS_JSON")
	rawReplay := getenv("COMMERCE_ACCOUNT_REPLAY_KEY")
	if len(rawKeys) == 0 || len(rawKeys) > 8192 {
		return nil, errKeyringConfig
	}
	var entries []struct {
		ID        string `json:"id"`
		KeyBase64 string `json:"key_base64"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(rawKeys))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&entries); err != nil {
		return nil, errKeyringConfig
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF || len(entries) < 1 || len(entries) > 16 {
		return nil, errKeyringConfig
	}
	keys := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if _, duplicate := keys[entry.ID]; duplicate {
			return nil, errKeyringConfig
		}
		key, ok := canonicalAccountKey(entry.KeyBase64)
		if !ok {
			return nil, errKeyringConfig
		}
		keys[entry.ID] = key
	}
	replay, ok := canonicalAccountKey(rawReplay)
	if !ok {
		return nil, errKeyringConfig
	}
	keyring, err := NewKeyring(activeID, keys, replay)
	if err != nil {
		return nil, errKeyringConfig
	}
	return keyring, nil
}

func canonicalAccountKey(raw string) ([]byte, bool) {
	key, err := base64.StdEncoding.DecodeString(raw)
	return key, err == nil && len(key) == 32 && base64.StdEncoding.EncodeToString(key) == raw
}
