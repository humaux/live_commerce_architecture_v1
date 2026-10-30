# FEAT-LIVE-REPLAY-FIX2 — PR #396

Date: 2026-10-31 (Asia/Shanghai). Assigned branch: `unit/feat-live-replay`.
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/feat-live-replay`.
Role: dispatched implementation lane; no delegation. Model family: GPT-6; exact runtime model/effort not exposed.

## Authority and current source

- Authority: current FIX2 card (including the explicit R2 formula), original FEAT-LIVE-REPLAY card, repository AGENTS.md and architecture sections 0/1/2/9/26/27, existing replay amendments in `contracts/storefront-v2.md` and `contracts/live-console-v1.md`.
- Fresh fetch and merge of `origin/r3/integration` completed before edits. Base: `a3dc78e4d5ec72831aaf6a376793f0f6a7a56425`.
- Remote PR head before this batch: tests-only `b78ca35aaf41dfb978040edb1a148774f6f69049`. Existing prepared implementation commit `e937748714c1cbe93026ee98ffe428f334d8b044` had not been pushed. The cited failure run does not test that implementation.
- RED run `41093988979`, attempt 1, tested synthetic merge `b009564a4522eef75a1da8ff09256f44974cbd93`. Fresh job logs below establish causal assertion RED (E3 failure evidence only).
- Latest base run `41105181453` was queued when inspected; no base GREEN or class (c) exemption is claimed.
- Humaux tools (search/get/store/canvas/coord/code index) are unavailable in this runtime. No shared-memory write, lock, claim or canvas update is claimed. This file is the repository handoff, not a Humaux substitute claimed as successful storage.
- Existing uncommitted DELIVERY/red.log edits and prior untracked evidence were preserved. Pre-edit tracked diff saved to `fix2-before.patch`; pre-merge HEAD was `e937748714c1cbe93026ee98ffe428f334d8b044`.

## Every failing job → test → evidence → class

Class (b) below means missing product behavior in the tests-only RED head. No old UI assertion was intentionally replaced and no test expectation was changed for FIX2. There is no class (a) rewrite or proven class (c) base failure.

| Failing job | Failing test/check | Original assertion/evidence | Class and product fix |
| --- | --- | --- | --- |
| `gate (shard:unit)` / `124202749403` | `TestLiveReplayOfficialPermalinkRead` | `live_replay_test.go:42`: `existing official live_videos read must request permalink_url`; `fix2-red-unit.log` | (b) Prepared `live_videos.go` appends `permalink_url` to the existing one-read Graph field list. |
| `gate (shard:unit)` / `124202749403` | `TestLiveReplayPermalinkTrustBoundary` / `official` | `live_replay_test.go:79`: `safe permalink="" want="https://www.facebook.com/synthetic.page/videos/777/"`; same log | (b) Decode and project the optional field through `safeReplayPermalink`. Only Graph-supplied HTTPS facebook.com/www.facebook.com numeric-video permalinks are retained; token/query/credentials/foreign/plugin inputs remain rejected. Existing negative assertions unchanged. |
| `gate (shard:g10)` / `124202749598` | `TestLiveReplayOfficialReadPersistsVideo` | Direct job log line 1676, `live_replay_test.go:72`: `successful official read must persist a durable live-video row`; `fix2-red-g10.log` | (b) Assigned migration 0213 installs the bounded same-transaction snapshot-completion persistence trigger, forced RLS and scoped projections. No historical Meta call. |
| `gate (shard:g10)` / `124202749598` | `TestLiveReplayPublicationAndBuyerReads` | Direct job log line 1679, `live_replay_test.go:123`: `GET: status=404 want=200`; same log | (b) Prepared merchant route/service adds publication permission/CAS/idempotency; buyer route reads public ENDED sessions using current catalog projection; exact BFF paths added. |
| `receipts` / `124217150611` | Failed unit/g10 receipts | `FAIL: shard:unit: status FAIL`, `FAIL: shard:g10: status FAIL`, `matrix-receipts: RED (2 problem(s))`; `fix2-red-receipts.log` | Downstream of (b), not a standalone receipt defect. No receipt gate change. |
| `required` / `124218328366` | Required aggregate verdict | `gate=failure aggregate=skipped ... receipts=failure`; `fix2-red-required.log` | Downstream of (b). No required-check change. |

`aggregate` / `124221494937` and `deploy` / `124197381189` were skipped, not additional failed tests. No browser job failed in this run; browser artifacts are unnecessary for these Go assertions. The CLI g10 log view returned no matching content; the official `actions/jobs/124202749598/logs` API supplied the raw lines instead.

## Implementation batch and pin

The batch publishes the previously prepared implementation after the integration merge. FIX2 adds the assigned migration-set ledger/count update and adapter invariant header. Baseline migration inventory was independently counted from `git ls-tree origin/r3/integration`: **139** R2 files; this branch adds exactly `migrations/0213_live_replay.sql`. Per the current explicit assignment, the exact pin becomes **140** (139 + 1). No migration number is invented, no migration is edited in FIX2, and the exact-count assertion remains exact.

No tests/fixtures were weakened, deleted or skipped. The gate adapter, contract-drift baseline, dispatcher inventory, locks and dependency manifests are unchanged. No UI page or Figma work is part of this backend card.

## Static evidence (E1 only)

| Command | Exit/result | Evidence |
| --- | --- | --- |
| `go vet ./internal/integrations/metareply ./internal/live ./internal/buyerhttp ./internal/httpapi ./tests/foundation` | 0 | `fix2-go-vet.log` (empty successful output) |
| `python3 scripts/check_packet.py` | 0; packet structure only | `fix2-check-packet.log` |
| `gofmt -l internal/integrations/metareply/live_videos.go tests/foundation/r2_integration_upgrade_test.go` | 0; no unformatted files | Tool receipt |
| `node scripts/dev/derive-browser-ownership.mjs` then `--check` | 0 / 0; 54 modes | Generated ownership artifact; exact R2 source hash refreshed |
| `bash scripts/dev/depmap.sh` | 0; no generated diff | `fix2-depmap.log` |
| `git diff --check` | 0 | Tool receipt |

TypeScript static checking: NOT_RUN (`tsc` and local executable absent). No dependency install attempted. No local behavioral tests, databases, browsers or servers ran.

## Scope, acceptance and handoff

Extra support paths to disclose in the PR: `tests/foundation/r2_integration_upgrade_test.go` (explicit FIX2 pin assignment), `internal/integrations/metareply/live_videos.go` and its existing new test (original card's explicit official-read extension), `tests/foundation/live_replay_test.go` (acceptance), `internal/buyerhttp/routes_test.go` (two additive route expectations, immutable legacy oracle retained), and generated `output/ci-select-backend-browser/browser-ownership.json`. Dependency map regeneration produced no diff. All production API/BFF/contracts/migration paths were assigned by the original card; no new out-of-module product expansion in FIX2.

GREEN is PENDING until this PR's own new head/synthetic merge CI passes. Independent review/E4, provider SANDBOX/LIVE, browser/UI parity and deployment are NOT_RUN. Own behavioral PASS is E0, not acceptance. UI: pending PARITY-NEWFEAT.

This is the single authorized FIX2 implementation batch. If the same assertion remains RED after it, stop and report BLOCKED with the exact new assertion/job; no third approach. After push, record exact Gates/Foundation workflow run IDs in the PR and session handoff; observe for at most 10 minutes and return WAIT_RUN for the integrator's completion relay. No merge, auto-merge, force-push, commit status or deployment.
