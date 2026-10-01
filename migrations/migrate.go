// Package migrations owns applying the embedded, forward-only, checksummed business SQL migrations
// (0001-) and the River schema, under one advisory lock, then exiting.
//
// It never edits or reorders an applied migration (a checksum mismatch stops the run), never runs
// down-migrations, and never runs from the API or a worker: cmd/migrate is its only production
// caller. Numbering: the current release branch owns 0060-0079 (0084 is integrator-assigned to worker-authority-split); only the integrator merges
// migrations.
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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

//go:embed *.sql post_river/*.sql
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
	postVersions, err := fs.Glob(files, "post_river/[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil || len(postVersions) == 0 {
		return fmt.Errorf("discover post-River migrations: %v", err)
	}
	knownVersions := append(slices.Clone(versions), postVersions...)
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
		if !slices.Contains(knownVersions, applied) {
			rows.Close()
			return fmt.Errorf("database migration unknown to this binary: %s", applied)
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if err = applyVersions(ctx, tx, versions); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	// River 006 adds an enum value needed by later steps. PostgreSQL requires
	// committing that step before use; the upstream runner owns those boundaries.
	// Independent native ledgers retain the 0031/0032 fail-closed readiness
	// fences until the final post-River cutovers commit. Queues alone do not
	// isolate River's schema-wide rescuer/scheduler/cleaner.
	for _, schema := range []string{"river", "river_meta", "river_payment", "river_expiry", "river_media"} {
		upstream, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{Schema: schema, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
		if err != nil {
			return err
		}
		if _, err = upstream.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
			return err
		}
	}
	// River InsertTx's ON CONFLICT returns the existing job by updating kind only.
	// Do not grant worker permissions to mutate state, attempts, payload or delete.
	if _, err = lockConn.Exec(ctx, `GRANT SELECT, INSERT, UPDATE(kind) ON river.river_job TO commerce_runtime; GRANT USAGE ON SEQUENCE river.river_job_id_seq TO commerce_runtime`); err != nil {
		return err
	}
	// River's ordinary worker login needs queue lifecycle/leader/client tables,
	// but receives no identity or commerce authority. Reapply after upstream
	// upgrades so only the actual River schema is covered (no default privileges).
	// T21-02: only the two workers that run a client on the main schema (claims-worker's default
	// lane, ads-worker's queue ads) hold it; commerce_worker is an empty legacy role, so every
	// Apply also strips whatever an older release granted it on the three legacy schemas
	// (post_river/0018 then asserts it holds nothing).
	if _, err = lockConn.Exec(ctx, `REVOKE ALL ON ALL TABLES IN SCHEMA river,river_payment,river_expiry FROM commerce_worker;
		REVOKE ALL ON ALL SEQUENCES IN SCHEMA river,river_payment,river_expiry FROM commerce_worker;
		REVOKE ALL ON SCHEMA river,river_payment,river_expiry FROM commerce_worker;
		GRANT USAGE ON SCHEMA river TO commerce_claims_worker,commerce_ads_worker;
		GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA river TO commerce_claims_worker,commerce_ads_worker;
		REVOKE ALL ON river.river_migration FROM commerce_claims_worker,commerce_ads_worker;
		GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA river TO commerce_claims_worker,commerce_ads_worker`); err != nil {
		return err
	}
	// Application-owned River routing depends on upstream tables. Keep its
	// backfill, trigger and checksum atomic, under the same session advisory lock.
	postTx, err := lockConn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = postTx.Rollback(cleanup)
	}()
	// Historical post0001/0002 need the original table while first upgrading.
	// Grant and revoke that compatibility access in this transaction, including
	// repeated Apply: a later failed post phase cannot leak obsolete grants.
	if _, err = postTx.Exec(ctx, `GRANT SELECT, INSERT, UPDATE(kind) ON river.river_job TO commerce_checkout_runtime;
		GRANT USAGE ON SEQUENCE river.river_job_id_seq TO commerce_checkout_runtime;
		GRANT SELECT ON river.river_job TO commerce_checkout_writer,commerce_integration_writer`); err != nil {
		return err
	}
	if err = applyVersions(ctx, postTx, postVersions); err != nil {
		return err
	}
	// Stripe webhook ingress inserts payment signal jobs through River
	// JobInsertFastMany: ON CONFLICT (unique_key) DO UPDATE SET kind=EXCLUDED.kind
	// needs column UPDATE(kind) at executor start (same reason as commerce_runtime
	// above). Column-level only: integration.guard_payment_job_family rejects any
	// kind/args/queue/identity change, and no other column, DELETE or TRUNCATE is
	// granted. Kept here (not in post_river/0012) so it is checksum-safe for
	// databases that already applied 0012 and is re-asserted on every Apply.
	if _, err = postTx.Exec(ctx, `GRANT UPDATE(kind) ON river_payment.river_job TO commerce_stripe_ingress`); err != nil {
		return err
	}
	// Reapply only lifecycle grants after future upstream additions, and only in
	// the final transaction: no Meta privileges leak from a partial cutover.
	if _, err = postTx.Exec(ctx, `GRANT USAGE ON SCHEMA river_meta TO commerce_meta_worker;
		GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA river_meta TO commerce_meta_worker;
		REVOKE ALL ON river_meta.river_migration FROM commerce_meta_worker;
		GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA river_meta TO commerce_meta_worker`); err != nil {
		return err
	}
	// Native family schemas isolate River maintenance; since T21-02 the SQL authority is split too:
	// river_payment only for the two payment authorities (SANDBOX and LIVE), river_expiry only for the
	// expiry worker, so a compromised ads/claims/expiry worker holds no payment queue privilege.
	if _, err = postTx.Exec(ctx, `REVOKE ALL ON river.river_job FROM commerce_checkout_runtime,commerce_checkout_writer;
		REVOKE UPDATE(kind) ON river.river_job FROM commerce_checkout_runtime;
		REVOKE UPDATE(queue) ON river.river_job FROM commerce_checkout_writer;
		REVOKE ALL ON river.river_job_id_seq FROM commerce_checkout_runtime;
		REVOKE ALL ON SCHEMA river FROM commerce_checkout_runtime,commerce_checkout_writer;
		GRANT USAGE ON SCHEMA river_payment TO commerce_payment_worker,commerce_payment_live;
		GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA river_payment TO commerce_payment_worker,commerce_payment_live;
		REVOKE ALL ON river_payment.river_migration FROM commerce_payment_worker,commerce_payment_live;
		GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA river_payment TO commerce_payment_worker,commerce_payment_live;
		GRANT USAGE ON SCHEMA river_expiry TO commerce_expiry_worker;
		GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA river_expiry TO commerce_expiry_worker;
		REVOKE ALL ON river_expiry.river_migration FROM commerce_expiry_worker;
		GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA river_expiry TO commerce_expiry_worker`); err != nil {
		return err
	}
	return postTx.Commit(ctx)
}

// Both phases use the same ledger and reject changed SQL, never overwrite it.
func applyVersions(ctx context.Context, tx pgx.Tx, versions []string) error {
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
	return nil
}
