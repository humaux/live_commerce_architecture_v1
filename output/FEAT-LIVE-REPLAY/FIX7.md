# FEAT-LIVE-REPLAY FIX7

Date: 2026-11-01. PR #396 → r3/integration. One implementation batch; no new RED push.
Executor: integrator-dispatched Codex implementation lane, no recursive delegation.
Runtime model/reasoning: not exposed by the runtime; UNKNOWN.
Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/feat-live-replay.
Branch: unit/feat-live-replay. Starting head: b63aa7436fb6f26d817c908182c2b7ce29c053ed.
Fetched/merged origin/r3/integration first: exit 0, Already up to date;
base 4b8c04c7213025c173e260db6f9c81cb5b818c61. Existing dirty reports/evidence preserved.

## Authority and scope

Authority: the human FIX7 ruling in this dispatch, AGENTS.md, architecture §§0/1/2/26/27,
contracts/live-console-v1.md replay amendment and contracts/storefront-v2.md replay amendment.
Read contracts/tasks.json DAG and existing dependency map; targeted rg/source reads substitute
for unavailable code-index tools. Humaux tools are unavailable: no claim, store, canvas update,
rejected/lesson enumeration or fine-rule retrieval was possible. No local/platform memory used.
Applied installed ponytail skill for the smallest existing-pattern implementation.

This batch changes only the two granted message entries, the replay spoof test and own
contract amendment, the exact KC03 exemption, generated browser ownership, red.log and this report.
No additional unapproved write path. Generated ownership is the only support path beyond
the enumerated source/contract/test/report paths; pre-approved by the dispatch.
No internal/buyerhttp change or dispatcher freeze movement. Migration remains 0219;
its contents, R2 pin, route-table-dispatchers.json, contractdrift baseline, gate-adapter
assertions and HistoricalPolicy tests are unchanged.

## Changes and privacy disposition

1. internal/httperror/error.go has exactly two additional fixed English messages:
   invalid_transition and no_official_video, each on its own line. Unknown-code fallback
   remains internal. No route-local error envelope.
2. The new replay spoof request now expects the established 400 invalid_json convention
   (merchant_account_http_test.go:130 / merchant_settings_http_test.go:171). Before/after
   direct DB reads additionally assert replay_public and replay_version are unchanged;
   reads are evidence only, and the request still uses the real merchant HTTP handler.
   The PR's own replay amendment specifies this result and no-write property.
3. **Choice (b).** No existing approved projection supplies a public session's complete
   offer identifiers, SKU identifiers and created_at ordering without buyer/claim context.
   Examined all migration definitions containing live.offers and all buyer-callable claims
   grants. claims.preview_link (0060:375–410) requires buyer/session GUCs and an unexpired
   claim-link hash; claims.preview_live_prices (0158:95–126) has the same prerequisites and
   returns keyword/price, not the required session offer projection. claims.live_prices
   (current 0177:188–240) requires buyer-owned bundles/lines or merchant-origin grants and
   supplied cart arrays; redeem_link/mark_applied mutate claim state. Merchant/worker
   projections (0123, 0128, 0148, 0165, 0175) are not existing public buyer enumeration.
   Reusing these would require inventing private buyer/claim authority or changing their
   security contracts. No suitable view exists. catalog.buyer_v2_product supplies current
   product data, but no session-to-offer relation; the replay flow already reuses it.

   The deny query exempts exactly commerce_replay_reader / live.offers only when its
   effective SELECT columns equal created_at,id,session_id,sku_id,store_id,tenant_id.
   It still rejects any table-level right or column INSERT/UPDATE/REFERENCES; no wildcard,
   other role or table is added. All six positive column assertions remain verbatim.
   Migration grants/RLS and production code are unchanged for this item.
4. HistoricalPolicy negative-control FAIL strings have a passing parent in the original
   g08 job; no change.

## Evidence and verification

Fresh GitHub original receipts: Gates **41281668151**, head b63aa7436fb6f26d817c908182c2b7ce29c053ed,
g08 job **124775984368**: KC03 foreign role commerce_replay_reader/live.offers;
g10 job **124775984253**: invalid_transition and no_official_video rewritten to internal,
and spoof status 400 versus the new test's prior 422. Existing tests-first RED is Gates
41093988979, unit 124202749403 and g10 124202749598. E3 failure evidence only, not GREEN.
Retrieved with gh run view --job --log-failed, filtered to relevant assertions.

Local static commands (all exit 0; **E1 only**):

- gofmt -w internal/httperror/error.go tests/foundation/live_replay_test.go tests/foundation/live_claims_schema_test.go
- go build ./internal/httperror ./internal/httpapi ./internal/live ./internal/buyerhttp
- go vet ./internal/httperror ./tests/foundation
- python3 scripts/check_packet.py (PASS_PACKET_STRUCTURE_ONLY)
- bash scripts/dev/depmap.sh (regenerated, no diff)
- node scripts/dev/derive-browser-ownership.mjs (55 modes), then the same command --check
- git diff --check

Source SHA-256:

| File | SHA-256 |
| --- | --- |
| internal/httperror/error.go | 4d48d3015fa6f7433f6d01b0a038aa0358c57d6fc1d3c38dd94e7652a51a58fb |
| tests/foundation/live_replay_test.go | c1c80d9b0f081af2edb743fc2596e681e73ce1d0b33d9f6b3cc57f18172a129a |
| tests/foundation/live_claims_schema_test.go | 29b72d4e6d9817692598d8ff04b758a0a2111116d642b71c765aabf2b0d87447 |
| contracts/live-console-v1.md | cdcf59c5836cacf6d8ab39d750acc77bcbd0ca49d9d0bb905feb9acec2be4de2 |

Risk: exact-head REAL_PG CI and independent Opus privacy review must validate item (b).
Author PASS = E0. GREEN is only this PR's own CI; pending after the one implementation push.
CI run IDs/head are recorded below after push and in the PR comment; no evidence-only push.
Rollback: revert only the FIX7 implementation commit, preserving prior evidence/dirty files.
No local tests, servers, fixtures, containers or long-running processes were started.
NOT_RUN: local tests/servers (prohibited), new-head CI completion, independent E4/privacy
review, SANDBOX/LIVE, UI/browser/design parity, merge/auto-merge and deployment.
Integrator owns the completion relay; worker will not poll longer than ten minutes.

STATUS: IMPLEMENTATION_PREPARED_CI_PENDING
