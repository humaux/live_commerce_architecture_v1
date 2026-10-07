// Purpose: Studio config split (R1 ruling G2: planning vs LiveKit media) and its API-side builders — the river_media-backed media planner the insert-only main-schema river client for the A5 live-session flow reads, and the one for the W6-05B operations-ledger query/retry routes. The API never starts a worker or queue; media-worker and claims-worker own those lifecycles.
// Depends on: live.NewMediaPlanner, river (river_media / river schemas), COMMERCE_STUDIO_ENABLED / COMMERCE_STUDIO_MEDIA_ENABLED, live.media_plan_ready (migrations).
// Used by: cmd/api main.go (studioPlanner + liveFlowJobs + operationJobs → httpapi.Options), cmd/api/studio_test.go.
package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/live"
)

var errStudioConfig = errors.New("studio_invalid_config")
var errStudioDatabase = errors.New("studio_database_unavailable")

// studioConfig splits Studio (R1 ruling G2): enabled = live-session planning, keyword claims and
// claim-source (COMMERCE_STUDIO_ENABLED); media = LiveKit rehearsal/input planning
// (COMMERCE_STUDIO_MEDIA_ENABLED, MOCK-only, never deployed in R1; requires enabled).
type studioConfig struct{ enabled, media bool }

func loadStudioConfig(getenv func(string) string, identityEnabled bool, addr string) (studioConfig, error) {
	if getenv == nil {
		return studioConfig{}, errStudioConfig
	}
	enabled, err := flag(getenv("COMMERCE_STUDIO_ENABLED"))
	if err != nil {
		return studioConfig{}, errStudioConfig
	}
	media, err := flag(getenv("COMMERCE_STUDIO_MEDIA_ENABLED"))
	if err != nil || (media && !enabled) {
		return studioConfig{}, errStudioConfig
	}
	if !enabled {
		return studioConfig{}, nil
	}
	if !identityEnabled || !privateIdentityAddress(addr) {
		return studioConfig{}, errStudioConfig
	}
	return studioConfig{enabled: true, media: media}, nil
}

// buildStudioPlanner builds the media planner only when media is on; planning-only Studio never
// touches the media subsystem (no live.media_plan_ready(), no river_media client).
// API owns only an insert-only River client; its lifecycle stays in media-worker.
func buildStudioPlanner(ctx context.Context, pool *pgxpool.Pool, config studioConfig) (*live.MediaPlanner, error) {
	if !config.media {
		return nil, nil
	}
	if ctx == nil || pool == nil {
		return nil, errStudioDatabase
	}
	var ready bool
	if err := pool.QueryRow(ctx, `SELECT live.media_plan_ready()`).Scan(&ready); err != nil || !ready {
		return nil, errStudioDatabase
	}
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river_media"})
	if err != nil {
		return nil, errStudioDatabase
	}
	planner, err := live.NewMediaPlanner(jobs)
	if err != nil {
		return nil, errStudioDatabase
	}
	return planner, nil
}

// buildLiveFlowJobs returns the insert-only main-schema River client the A5 live-session flow routes
// use to enqueue read-only external operations (meta.live_videos, migrations/0118). The claims-worker
// owns the dispatch lifecycle; the API never starts a worker or queue on this client. nil when Studio
// is off (the A5 routes are unmounted anyway). Unlike buildStudioPlanner this is the default "river"
// schema, matching the ads/meta-connect/accounts builders — not river_media.
func buildLiveFlowJobs(pool *pgxpool.Pool, enabled bool) (*river.Client[pgx.Tx], error) {
	if !enabled {
		return nil, nil
	}
	if pool == nil {
		return nil, errStudioDatabase
	}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		return nil, errStudioDatabase
	}
	return jobs, nil
}

// buildOperationJobs returns the insert-only main-schema River client the operations-ledger query/retry routes (W6-05B, internal/httpapi/operations.go)
// use to enqueue the follow-up external_operation_v1 job (default or ads queue). The claims-worker / ads-worker own the dispatch lifecycle; the API never
// starts a worker or queue on this client. Always built: the ledger is a merchant surface of every deployment, independent of Studio.
func buildOperationJobs(pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	if pool == nil {
		return nil, errStudioDatabase
	}
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		return nil, errStudioDatabase
	}
	return jobs, nil
}
