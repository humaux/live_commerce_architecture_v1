# T07 private Meta runtime — acceptance record

Status: **ACCEPTED_LOCAL_PRIVATE_RUNTIME — MIso01–05 / MR**. Not a public deployment or a
customer/Meta configuration change. Contract:
[Meta runtime v1](../../contracts/meta-runtime-v1.md); maintenance:
[runtime call map](meta-runtime.md).

## Engineering ownership

- Integrator baseline `5523826`; contract freeze `19ac324`, clarified cleanup
  budget `9337caa`; shared platform/SQL implementation `00f3814`.
- `integration_worker`, actual `gpt-6-sol/high`, base `19ac324`, reused clean
  `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926` on branch
  `commerce/meta-runtime-go-20260926`. Allowed runtime/environment, API Meta
  wiring, new Meta CLI and tests only; source `b8d2a06` integrated as `252c11a`.
  See [author record](2026-09-26-meta-runtime-go-author.md).
- Independent `test_worker`, actual `gpt-6-sol/high`, same frozen base, isolated
  `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926` on branch
  `commerce/meta-runtime-tests-20260926`. Allowed new foundation runtime tests
  and only the migration-count adjustment in the existing consumer upgrade.
  Initial tests `beeb9da` integrated as `5e51f90`; directed diagnostic `d576f5a`
  integrated as `afe7c6c`. See [test author record](2026-09-26-meta-runtime-pg-author.md).
- Independent `security_reviewer`, actual `gpt-6-astra/high`, read-only design,
  shared source and Go candidate review. Two design P1 and one startup P2 were
  closed before implementation. Subsequent real-process execution exposed the
  P1 cross-queue maintenance defect below; earlier source review was bounded
  and does not override this failed acceptance.

No recursive delegation, overlapping writer paths, new dependency or broker.
Each worker has its own task/lock and durable memory/canvas evidence.

## Root-observed checks (bounded claims)

| Check | Actual result | Evidence |
| --- | --- | --- |
| Fresh 0030 migration + existing MI tests | exit 0, 20 top-level PASS, foundation 6.006s | `/Volumes/data/output/meta-runtime-migration-smoke-20260926.log` |
| Platform/Meta/API/Meta CLI race packages on `252c11a` | exit 0; 2.736s / 2.809s / 3.310s / 2.590s | `/Volumes/data/output/meta-runtime-root-unit-20260926.log` |
| Existing browser identity/settings and actual API account restart | exit 0; 3 top-level PASS, foundation 13.492s | `/Volumes/data/output/meta-runtime-root-browser-identity-20260926.log` |
| Root MR focused on `5e51f90` | **exit 1; 5 PASS / 1 FAIL**, foundation 18.407s | `/Volumes/data/output/meta-runtime-root-focused-20260926.log` |

The browser gate uses real Next/Go/PG with a signed **MOCK IdP**, not a live
identity provider. Root inspected desktop/mobile account screenshots under
`output/playwright/settings-real-20260926T124322.264479000`; layout and controls
remain legible. Existing tests also assert three-language status boundaries.
This matters because the API now shares one ten-second assembly deadline across
the main, identity, buyer and Meta dependencies; running handlers must not retain
that cancelled startup context.

SHA-256 (logs above, in table order):

```
f13b75e09a63fff703489886aed3459b40219646ea29dc7ace11c0e0daa9040d
66ba9de349f55ec87afbb58f972b62c1f4e13ef487f6a9ae63d1b2b733bc1cd6
62f5729536aef748f629497d4c33f3bf65ca039c038c46d6e5a8883727bc072a
```

## Review-driven corrections

1. Queue readiness alone missed disabled/replaced social commit guards. The
   frozen design and 0030 audit now check all four guards' exact metadata.
2. Valid role names, database names and matching restored event IDs cannot prove
   shared storage. A random database-local transaction-lock probe rejects split
   pools without adding permanent records or privileged reads.
3. `jobqueue.Run` bounds Start, not preceding constructors. Construction now has
   shared deadlines, with independent rollback cleanup contexts.
4. Root caught a candidate router validation loop storing nil map values and
   testing value non-nil for duplicate detection. The author uses key existence;
   direct constructor tests cover two verifiers for the same path, not merely
   environment-level duplicate rejection.
