# FIX5 — FEAT-LIVE-REPLAY / PR #396

2026-11-01. Dispatched implementation lane, branch `unit/feat-live-replay`, worktree `.worktrees/feat-live-replay`. No recursive delegation or duplicate claim. Runtime model/effort UNKNOWN. Authority: human FIX5 escalation card; AGENTS.md; 架构.md §§0/1/2/26/27; contracts/live-console-v1.md:1294 and contracts/storefront-v2.md:515; current source and GitHub original CI logs. Read Humaux development detail `7bdb220b-0798-4b1d-aff5-16ee94cae75f`; rejected representative subset 3/9, coverage 0.3333 (not exhaustive). Ponytail applied.

Fetched and merged `origin/r3/integration` before edits, exit 0 / Already up to date. Integration base `4b8c04c7213025c173e260db6f9c81cb5b818c61`; starting local merge SHA `812148daa0b35b4bb32da22e519e37a0a962142d`. All four items are the **first directed FIX5 repair** under the explicit escalation grant. Migration remains **0219**, assigned R2 count **143**. No production operation.

## Failure → root cause → change

| Failure / original run and job | Root cause | Change (file:line) |
| --- | --- | --- |
| Foundation `41275260986 / 124756060533`, check-backend-coverage | New merchant replay adapter lacked file-level coverage classification. No browser spec/runner exercises the replay endpoint; the replay contract explicitly leaves UI pending PARITY-NEWFEAT. | One BACKEND_ONLY entry, `scripts/dev/pr-modes.mjs:153`, naming `TestLiveReplayPublicationAndBuyerReads` (g10). Nothing else in this registry changed. |
| Gates `41275260978 / 124756873935`, TestPublishedStorefrontAuthorityAndConstraints, owner/ACL/search_path | The resolver's **existing closed ACL expectation**, not a missing new function owner, rejected the new reader's required origin-resolution EXECUTE. All four new functions already have explicit owner, PUBLIC revoke and exactly `search_path=pg_catalog`. | Add only `commerce_replay_reader` to the resolver expectation, `tests/foundation/published_storefront_test.go:233`; add exact owner/caller/search_path assertions for all four 0219 definers at :239. Preserve the resolver's owner, search_path and every prior grantee. Existing migration authority: :32/:46–47, :71–82, :87–88/:115–118, :122–123/:129–131. |
| Gates `41275260978 / 124756873395`, TestLiveClaimsKC03Schema/privilege-matrix | The six extra entries are **six columns on live.offers**, each actually read by buyer_replays. The fixed closed matrix did not register the new restricted reader. Separately, its broad replay_videos table SELECT included unused requested_at. | Add only those exact six column expectations with reasons, `tests/foundation/live_claims_schema_test.go:216–225`. Narrow the unmerged migration's original broad grant to seven read columns at `migrations/0219_live_replay.sql:54–56`; requested_at gets no reader grant. No offer INSERT/UPDATE or table-level SELECT added. |
| Gates `41275260978 / 124756873477`, TestLiveReplayPublicationAndBuyerReads, initial GET 422/want200 | A zero-length, EOF-returning reader is a legitimate empty request; studioRoute rejects its non-nil Body identity before replay authorization. | `internal/httpapi/live_replay.go:21,36–55`: replay GET proves an otherwise unframed reader empty with a one-byte bounded read, then delegates to unchanged claimsRoute. Nonempty/erroring readers return 422/private no-store. ContentLength/TransferEncoding, query, key, UUID, server scope and permission guards remain in force. The reproducing GET and its 200 assertion at `tests/foundation/live_replay_test.go:100,123` are unchanged. |
| Gates `41275260978 / 124756873741`, bundle:07 TestBrowserTaiwanCvs/WebKit | **BASE, not reached by this diff** per FIX5 ruling. Earlier evidence: `fix4-cvs-webkit.log:13–28`, missing `cvs-candidate-000005` at `tests/storefront/cvs-picker.mjs:392`. This is not a newly reproduced base execution. | **No change** to CVS code, harness, timeout, threshold or fixture. Keep this evidence for its owning lane. |

## Least-privilege audit against function bodies

The new role has NOLOGIN/NOBYPASSRLS and no membership grant. Table RLS remains forced; buyer/runtime receive only specific definer EXECUTE. Below are **all** replay-reader SELECT grants, checked against 0219 function bodies. No other columns remain granted; editing the PR's unmerged initial grant removes the unused right before it can be applied.

