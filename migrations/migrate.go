// Package migrations applies forward-only, checksummed business migrations.
package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

//go:embed *.sql
var files embed.FS

var ErrMigrationBusy = errors.New("another migration is running")

// Apply requires an explicitly provisioned migration-owner pool, never the API pool.
// Embedded numbered SQL files apply in lexical order; old checksums never change.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Only the lock winner detaches a dedicated connection across upstream
	// transaction boundaries. Waiters remain pool-bounded and fail fast.
	acquired, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	var locked bool
	if err = acquired.QueryRow(ctx, `SELECT pg_try_advisory_lock(718020260920)`).Scan(&locked); err != nil {
		// A lost response may hide an acquired session lock. Close, never reuse it.
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		_ = acquired.Conn().Close(cleanup)
		stop()
		acquired.Release()
		return err
	}
	if !locked {
		acquired.Release()
		return ErrMigrationBusy
	}
	lockConn := acquired.Hijack()
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = lockConn.Close(cleanup) // Closing also releases the session advisory lock.
	}()
	tx, err := lockConn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	versions, err := fs.Glob(files, "[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil || len(versions) == 0 {
		return fmt.Errorf("discover migrations: %v", err)
	}
	rows, err := tx.Query(ctx, `SELECT version FROM public.lc_schema_migrations ORDER BY version`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var applied string
		if err = rows.Scan(&applied); err != nil {
			rows.Close()
			return err
		}
		if !slices.Contains(versions, applied) {
			rows.Close()
			return fmt.Errorf("database migration unknown to this binary: %s", applied)
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, version := range versions {
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
				return fmt.Errorf("apply %s: %w", version, err)
			}
			if _, err = tx.Exec(ctx, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES ($1,$2)`, version, want); err != nil {
				return err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	upstream, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{Schema: "river", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		return err
	}
	// River 006 adds an enum value needed by later steps. PostgreSQL requires
	// committing that step before use; the upstream runner owns those boundaries.
	// Its ledger and the business checksum make an interrupted Apply resumable.
	if _, err = upstream.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}
	// River InsertTx's ON CONFLICT returns the existing job by updating kind only.
	// Do not grant worker permissions to mutate state, attempts, payload or delete.
	if _, err = lockConn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE(kind) ON river.river_job TO commerce_runtime; GRANT USAGE ON SEQUENCE river.river_job_id_seq TO commerce_runtime`); err != nil {
		return err
	}
	// Checkout schedules only its fixed expiry job in the caller's transaction.
	// The private writer can read that row to prove kind/args before holding stock.
	if _, err = lockConn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE(kind) ON river.river_job TO commerce_checkout_runtime;
		GRANT USAGE ON SEQUENCE river.river_job_id_seq TO commerce_checkout_runtime;
		GRANT SELECT ON river.river_job TO commerce_checkout_writer`); err != nil {
		return err
	}
	// River's ordinary worker login needs queue lifecycle/leader/client tables,
	// but receives no identity or commerce authority. Reapply after upstream
	// upgrades so only the actual River schema is covered (no default privileges).
	if _, err = lockConn.Exec(ctx, `GRANT USAGE ON SCHEMA river TO commerce_worker;
		GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA river TO commerce_worker;
		REVOKE ALL ON river.river_migration FROM commerce_worker;
		GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA river TO commerce_worker`); err != nil {
		return err
	}
	return nil
}
