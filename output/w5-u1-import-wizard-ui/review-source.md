<!-- Purpose: scoped independent source/security review of W5-U1 import wizard and history UI.
Depends on: frozen Go backend-interface.md, current parent TS/Next source and hash bindings below.
Used by: W5-U1 parent author and integrator; never runtime/CI acceptance. -->
# W5-U1 source review

- Reviewer: `codex-w5-u1-ui-sub-review`, read-only child; parent task `a83bad3d-2b4d-4b4a-b598-85f9e4a99cea`.
- Parent base HEAD `dc11b5ce8ceaf3c755d7192545f34f3c579108cf` plus current uncommitted UI/wire/history work; source hashes below are the binding.
- Configured reviewer model/effort: gpt-6.1-sol/high, same family; exact runtime model identifier unavailable. This is a distinct-agent SOURCE_REVIEW, not independent browser/PG proof.
- Applied security-best-practices skill for the requested security scope, with React/Next/browser guidance. No source changes, tests, PG, browser, Next, delegation, credentials or external actions by reviewer.
- Current SOURCE_REVIEW verdict after the latest real-helper correction below: no remaining concrete P0/P1 in the scoped source. R1 and the subsequently discovered R2/older no-P1 statement are historical; R2 was OPEN until this correction. Browser/PG/Next acceptance remains pending.

## R1 — P1: scope concealment silently drops an unconfirmed commit

**Source:** `ImportWizard.tsx:34,46-50,60-63,80-81,113-122` at SHA `28e4c1ac285831def5d3e734a49e7edb9c475ec7b912187878a1d6487673980f`.

**Impact:** after an import may have committed, returning to this page can show a fresh wizard without telling the merchant that the previous result remains unconfirmed, encouraging a changed-file/mapping attempt instead of reconciliation.

The hide/pagehide handler clears scope. Child cleanup correctly aborts and drops private File/pending data, but neither path updates root `expiredUnknown`. Only the explicit unauthorized-result branch calls `onSignedOut`; the post-send aborted/unmounted guard suppresses that branch on concealment. A previously UNKNOWN command suffers the same drop. On visibility/pageshow, `establish()` remounts a clean form, with no previous-result notice.

Minimal counterexample for a behavioral red: start a commit and hold its reply after the one upload is dispatched; hide/pagehide the page; return visible; resolve the old receipt. Assert zero automatic resend, old File/mapping not retained/replayed, fresh readiness false, and a visible old-result-unconfirmed notice. Repeat with a returned UNKNOWN before concealment. Current source fails the notice predicate while preserving the other concealment predicates.

Minimal fix: before cleanup clears a pending/in-flight commit, report uncertainty to a stable root callback/ref. Retain only a root boolean notice, not any uploaded bytes, mapping, receipt authority or automatic retry. Known receipt/settled refusal/stale state should clear the pending marker before later cleanup. Preserve an already-true old-result marker when later no-effect auth checks fail; do not reinterpret that earlier UNKNOWN as settled. Parent owns the actual red→green implementation.

The existing `WorkspaceFrame` already hides/unmounts children on local/storage/BroadcastChannel logout and unauthorized workspace refresh (`WorkspaceFrame.tsx:86-116,397-433`). Therefore this report does **not** claim a separate logout authorization bypass.

## Source-confirmed boundaries

