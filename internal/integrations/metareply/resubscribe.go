// Purpose: the inbox read side's durable resubscribe jobs (migration 0122, integration.meta_resubscribe_jobs): the
// claims-worker end that leases one job, POSTs /{page-id}/subscribed_apps?subscribed_fields=feed,messages with the head
// Page token the API sealed to the PUBLIC ring, and finishes it. Only a definite not-applied answer (429/503) is retried
// (bounded by the SQL); everything ambiguous is UNKNOWN and never repeated.
// Depends on: integration.claim_meta_resubscribe / integration.finish_meta_resubscribe (0122), internal/integrations/
// meta/pagetoken/pageopen (v2 private ring), internal/integrations/meta/oauth (Graph), livecommerce/internal/platform.
// Used by: cmd/claims-worker (run loop, the only process holding the private ring).

package metareply

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/platform"
)

// Resubscriber runs the inbox read-side's durable subscription jobs (migration 0122, integration.meta_resubscribe_jobs):
// POST /{page-id}/subscribed_apps?subscribed_fields=feed,messages with the head Page token the API sealed to the PUBLIC
// ring and can never open again. It lives in the claims-worker, the only process holding the private ring. The migration
// backfill enqueues one job per active connection; the read-back of the result into integration.binding_capabilities and the
// capability probe are W1-01B (out of LC-B3's scope — the subscription itself is the deliverable here).
//
// Retry rule: a subscribe POST changes Meta state, so only a definite "not applied" answer (HTTP 429/503) is retried
// (bounded by the SQL: 5 attempts, exponential backoff). Anything ambiguous (transport error, timeout, other 5xx, a body
// that is not success=true, an expired lease) is UNKNOWN and never repeated; a 4xx is a definite refusal (FAILED). Every
// terminal state wipes the sealed token and audits.
type Resubscriber struct {
	pool  *pgxpool.Pool
	keys  *PageTokenKeyring
	v2    *pageopen.Keyring // nil: a v2 token cannot be opened; the job retries (bounded) and then fails with its token wiped
	graph *metaoauth.Graph
	idle  time.Duration
}

const (
	resubscribeCallTimeout = 15 * time.Second // < the 60 s SQL lease, so an expired lease really means "outcome unknown"
	resubscribeIdle        = 5 * time.Second
)

// NewResubscriber validates pool (the commerce_claims_worker authority) and the Graph config exactly like RoutesV2.
func NewResubscriber(pool *pgxpool.Pool, keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) (*Resubscriber, error) {
	if pool == nil || keys == nil || cfg.Validate() != nil {
		return nil, ErrConfig
	}
	if err := platform.ValidateWorkerPool(context.Background(), pool, platform.WorkerClaims); err != nil {
		return nil, err
	}
	hc := &http.Client{Timeout: resubscribeCallTimeout}
	if cfg.HTTPClient != nil {
		copied := *cfg.HTTPClient
		hc = &copied
		if hc.Timeout == 0 || hc.Timeout > resubscribeCallTimeout {
			hc.Timeout = resubscribeCallTimeout
		}
	}
	graph, err := metaoauth.NewGraph(cfg.GraphBaseURL, cfg.GraphVersion, hc)
	if err != nil {
		return nil, ErrConfig
	}
	return &Resubscriber{pool: pool, keys: keys, v2: v2, graph: graph, idle: resubscribeIdle}, nil
}

// SetIdle changes the poll interval (tests use a short one).
func (r *Resubscriber) SetIdle(d time.Duration) { r.idle = d }

// Run polls until ctx ends. Fixed log fields only: an error value can carry row data through a driver message.
func (r *Resubscriber) Run(ctx context.Context) {
	for ctx.Err() == nil {
		did, err := r.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Warn("meta_resubscribe_error", "err", "database")
		}
		if did && err == nil {
			continue // backlog: no sleep between jobs
		}
		select {
		case <-ctx.Done():
		case <-time.After(r.idle):
		}
	}
}

// RunOnce leases and executes at most one job (false = nothing due).
func (r *Resubscriber) RunOnce(ctx context.Context) (bool, error) {
	var job struct {
		id, tenant, store, binding, page, keyID string
		version                                 int64
		nonce, ct                               []byte
		attempt                                 int
	}
	// integration.claim_meta_resubscribe: definer commerce_integration_writer; leases one due job (an expired lease is closed UNKNOWN).
	err := r.pool.QueryRow(ctx, `SELECT o_job::text,o_tenant::text,o_store::text,o_binding::text,o_page,o_version,o_key_id,o_nonce,o_ciphertext,o_attempt
		FROM integration.claim_meta_resubscribe()`).Scan(&job.id, &job.tenant, &job.store, &job.binding, &job.page, &job.version, &job.keyID, &job.nonce, &job.ct, &job.attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	outcome, code := r.attempt(ctx, job.tenant, job.store, job.binding, job.page, job.keyID, job.version, job.nonce, job.ct)
	clear(job.ct)
	// A cancelled ctx (shutdown) between the call and here leaves the lease to expire: UNKNOWN, never repeated.
	fin, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	// integration.finish_meta_resubscribe: wipes the sealed token on a terminal state and audits it.
	_, err = r.pool.Exec(fin, `SELECT integration.finish_meta_resubscribe($1::uuid,$2,$3)`, job.id, outcome, code)
	return true, err
}

// attempt opens the token and makes the one subscribe POST; it returns the finish outcome and a fixed code.
func (r *Resubscriber) attempt(ctx context.Context, tenant, store, binding, page, keyID string, version int64, nonce, ct []byte) (outcome, code string) {
	secret, err := openPageToken(r.keys, r.v2, PageTokenScope{TenantID: tenant, StoreID: store, BindingID: binding, Provider: "facebook", AssetID: page, Version: version},
		keyID, nonce, ct)
	if err != nil {
		return "RETRY", "token_unavailable" // config (private ring not mounted yet) may be fixed before the bounded attempts run out
	}
	defer clear(secret.Reveal())
	callCtx, cancel := context.WithTimeout(ctx, resubscribeCallTimeout)
	defer cancel()
	rep, err := r.graph.Do(callCtx, http.MethodPost, page+"/subscribed_apps", url.Values{"subscribed_fields": {"feed,messages"}}, secret.Reveal(), nil)
	return classifyResubscribe(rep, err)
}

// classifyResubscribe maps one Graph answer to the finish outcome (see the type comment for the retry rule).
func classifyResubscribe(rep metaoauth.Reply, err error) (outcome, code string) {
	switch {
	case err != nil:
		return "UNKNOWN", "graph_unconfirmed"
	case rep.OK():
		var ok struct {
			Success bool `json:"success"`
		}
		if json.Unmarshal(rep.Body, &ok) == nil && ok.Success {
			return "SUCCEEDED", "graph_resubscribed"
		}
		return "UNKNOWN", "graph_unconfirmed"
	case rep.Status == http.StatusTooManyRequests || rep.Status == http.StatusServiceUnavailable:
		return "RETRY", "graph_busy"
	case rep.Status >= 400 && rep.Status < 500:
		return "FAILED", "graph_refused"
	default:
		return "UNKNOWN", "graph_unconfirmed"
	}
}
