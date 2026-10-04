# meta-multi-page backend follow-up — summary

Branch `unit/meta-multi-page-tests`. Backend-only follow-up to the reviewer's findings on unit
meta-multi-page (backend 452556d + reviewer tests 152a11b). Each item is pinned by a red-first test that
fails against the old code and passes against the fix.

## What changed

1. **Per-Page disconnect takes the same per-store advisory lock as connect** (P2-1).
   `meta_connect_disconnect` in `migrations/0108_meta_multi_page.sql` now takes
   `pg_advisory_xact_lock(hashtextextended(jsonb_build_array('meta-connect-store', out_tenant, p_store)::text, 0))`
   (the same key `meta_connect_finish` takes) before the `SELECT … FOR UPDATE` on the connection row, so a
   disconnect racing a reconnect of the same Page serializes with it and cannot unsubscribe a newly-live Page.
   Gates: `TestMetaConnectIndDisconnectLock` (held-lock + `pg_stat_activity` observation, no sleeps) and
   `TestMetaConnectIndDisconnectReconnectRace` (barrier-released disconnect vs reconnect, 6 rounds, asserts the
   Page is never `connected` with an open `PENDING/LEASED` unsubscribe job).

2. **gofmt drift in `internal/metaconnect/errors.go`** — reformatted.

3. **`tests/foundation/meta_connect_gate_test.go`** — the removed `already_connected` code is no longer
   tolerated; the gate asserts only the current codes.

4. **B1 — retiring a source whose route was disabled by disconnect** (`binding_missing` fix).
   `live.put_claim_source` (released in 0064) is re-created in-place in the unreleased
   `migrations/0108_meta_multi_page.sql` with only the route lookup branched: an *active* source still requires
   an enabled route, but a *deactivating* source (`p_active = false`) resolves its binding without the `enabled`
   filter, so a Studio scene can be rebound from a disconnected Page's post to a live Page's post. Tenant / store /
   version / ownership checks are unchanged, and a NEW active source still requires an enabled route.
   Gate: `TestMetaConnectIndRetireDisabledSource`.

5. **B2 — Pick and Disconnect pass `Idempotency-Key` into `internal/command.Run`** (Start already did).
   Same key + same body replays the first result (no second Meta call / job / audit); same key + different body
   is refused `409`. The request body includes `principal_id`, so another principal reusing the key conflicts.
   Gates: `TestMetaConnectIndPickDisconnectIdempotent` (`pick replays by key and reaches Meta once`,
   `disconnect replays by key and enqueues one job`).

## Files

- `migrations/0108_meta_multi_page.sql` — disconnect store lock; `CREATE OR REPLACE FUNCTION live.put_claim_source`.
- `internal/metaconnect/service.go` — `Pick`/`Disconnect` signatures and `command.Run` idempotency (probe-replay pattern).
- `internal/httpapi/meta_connect.go` — pass `Idempotency-Key` through the pick/disconnect handlers.
- `internal/metaconnect/errors.go` — gofmt only.
- `tests/foundation/meta_connect_gate_test.go` — assert current codes only.
- `tests/foundation/meta_multi_page_fix_test.go` — new independent gates MPG10–MPG12.

## Verification

- `go build ./...` — ok
- `go vet ./...` — ok
- `gofmt -l` — clean
- `bash scripts/dev/check-gates.sh` — ok (55 modes, all documented)
- `bash scripts/dev/test-focused.sh '^(TestMetaConnect|TestMetaClaims|TestMetaIntake|TestT06WorkerAuthorityAndFunctionACL)'`
  — `top-level PASS=62 FAIL=0 SKIP=1 exit=0` (see `test-focused.log`)

Logs: `test-focused.log`, `static-checks.log`.