| Scope | Evidence and source result |
| --- | --- |
| Original bytes/request identity | Header parser reads only the first CSV record from bounded slices; it never replaces the File. Client sends that Blob directly. UI freezes the requested mapping and expected count in the commit attempt, and does not replace it with resolved preview mapping. Retry uses the same attempt; `{}`/explicit empty fields are not silently changed to omitted/null variants (`import-header.ts:13-49`; `import-client.ts:24-39`; `ImportWizard.tsx:110-118,135-138`). |
| Private projections | BFF parses closed 200/409 backend DTOs then constructs safe rows without any external_id. It validates failed IDs empty before projection. UI table shows row numbers/local outcome/code/warnings only, not names/phone/email/status/raw cells (`import-model.ts:53-80`; `import-proxy.ts:50-56`; `ImportWizardSteps.tsx:48-54`). Source search found no raw-data storage/logging/HTML injection sink in the scoped files. |
| UNKNOWN/manual retry | Client has one fetch, no retry loop. Post-dispatch commit fence/JSON/schema/transport failure remains uncertain. UI ORs uncertainty into the original attempt; a stale or refusal retry cannot unlock fresh file/mapping/type changes. Unauthorized concealment reports `result.uncertain || p.unknown`; payload is dropped rather than replayed in the new actor scope (`import-client.ts:33-53`; `ImportWizard.tsx:104-144`). R1 concerns the separate hidden/unmount branch. |
| 409 direct stale | Client/BFF distinguish a full fresh preview from coded idempotency_conflict. Known stale returns to preview and requires explicit confirmation; an UNKNOWN retry stays locked to the old attempt (`import-client.ts:44-52`; `import-proxy.ts:50-62`; `ImportWizard.tsx:131-138,196-197`). |
| Server authority/CSRF/deadline | Exact leaf vocabulary avoids `[kind]`/`[batch]` collisions. Grammar checks before auth; bearer derived only from server cookie, server store membership checked, privacy permission checked when provided, Origin+CSRF required; Go remains final authority. Native trusted-origin fetch carries bearer/Accept/text-csv only, refuses redirects and combines request abort with 75s bounded signal rather than generic 6s (`import-request.ts:10-29`; `import-proxy.ts:14-43`; exact route `:6-18`). |
| Failure files | Preview-only local CSV derives exclusively from safe failed verdicts. Batch CSV validates backend failed-only rows with blank ID, allowed row/code fields, then emits row/outcome/code only; no original cells or source-ID column. Download final fence rechecks abort/hidden/cookie synchronously after the awaited boundary, before creating the URL/click (`import-view-model.ts:12-16`; `import-bff.ts:22-28`; `import-client.ts:59-75`; `ImportWizard.tsx:153-161`). |
| Historical read-only facts | Closed model enforces TWD, safe ≤10^12 minor units, canonical 22 TW cities/null, Unicode rune bounds, date normalization, unique external text order IDs and page caps. Component displays money/time/archive text without links/payment actions, totals accumulation or writes. GET client/BFF checks scope/session and strict limit/after; paging resets with keyed store/customer/boundary scope (`import-history-model.ts:14-68`; history client `:9-18`; component `:15-58`; exact history leaf `:9-41`). |
| Page/readiness integration | New page resolves server store; nav:false adds no primary navigation. Entry requires privacy; history subtree mounts only active imported customers. Customer readiness is local to keyed ImportForm, initially false and unlocked only by positive customer commit receipt; not stored, not inferred from a preview or historical count (`ImportWizard.tsx:61,75,128,178`; CustomerDetail changed lines `166-170`; Customers import link; customers route nav:false). |

No universal erased-history GET404 assertion is justified: the actual SQL may return an empty archive for an inactive owner retained by checkout/bundle visibility. Parent active/imported gating immediately removes the subtree; the earlier backend-interface.md records the exact source limitation.

## Limits

Real native chooser lifecycle, 390px WebKit rendering, signed-session cookie timing, real PG/import replay/erasure, route startup/build and full click-sweep remain NOT_RUN by this reviewer. Test source inspection and author's claimed passes are not reviewer runtime evidence. A mutable source change invalidates the affected hash; this initial report cannot approve merge while R1 remains open.

## Initial source SHA-256

