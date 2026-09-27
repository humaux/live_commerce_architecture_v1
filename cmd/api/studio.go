package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/live"
)

var errStudioConfig = errors.New("studio_invalid_config")
var errStudioDatabase = errors.New("studio_database_unavailable")

type studioConfig struct{ enabled bool }

func loadStudioConfig(getenv func(string) string, identityEnabled bool, addr string) (studioConfig, error) {
	if getenv == nil {
		return studioConfig{}, errStudioConfig
	}
	enabled, err := flag(getenv("COMMERCE_STUDIO_ENABLED"))
	if err != nil {
		return studioConfig{}, errStudioConfig
	}
	if !enabled {
		return studioConfig{}, nil
	}
	if !identityEnabled || !privateIdentityAddress(addr) {
		return studioConfig{}, errStudioConfig
	}
	return studioConfig{enabled: true}, nil
}

// API owns only an insert-only River client; its lifecycle stays in media-worker.
func buildStudioPlanner(ctx context.Context, pool *pgxpool.Pool, config studioConfig) (*live.MediaPlanner, error) {
	if !config.enabled {
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
