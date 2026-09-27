# T08 LMW01–05: independent media worker process

Status: **PASS_LOCAL_MOCK_PROCESS_ONLY** (2026-09-27).
Root final source/test baseline `320dad6`: focused 8 and full 645 top-level
tests passed, 0 FAIL / 0 SKIP. This is not Cloud, deployment or global G06.

## Source, ownership and independent review

- Frozen interface `6b69a25`: [LMW contract](../../contracts/live-media-runtime-v1.md).
- Source author `media_runtime_source`, commerce_worker role, gpt-6-sol / medium;
  base `6b69a25`, branch `commerce/media-runtime-20260927`, isolated worktree
  `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`. Author `6bbe24c`
  integrates as `fda7082`; source owns only `cmd/media-worker/main.go` and
  `internal/integrations/livekit/worker_env.go`.
- Fixed-source independent static review Humaux
  `66b16a27-340a-4692-bc3d-7e764cacd81d`: no confirmed scoped P0/P1. Actual
  reviewer model/effort was not exposed. Static review is not process evidence.
- Independent test author `media_runtime_tests`, test_worker role,
  gpt-6-sol / high; same frozen base, branch `commerce/media-runtime-tests-20260927`,
  isolated worktree `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`.
  Test-only commits `31297d9`, `a9223e6`, `9f3f02f` integrate as `3a2eec6`,
  `1c1638c`, `320dad6`. Owns command/config unit tests, foundation runtime test
  and the focused runner selector. No source/migration/dependency changes.
- Root independently reviewed test causality and reran focused/full gates.
  Subsequent `f7c57b6` and `0af7e82` were Studio contract documents only; no
  executable source or acceptance runner changed during the root full run.

## What the gates actually prove

| Gate | Observed causal evidence |
| --- | --- |
| LMW01 | Disabled command reads only flag; enabled malformed config and nil context fail before DB; bounded exact JSON, duplicate keys, canonical values and secret redaction |
| LMW02 | Local TLS matching-host positive and separate wrong-CA/wrong-host negatives; fixed loopback dial, hostile proxy zero requests; actual private RoundTripper count exactly one on redirect, not merely one server hit |
| LMW03 | Built command rejects wrong/mixed roles, individually valid pools connected to different physical databases, and readiness drift; partial failure releases connections |
| LMW04 | Actual process consumes durable prepared attempt, one provider Start and persisted observation; native other-queue nonempty row stays byte-identical; authorized durable Stop obtains coherent terminal proof |
| LMW05 | Signal stops process without provider Stop; restart keeps attempt and budget, then one authorized Stop; missing original project/key retains uncertainty without replacement side effects and restored configuration resumes |

The test harness uses `-race`; the separately built process executable is not
race-instrumented. These are local TLS doubles, not real provider requests.

## Retained repairs and commands

Independent failures remain under `/Volumes/data/output/`:
`lmw-independent-20260927-first.log` (compile fixture errors), `second.log`
(synthetic terminal timestamp ordering), `third.log` (foreign Meta job fixture
rejected by its valid SQL trigger). Corrected only tests; no product admission,
TLS validation, signal deadline or authority guard was weakened.

Independent `bash scripts/dev/test-local.sh --live-media-runtime` at `a9223e6`
exited 0; log `lmw-independent-20260927-fourth.log`, SHA256
`9e5c02be9b9e3e30eb5ed00542c161124b21b67b3d18c093a3d45bc7a56af30f`.
Subsequent `9f3f02f` tightened redirect and enabled-nil-context unit assertions;
its unit race/vet passed, but independent PG was not rerun at that SHA. The
following root runs cover the final integrated tree instead of claiming otherwise.

- Root `bash scripts/dev/test-local.sh --live-media-runtime`, source/tests
  `320dad6`: actual exit 0, **8 PASS / 0 FAIL / 0 SKIP**; foundation 26.052s.
  Log `/Volumes/data/output/lmw-root-focused-20260927.log`, SHA256
  `2fa56ca157d078ddc6ba9b1ef8172570d5e30ddd7b1bbfa63676c9f69ca2dc40`.
- Root `bash scripts/dev/test-local.sh`, same source/tests: actual exit 0,
  **645 PASS / 0 FAIL / 0 SKIP**, 33 tested packages; foundation **644.643s**.
  Runner executes serial-package `go test -race -count=1 -timeout=900s ./...`
  followed by `go vet ./...`; both successful. Log
  `/Volumes/data/output/lmw-root-full-20260927.log`, SHA256
  `0faa790afa5828362121e3ff2d2d0e33d2f6c611d4e97d176dbb246725e28fde`.
- Runner-owned PostgreSQL fixture `lc-foundation-test-48689` was absent on
  postflight; Bash/go/foundation PIDs 48689/48709/48901 were gone. Other task
  containers were not removed. No customer service or stream was stopped.

## Maintenance and remaining gates

See [runtime call map and diagnosis](media-worker-runtime.md). Reuses native
River/pgx, LME/LMR and Go TLS without a new dependency, queue or migration.
No authority is minted by startup. Keep old frozen key/project versions until
their attempts are resolved; restart is not a retry or new Start authorization.

Merchant HTTP/BFF and approved Studio UI, room publishing and participant
tokens, LIVE credential/destination provisioning, real Cloud acceptance,
deployment supervision, device/browser testing and post-escalation recovery
remain downstream. T08/T09 and the complete SaaS are not finished.
