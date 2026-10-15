# PE-FIX-1 round 2 delivery (PR #64)

Status: NOT_RUN. Nothing was executed locally (per instruction); CI is the first run.

## Fixed
- P1-1: `ProductDocumentForm` passes `null` photos to `write.save` in edit mode when the image list is unknown, so `needsMainImage` skips the client check. The server still enforces it. `write.save` never sends photos in edit mode.
- P2 batch: if an upload succeeded but clearing the journal failed, the batch now stops and the summary shows the "unknown, do not re-upload" copy instead of a plain failure.
- Test added: `tests/admin/product-media-v2.spec.ts`, "an active product saves a name edit while the image list cannot be read". It stubs the image-list GET to 500 (`// FIXTURE`), then edits the name with real clicks, saves, reloads, and checks the name and the detail-row count.

## Open P2s (not data integrity)
- Oversize fixture is a non-decodable 2 MiB+ buffer. It exercises the preprocess refusal, not the `MAX_PHOTO_BYTES` guard in `product-media-client.ts`. A corrupt JPEG over 2 MiB now says "compress and upload again".
- Leave-guard copy ("wait for it to finish") is misleading when `write.recoveryBlocked` is the lock cause, since that state does not clear itself.
- Integrator should run `git diff origin/r3/integration HEAD -- tests/ | grep '^-'` to confirm no assertion was removed.

## Click ledger (new test)
| Page | Control | Action | Expected | Actual |
|---|---|---|---|---|
| editor, list read failing | product-name | fill | value changes | NOT_RUN |
| editor | product-save | click | PUT 200; name persists after reload; images unchanged | NOT_RUN |

# PE-FIX-1 round 3 (PR #64) — Status: NOT_RUN (no local runs; CI is the first run)

## Root cause (test 9, `product-name` disabled after reload)
- Not the round-2 `null` photos change and not a create journal. The fence key is `product-document-pending:<store>:<productId>` (edit) and is kind-less; the lock copy is shared with create.
- `save()` writes the fence before the PUT; it was removed only after the PUT *and* the confirm `readProduct`. The test reloaded right after the PUT response, before the client finished that chain, so the fence survived and `recoveryBlocked` locked the form on load. Genuine race, product-side window plus a test that did not wait for the page.
- Fix (`use-product-document.ts`): for an edit, clear the fence as soon as the PUT returns 200 (committed, idempotent, nothing to duplicate). Failures/UNKNOWN keep it. If the confirm read then fails, the retry button still works from `op.id`. Create keeps the fence until confirmed.
- Test (`product-media-v2.spec.ts` test 9): added a wait for the visible saved message before unroute/reload. The final restore step is unchanged. A reload in the PUT-response window remains a genuine unknown and still locks (by design).
- No pure-logic unit test: the logic lives in a React hook with sessionStorage; the browser case is the regression.

## Proposed lock copy (NOT applied; semantics unchanged)
- Edit: 「上一次商品修改的結果尚未確認。請先重新整理本頁查看商品是否已更新，確認前請勿重複送出；若仍被鎖定，請聯絡管理員核對命令回執。」
- Create (current key): 「先前的商品新增結果尚未確認。請先到商品列表查看是否已建立，勿重複新增；若找不到，請聯絡管理員核對命令回執。系統未快取草稿內容。」
- Needs a mode-aware copy key (`recoveryRequiredEdit`); note the lock does not clear on refresh (sessionStorage), so "refresh" guidance only holds if the product reload clears it — decide with owner before shipping.
