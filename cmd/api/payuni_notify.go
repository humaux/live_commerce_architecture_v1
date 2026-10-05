// payuni_notify.go owns the API process wiring of POST /v1/hooks/payuni/notify/{endpoint_token}
// (contracts/payuni-wire-v1.md Amendment W4-01B). It never reads LIVE keys or the hosted signing
// callbacks, never opens a pool unless the flag is on, and never admits a LIVE profile.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/payments/payuninotify"
	"livecommerce/internal/platform"
)

var errPayuniNotifyConfig = errors.New("payuni_notify_invalid_config")
var errPayuniNotifyDatabase = errors.New("payuni_notify_database_unavailable")

type payuniNotifyConfig struct {
	enabled bool
	dsn     string
	profile string
	keys    *accounts.Keyring
}

func (payuniNotifyConfig) String() string     { return "payuniNotifyConfig{redacted}" }
func (c payuniNotifyConfig) GoString() string { return c.String() }
func (payuniNotifyConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal("payuniNotifyConfig{redacted}")
}

// loadPayuniNotifyConfig reads the notify-only flag, ingress DSN and payment profile, and opens the
// shared account keyring (the same custody that sealed each connection's HashKey/HashIV). LIVE is
// refused: a notification is never admitted against a live merchant (payuninotify rejects LIVE).
func loadPayuniNotifyConfig(getenv func(string) string, addr string) (payuniNotifyConfig, error) {
	var c payuniNotifyConfig
	if getenv == nil {
		return c, errPayuniNotifyConfig
	}
	enabled, err := flag(getenv("COMMERCE_PAYUNI_NOTIFY_ENABLED"))
	if err != nil {
		return c, errPayuniNotifyConfig
	}
	if !enabled { // disabled reads nothing else
		return c, nil
	}
	if !privateIdentityAddress(addr) {
		return payuniNotifyConfig{}, errPayuniNotifyConfig
	}
	c.dsn = getenv("COMMERCE_PAYUNI_INGRESS_DATABASE_URL")
	if len(c.dsn) < 1 || len(c.dsn) > 8192 || strings.TrimSpace(c.dsn) == "" {
		return payuniNotifyConfig{}, errPayuniNotifyConfig
	}
	c.profile = getenv("COMMERCE_PAYMENT_PROFILE")
	if c.profile != "PROVIDER_MOCK" && c.profile != "SANDBOX" {
		return payuniNotifyConfig{}, errPayuniNotifyConfig
	}
	if c.keys, err = accounts.LoadKeyring(getenv); err != nil {
		return payuniNotifyConfig{}, errPayuniNotifyConfig
	}
	c.enabled = true
	return c, nil
}

// buildPayuniNotifyHandler opens the dedicated notify ingress pool (never the merchant pool), proves
// it points at the same database as mainPool, and builds the handler. The returned close func
// releases the ingress pool.
func buildPayuniNotifyHandler(ctx context.Context, mainPool *pgxpool.Pool, c payuniNotifyConfig) (http.Handler, func(), error) {
	if !c.enabled {
		return nil, func() {}, nil
	}
	if ctx == nil || mainPool == nil || c.keys == nil {
		return nil, nil, errPayuniNotifyConfig
	}
	startup, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	ingressPool, err := platform.OpenPayuniIngressPool(startup, c.dsn)
	if err != nil {
		return nil, nil, errPayuniNotifyDatabase
	}
	preflight, done := context.WithTimeout(startup, 5*time.Second)
	defer done()
	if err := platform.ValidateSameDatabase(preflight, mainPool, ingressPool); err != nil {
		ingressPool.Close()
		return nil, nil, errPayuniNotifyDatabase
	}
	inbox, err := payuninotify.NewInbox(preflight, ingressPool, c.keys, c.profile)
	if err != nil {
		ingressPool.Close()
		return nil, nil, errPayuniNotifyDatabase
	}
	h, err := payuninotify.NewHandler(inbox)
	if err != nil {
		ingressPool.Close()
		return nil, nil, errPayuniNotifyDatabase
	}
	return h, ingressPool.Close, nil
}

// mountPayuniNotify reserves the whole /v1/hooks/payuni namespace, raw and path.Clean'ed, before
// ServeMux can rewrite aliases (as mountMeta does); the handler itself rejects every non-literal path.
func mountPayuniNotify(fallback, payuniHandler http.Handler) http.Handler {
	if payuniHandler == nil {
		return fallback
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/hooks/payuni") ||
			strings.HasPrefix(path.Clean(r.URL.Path), "/v1/hooks/payuni") {
			payuniHandler.ServeHTTP(w, r)
			return
		}
		fallback.ServeHTTP(w, r)
	})
}
