package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/core"
)

var errAccountConfig = errors.New("invalid account configuration")

type accountConfig struct {
	enabled bool
	keys    *accounts.Keyring
}

// Disabled mode reads only its feature flag. No account secret is inspected,
// decoded, or passed into a River client until identity and listener gates pass.
func loadAccountConfig(getenv func(string) string, identityEnabled bool, addr string) (accountConfig, error) {
	var config accountConfig
	enabled, err := flag(getenv("COMMERCE_ACCOUNTS_ENABLED"))
	if err != nil {
		return config, errAccountConfig
	}
	if !enabled {
		return config, nil
	}
	if !identityEnabled || !privateIdentityAddress(addr) {
		return config, errAccountConfig
	}
	config.keys, err = loadAccountKeys(getenv)
	if err != nil {
		return accountConfig{}, errAccountConfig
	}
	config.enabled = true
	return config, nil
}

// Both merchant administration and hosted signing must read the same historical
// keyring format. This parser grants no merchant HTTP authority by itself.
func loadAccountKeys(getenv func(string) string) (*accounts.Keyring, error) {
	activeID := getenv("COMMERCE_ACCOUNT_ACTIVE_KEY_ID")
	rawKeys := getenv("COMMERCE_ACCOUNT_KEYS_JSON")
	rawReplay := getenv("COMMERCE_ACCOUNT_REPLAY_KEY")
	if len(rawKeys) == 0 || len(rawKeys) > 8192 {
		return nil, errAccountConfig
	}
	var entries []struct {
		ID        string `json:"id"`
		KeyBase64 string `json:"key_base64"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(rawKeys))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&entries); err != nil {
		return nil, errAccountConfig
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF || len(entries) < 1 || len(entries) > 16 {
		return nil, errAccountConfig
	}
	keys := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if _, duplicate := keys[entry.ID]; duplicate {
			return nil, errAccountConfig
		}
		key, ok := canonicalAccountKey(entry.KeyBase64)
		if !ok {
			return nil, errAccountConfig
		}
		keys[entry.ID] = key
	}
	replay, ok := canonicalAccountKey(rawReplay)
	if !ok {
		return nil, errAccountConfig
	}
	keyring, err := accounts.NewKeyring(activeID, keys, replay)
	if err != nil {
		return nil, errAccountConfig
	}
	return keyring, nil
}

func canonicalAccountKey(raw string) ([]byte, bool) {
	key, err := base64.StdEncoding.DecodeString(raw)
	return key, err == nil && len(key) == 32 && base64.StdEncoding.EncodeToString(key) == raw
}

func buildAccountsService(pool *pgxpool.Pool, config accountConfig) (*accounts.Service, error) {
	if !config.enabled {
		return nil, nil
	}
	if pool == nil || config.keys == nil {
		return nil, errAccountConfig
	}
	// The runtime pool is used solely by River's transactional insert path. No
	// queue or worker is started in the API process.
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		return nil, errAccountConfig
	}
	bindings, err := core.New(jobs)
	if err != nil {
		return nil, errAccountConfig
	}
	service, err := accounts.New(config.keys, bindings)
	if err != nil {
		return nil, errAccountConfig
	}
	return service, nil
}
