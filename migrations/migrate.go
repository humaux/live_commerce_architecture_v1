// Package migrations applies forward-only, checksummed business migrations.
package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

//go:embed *.sql
var files embed.FS

// Apply requires an explicitly provisioned migration-owner pool, never the API pool.
// ponytail: a single forward migration now; add ordered migration discovery when a second exists.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(718020260920)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	const version = "0001_foundation.sql"
	body, err := files.ReadFile(version)
	if err != nil {
		return err
	}
	want := fmt.Sprintf("%x", sha256.Sum256(body))
	var have string
	if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT checksum FROM public.lc_schema_migrations WHERE version=$1),'')`, version).Scan(&have); err != nil {
		return err
	}
	if have != "" && have != want {
		return fmt.Errorf("migration checksum mismatch: %s", version)
	}
	if have == "" {
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES ($1,$2)`, version, want); err != nil {
			return err
		}
	}
	upstream, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{Schema: "river", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		return err
	}
	if _, err = upstream.MigrateTx(ctx, tx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}
	// Insert-only API role. A future worker receives separately scoped operational privileges.
	if _, err = tx.Exec(ctx, `GRANT SELECT, INSERT ON river.river_job TO commerce_runtime; GRANT USAGE ON SEQUENCE river.river_job_id_seq TO commerce_runtime`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
