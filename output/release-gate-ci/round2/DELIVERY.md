<!-- Purpose: SHA-bound second-round CI cost measurements and acceptance status. -->
<!-- Depends on: original GitHub Actions receipts, current human CI-only charter and existing gate registry. -->
<!-- Used by: PR78 independent review and integrator; not a replacement gate or timing authority. -->
# Round 2 — cumulative job cost, unchanged coverage

Status: implementation candidate; runtime benefit **NOT_RUN**. Prior round acceptance remains in `../DELIVERY.md`.

- Role: primary CI implementer; runtime model/effort labels unavailable (UNKNOWN). Two independent read-only cost/source auditors; worktree `.worktrees/release-gate-ci`, branch `unit/release-gate-ci`.
- Human authority: 2026-10-17 request to reduce cumulative job minutes, preserve achieved long-tail improvement, no reduced coverage/weakened gates, GitHub-only execution. Coordination task `87135804-7943-49c9-b987-a51011d1d5f7`.
- Existing authority: `gates.yml` → `ci-plan.mjs` → generated `shard-plan.json`; actual test invocation and PG isolation stay in the single `test-local.sh` registry. No alternate test inventory or gate verdict.
- Baseline source `a8814b1f751ae468f4e02e2760e81c16977a946a`, actual PR execution tree `26b25e2bd870362fc0747470423ed82105f54985`, Gates run `38935079717`.
- Same foundation union (`unit + g01…g10`): cumulative job **11051 s / 184.18 min**, longest Gate **1188 s**; 1521 planned Test names plus one Fuzz seed corpus, 1512 PASS / 10 unchanged SKIP / 0 FAIL. The previous round improved longest Gate but increased cumulative job time; it is not a cost win.
- Whole baseline Gates run: 80 jobs, 79 SUCCESS / 1 accepted deploy SKIP, cumulative **36994 s / 616.57 min**. This scope includes browsers and aggregates and is separate from the eleven-mode foundation union.

## Measurement and minimum intervention

Original ten package runtimes sum to 8759.273 s; their Gate steps total 9912 s. The mixed difference is 1152.727 s (19.21 min), including compiler/startup overhead, **not proven all compilation**. Unit Gate is 682 s; all eleven jobs add 457 s outside Gate. Explicit repeated costs: pinned Docker pull 100 s, Go module prefetch 16 s, Node install 52 s. Sources: archived `../final-38935079717/jobs.json` and ten complete package logs. g10 raw Gate-to-first-Test interval is 122.49 s.

Only intervention: enable the **already pinned setup-go action's native module/build cache** in the matrix job with `go.sum` as its dependency hash. The pinned upstream `src/cache-restore.ts` / `src/package-managers.ts` confirms OS/architecture/image/Go/hash partitioning and `GOMODCACHE` + `GOCACHE`, not a database or receipt cache. Go's compiler hashes source and flags; all existing `-race -count=1`, package/test selectors, vet, fixtures, migrations, shard counts, real 900 s clocks and deadlines remain unchanged. Cache misses remain ordinary cold builds. No new action, dependency or producer job.

Independent read-only pre-push review: E1 PASS, no correctness/coverage blockers. Measurement risk: matrix legs share the pinned action's immutable cache key; the first successful writer can seed only a partial compilation cache, and exact hits do not enrich it. Measure the cold run and a same-PR/same-source warm attempt separately; do not assume complete cache coverage or claim savings before those original receipts. Cache restore/save overhead is included in job elapsed time.

New Node tests assert the real workflow/runner contract, then inject disabled cache, wrong manifest, lost race, result-reuse count and lost selector and require the **same assertions** to reject them. Run only in GitHub, registered in existing `test-node.sh`. This is in-test red→green evidence; it does not claim a runtime cache benefit.

## Pending receipts

- Candidate source SHA: pending commit; rollback checkpoint `b583861b85c75a944d555ef6118d3c7034f15b8f`.
- GitHub run IDs / actual execution trees: NOT_RUN, to register immediately after authorized push/dispatch.
- Compare the same eleven-mode union, including cache restore/save overhead and cold/warm state. Goal: job total <184.18 min **and** longest Gate ≤1188 s; do not equate a forecast or queue reduction with compute saved.
- Separately report entire fixed-required matrix job minutes; never substitute a narrower dispatch for that whole-workflow scope.
- Verify the unchanged 1522-result name multiset and ten SKIP names; required static/Node/browser/aggregate gates must stay green at final source.
- CI faults/timeouts retain raw evidence; deadline 150 min from dispatch, no duplicate dispatch while a run is active.
- No local commands used for tests, validators, generation, formatters, servers or containers. No merge, deployment, product change, provider traffic, production secrets or data access. SANDBOX/LIVE/SMTP/full-secret smoke remain NOT_RUN.
