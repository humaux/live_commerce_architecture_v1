# Unit worker-authority-split — T21-02/T21-03: one DB authority per worker (P1)

Role: commerce_worker (mid tier); reviewer top tier. Base `23185be`. Worktree
`.worktrees/worker-authority-split`, branch `unit/worker-authority-split`. No delegation, no new
dependency. Migration number **0084** (integrator-assigned).
Source: `output/r3-t21-review/FINDINGS.md` T21-02 and T21-03 (read both fully). `AGENTS.md` 必守.

## Problem
`lc_expiry_worker`, `lc_payment_sandbox`, `lc_payment_live`, `lc_claims_worker`, `lc_ads_worker` all
inherit `commerce_worker` (deploy/postgres/logins.tsv), so a compromised ads/claims/expiry worker can
claim a payment operation and record a fake Stripe "paid" → order confirmed with no money.

## Decisions (binding)
- WA1 New NOLOGIN authorities in 0084: `commerce_payment_worker` (sandbox), `commerce_payment_live`
  (live), `commerce_expiry_worker`, `commerce_ads_worker`, `commerce_claims_worker`. Inventory every
  privilege `commerce_worker` holds today (query the catalog in a test DB: `information_schema` table/
  routine/usage privileges + `pg_default_acl` + RLS policies `TO commerce_worker`), then GRANT to each
  new authority exactly what its process uses (derive from the Go code paths of each cmd/*-worker:
  which SQL functions/tables it calls), and finally REVOKE from `commerce_worker`, leaving it with
  nothing that any worker no longer needs (keep the role if migrations/tests reference it; it may
  stay as an empty legacy role — say so in a comment). Write the inventory to
  `output/worker-authority-split/privileges-before.tsv` and `-after.tsv`.
- WA2 Policies: every RLS policy `TO commerce_worker` is recreated for the right new authority(ies).
  River queues: river_payment DML only for the two payment authorities; river_expiry only expiry; ads
  queue only ads. `integration.claim_operation` additionally refuses an operation whose provider/
  actor kind does not belong to the calling authority (derive from `current_user` membership with
  `pg_has_role`, never from an argument).
- WA3 T21-03: Stripe/PAYUNi definers derive the allowed execution profile from the caller's authority
  (`commerce_payment_live` ⇒ LIVE only, `commerce_payment_worker` ⇒ SANDBOX/MOCK only), not from a
  caller-supplied value. Separate LIVE keyring key id mounted only by payment-worker-live: only if it
  fits in this unit without changing the Stripe live contract; otherwise record as follow-up in
  DEVIATIONS.md.
- WA4 `deploy/postgres/logins.tsv` maps each login to its new authority; provision-logins needs no new
  secret (logins already exist). `internal/platform.ValidateWorkerPool` (and any per-process startup
  check) verifies the specific authority for that process. Smoke S13's membership matrix updated.
- WA5 Existing ACL gates (`TestT06WorkerAuthorityAndFunctionACL`, `TestPoolAuthority`, stripe authority
  tests, external_operation_test.go:112) are updated to the new expected shape — tightened, never
  loosened; add the failing-today test from FINDINGS (non-payment logins have no river_payment /
  record_stripe_observation privilege; cross-authority claim_operation refused).

## Write paths
`migrations/0084_worker_authorities.sql` (+ `migrations/migrate.go` only if grants live there),
`deploy/postgres/logins.tsv`, `deploy/scripts/smoke.sh` (S13 only), `internal/platform/**`,
`cmd/*-worker/**` (startup authority check only), `tests/foundation/**` (ACL tests), docs touching role
names (`docs/engineering/dependencies.md`, contracts/external-operation-v1.md role table if present).

## Done when
Static set exit 0; `bash scripts/dev/test-focused.sh '<ACL+worker regex>'` and then the full payment,
expiry, ads, claims regexes green (lesson: after a grant migration run all ACL-class gates, not just the
new one). Evidence → `/Volumes/data/live_commerce_architecture_v1/output/worker-authority-split/`.
