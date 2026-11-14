// Purpose: assemble Meta ingress and isolated consumers; API/worker callers supply scoped pools.
// Depends on: existing inbox, verifier, platform pool checks and River.
// Invariant: literal configured identity admission precedes methods; no worker starts in the API.
package meta

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/platform"
)

var ErrRuntimeDatabase = errors.New("meta: runtime database unavailable")

// NewWebhookRouter admits only literal, configured callback paths. It borrows
// the ingress pool, starts no River worker and never trusts a forwarded host.
func NewWebhookRouter(ctx context.Context, pool *pgxpool.Pool, keys *PayloadKeyring,
	endpoints []WebhookEndpoint) (http.Handler, error) {
	if ctx == nil || pool == nil || keys == nil || len(endpoints) < 1 || len(endpoints) > 16 {
		return nil, ErrRuntimeConfig
	}
	routes := make(map[string]http.Handler, len(endpoints))
	for _, endpoint := range endpoints {
		_, duplicate := routes[endpoint.Path]
		if endpoint.Verifier == nil || !endpoint.Verifier.valid() ||
			endpoint.Path != "/v1/meta/webhooks/"+endpoint.Verifier.appID+"/"+endpoint.Verifier.object ||
			duplicate {
			return nil, ErrRuntimeConfig
		}
		routes[endpoint.Path] = nil
	}
	preflight, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	inbox, err := NewInbox(preflight, pool, keys)
	if err != nil {
		return nil, ErrRuntimeDatabase
	}
	var ready bool
	if err := pool.QueryRow(preflight, `SELECT meta_inbox.runtime_ready()`).Scan(&ready); err != nil || !ready {
		return nil, ErrRuntimeDatabase
	}
	for _, endpoint := range endpoints {
		h, err := NewInboxHandler(endpoint.Verifier, inbox)
		if err != nil {
			return nil, ErrRuntimeConfig
		}
		routes[endpoint.Path] = h
	}
	return webhookDispatch(routes), nil
}

// webhookRoute rows govern transport shape, never configured identity admission.
type webhookRoute struct{ Method, Path string }
type WebhookRouteTable struct{}

var WebhookRoutes = WebhookRouteTable{}

func (WebhookRouteTable) HasPrefix(path string) bool { return strings.HasPrefix(path, "/v1/meta") }

//contractdrift:routes /v1/meta matchWebhookRoute webhookMethodAllowed mount=WebhookRoutes
var webhookRows = []webhookRoute{
	{Method: http.MethodGet, Path: "/v1/meta/webhooks/{app_id}/{object}"},
	{Method: http.MethodPost, Path: "/v1/meta/webhooks/{app_id}/{object}"},
}

type webhookMatch struct{ pattern, literalPath string }

func matchWebhookRoute(u *url.URL) webhookMatch {
	for _, entry := range webhookRows {
		if match, ok := matchWebhookPattern(entry, u); ok {
			return match
		}
	}
	return webhookMatch{}
}

func webhookMethodAllowed(pattern, method string) bool {
	for _, entry := range webhookRows {
		if entry.Path == pattern && entry.Method == method {
			return true
		}
	}
	return false
}

// Match the original URL without cleaning or unescaping. The resulting literal
// key must still be admitted by the constructor's configured endpoint map.
func matchWebhookPattern(entry webhookRoute, u *url.URL) (webhookMatch, bool) {
	if u.RawPath != "" || u.EscapedPath() != u.Path {
		return webhookMatch{}, false
	}
	pattern, path := strings.Split(entry.Path, "/"), strings.Split(u.Path, "/")
	if len(pattern) != len(path) {
		return webhookMatch{}, false
	}
	for i, segment := range pattern {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			if path[i] == "" || path[i] == "." || path[i] == ".." {
				return webhookMatch{}, false
			}
		} else if segment != path[i] {
			return webhookMatch{}, false
		}
	}
	return webhookMatch{pattern: entry.Path, literalPath: u.Path}, true
}

type webhookDispatcher struct{ routes map[string]http.Handler }

func webhookDispatch(routes map[string]http.Handler) http.Handler {
	return &webhookDispatcher{routes: routes}
}

// ServeHTTP owns path and method admission; the configured handler retains all
// challenge, signature, body, security-header and durable inbox behavior.
func (d *webhookDispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	match := matchWebhookRoute(r.URL)
	h := d.routes[match.literalPath]
	if match.literalPath == "" || h == nil {
		http.NotFound(w, r)
		return
	}
	if !webhookMethodAllowed(match.pattern, r.Method) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Allow", "GET, POST")
		writeCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	h.ServeHTTP(w, r)
}

// NewConsumerClient creates only the fixed Meta queue and worker. Its separate
// River schema isolates schema-wide maintenance; the lifecycle pool and the
// consumer's SQL authority remain separate.
func NewConsumerClient(ctx context.Context, workerPool, consumerPool *pgxpool.Pool,
	keys *PayloadKeyring, concurrency int) (*river.Client[pgx.Tx], error) {
	return NewConsumerClientWithClaims(ctx, workerPool, consumerPool, keys, ClaimsActorKey{}, concurrency)
}

// NewConsumerClientWithClaims is NewConsumerClient with claim staging (meta-claims-intake-v1
// §5.1) when actor holds a key; the zero key is exactly NewConsumerClient. It adds no pool, no
// River job kind and no privilege: the consumer's only new authority is EXECUTE on
// meta_inbox.stage_claim_intake.
func NewConsumerClientWithClaims(ctx context.Context, workerPool, consumerPool *pgxpool.Pool,
	keys *PayloadKeyring, actor ClaimsActorKey, concurrency int) (*river.Client[pgx.Tx], error) {
	if ctx == nil || workerPool == nil || consumerPool == nil || keys == nil ||
		concurrency < 1 || concurrency > 16 {
		return nil, ErrRuntimeConfig
	}
	preflight, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := platform.ValidateMetaWorkerPool(preflight, workerPool); err != nil {
		return nil, ErrRuntimeDatabase
	}
	worker, err := NewConsumerWorkerWithClaims(preflight, consumerPool, keys, actor)
	if err != nil {
		return nil, ErrRuntimeDatabase
	}
	if err := platform.ValidateSameDatabase(preflight, workerPool, consumerPool); err != nil {
		return nil, ErrRuntimeDatabase
	}
	var ready bool
	if err := workerPool.QueryRow(preflight, `SELECT meta_inbox.runtime_ready()`).Scan(&ready); err != nil || !ready {
		return nil, ErrRuntimeDatabase
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, worker)
	client, err := river.NewClient(riverpgxv5.New(workerPool), &river.Config{
		Schema: riverSchema, Workers: workers,
		Queues: map[string]river.QueueConfig{inboxQueue: {MaxWorkers: concurrency}},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		return nil, ErrRuntimeDatabase
	}
	return client, nil
}
