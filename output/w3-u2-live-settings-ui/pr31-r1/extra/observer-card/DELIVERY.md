# PR31 extra observer tests

Task: e0cab99c-sub-observer-tests; parent: e0cab99c-5ed3-4d22-a3cd-88700fb57c71.
Base: 2c4d2f5320d9172e6f0094341bb81f7c9ccb1093
Head: 0d9e0044a7e5f36dd8c9aee8fa099cb9b3d5ddac
Branch/worktree/role/model and exact commands: manifest.json.

Changes: tests/admin/comment-stream-hooks.test.ts and tests/admin/live-console.spec.ts only. No production edits, recursive delegation or push. Existing scope-loss test uses the truthful BFF store_scope_lost code, retaining every clear and zeroLateA8 assertion. Actual BuyerPanel/BuyerRestriction GET metadata drives the browser spec's actual AST-extracted waiter predicate. Only the strict GET exact store/session/check arm is added; existing arms, timeout15000 and all post-wait assertions remain. afterEach preserves comment ledger and adds path/category/status/method-only scope ledger.

Evidence: red.log 22 PASS/3 FAIL; red test/spec exact snapshots and sha256 hashes retained. observer-green.log 2 PASS/0 FAIL, render-green.log 4 PASS/0 FAIL. Final baseline focused union: focused-baseline.log 72 PASS/2 FAIL exit1. Product-red.log contains those two reproducible failures. Initial dependency failure is retained in environment-red.log, resolved by offline frozen install (deps.log; no downloads).

Unresolved: healthy A13+healthy A2/A8 with ordinary restriction not_found404 still globally disables console; scheduled A2 timer admits one read1->2 when the existing hook host holds passive cleanup after real privacy expiry. Parent owns production fixes and independent rerun. No completed product claim.

NOT_RUN: parent product green, independent acceptance, browser/REAL_PG, Go/build/full-suite, SANDBOX/LIVE. E3 observer only, MOCK real source seams.

No spawned/background test processes remain. Installed local ignored dependencies retained for parent reproductions; no other task fixtures or caches removed.
