# Seed-pool lifetime correction evidence

Status: ACCEPTED_LOCAL, SPL01–04. Author/root real-PG, full regression and
independent source/security/PG-evidence reviews pass on source `c7d2703`.

## Cause and correction

The ID-only calls in `TestMetaRuntimeIsolationTwoWayRealMaintenance` ran four
`ewSetup` producers. Each opened five role pools, retaining all until top-level
cleanup even though the worker phase only needed committed job IDs. The prior
same-30-slot run at `ba342c3` failed with native `53300`; its logs and hashes
remain in [the diagnostic record](2026-09-27-worker-start-diagnostics.md).

`ewCloseSeedPools` reuses the existing legacy cleanup pattern, closing only
checkout, ordinary worker, buyer runtime/issuer and identity pools owned by a
finished seed. It rejects owner/shared-role aliases, requires actual nonempty
pool/backend evidence, observes zero backends after close and pings the shared
owner/runtime pools. ID-only legacy siblings use the same helper; the still-used
`queries[0]` payment harness stays live. Production pools/config are unchanged.

The old failing process did not capture backend counts. The statement that
there were 27 potential pool objects was a source inventory, not a measured
27-connection snapshot. The successful corrected run below proves the retained
seed resources have real connections and that early cleanup removes them.
It does not retroactively measure every connection in the old failure.

## Ownership and actual evidence

- Contract `40d0354`, minimum gate decision `ed2eb1c`. Independent review did
  not require a separate artificial capacity-negative for this test-only change;
  retained RED, measured release, original GREEN and existing failure/cancellation
  cleanup tests remain required. No all-capacity-path coverage claim.
- Author `test_worker`, gpt-6-sol/high, reused worktree
  `commerce-meta-inbox-go-20260926`, branch `commerce/seed-pool-lifetime-20260927`,
  base `40d0354`. Only `expiry_runtime_test.go`, `meta_isolation_maintenance_test.go`
  and `legacy_runtime_isolation_test.go` changed. Author `e60b8f6` → root `c7d2703`.
  Compile-only and diff checks exit 0; those checks do not execute PG gates.
- Author `bash scripts/dev/test-local.sh --meta-runtime`: actual exit 0,
  11 top-level PASS / 0 FAIL / 0 SKIP, foundation 57.211s. Original two-way
  maintenance test PASS 10.41s. Log
  `/Volumes/data/output/seed-pool-author-meta-runtime1-20260927.log`, SHA256
  `eb38700c5f859e5acb7fd67db48c2d6bf26976e44fcf6fded9cd03d2dbd0e203`.
- All 20 seed-role observations: native max_connections=30,
  reserved_connections=0, superuser_reserved_connections=3; pool total=idle=1,
  acquired=0, max=8; exact-role backend count 1 → 0. Shared owner/runtime Ping
  assertions and all original native maintenance positives/foreign snapshots
  pass. No capacity, deadline or queue-control change.
- Independent source review (gpt-6-astra/high): no concrete P0/P1/P2. Author
  test-owned container listing was empty after the run; no customer resource
  was removed. Independent PG evidence review also verified the 20 role counts
  and unchanged maintenance/foreign assertions.
- Root `c7d2703` `bash scripts/dev/test-local.sh`: actual exit 0, 568 top-level
  PASS / 0 FAIL / 0 SKIP, full race/vet; foundation 438.466s, Meta 2.489s.
  Log `/Volumes/data/output/local-recovery-root-full2-20260927.log`, SHA256
  `1eed047ef2dad34a5fae3af0064a44fdc4721bef058b3ce4a152559dfa552821`.
  Original TwoWay passes 9.24s with the same 20 role observations. Across the
  corrected Meta and legacy sites, 70 seed-role release checks pass. Full
  startup-failure, restart, legacy isolation and all five LRC tests pass too.
  `docker ps -a --filter label=livecommerce.fixture` returns no containers.
  Later commits during this run changed only docs. Independent security and
  PG evidence reviewers verified the source/hash/counts/cleanup and accepted
  the bounded scope, without remaining concrete P0/P1/P2. Evidence memory
  `80bd492f-8366-473a-90d7-f9ddcc7e213a` and the linked LRC record retain details.

The failure and successful fix are local test-resource evidence, not production
capacity sizing, public-provider qualification or whole-SaaS release acceptance.
