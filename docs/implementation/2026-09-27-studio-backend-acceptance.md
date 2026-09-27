# Studio backend: independent and root evidence

Status: **PASS_LOCAL_MOCK_BACKEND_ONLY** (2026-09-27).
Main source/tests `2be9cd2`: focused 8 and full 652 passed. No Studio UI, Cloud,
LIVE destination or production acceptance is implied. Later BFF transport evidence
is [separate](2026-09-27-studio-bff-acceptance.md). STU04 page workflow remains
NOT_RUN; T08/T09 and the SaaS are incomplete.

## Ownership and fixed source

- Frozen interface: `887f79e`, [Studio contract](../../contracts/studio-v1.md).
- Source `media_runtime_source`, commerce_worker, actual gpt-6-sol / medium;
  base `887f79e`, branch `commerce/studio-source-20260927`, worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`. Seven scoped product
  files only: migration 0038, domain and HTTP Studio, pagination, API composition.
  `47f3926`, `5fba16e`, `ab5c3eb` integrate as `a42618c`, `d0b2a17`, `f02e55f`.
- Independent tests `media_runtime_tests`, test_worker, actual gpt-6-sol / high;
  same base, branch `commerce/studio-tests-20260927`, worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`. Six test/runner
  files; `ca80962` integrates as `2be9cd2`. No product or dependency edits.
- Independent reviewer `livekit_protocol_impl`, security reviewer; actual model/
  effort not exposed. Second bounded repair review found no remaining confirmed
  scoped P0/P1 (Humaux `f066cbf3-bc6f-4fab-a340-c9629c4aceb3`). Static review alone
  is not execution evidence. Root independently reran the integrated tests.

## Retained failures and causal repairs

First PG execution proved a valid read was forbidden: `GetStudio` had omitted
the transaction-local authorization revision required by its SECURITY DEFINER
projection. The first source repair reused the existing planner's revision GUC.

Independent review and PG tests then proved the initial coherence guard was too
narrow: `GetDraft` could read DRAFT, a concurrent Start could commit READY and an
attempt, and the private read could return that new attempt with the old draft.
The guard only checked non-null prepared candidates. A second causal test covered
an edit while no prepared candidate existed. Repair `ab5c3eb` rereads the whole
draft after the projection and rejects a mismatch, without locks or new state.
Both tests failed before the repair and passed after it. Two bounded source repair
batches were used; no test threshold or authorization constraint was weakened.

Failure logs retained in `/Volumes/data/output/`: `studio-backend-first-20260927.log`,
`studio-backend-interleave-first-20260927.log`, `studio-backend-two-races-first-20260927.log`.
Intermediate fixture failures (empty GET body and FK cleanup order) were test
repairs, not reasons to weaken production constraints. A suspected cache leak on
unsupported methods was corrected in review: global middleware already sent
`no-store`; the repair adds the exact contract's `private` directive.

## Actual gates

| Gate | Observed boundary |
| --- | --- |
| STU01 | Actual PG draft create/edit/list/get, replay/conflict, keyset and store/permission separation |
| STU02 | Raw SQL exact safe projection, runtime-only ACL, no raw private SELECT, foreign token/revocation/expiry/association negatives; strict HTTP fields/query; two causal concurrent read tests |
| STU03 | Built API default-off/enabled, exact prepared authority → one Start/replay → UNKNOWN/observation → API/worker restart → authorized Stop → persisted TERMINAL; no duplicate Start or budget reset |
| STU04 | NOT_RUN: approved UI and real Next browser workflow still required |
| STU05 | Static review, focused and full PG/race/vet pass; BFF build/browser recorded separately; full UI/deploy acceptance not implied |

- Independent `bash scripts/dev/test-local.sh --studio-backend`: exit 0, eight
  named tests, log `/Volumes/data/output/studio-backend-raw-sql-first-20260927.log`,
  SHA256 `e775e590395fb6a4559e6f980c6e1434389e0565daffacddc672fa02867b65d5`.
  Focused vet and diff checks also exit 0.
- Root same command at `2be9cd2`: actual exit 0, **8 PASS / 0 FAIL / 0 SKIP**;
  foundation 12.539s. Log `/Volumes/data/output/studio-root-focused-20260927.log`,
  SHA256 `b42369dd3bca2bb6584939be970bdea6fd07fefa835894d038786bbfcd3a3159`.
- Root `bash scripts/dev/test-local.sh`: actual exit 0 on unchanged executable
  source/tests `2be9cd2`, **652 PASS / 0 FAIL / 0 SKIP**, 33 packages; foundation
  658.512s, followed by successful `go vet ./...`. Log
  `/Volumes/data/output/studio-root-full-20260927.log`, SHA256
  `e93da509e0ae7b4996121469cf624592a9c78a487efa4d4110354df50859d1bc`.
  During this run only documentation commit `2c7769d` was added; no executable
  source, tests or runner changed. The later BFF integration does not alter Go
  product code; it has separate focused browser/TypeScript evidence.

These are local PG18/OIDC/TLS fixtures, not customer data or provider requests.
The race-enabled harness builds separate API/worker executables without `-race`.
Tagged STU03 runs in the focused selector, not the untagged full suite. See
[assembly and diagnosis](studio-api-runtime.md). After actual exit the root
full-run container `lc-foundation-test-54670` and shell PID 54670 were absent.
The process test also asserts named API/worker pool cleanup. Other tasks' resources
were not removed. Invalid flag/identity/listener cases are unit-level evidence;
actual API process default-off and enabled positive admission are covered. Do not
upgrade that statement to an exhaustive process-fault matrix.
