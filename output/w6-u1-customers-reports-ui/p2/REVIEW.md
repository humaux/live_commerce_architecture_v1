<!-- Purpose: preserve current W6 P2 local evidence and narrow review.
Depends on: commands in DELIVERY and original main raw SHA256=2b91c3ebd95f4747df7df1f0ef3cda42efa0c65b46cef952ba6323b1687bca46.
Used by: integrator normal/calibration retry. -->
<!-- Purpose: narrow source-only review of W6 customer query dispatch and Cookie-varying private report output.
Depends on: the two applied root routes, unchanged query/authority helpers, frozen native-route tests and existing seed diagnosis.
Used by: W6 root/integrator; not browser/Next/PG acceptance by reviewer. -->
# W6 P2 source review

- Read-only child `codex-w6-u1-ui-sub-p2-review`, parent task `e052872b-32cf-4a78-bbb4-b8d1e106023f`; no additional claim/delegation.
- Parent worktree `.worktrees/w6-u1-customers-reports-ui`, base `00979c2779bcdca384cc8cf31fd8e0678d5a748e` plus the two route changes. This base preserves the earlier `19a4` seed correction and external `4e4` readiness/spec work; the latter is not reviewer-authored.
- Native-route tests are frozen in test-only commit `fe6794a808a65b5c6106acda9ce135e9a120a546` in `.worktrees/w6-u1-p2-tests`. Temporary product copies in that tree were restored; product candidate hashes refer to the root tree.
- Verdict: both requested P2 changes are source-accepted; no new concrete P0/P1 found in this limited delta. No broader W6 re-review, source edit, test/Node/Next/PG/browser execution or external write by reviewer.

## Customer query dispatch

`apps/admin/app/api/stores/[store]/customers/route.ts:11` now uses the existing `customerTagsRoute(method,"customers",rawSearch)` classification instead of decoded `searchParams.has("tag")`. Literal `tag=` requests remain with W6's closed query/body/store/permission authority. Invalid/empty/duplicate/foreign tag combinations are still refused by the unchanged W6 validator.

A bare `?tag` or encoded tag key is not classified as a W6 `customer-list` (`customer-tags-request.ts:42-48`), so it falls back to the same legacy GET with `resource:["customers"]` (`customers/route.ts:12`). The legacy list query validator rejects such segments (`customers-request.ts:60-68`), and its existing route turns that refusal into 422 before authority/data transport (`[...resource]/route.ts:147`). Thus malformed syntax no longer becomes an unsupported-resource 405. Normal untagged search/cursor requests preserve legacy behavior; no new parser, permission rule, forwarding header or tenant/store trust is introduced.

## Report response headers

`reports/[report]/route.ts:11` adds `Vary: Cookie` to its existing `privateHeaders` alongside `private, no-store` and `nosniff`. Both successful CSV attachment (`:64`) and JSON (`:67`) use that same object. This does not alter authorization, CSV CSRF/Origin, closed JSON/filename projection, trusted transport or time budget. The review does not claim error helpers now add Vary; those paths are outside the requested private-success header change.

## Native-route test seam

`w6-real-route-loader.mjs` loads actual routes, auth/backend and cookie/CSRF/error helpers. It supplies standard Node AsyncLocalStorage and resolves installed Next server-only/headers modules plus repository aliases; it does not replace business/helper functions (`:18-43`). Only upstream fetch replies are controlled (`:49-70`). Synthetic request cookies remain fixture data, not a signed-browser-login proof.

`w6-route-seam.test.mjs:8-18` asserts bare/encoded tag keys return 422/invalid_request with zero upstream calls. Its JSON and CSV cases (`:21-39`) require actual authenticated-store lookup, trusted bearer/no forwarded browser cookies, private cache, Cookie Vary, nosniff and no upstream Set-Cookie exposure. Malformed report queries and wrong CSRF remain refused by real helpers (`:42-51`).

Only the old fake customer-dispatch block in `w6-integration.test.ts` is replaced. All five original valid/malformed query cases remain, plus a positive valid tag UUID. Instead of mocked string-returning route functions, it now asserts real 200/422 status, safe payload, unchanged forwarded search and zero invalid-query transport. Unrelated SSR/navigation tests and older report tests are not broadly rewritten. No threshold/assertion is removed to make this delta pass.

Author's `p2/checks.json` records a native-route red exit1, 6 PASS/3 FAIL/0 SKIP (named bare-tag 405 versus422 and missing Cookie Vary for JSON/CSV), then green exit0, 9 PASS/0 FAIL/0 SKIP on the candidate hashes. Those artifacts are read as author evidence, not reviewer execution or independent runtime acceptance. Parent independently owns final checks.

## Existing seed evidence and limits

The previous seed root causes are reused, not re-investigated: report-only checkout snapshots yielded non-NULL empty phone_last3 and were not valid merchant-order details; real import profiles provide valid customer visibility without those anchors. The USD fixture uses Stripe/USD/700 rather than a TWD-only PayUni amount. No Go/SQL seed change occurs in this P2 delta.

Parent's `focused-seed-current.status.json` records PID83143 finished with the exact DB-only seed regex, browser build tag and both W6 acceptance env flags, pinned to base00979; its log reports 1 PASS/0 FAIL/0 SKIP/exit0. This is seed-only proof from the parent; it does not execute or certify the two TS changes, signed browser journeys, current external readiness/spec work, full normal/calibration CI or Next build.

Reviewer has no pending/background process or exec session. Author Node green and parent PG seed green are not promoted to full browser/CI acceptance. A future change invalidates its affected hash.

## SHA-256 bindings

| File / source location | SHA-256 |
| --- | --- |
| Root `apps/admin/app/api/stores/[store]/customers/route.ts` | `9cc4be843615235d214db9fa2f332a6889022cdb01684041eb6a454d34348474` |
| Root `apps/admin/app/api/stores/[store]/reports/[report]/route.ts` | `083268654682222a7ff0c30dab5f3d5c8adb03d13f37400e05d6cf7ad8046134` |
| Root `apps/admin/lib/customer-tags-request.ts` (unchanged) | `9ab543d756c079bd214f4216d545ae2bc7988452cb8aa33cbd9256f61cfbf5c5` |
| Root `apps/admin/lib/customers-request.ts` (unchanged) | `a4bbb087b4fc340cc609abe17c98a31d33e48765e7e1edfec206c3119b25b998` |
| Test commit `tests/admin/w6-route-seam.test.mjs` | `4404a6c5361a654c2f34b706c57d0075b8786e24bd5741106d6fbdbb1ec0dbb4` |
| Test commit `tests/admin/w6-real-route-loader.mjs` | `4c432abea09e572e9fe5df3174b5941ead4c001de234f9305ef8dc4cdd1dae98` |
| Test commit `tests/admin/w6-integration.test.ts` | `653bcf5e715e5655fafebdf06df0b1ccc9a450478c46e5e691d4b21d3a51a9e6` |
