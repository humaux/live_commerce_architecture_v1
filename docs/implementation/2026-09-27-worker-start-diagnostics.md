# Bounded worker-start diagnostic evidence

Status: ACCEPTED_LOCAL, WSD01–04. WSD03 captured a native failure; root full2
and independent sign-off pass after the separate seed-pool lifetime fix.

## Source and ownership

Frozen contract `f2c03ac`; author `startup-diagnostic-author`, gpt-6-sol/high,
reused `commerce-meta-inbox-go-20260926`, branch
`commerce/startup-diagnostics-20260927`. Only `internal/jobqueue/run.go` and
`run_test.go` were writable. Author `4a726d9` was integrated as root `ba342c3`.
Independent gpt-6-astra/high source review found no concrete P0/P1/P2.

`cmd/{meta,payment,expiry}-worker` call `jobqueue.Run` → `startWorker` → native
River `Start`. Existing pgx `PgError`, standard errors/context/net and slog
provide the diagnostic; no dependency or framework was added. Only fixed
phase/category values and explicitly allowlisted SQLSTATE cross the log
boundary. Error strings, config and credentials are never rendered. ErrStart,
10-second watchdog, join/disarm, cancellation and worker lifetime remain intact.

## Actual commands and evidence

- Author four-package `go test -race -count=1` and `go vet`: exit 0.
  Race log `/Volumes/data/output/worker-start-author-race-final-20260927.log`,
  SHA256 `279ac0ffc0e959946b6cbf045d4f0f3e551a52c99f60e391db04e744e8ded6a3`.
- Root `ba342c3`, same four packages: all PASS (jobqueue 2.087s, meta-worker
  1.670s, payment-worker 1.667s, expiry-worker 1.672s). Root race log
  `/Volumes/data/output/worker-start-root-unit-20260927.log`, SHA256
  `bcdfd1d226e2c7e9904ea660ee34f6d7bbc82a09c22e164c1aa1ca7bc5bbea1d`.
  Vet completed with exit 0 and empty
  `/Volumes/data/output/worker-start-root-vet-20260927.log`, SHA256
  `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- One approved root `bash scripts/dev/test-local.sh --meta-runtime`: exit 1,
  10 top-level PASS / 1 FAIL / 0 SKIP, foundation 49.645s. Log
  `/Volumes/data/output/worker-start-root-meta-runtime1-20260927.log`, SHA256
  `5bab653b57ff5f55bdea9ecf0772b8b6205588782e9082f702f564f2dfee48ee`.
  `TestMetaRuntimeIsolationTwoWayRealMaintenance` failed at worker readiness
  (4.20s), preserving
  `/Volumes/data/output/meta-runtime-process-meta-maintenance-26417076f1c5.log`,
  SHA256 `f3bea1a95480d7f2acfea36bb076efd7f529c7f7b818c903262d9b96d9001862`.

The new witness is `phase=river_start_returned category=sqlstate sqlstate=53300`.
[PostgreSQL 18](https://www.postgresql.org/docs/18/errcodes-appendix.html) defines
53300 as `too_many_connections`. This identifies this run's native failure,
not the origin of every occupied connection or the cause of earlier generic
failures. No unchanged retry or deadline/capacity increase is authorized by it.
The next investigation compares seed-pool lifetime with an existing promptly
closed-pool pattern in `legacy_runtime_isolation_test.go`.

That comparison led to the [test-only lifetime correction](2026-09-27-seed-pool-lifetime.md)
at `e60b8f6` / root `c7d2703`: author same-capacity original scenario and
11-test subset pass with actual five-pool release observations. Root full2 on
`c7d2703` also passes 568 tests, race and vet, actual exit 0. Log
`/Volumes/data/output/local-recovery-root-full2-20260927.log`, SHA256
`1eed047ef2dad34a5fae3af0064a44fdc4721bef058b3ce4a152559dfa552821`.
Final independent source/security and PG evidence reviews accepted this bounded
scope; see the linked recovery record. The safe diagnostic itself is not the
cause fix.

## Limits

The original LRC full1 failure and this negative run are retained. This change
does not itself fix the capacity failure or grant production acceptance. Fixture
listing was empty after this run; no customer resource was cleaned up.
Buyer apps, API/BFF, catalog, checkout/payment source, migrations and browser
tests remain unchanged versus main `519fb16`; the changed helper is called only
by the three worker commands. Earlier order/payment browser evidence is reused,
not called a new browser run. The full worker/process regression has now passed
on `c7d2703`, with independent final source and evidence acceptance.