| Worktree-relative file | SHA-256 |
| --- | --- |
| `apps/admin/components/ImportWizard.tsx` | `28e4c1ac285831def5d3e734a49e7edb9c475ec7b912187878a1d6487673980f` |
| `apps/admin/components/ImportWizardSteps.tsx` | `6dfb49518ada837bd085ce570a62ab0e047f55883a7ea0d131e5b7834535f299` |
| `apps/admin/lib/import-view-model.ts` | `38ff2e0c0deb224cd1f33621907f9aba5ac8cb65612dccdd7e5d584a48dc50ef` |
| `apps/admin/lib/import-view-copy.ts` | `5f7cb79179b2185939e123961d4749eceab1634786395a1eea68457217acb1b2` |
| `apps/admin/lib/import-client.ts` | `7b54b2d02b92b7006c832c99cacecb728d1d5d00a7d4f351a72a9f229e2c4fac` |
| `apps/admin/lib/import-model.ts` | `8711c42cfd9e9bf2730e5fb130fdf6623e659796017f049cfcb79989b8543928` |
| `apps/admin/lib/import-header.ts` | `aaeb4ad2459f8ff0a43375be9eb29c2bf3333fe1eac53498d71b13ec52ee71e5` |
| `apps/admin/lib/import-request.ts` | `4049c137e11a980cdf0f421d8b1c54e4db94d3ac34d02ad1e7031e62a95e073e` |
| `apps/admin/lib/import-proxy.ts` | `df551727b4f6fc7addb7013f3b5585be015433efa447297d5d03bc79cb048d5c` |
| `apps/admin/lib/import-bff.ts` | `a94ab33e8f9689ad95eec6e9877efdbf85d8031df7f38423f1c670a0474fb4ed` |
| `apps/admin/lib/import-history-model.ts` | `214c106aa7ba18e6c8b1d5f61f3ace99ef0de58771b80d6bff0f36f23ac1ca76` |
| `apps/admin/lib/import-history-client.ts` | `70ab0559bd4bcaea5e4d39340e518917e80aedcb0d99b8401965be1bc120e197` |
| `apps/admin/components/CustomerHistoricalOrders.tsx` | `eb35beb092129eacee0c5ed8079815dbc49bd0491c63734c919eb155cc342108` |
| `apps/admin/app/api/stores/[store]/imports/[param]/[action]/route.ts` | `2cfebf8ff2e0a523726444917d9e1d3ebedcbdcf3336df31512eeb50b2f523f1` |
| `apps/admin/app/api/stores/[store]/customers/[customer]/historical-orders/route.ts` | `aa192f086ebebf9c7bd00bf0428449165c75664ff4ad49ef27fad077439cd3ba` |
| `apps/admin/app/[locale]/customers/import/page.tsx` | `98923bd26707f901a1d797e539040a7bbc7c1889b8ac150e3b772714a86366e2` |
| `apps/admin/components/CustomerDetail.tsx` | `be7a3de0cba8162077dc8d67046ec75cf41f059750dd6e48f2f25fe37a5e9230` |
| `apps/admin/components/Customers.tsx` | `161e92b1649600d36d6c82362da6511d764e63d1bf91151ce02baaf9a1030e2d` |
| `apps/admin/src/features/customers/routes.ts` | `7346ff49a40742f6cfdacd2c3196fde801243599b2e74863118ec801d492995a` |

## Corrective source re-review — R1 RESOLVED

The current `ImportWizard.tsx` reports any still-pending commit via `onUnconfirmed()` **before** aborting/dropping its private form state (`:81-85`). Root wiring retains only a boolean, and later auth refusal uses sticky OR (`:62-64`), so an older UNKNOWN is not cleared by a newer no-dispatch failure. Confirmed receipt/known stale/settled refusal clear the pending command before subsequent cleanup. The existing late-result mounted/abort/hidden guard prevents stale outcome application or implicit replay. No File, mapping or command ref is lifted outside the private form.

The new notice describes a historical unconfirmed import, rather than asserting that an unrelated fresh receipt confirms it. The always-visible scope copy also says to check with the original file/settings when no completed result was seen (`import-view-copy.ts:7-9`). This closes the reviewed private-form hide/conceal/remount gap while the `ImportWizard` root remains alive. It is **not** durable recovery across destruction of the complete root, a full reload or a new task/session; no such persistence is claimed.

The actual `ImportForm` AST-hook regression source (`import-coordinator.test.mjs:13-17,24-57,60-90`) checks the original requested mapping including an empty optional field, UNKNOWN → forbidden refusal → explicit same-File receipt, and unmount of a dispatched deferred commit producing one uncertainty callback, an aborted signal and zero extra uploads/late replay. It exercises production callback/effect code with controlled React/transport substitutes; it does not simulate native chooser, real pagehide/visibility events or browser persistence. Parent supplied red/green logs; this reviewer inspected the source and ran none of those tests.

Also inspected the small new download catch fence (`ImportWizard.tsx:154,175-178`): a missing/rotated cookie unmounts the private scope rather than leaving a stale preview after the read failure. It does not resend an import. Three-locale consent/history/replay/limits/error copy and history read-only copy remain consistent with the frozen backend limits.