5. Initial poison tests used an unlinked bad event ID in every case, masking
   distinct kind/queue/args/key faults. Independent tests now mutate one field
   of an actually admitted linked job and restore its READY baseline each time;
   absent linkage is a separate case. No product checks were weakened.

## P1: schema-wide maintenance violates MR04

`TestMetaRuntimeRealAPIBinariesPageInstagramRestart` observed an unrelated
payment/expiry/default row change while the actual Meta worker ran. The test
author's directed diagnostic leaves a valid `payment_mock_v1` job scheduled,
sets it due **before** its baseline snapshot, and observes the actual row
transition over a bounded maintenance window. Result: job 7,
`changed_fields=[state]`, `scheduled` → `available`, attempt 0 → 0.

Independent diagnostic source `d576f5a`: actual focused exit 1, 5 PASS / 1 FAIL,
foundation 19.424s. Evidence:
`/Volumes/data/output/meta-runtime-pg-crossqueue-diagnostic-20260926.log`, SHA-256
`21f5433edeaabcd62a07c9f323d279711fa705717532b63f025224fcc1575d2a`.
The same diagnostic adds a passing naturally-expired probe cleanup test.

Root independently reran the integrated diagnostic at `8ed76d6`: actual exit 1,
5 PASS / 1 FAIL, foundation 20.407s; **the same state-only delta** and no attempt.
`/Volumes/data/output/meta-runtime-root-deterministic-failure-20260926.log`, SHA-256
`ae7a3099cf55e7fdda36b2553ffdc6746f1b353b6cc87df23da9b4d892f78e42`.

