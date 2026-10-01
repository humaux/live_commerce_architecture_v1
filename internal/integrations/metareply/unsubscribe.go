package metareply

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	metaoauth "livecommerce/internal/integrations/meta/oauth"
	"livecommerce/internal/integrations/meta/pagetoken/pageopen"
	"livecommerce/internal/platform"
)

// Unsubscriber runs the merchant disconnect's durable jobs (migration 0100, integration.meta_unsubscribe_jobs): DELETE
// /{page-id}/subscribed_apps with the Page token the API sealed to the PUBLIC ring and can never open again. It lives in the
// claims-worker, the only process holding the private ring. Best effort by contract ("Merchant connect (R4)" 4): the binding and the
// route are already disabled when a job exists, so a leftover Meta-side subscription only yields quarantined events.
//
// Retry rule: DELETE changes Meta state, so only a definite "not applied" answer (HTTP 429/503) is retried (bounded by the SQL: 5
// attempts, exponential backoff). Anything ambiguous (transport error, timeout, other 5xx, a body that is not success=true, an expired
// lease) is UNKNOWN and never repeated; a 4xx is a definite refusal (FAILED). Every terminal state wipes the sealed token and audits.
type Unsubscriber struct {
	pool  *pgxpool.Pool
	keys  *PageTokenKeyring
	v2    *pageopen.Keyring // nil: a v2 token cannot be opened; the job retries (bounded) and then fails with its token wiped
	graph *metaoauth.Graph
	idle  time.Duration
}

const (
	unsubscribeCallTimeout = 15 * time.Second // < the 60 s SQL lease, so an expired lease really means "outcome unknown"
	unsubscribeIdle        = 5 * time.Second
)

// NewUnsubscriber validates pool (the commerce_claims_worker authority) and the Graph config exactly like RoutesV2.
func NewUnsubscriber(pool *pgxpool.Pool, keys *PageTokenKeyring, v2 *pageopen.Keyring, cfg Config) (*Unsubscriber, error) {
	if pool == nil || keys == nil || cfg.Validate() != nil {
		return nil, ErrConfig
	}
	if err := platform.ValidateWorkerPool(context.Background(), pool, platform.WorkerClaims); err != nil {
		return nil, err
	}
	hc := &http.Client{Timeout: unsubscribeCallTimeout}
	if cfg.HTTPClient != nil {
		copied := *cfg.HTTPClient
		hc = &copied
		if hc.Timeout == 0 || hc.Timeout > unsubscribeCallTimeout {
			hc.Timeout = unsubscribeCallTimeout
		}
	}
	graph, err := metaoauth.NewGraph(cfg.GraphBaseURL, cfg.GraphVersion, hc)
	if err != nil {
		return nil, ErrConfig
	}
	return &Unsubscriber{pool: pool, keys: keys, v2: v2, graph: graph, idle: unsubscribeIdle}, nil
}

// SetIdle changes the poll interval (tests and the browser gate use a short one).
func (u *Unsubscriber) SetIdle(d time.Duration) { u.idle = d }

// Run polls until ctx ends. Fixed log fields only: an error value can carry row data through a driver message.
func (u *Unsubscriber) Run(ctx context.Context) {
	for ctx.Err() == nil {
		did, err := u.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Warn("meta_unsubscribe_error", "err", "database")
		}
		if did && err == nil {
			continue // backlog: no sleep between jobs
		}
		select {
		case <-ctx.Done():
		case <-time.After(u.idle):
		}
	}
}

// RunOnce leases and executes at most one job (false = nothing due).
func (u *Unsubscriber) RunOnce(ctx context.Context) (bool, error) {
	var job struct {
		id, tenant, store, binding, page, keyID string
		version                                 int64
		nonce, ct                               []byte
		attempt                                 int
	}
	// integration.claim_meta_unsubscribe: definer commerce_integration_writer; leases one due job (an expired lease is closed UNKNOWN).
	err := u.pool.QueryRow(ctx, `SELECT o_job::text,o_tenant::text,o_store::text,o_binding::text,o_page,o_version,o_key_id,o_nonce,o_ciphertext,o_attempt
		FROM integration.claim_meta_unsubscribe()`).Scan(&job.id, &job.tenant, &job.store, &job.binding, &job.page, &job.version, &job.keyID, &job.nonce, &job.ct, &job.attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	outcome, code := u.attempt(ctx, job.tenant, job.store, job.binding, job.page, job.keyID, job.version, job.nonce, job.ct)
	clear(job.ct)
	// A cancelled ctx (shutdown) between the call and here leaves the lease to expire: UNKNOWN, never repeated.
	fin, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	// integration.finish_meta_unsubscribe: wipes the sealed token on a terminal state and audits it.
	_, err = u.pool.Exec(fin, `SELECT integration.finish_meta_unsubscribe($1::uuid,$2,$3)`, job.id, outcome, code)
	return true, err
}

// attempt opens the token and makes the one DELETE; it returns the finish outcome and a fixed code.
func (u *Unsubscriber) attempt(ctx context.Context, tenant, store, binding, page, keyID string, version int64, nonce, ct []byte) (outcome, code string) {
	secret, err := openPageToken(u.keys, u.v2, PageTokenScope{TenantID: tenant, StoreID: store, BindingID: binding, Provider: "facebook", AssetID: page, Version: version},
		keyID, nonce, ct)
	if err != nil {
		return "RETRY", "token_unavailable" // config (private ring not mounted yet) may be fixed before the bounded attempts run out
	}
	defer clear(secret.Reveal())
	callCtx, cancel := context.WithTimeout(ctx, unsubscribeCallTimeout)
	defer cancel()
	rep, err := u.graph.Do(callCtx, http.MethodDelete, page+"/subscribed_apps", nil, secret.Reveal(), nil)
	return classifyUnsubscribe(rep, err)
}

// classifyUnsubscribe maps one Graph answer to the finish outcome (see the type comment for the retry rule).
func classifyUnsubscribe(rep metaoauth.Reply, err error) (outcome, code string) {
	switch {
	case err != nil:
		return "UNKNOWN", "graph_unconfirmed"
	case rep.OK():
		var ok struct {
			Success bool `json:"success"`
		}
		if json.Unmarshal(rep.Body, &ok) == nil && ok.Success {
			return "SUCCEEDED", "graph_unsubscribed"
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
