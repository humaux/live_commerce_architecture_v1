# T07 private Meta runtime — acceptance record

Status: **CANDIDATE / MR01–05 IN_PROGRESS**. Not a public deployment or a
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
  Final test source and actual gates pending.
- Independent `security_reviewer`, actual `gpt-6-astra/high`, read-only design,
  shared source and Go candidate review. Two design P1 and one startup P2 were
  closed before implementation. Current reviewed candidate has no confirmed
  open P0/P1/P2; final evidence review is still required.

No recursive delegation, overlapping writer paths, new dependency or broker.
Each worker has its own task/lock and durable memory/canvas evidence.

## Root-observed checks (bounded claims)

| Check | Actual result | Evidence |
| --- | --- | --- |
| Fresh 0030 migration + existing MI tests | exit 0, 20 top-level PASS, foundation 6.006s | `/Volumes/data/output/meta-runtime-migration-smoke-20260926.log` |
| Platform/Meta/API/Meta CLI race packages on `252c11a` | exit 0; 2.736s / 2.809s / 3.310s / 2.590s | `/Volumes/data/output/meta-runtime-root-unit-20260926.log` |
| Existing browser identity/settings and actual API account restart | exit 0; 3 top-level PASS, foundation 13.492s | `/Volumes/data/output/meta-runtime-root-browser-identity-20260926.log` |

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

## Outstanding acceptance

MR02–05 constructors, real API/worker processes, retention/restart, all old
MI/MC tests, final complete PG/race/vet and independent final review remain
pending in this record. Do not present preliminary SQL checks or browser
regression as complete runtime acceptance. No UI was redesigned this increment.

Public callback deployment, OAuth route-proof issuance, secret rotation,
outbound policy, social UI and complete SaaS acceptance remain separate work.
