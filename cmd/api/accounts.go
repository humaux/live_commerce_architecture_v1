package main

import (
	"errors"

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

// Both merchant administration and hosted signing use the shared historical
// keyring format. Loading keys grants no merchant HTTP authority by itself.
func loadAccountKeys(getenv func(string) string) (*accounts.Keyring, error) {
	keyring, err := accounts.LoadKeyring(getenv)
	if err != nil {
		return nil, errAccountConfig
	}
	return keyring, nil
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
