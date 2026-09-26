# T07 private Meta runtime — acceptance record

Status: **FAILED_MR04 / NOT_ACCEPTED / REVISION_REQUIRED**. Not a public deployment or a
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
**Contract approved, implementation absent, MIso01–05 NOT_RUN.** This does not
change the failed runtime verdict or the separate legacy-worker maintenance P1.

## Outstanding acceptance

MR04 remains failed. The early failure means later assertions within that
process test, including key-restoration/restart, are not accepted by the root
run even if an earlier author's timing-dependent run reached them. All old
MI/MC tests, final complete PG/race/vet and independent final review must be
rerun on the corrected candidate. No new full-regression pass is claimed.
No UI was redesigned this increment.

Public callback deployment, OAuth route-proof issuance, secret rotation,
outbound policy, social UI and complete SaaS acceptance remain separate work.
