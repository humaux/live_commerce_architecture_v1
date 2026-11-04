# FEAT-BUYER-RMA FIX6 — CI failures at 3d7b1905b

2026-11-04; PR #395, branch unit/feat-buyer-rma, worktree .worktrees/feat-buyer-rma. Base HEAD 3d7b1905b.
Gates run 41777847107, Foundation run 41777847105. Local static only (gofmt -l, go vet, node --check,
ownership generator + --check, contractdrift). No local tests/servers, no push, no Humaux writes.

## Failure → root cause → change

| Failing job / test | Root cause (file:line) | Origin | Change (file:line) |
| --- | --- | --- | --- |
| gate (shard:g10) 126403306102 — TestBuyerReturnRequests/BR05 `unsafe or incorrect error envelope` (buyer_return_requests_test.go:329) | The 0221 definer raises `request_open` (migrations/0221_buyer_return_requests.sql:129) and returns.MapError maps it to 409 (internal/returns/returns.go:45), but the shared error writer only emits codes listed in its messages map and rewrites every unlisted code to `internal` (internal/httperror/error.go:304, "ruling 15"). `request_open` was never added, so the losing buyer got 409 `internal`. Every other return-request code was already listed. | Pre-existing in #395 (BR05 never ran past the header failures in FIX4/FIX5). Not from the #394 merge. | Product fix: add `request_open` to the messages map, internal/httperror/error.go:169-170. |
| gate (shard:g10) 126403306102 — TestBuyerReturnRequests/BR03 `no rows in result set` (buyer_return_requests_test.go:234 at 3d7b1905b) | **Fixture correction, not a product change.** `tcvNew{stripe:true}` builds its store through psSetupItemsOn, which copies the base fixture and replaces `tenantA`, `storeA1`, `principalA` and `tokens` (only `"a"`), tests/foundation/payment_start_test.go:72-74. BR03 combined that copy's new `tenantA` with the base `storeA2`, which belongs to the base tenant. No grant row can match that pair, and `tokens["a2"]`/`tokens["b"]` were empty strings. So the test never reached any product assertion. | Pre-existing in #395's own test. Not from the merge. | tests/foundation/buyer_return_requests_test.go:224-236: the foreign stores, their tenants, the merchant tokens and the principal lookup now come from the seeded base fixture `e.r.f` (storeA2/principalA2 in base tenantA, storeB/principalB in tenantB). The assertions are unchanged: buyer 404 on both foreign origins, merchant 404 on foreign detail/approve, then 403 for staff without fulfillment:write. Disclosed limit: the order store sits in its own random tenant, so both foreign stores are now cross-tenant. A same-tenant/other-store case never existed in this harness, because storeA2 was never in the order's tenant. |
| Foundation 126402251498 and gate (bundle:09) 126403306143 — tests/admin/shell-architecture.test.mjs G-UI3/G-UI5 `apps/storefront/lib/buyer-server.ts: lines 1340 > 1331` | Commit 3d7b1905b swapped the one-line namespace import for a 10-line named-import block. That added 9 lines to the ratcheted legacy module, whose cap is 1331. | From 3d7b1905b (in this PR, after the merge). | apps/storefront/lib/buyer-server.ts:5: the same named imports in one statement on one line (now 1331 lines, at the cap, unchanged). The cap and exemption are untouched. contractdrift still resolves the BFF graph (errors=0). |
| gate (--browser-webkit) 126403306125 — TestBrowserTaiwanCvs/WebKit (browser_taiwan_cvs_test.go:160) → tests/storefront/cvs-picker.mjs:392 `cvs-candidate-024793` not visible, zh-CN desktop picker, first brand FAMI | **Not established as caused by #395.** zh-TW passed, then zh-CN failed. The failure is right after the brand-switch quote commit that the test comment at cvs-picker.mjs:379-381 already calls racy. #395's buyer-server.ts diff vs 573f105d4 only adds return-request routes, bodies and validators: buyerReturnRoute returns undefined for `quotes`, `cvs-stores` and `destination`, validBuyerReturnBody/Response pass non-return targets unchanged, and the retry flags change only for return mutations. #394 (742b04380, run 41773728645) and 16 other recent branch runs passed webkit. This head is the first webkit run on this branch. | Unknown / probable timing flake. No #395 cause found. | None. I did not edit the test, add a timeout or add a retry. Needs one CI re-run on the new head to tell a flake from a regression. If it fails again, escalate (second directed fix). |
| receipts 126412126627, required 126412767607 | Aggregates of shard:g10, --browser-admin-shell (bundle:09) and --browser-webkit. | — | Resolved by the rows above, apart from the webkit row. |

## Static evidence (local, this commit)

- gofmt -l internal/httperror tests/foundation/buyer_return_requests_test.go: no output
- go vet ./internal/httperror ./tests/foundation/: exit 0
- node --check apps/storefront/lib/buyer-server.ts: exit 0
- node scripts/dev/derive-browser-ownership.mjs, then --check: exit 0; it re-hashed buyer_return_requests_test.go in output/ci-select-backend-browser/browser-ownership.json
- go run ./scripts/dev/contractdrift: exit 0, errors=0

Evidence level: E1 (structure). Behavioural GREEN for BR03/BR05, G-UI3 and webkit has NOT_RUN and is CI-only. Author judgment is E0.
NOT_RUN: local Go/PG/browser tests (prohibited), current-head CI, independent review.
