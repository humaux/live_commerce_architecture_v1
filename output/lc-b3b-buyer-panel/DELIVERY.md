# LC-B3b PR #12 review round

- Author: Codex-3 (exact runtime model ID unavailable). Start: `06a03c6c4c3ecb73cabecb783c15c3b99877152e`, branch `unit/lc-b3b-buyer-panel`, assigned worktree `.worktrees/lc-b3b-buyer-panel`. Commit after each green finding; no push or GitHub replies. G07 NOT_RUN by explicit owner instruction; integrator owns independent review and wider acceptance.
- Independent test author: test_worker, assigned gpt-6.1-sol/high, base06a03c6c, isolated `.worktrees/lc-b3b-review-tests`, owns only three `live_console_buyer_panel_review_*_test.go` files. Read-only cursor/ACL mapper: explorer, assigned gpt-6-luna/medium. No recursive delegation. Root owns production changes and serial PG execution.
- Canonical evidence: main checkout `output/lc-b3b-buyer-panel/review-round/`; committed copies here preserve logs across worktree cleanup. Every run status binds HEAD, aggregate Go/SQL source SHA256, command, timeout, exit, and unchanged-source check. REAL_PG uses test-focused's disposable PG and real HTTP router; Meta LIVE and browser acceptance are NOT_RUN.

## 4209955005 — ordinary claim checkout orders

- Retained the WIP UNION in 0165 after verifying it: `claims.order_origins` includes price-neutral orders and UNION deduplicates order IDs across origin/live-price/A16 records. Existing tenant/store predicates and latest20 projection remain intact. Added the explicit0113 prerequisite/header and corrected the A13 contract source list. No new grant;0113 already grants `claims_writer` SELECT on order_origins. All definer OWNER/SECURITY DEFINER/search_path/REVOKE/GRANT/COMMENT attributes retained.
- Corrected the stopped finisher's invalid fixture: no live price is SQL NULL, not0 (0092 CHECK). Original assertions preserved. Independent `TestLiveConsoleBuyerPanelReviewOriginProjections` adds triple-source dedupe, latest20 ordering, full23/24 ordinal, unconfirmed/cancelled exclusions, bundle/conversation/I09 scope and orders:read omission. Existing `TestLiveConsoleBuyerPanelPriceNeutralOrigin` retained.
- REAL_PG RED: temporarily restore only parent0165 projection while retaining tests; `bash scripts/dev/test-focused.sh 'TestLiveConsoleBuyerPanel(PriceNeutralOrigin|ReviewOriginProjections)$'` → PASS=0 FAIL=2 SKIP=0 exit=1 (orders1 instead of20; neutral order absent). `p1-red-origin.log`. First run `p1-red.log` failed fixture CHECK before product assertions and is explicitly excluded.
- REAL_PG GREEN: restored WIP projection + valid fixture; `bash scripts/dev/test-focused.sh 'TestLiveConsoleBuyerPanel|TestLiveClaimsKC03'` → PASS=13 FAIL=0 SKIP=0 exit=0. `p1-green.log`. E3 local REAL_PG only. Tests did not read a changing migration.

## Remaining review steps

- 4209955021: pending session filter before SQL bound, separate REAL_PG red/green + commit pending.
- 4209955037: additive live-comment opaque keyset and LC-B3b A8 amendment, separate REAL_PG red/green + commit pending.
- Final specified focused regex, go vet and go build pending. No G07 will run in this author round.
