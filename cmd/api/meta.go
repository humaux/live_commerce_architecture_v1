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

	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/platform"
)

var errMetaConfig = errors.New("meta_api_invalid_config")
var errMetaDatabase = errors.New("meta_api_database_unavailable")

type metaConfig struct {
	enabled   bool
	dsn       string
	keys      *meta.PayloadKeyring
	endpoints []meta.WebhookEndpoint
}

func (metaConfig) String() string               { return "metaConfig{redacted}" }
func (c metaConfig) GoString() string           { return c.String() }
func (metaConfig) MarshalJSON() ([]byte, error) { return json.Marshal("metaConfig{redacted}") }

func loadMetaConfig(getenv func(string) string, addr string) (metaConfig, error) {
	var c metaConfig
	if getenv == nil {
		return c, errMetaConfig
	}
	enabled, err := flag(getenv("COMMERCE_META_WEBHOOK_ENABLED"))
	if err != nil {
		return c, errMetaConfig
	}
	if !enabled {
		return c, nil
	}
	if !privateIdentityAddress(addr) {
		return metaConfig{}, errMetaConfig
	}
	c.dsn = getenv("COMMERCE_META_INGRESS_DATABASE_URL")
	if len(c.dsn) < 1 || len(c.dsn) > 8192 || strings.TrimSpace(c.dsn) == "" {
		return metaConfig{}, errMetaConfig
	}
	c.keys, err = meta.LoadPayloadKeyring(getenv)
	if err != nil {
		return metaConfig{}, errMetaConfig
	}
	c.endpoints, err = meta.LoadWebhookEndpoints(getenv)
	if err != nil {
		return metaConfig{}, errMetaConfig
	}
	c.enabled = true
	return c, nil
}

func buildMetaHandler(ctx context.Context, mainPool *pgxpool.Pool, c metaConfig) (http.Handler, func(), error) {
	if !c.enabled {
		return nil, func() {}, nil
	}
	if ctx == nil || mainPool == nil || c.keys == nil || len(c.endpoints) == 0 {
		return nil, nil, errMetaConfig
	}
	startup, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	ingressPool, err := platform.OpenMetaIngressPool(startup, c.dsn)
	if err != nil {
		return nil, nil, errMetaDatabase
	}
	preflight, done := context.WithTimeout(startup, 5*time.Second)
	defer done()
	if err := platform.ValidateSameDatabase(preflight, mainPool, ingressPool); err != nil {
		ingressPool.Close()
		return nil, nil, errMetaDatabase
	}
	h, err := meta.NewWebhookRouter(preflight, ingressPool, c.keys, c.endpoints)
	if err != nil {
		ingressPool.Close()
		return nil, nil, errMetaDatabase
	}
	return h, ingressPool.Close, nil
}

// Reserve the complete Meta namespace before ServeMux can clean aliases. The
// Meta router receives the original request and rejects every nonliteral path.
func mountMeta(fallback, metaHandler http.Handler) http.Handler {
	if metaHandler == nil {
		return fallback
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/meta") ||
			strings.HasPrefix(path.Clean(r.URL.Path), "/v1/meta") {
			metaHandler.ServeHTTP(w, r)
			return
		}
		fallback.ServeHTTP(w, r)
	})
}
