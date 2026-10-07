# LC-B3b PR #12 review round

- Author: Codex-3 (exact runtime model ID unavailable). Start: `06a03c6c4c3ecb73cabecb783c15c3b99877152e`, branch `unit/lc-b3b-buyer-panel`, assigned worktree `.worktrees/lc-b3b-buyer-panel`. Commit after each green finding; no push or GitHub replies. G07 NOT_RUN by explicit owner instruction; integrator owns independent review and wider acceptance.
- Independent test author: test_worker, assigned gpt-6.1-sol/high, base06a03c6c, isolated `.worktrees/lc-b3b-review-tests`, owns only three `live_console_buyer_panel_review_*_test.go` files. Read-only cursor/ACL mapper: explorer, assigned gpt-6-luna/medium. No recursive delegation. Root owns production changes and serial PG execution.
- Canonical evidence: main checkout `output/lc-b3b-buyer-panel/review-round/`; committed copies here preserve logs across worktree cleanup. Every run status binds HEAD, aggregate Go/SQL source SHA256, command, timeout, exit, and unchanged-source check. REAL_PG uses test-focused's disposable PG and real HTTP router; Meta LIVE and browser acceptance are NOT_RUN.

## 4209955005 — ordinary claim checkout orders

- Retained the WIP UNION in 0165 after verifying it: `claims.order_origins` includes price-neutral orders and UNION deduplicates order IDs across origin/live-price/A16 records. Existing tenant/store predicates and latest20 projection remain intact. Added the explicit0113 prerequisite/header and corrected the A13 contract source list. No new grant;0113 already grants `claims_writer` SELECT on order_origins. All definer OWNER/SECURITY DEFINER/search_path/REVOKE/GRANT/COMMENT attributes retained.
- Corrected the stopped finisher's invalid fixture: no live price is SQL NULL, not0 (0092 CHECK). Original assertions preserved. Independent `TestLiveConsoleBuyerPanelReviewOriginProjections` adds triple-source dedupe, latest20 ordering, full23/24 ordinal, unconfirmed/cancelled exclusions, bundle/conversation/I09 scope and orders:read omission. Existing `TestLiveConsoleBuyerPanelPriceNeutralOrigin` retained.
- REAL_PG RED: temporarily restore only parent0165 projection while retaining tests; `bash scripts/dev/test-focused.sh 'TestLiveConsoleBuyerPanel(PriceNeutralOrigin|ReviewOriginProjections)$'` → PASS=0 FAIL=2 SKIP=0 exit=1 (orders1 instead of20; neutral order absent). `p1-red-origin.log`. First run `p1-red.log` failed fixture CHECK before product assertions and is explicitly excluded.
- REAL_PG GREEN: restored WIP projection + valid fixture; `bash scripts/dev/test-focused.sh 'TestLiveConsoleBuyerPanel|TestLiveClaimsKC03'` → PASS=13 FAIL=0 SKIP=0 exit=0. `p1-green.log`. E3 local REAL_PG only. Tests did not read a changing migration.

## 4209955021 — pending session filtering before LIMIT

- Root cause: Go filtered the store's newest50 rows after SQL bounded the result; another session could consume all50. Added `inbox.link_pending_bundles(int,uuid)` in0165, preserving the0128 one-argument function. The new definer applies authenticated tenant/store and optional session predicates before ORDER BY/LIMIT; Go requests the existing10-row UI bound directly. No new table privilege.
- New overload retains STABLE/SECURITY DEFINER/fixed search_path/OWNER/REVOKE/GRANT/COMMENT. Exact caller ACL pinned to integration_writer+runtime and KC03's explicit reachable-function inventory extended by exactly this signature; legacy0128 pin unchanged.
- REAL_PG RED `bash scripts/dev/test-focused.sh '^TestLiveConsoleBuyerPanelReviewPendingSessionBeforeLimit$'` → PASS=0 FAIL=1 SKIP=0 exit=1: all/unreplied each returned0 instead of the desired older pending bundle behind51 newer other-session rows. Positive control proves distractors are visible. `p2-red.log`.
- REAL_PG GREEN `bash scripts/dev/test-focused.sh 'TestLiveConsoleBuyerPanel|TestLiveClaimsKC03|TestLiveConsoleSendMigration0128ExactACL'` → PASS=15 FAIL=0 SKIP=0 exit=0. `p2-green.log`; E3 local, source unchanged during each run.

## 4209955037 — complete live-comment keyset pagination

- Root cause: only non-bundle conversations contributed next_cursor, and liveCommentItems ran only on the first page. Added the four-argument `inbox.live_comment_bundles(uuid,int,timestamptz,uuid)` overload with `(created_at,id)` descending keyset/session/scope predicates before LIMIT. The two-argument function remains. Live-comment pages use the existing opaque A8 cursor format with the last returned stored timestamp and bundle ID; other filters retain their conversation cursor and first-page pending-link append.
- LC-B3b §11 A8 and its amendment now describe keyset pagination and the permitted empty tail after a full final page. New function retains all definer/security attributes; exactACL and KC03 pins add exactly the new signature. No new table privilege, dependency or migration number.
- REAL_PG RED `bash scripts/dev/test-focused.sh '^TestLiveConsoleBuyerPanelReviewLiveCommentPagination$'` → PASS=0 FAIL=1 SKIP=0 exit=1: first page had7 unseen rows and no cursor. Positive3-row session control passed. `p3-red.log`.
- REAL_PG GREEN `bash scripts/dev/test-focused.sh 'TestLiveConsoleBuyerPanel|TestLiveClaimsKC03|TestClaimsRetentionCRP02|TestLiveConsoleSendMigration0128ExactACL|Inbox'` → PASS=41 FAIL=0 SKIP=0 exit=0. `p3-green.log`. Independent test covers11 rows/limit4, timestamp ties crossing page boundaries, exact UUID order, no duplicates/missing rows, both sessions, full-page terminal tail, excluded manual/purged/foreign rows and positive foreign-tenant control. E3 local REAL_PG, no assertion weakened.
- Read-only mapper reread production diff: no concrete issue found in its scope; old signatures, scope/permission checks, keyset pairing and exact inventories retained. This is source review, not the integrator's independent acceptance.

## Final validation

- Final specified focused regex, go vet and go build pending. No G07 will run in this author round.
