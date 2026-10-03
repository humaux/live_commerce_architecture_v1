# SUPERSEDED — first Taiwan-only ruling; not current acceptance

Scope: only `/Volumes/data/live_commerce_architecture_v1/.worktrees/ads-tw-regulation`, branch `unit/ads-tw-regulation`, base `b0f835c307eb1776dfb846f2581638991248f316`. No push, merge, deployment, credentials, production account, or live advertising operation.

## Changes

| Item | Status | Evidence |
|---|---|---|
| TW-only regulated category, including mixed country lists and both templates; no regulation identities | PASS / MOCK | `taiwan_test.go`, adapter RED → GREEN |
| Exact 100/3858495 → `FAILED_FINAL/tw_advertiser_unverified`; other errors unchanged | PASS / MOCK | create and read tests; activate/pause remain UNKNOWN |
| Failed draft exposes reason | PASS / REAL_PG + MOCK | `TestMetaAdsTaiwanFailedDraftReason`; authenticated GET, event readback, no downstream creation, campaign remains PAUSED |
| Three-language guidance, help link, explicit new-attempt retry, setup checklist | IMPLEMENTED; browser pending | Current-attempt/latest-failure projection, no new writer or retry policy |
| Sandbox TW assertion | WRITTEN / NOT_RUN | Owner-only MA-S1; TW expects exact verification refusal, HK retains chain control |
| Contract | PASS / DESIGN | F23/F24; U1/U2 evidence bounds and U11 merchant prerequisite; reason API and classification documented |

## API audit and safety

The API already projects `ops[].code` from `integration.operations.result_code`; no SQL/API migration is needed. Normal completion also writes that value to `operation_events.reason_code`. When a binding changes concurrently, the event annotation can instead be `completed_binding_changed`, whereas the operation code still preserves Meta's result. The UI must use the result, not replace it with that annotation. It filters `attempt == publish_attempt`, `state == FAILED_FINAL`, then selects the latest `updated_at`; historic attempts cannot trigger the current alert. No historical rows were rewritten.

Retry is the existing explicit publish command with current `publish_attempt` and a fresh Idempotency-Key. Backend approval, allowance, attempt CAS, max-attempt, and prior-attempt-nonspending rules are unchanged. UNKNOWN is not converted into a retryable failure. No per-ad-set beneficiary/payer picker was introduced.

## Commits

- `f07e1d71` — adapter, PG reason projection proof, sandbox assertion, contract.
- `94dfb152` — merchant guidance/checklist, model tests, real-click browser coverage.
- Final evidence commit: pending.

## Commands / exits

| Command | Exit | Evidence |
|---|---:|---|
| `go test ./internal/integrations/meta_ads -run '^TestTaiwan' -count=1` before fix | 1 (expected RED) | `adapter-red.log`: TW category missing, verification error still graph_100 |
| `go test ./internal/integrations/meta_ads -count=1` after fix and review additions | 0 | `adapter-green-final.log` |
| `go build ./...` | 0 | `go-build.log`, final rerun pending |
| `go vet ./...` | 0 | `go-vet.log`, final rerun pending |
| `gofmt -l` changed Go files | 0, empty output | `gofmt.log`; all tracked Go final rerun pending |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates.log`; tracked-file final rerun pending |
| `bash scripts/dev/test-node.sh` | 0 | `test-node.log` |
| `node --test --experimental-strip-types tests/admin/ads-model.test.ts` | 0 | `ads-model.log`: 14 PASS |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc.log` |
| `bash scripts/dev/test-focused.sh '^(TestMetaAds\|TestAds\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | `pg-focused-final.log`: top-level 23 PASS / 0 FAIL / 4 sandbox SKIP |
| `bash scripts/dev/test-local.sh --browser-meta-ads` | RUNNING | `browser-meta-ads.log` |
| `bash scripts/dev/test-local.sh --browser-click-sweep` | QUEUED | evidence pending |

Initial focused attempt exited 1 at compile time because the new test used `foundation` instead of the repository's external `foundation_test` package. Corrected the package declaration; the entire requested regex was rerun successfully. `pg-focused.log` is retained, not counted as the intended red evidence.

Queue wrapper `test-focused.sh` sets `LC_TEST_LOCK_WAIT=14400`, waits on the shared machine PG lock, never deletes a foreign lock, and gives child runners a task-local lock. `exits.tsv` records wrapper/static command exits. No assertions or thresholds were removed or weakened.

## Click ledger / screenshots

Browser acceptance pending. New MA09 Taiwan test uses the real BFF/Go/PG, with a runner-only fake Graph rejection. It exercises create, approve, publish, both help links, persisted reload, explicit retry with new key/attempt, and pause. The external help destination is a disclosed navigation MOCK; it does not claim Meta help content was verified. Three locales and 1586×992 / 390×844 screenshots are planned.

## Independent review / roles

All participants used this worktree/base. Root was the only writer; no writing subagent or separate branch was created.

- `ads_reason_map`: read-only explorer, configured `gpt-6-luna / medium`; responsibility API projection/retry map; no write paths.
- `ads_test_map`: read-only explorer, configured `gpt-6-luna / medium`; responsibility test seam/UI audit; no write paths.
- `ads_tw_review`: read-only security_reviewer, configured `gpt-6.1-sol / high`; no write paths. No unresolved scoped P0/P1/P2 after rechecking checklist-link realclick coverage and read-path rejection tests. Reviewer did not run PG/browser. Humaux title: `ads-tw-regulation b0f835c3 final static review: both P2 closed, no scoped P0/P1/P2`.
- Main Codex: implementation and independent local test execution; runtime exact model identifier not exposed in the session.

Skills: `frontend-architect` used to retain the existing read/model/command separation; `playwright` used with repository fixtures for real-click acceptance, not production browser automation.

## NOT_RUN / limitations

- SANDBOX and LIVE advertising: explicitly not run. Focused suite's 4 sandbox tests skip at their environment gates; no keys read.
- Full G07: not triggered; no migrations, grants/policies/definers, checkout or storefront runtime modified.
- Optional R04 livekit binary suite: NOT_RUN from `test-node.sh` because `COMMERCE_R04_LIVEKIT_BINARY` unset.
- Current Meta help fetch required login; F23/F24 rely on the owner's sandbox evidence and frozen integrator ruling, not an invented web readback.
- Browser acceptance and final evidence remain pending; this is not a release approval.