Root cause: pinned River v0.40.0 `client.go:951–1063` wires leader maintenance
by schema, not by `Queues`; `JobSchedule` scans every due scheduled/retryable
job in that schema. Source inspection additionally found the global rescuer's
unknown-kind discard path. The latter is an additional risk, not the measured
state delta above. Official corroboration:
[maintainer explanation](https://github.com/riverqueue/river/discussions/343),
[native schema configuration](https://riverqueue.com/docs/alternate-schema).

Rejected: changing the fixture to available/far-future, raising schedule
intervals, disabling maintenance only in tests, or repeated timing-dependent
green runs. Those hide the root cause. Preserve the deterministic failing test
until a real boundary passes it. The proposed native same-PG schema revision
is in [its own contract](../../contracts/meta-runtime-isolation-v1.md).

Independent preflight of that revision approved `84cc26f` after two bounded
rounds, P0/P1/P2 zero. The first round required a preparation-phase non-ready
fence, precise partial-Apply rollback semantics and a strict new-schema guard;
the final revision also preserves historical upgrade fixture boundaries.
Contract frozen `647e517`; implementation commits `b8271fa` (pool/role),
`cecec81` (fixed clients), `aca3148` (forward schema/cutover), `856a4b1`
(effective ACL startup check). **MIso01–05 are in progress, not accepted.**
This does not erase the historical failed runtime verdict or close the separate
legacy-worker maintenance P1.

The independent reviewer found direct/inherited custom non-owner object grants
were not rejected by role-name/owner checks. The PG author reproduced both cases
on the prepatch candidate, with a clean Meta-worker positive control:
`/Volumes/data/output/meta-isolation-acl-prepatch-20260926.log`, lines 1–8,
exit 1, SHA-256 `3e438a39283e0cae282e9b002d0e9dc1404c2990a4c8ae6a308c113d3f7a98d7`.
`856a4b1` adds the shared effective table/column/sequence/CREATE checks.
At that checkpoint source-only closure had no new P0/P1/P2, while expanded PG
negatives were still pending; their final evidence is below. Native privilege semantics follow the
[PostgreSQL 18 ACL functions](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-ACCESS-TABLE).

Root four-package race on that patch exited 0 (platform 1.589s, Meta 1.819s,
CLI 1.419s, API 1.446s):
`/Volumes/data/output/meta-isolation-root-acl-unit-20260926.log`, SHA-256
`788300f1718ef1c98071a582bca677193771bae5ebcc5eb2b7ddcb285cdb4d56`.
Root `--checkout` on `aca3148` hit its aggregate 120s test-binary limit after
24 top-level passes and no assertion failures. This is **not a pass**; its log is
`/Volumes/data/output/meta-isolation-root-migration-checkout-20260926.log`.
The new-schema test migrations and complete regression are still required.

Follow-up root checks on `856a4b1` (plus documentation only):

| Check | Actual result | Evidence |
| --- | --- | --- |
| Checkout with aggregate 240s budget, unchanged per-operation deadlines/assertions | exit 0, 27 top-level PASS, 76.016s | `/Volumes/data/output/meta-isolation-root-checkout-240s-20260926.log` |
| Browser identity/settings + actual API account restart | exit 0, 3 top-level PASS, 15.735s | `/Volumes/data/output/meta-isolation-root-browser-identity-20260926.log` |

SHA-256 respectively: `32dff3c579199ed528efb76decfaf80b5c4bc08dafd22b79109f9690f8c97584`,
`9ecd275aaeebdff4df9695368a00ee1a4935220460414451c941a644c15b0074`.
Root inspected the desktop/mobile account images under
`output/playwright/settings-real-20260926T133851.714959000`; labels, controls and
saved-versus-qualified state remain legible. This is real browser/Next/Go/PG
with a signed **MOCK IdP**, not provider approval.

The author's next PG run passed nine ACL negatives but exposed a separate
readiness error: `42501 permission denied for schema river`. The guard metadata
function's SECURITY DEFINER owner had correctly lost old-schema USAGE, while
its `to_regclass` lookup still required it. `d702bb1` resolves the exact legacy
relation through `pg_trigger`/`pg_class`/`pg_namespace` instead, preserving every
guard check and withholding old-schema privileges. Source-only review closed
this cause without new P0/P1/P2; post-fix MR/MIso evidence was then pending and
is recorded in the final root gates below.

## Full-regression failures retained

Root `5259b5f` ran 529 top-level passes without an assertion failure, then hit
the 360s aggregate foundation deadline (360.497s). This is not a pass; vet did
not run. `43a83d0` raises only that additive suite budget to 600s; individual
SQL/process deadlines and production maintenance defaults are unchanged.
Evidence: `/Volumes/data/output/meta-isolation-root-full-checkpoint-20260926.log`,
SHA-256 `98e092d88ffd2f419ec3d576ccb6238e6636f66ebdfb0e8e3a8a1475c29c9c22`.

The subsequent frozen `e10c4bb` full run actually exited 1 with **551 top-level
PASS / 1 FAIL / 0 SKIP**, foundation 412.459s. This time it was a real failure,
not an aggregate timeout. `TestMetaRuntimeIsolationTwoWayRealMaintenance`
failed before `expiry_worker_ready`; the actual CLI logged
`expiry_worker_queue_unready`. Vet was again not executed.
Evidence: `/Volumes/data/output/meta-isolation-root-full-final-20260926.log`,
SHA-256 `91e15d8146886c29fdd371f251ba7c719d37eb98fd68ebd7a3aa77fa16d5c78a`;
preserved CLI log `/Volumes/data/output/meta-runtime-process-old-maintenance-060674974839.log`,
SHA-256 `852130b8845b96818d1e9f78b16615b2affc88385f7dcf5408f3c7580602f3f1`.

Source diagnosis: this new process test incorrectly uses the shared
`miSetup`/`fixture` database. Earlier `TestBuyerCheckoutActualRiverExpiry/early`
intentionally relocates its linked expiry job to a private unit-test queue
and leaves it scheduled. The real expiry CLI correctly rejects that active
wrong-queue row. The focused Meta run did not include the earlier test.
Use the existing `mrFixture`/`mrSetup` fresh-cluster helper for this process
gate; do not weaken expiry readiness, delete shared test rows, or remove the
maintenance-eligible positive controls. Directed before/after proof and a new
complete root regression are required before acceptance.

That exact two-test causal selector now has retained before/after evidence:
`TestBuyerCheckoutActualRiverExpiry/early` passes on both runs; the Meta
maintenance case fails before and passes after `8c457e7` (root `593291e`).
The fix only changes `miSetup(t)` to `mrSetup(t, mrFixture(t))` and adds two
intent-comment lines. No product code or readiness/maintenance assertion was
changed. Postfix result: 2 top-level PASS, 11.390s, exit 0.

- Before: `/Volumes/data/output/meta-isolation-early-only-prepatch-20260926.log`,
  SHA-256 `170ad63a67f90e7b0cfccf7f3ff88b6c9ffec1aa29ecfe367ebef1084fd78276`.
- After: `/Volumes/data/output/meta-isolation-early-only-postfix-20260926.log`,
  SHA-256 `8051287315552ebe51b5226ab9f9fcde8adf90f81c36c49034865118d1680a87`.

Root `593291e` includes this fix and the nine-line causal SQLSTATE addendum
(`ab63df4` integrated as `a2eada9`); all product sources remain identical to
`d702bb1`. Its complete PG/race/vet run has finished at
`/Volumes/data/output/meta-isolation-root-full-isolated-final-20260926.log`.

## Final root gates — source and tests frozen at 593291e

Documentation-only `37ef4be` was committed while the run was active; the diff
from `593291e` for `internal`, `cmd`, `migrations`, `tests` and `scripts` is empty.
This is one exact-source full run including the fixture fix and SQLSTATE
assertions, not a mixture of earlier full and later partial runs.

| Check | Actual result | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-local.sh` | exit 0; **552 top-level PASS, 0 FAIL, 0 SKIP**; foundation 385.304s; all-package race and subsequent `go vet ./...` complete | `/Volumes/data/output/meta-isolation-root-full-isolated-final-20260926.log` |
| Same-source `--browser-identity` | exit 0; **3 top-level PASS, 0 FAIL, 0 SKIP**; foundation 12.305s | `/Volumes/data/output/meta-isolation-root-browser-current-20260926.log` |

SHA-256 respectively:
`a9c37c9b2b3df110a109edb833e0bef9c7bfbaad1f5a34ce89871cfc3ff970ab`,
`d9393871faf0bd80fc5139b7e5d301924675249c437f4139b8d4c5bfcb5f6485`.
Root inspected desktop/mobile `a-*-account.png` under
`output/playwright/settings-real-20260926T142343.846531000`; fields, status and
controls remain legible, with saved credentials explicitly not represented as
provider qualification. The browser uses real Next/Go/PG and a signed **MOCK IdP**.
Both runs cleaned their labelled PG fixtures; a post-run listing is empty.
There were no customer, provider or production writes.

| Contract gate | Root evidence within that full run |
| --- | --- |
| MIso01 authority | Dedicated, mixed/owner/system role matrix; eleven effective ACL cases; borrowed-pool validation; direct SQL cross-lane rejection |
| MIso02 populated upgrade | Real old ingress Page/IG plus duplicate/quarantine and pruned receipts; all persisted job fields, paused queue, private/business snapshots and sequence high-water; Apply twice |
| MIso03 failure/resume | Running, poison, nonempty destination and lock contention; exact causal SQLSTATE; no post-phase partial changes; preparation remains fenced; same-fixture retry |
| MIso04 maintenance isolation | Real Meta and expiry CLIs; due scheduled/retryable, stale running and retention-eligible terminal positive controls; converse controls re-armed before full-row snapshot; case passes in 12.13s |
| MIso05 regressions | All MI/MC/MR, actual API Page/IG restart/key restoration, cleanup and historical upgrades; complete race/vet plus same-source browser gates |

Independent `security_reviewer` read back exact `593291e` sources, both final
logs/hashes, fixture cleanup and the causal fixture correction. Final verdict:
**MIso01–05 and MR accepted locally; no open P0/P1/P2 in this bounded increment**.
Humaux evidence title: `T07 593291e MIso01-05 and MR bounded LOCAL accepted after
full552 and browser3`. The original MR04 failure remains historical evidence of
the pre-isolation defect; it is not erased by the corrected candidate. No UI
was redesigned this increment.

## Remaining boundary

Public callback deployment, OAuth route-proof issuance, secret rotation,
outbound policy, social UI and complete SaaS acceptance remain separate work.
The legacy payment/expiry/external clients' shared-`river` maintenance P1 also
remains open; this Meta boundary does not certify them as mutually isolated.