No additional concrete P0/P1 found after the source correction. The initial R1 is historical/resolved; the table below replaces affected initial bindings and adds the correction test/copy bindings. The other 17 initial bindings were unchanged at final readback.

| Final/corrective source file | Current SHA-256 |
| --- | --- |
| `apps/admin/components/ImportWizard.tsx` | `a2bed6c5fb961e37bfafc72a44f6752e610166b175f67f0c125c563be412a4d1` |
| `apps/admin/lib/import-view-copy.ts` | `ef9b2a6f3b2133e1e00dfbaade6a935feb9acb2c7055c3c6cbdeb55efd9cde35` |
| `tests/admin/import-coordinator.test.mjs` | `2261d3b42ec22c34f871499538e67d2aac1a9876bd82e546c8e13e4cc973d22e` |
| `apps/admin/lib/import-copy.ts` | `92388db9142fd8312962bc0ee58c461bdffead1604afc409a113e079ed6711e7` |
| `apps/admin/lib/import-history-copy.ts` | `c89236114810e7067b012a32baf0b4cec4031ef7766fe5271b83c78652bf4f37` |

## Final two-delta re-review

No new concrete P0/P1 in the two requested behavioral deltas; no full-scope audit or runtime test was repeated.

- Known first authorization refusal with `uncertain:false` and no earlier UNKNOWN now clears pending **before** `onSignedOut` unmounts the form (`ImportWizard.tsx:125-130`). Thus cleanup cannot create a false historical-unconfirmed notice. An older `p.unknown` or a post-dispatch uncertain outcome remains unconfirmed and retains the boolean; the immutable same-request/no-auto-retry rules are unchanged.
- Restart is disabled at the initial select stage (`:229`), where it would otherwise leave all state unchanged. Other meaningful restart stages retain the original busy/UNKNOWN lock. No click assertion or lint threshold is weakened.

The fourth actual ImportForm AST-hook case asserts sign-out receives false and unmount emits zero uncertainty callbacks for the definite first refusal (`import-coordinator.test.mjs:92-96`). The earlier immutable mapping, UNKNOWN→403→same-File receipt and pending-unmount cases remain. This is source inspection only; parent's red/green execution is not attributed to reviewer.

The following hashes replace the earlier bindings; the complete final set contains 22 distinct source/test/copy files, all matched at this readback. Reviewer started no long-running/background process and has no pending exec session. Browser/PG/Next/full CI remain NOT_RUN by reviewer.

| Latest changed source | SHA-256 |
| --- | --- |
| `apps/admin/components/ImportWizard.tsx` | `3ae72d7e740475049473da09fa0b2fe6c10559d3000bca523e6894a13ab61540` |
| `tests/admin/import-coordinator.test.mjs` | `c69a9e901dd97c4dfc8c2ed7d600acc3161aa64673b596e8d82c8e536027a417` |

## Real shared-error seam — R2 OPEN history, now RESOLVED in source

The preceding no-P1 statement missed the actual shared `auth.localError` cache-policy seam and was withdrawn after the parent identified it. Real `localError` emits five closed keys (`code/message/request_id/retryable/details`) with `Cache-Control: no-store` (`auth.ts:124-145`); the old import client required exact `private, no-store` for every answer, incorrectly converting known refusals into generic retry_later/commit uncertainty. Earlier mocked helpers did not expose that mismatch. Old affected source/test evidence is historical, not proof of this repair.

Production correction from wire source `059855c3`, with final tested-source checkpoint `6d61a1d4`, is accepted in the parent candidate. `import-client.ts:17-29,55-71` now accepts no-store only for a bounded, closed five-key refusal with an allowed matching status/code, canonical request ID, retryability flag and empty details; 413/415/422 invalid_request are included. Public-cache/malformed/unknown/mismatched/data-bearing errors fail closed. Successful receipts and 200/direct409 previews still require the exact private policy. The real session/cookie/hidden fences, post-dispatch UNKNOWN classification, original File/mapping/count and absence of automatic retry remain intact. `import-model.ts`, `import-proxy.ts` and shared `auth.ts` are unchanged.