| Table / retained columns | Exact use |
| --- | --- |
| live.replay_videos: tenant_id,store_id,binding_id,post_id | replay_permalink :74 / buyer_replays :100: exact source join. |
| live.replay_videos: permalink_url | :73,77 / :99,102: projection and nonempty filter. |
| live.replay_videos: observed_at,video_id | :78 / :102: newest observation and deterministic tie break. |
| live.sessions: tenant_id,store_id,id,title,lifecycle,lifecycle_at,replay_public | buyer_replays :95–105: exact store/session join, public/ENDED predicate and display/end-time order. |
| live.claim_sources: tenant_id,store_id,session_id,platform,binding_id,source_object_id,active | replay_permalink :73–77 / buyer_replays :99–102: exact active Facebook binding/post source. |
| live.offers: tenant_id | buyer_replays :112–113: tenant join and fence. |
| live.offers: store_id | buyer_replays :112–113: store join and fence. |
| live.offers: id | buyer_replays :111: offer identifier and stable tie break. |
| live.offers: session_id | buyer_replays :113: requested session only. |
| live.offers: sku_id | buyer_replays :111–112: SKU identity and current catalog product join. |
| live.offers: created_at | buyer_replays :111: display order. |
| catalog.skus: tenant_id,store_id,id,product_id | buyer_replays :111–112: scoped SKU join and product identifier; current price/stock remain catalog projection authority. |
| control.stores: tenant_id,id,active | buyer_replays :93: tenant lookup for the already published-origin store, active only. |
| **Removed: live.replay_videos.requested_at and broad table SELECT** | No replay definer reads requested_at; it belongs solely to the fenced completion-write conflict predicate at :42. |

EXECUTE: buyer.resolve_published_store(text) → reader for published origin resolution (:91); catalog.buyer_v2_product(text,text) → reader for current product projection (:125). No customer/actor/token/price-history data grant.

## Classification derivation and limits

Ran the requested existing static derivation tool, **exit 1**: `derive-narrow-covers.mjs:1742` stops at unrelated `internal/httpapi/profile.go: no domain service coverage`. See `fix5-covers-derivation.log`. Did not change that tool, its registry rules or unrelated classification. This generator failure is not a CI test verdict.

Independently searched replay endpoint strings in tests and mode registry: only `tests/foundation/live_replay_test.go:97,200–202,257,266` calls them; no browser spec/runner does. Used the card's explicitly permitted single BACKEND_ONLY fallback based on this evidence, **not** on the generator's missing coverage. No lc_covers line was guessed or rewritten. Any interim derivation evidence output was left outside this commit.

## RED / verification / handoff

Existing tests-first RED remains `41093988979`, unit `124202749403` (new permalink request/normalization assertions), g10 `124202749598` (durable row missing; route 404). The current PR's CI above reproduces all four directed FIX5 failures before these repairs; refreshed exact excerpts are `fix5-red-{foundation,g05,g08,g10}.log`, appended to `red.log`. Reuse those causal failures; no redundant tests-only push or local behavioral execution. E3 here means **failure evidence only**, not acceptance.

Static E1: gofmt, go build ./internal/httpapi ./internal/live ./internal/buyerhttp, go vet of those packages, go vet ./tests/foundation, node --check scripts/dev/pr-modes.mjs, python3 scripts/check_packet.py, git diff --check all exit 0. Packet-check's incidental timestamp output restored to its prior content. Browser ownership regenerated then --check: exit 0/0, 55 modes; generated file included. depmap regenerated: exit 0/no diff. No new nonstandard dependency. Logs retained in this directory.

This batch's outside-original-path support changes, explicitly granted by FIX5: scripts/dev/pr-modes.mjs (one entry only), tests/foundation/published_storefront_test.go (exact additive expectations), tests/foundation/live_claims_schema_test.go (exact additive expectations); generated output/ci-select-backend-browser/browser-ownership.json. No unapproved source-path change. No edits to internal/buyerhttp, dispatcher pin, baseline, OpenAPI, module/lock files, R2 pin or unrelated worktrees. Pre-existing dirty FIX2.md and untracked prior evidence retained, not included in this batch.

Rollback for this batch: inspect the FIX5 commit diff against starting local SHA `812148daa0b35b4bb32da22e519e37a0a962142d`; revert only that commit through the integrator if needed. Do not reset the worktree or discard pre-existing evidence. No production rollback required.

GREEN and exact pushed-head run/job ids: pending the single implementation push; record in a PR comment, never an evidence-only follow-up push. Author PASS = E0. E4 privacy review remains with the integrator's assigned reviewer. Wait budget ≤10 minutes; workflow maximums bound execution; integrator completion relay is the wake-up path. Existing unrelated CVS failure may still block the aggregate and is not exempted here.

NOT_RUN: all local tests/servers (prohibited); post-fix GitHub CI completion/REAL_PG/browser; independent E4/privacy review; provider SANDBOX/LIVE; UI/design parity; merge; deployment. No local test/server/container/fixture started. Static commands have terminated; no unmanaged background process remains.

STATUS: WAIT_CI — FIX5 implementation prepared; E1 static only, GREEN NOT_RUN.