The rewritten wire test imports the actual client, exported route, auth/settings helpers and model (`import-client.test.mjs:13-17`). The browser-relative network bridge calls the actual route with native `Request` (`:34-47`); only upstream fetch data and unavailable Node DOM/blob facilities are controlled. It checks real localError/no-store refusals, real CSRF/cookie/Origin guards, original bytes and trusted forwarding, private success/stale projection, malformed/public/error data rejection and UNKNOWN/cancellation/no-retry. The loader redirects only repository TS resolution and the installed Next server-only marker, not auth/localError/cookie/CSRF behavior (`import-wire-real-loader.mjs:8-15`). Worker initial setup failures are not production calibration; the separately reported isolated envelope red is the relevant R2 regression. Reviewer executed neither red nor green.

History test-only checkpoint `afefed5e` is also accepted: exactly the BFF/client tests and two-line loader resolution change. Actual history route/auth/backend/settings/model are dynamically imported; real helper-issued session/CSRF cookies are used. The tests intercept only HTTP edges, call the real GET with native Requests and preserve real no-store 401/403/404/503 classifications, cookie expiry, scope/query refusals, private success, post-read session fence and rejected tainted-city/extra-field data (`import-history-bff.test.mjs:14-16,24-51,54-107`; client test `:15-18,26-55,57-92`). `next/headers` resolves to the installed `next/headers.js`, with no helper replacement (`import-history-test-loader.mjs:11-12`). The network-only mutation is meaningful actual-route/model coverage; source inspection is not execution. No product/history/CSS/auth change is made by this test-only delta.

No remaining concrete P0/P1 found in this narrow repaired boundary/test seam. Parent reports finished fresh focused49, full Node, strict TS and gate/hash checks; those executions belong to the parent, not this reviewer. Real signed browser/Native WebKit/Next/PG import/replay acceptance remains NOT_RUN by reviewer and separate from controlled Node seams. Node DOM bridges do not prove native browser cookie-jar/chooser timing.

The history worker's older CSS hash `87b0e1fc…` is an intentionally superseded child checkpoint after the parent token rewrite. Current CSS is bound below at `1a5c8b64…`; old worker CSS evidence cannot certify it, and actual CSS/browser390 verification remains NOT_RUN here.

The following bindings replace affected older entries and add the exact shared helper/test seam/current CSS. The complete current report has 29 distinct bindings; older repeated rows are historical. Parent owns the broader final 47-file manifest. Reviewer has no pending/background process or exec session and started no tests. Append permission is returned to the parent after this handoff.

| Latest real-helper seam file | SHA-256 |
| --- | --- |
| `apps/admin/lib/import-client.ts` | `82b73b1641ceecc2561800cd68eb9ebb52a359512e1ba9404b320bc3ee020050` |
| `tests/admin/import-client.test.mjs` | `b033cff7e733c4ada4865a7729a6337c97b263c5dfe7f1e6745c109dcca65307` |
| `tests/admin/import-wire-real-loader.mjs` | `145439face274a8271a7f5c4759f345642f5eb073cb2e863c4a864df228cfd94` |
| `tests/admin/import-history-bff.test.mjs` | `a9410e5984c9d66ec6383cdcd7996c83a67e9a0e5d4a029b02e6aeab650a55e5` |
| `tests/admin/import-history-client.test.mjs` | `490106b956176cafe64f9b1c4daf61cea72f1c53a56bf85b8c57b7b3d836be84` |
| `tests/admin/import-history-test-loader.mjs` | `60d51c21ec4a9b6f8c97833f994469c75628c1613f3c8bd916e649da3ce36117` |
| `apps/admin/lib/auth.ts` | `0ad0e3af716c76a702f513083aec27eb3c5eebd61614ab513a276aa254cf0126` |
| `apps/admin/components/import-history.css` | `1a5c8b645b0d11e785091414e7bf60d3640bf9031653669743a9f9840b5d81ba` |

## Interim shared-helper seam finding — OPEN (2026-10-07)

Root merged originba1d1579 PROCESS and confirmed sendImport rejects actual auth.localError Cache-Control:no-store before coded-error classification. The prior hand-written auth/localError test helper returned private,no-store. Earlier no-remaining-P0/P1 wording is historical and does not approve this boundary. Narrow correction and actual route/Request/shared-helper red-green tests are pending; affected hashes/checks will be rebound. Independent reviewer was notified and agrees the earlier verdict does not cover this seam. No tests or thresholds have been weakened.
