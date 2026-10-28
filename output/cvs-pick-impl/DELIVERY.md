# CVS-PICK-IMPL — RED batch, implementation pending

2026-10-29. **E1 syntax only; no runtime acceptance. Author judgment E0.**
Card: integrator-dispatched CVS-PICK-IMPL including REAL-CASE AMENDMENTS A1–A5.
Role: implementation worker; runtime identifies Codex / GPT-6, exact model and effort UNKNOWN.
Branch `unit/cvs-pick-impl`; base `2bc6d482606a2b0583af15789086fdad727b27ec`.
Only this assigned worktree was written. No delegation, production actions or merges.

This first commit supplies all requested RED test families. The implementation push must wait for CI to show the new assertion failures; CI identifiers go in `red.log` and the PR comment. No local tests, browsers, builds or servers were started.

## Authority and immutable inputs

- Current human implementation card is the requirement authority; no backend route, migration, lockfile or contract edit is authorized.
- Repository `AGENTS.md`, `架构.md` §§0/1/2/26/27, `contracts/taiwan-cvs-logistics-v1.md` §16.1 and CF-LOG amendment, `contracts/buyer-destination-v1.md` govern server authority, namespace strings, no per-query API and explicit confirmation.
- Picker signed source: `.worktrees/cvs-design/output/cvs-design/DELIVERY.md`, v2–v5, SHA256 `76abe1ad413654b57b4115ae5b998a848b45c38803da81f8cb78c39e3be9f011`.
- Matcher signed source: `.worktrees/ux-cvs-match/output/ux-cvs-match/DELIVERY.md`, MATCHING SPEC / parity / locale / wiring and fix2, SHA256 `908b4c506fd4f1d1c0a65037ec62f058ea82e511418117fab97f5803e01f3b76`. Fix2 states are desktop/mobile `1431:71405`/`71485` (R1), `1431:71443`/`71523` (R7); the delivery explicitly retains buyer matching semantics without adding buyer-specific frames.
- Authoritative data: `internal/fulfillment/cvsdata/stores-20261028.json`, supplied by #262; no freshness re-verification performed.
- Sanitized real-case fixture copied byte-for-byte from integrator's `output/integrator/tools/cards/cvs-match-real-cases.json`; SHA256 `0b9688b8d7b1fe4ed34162d504d8388ebe548949b7882355c5ef194b3987aa53`. No buyer identity/phone introduced. Existing browser synthetic recipient fixture retained.
- R7's **source** address contains full-width `６９號Ｂ１`; the design's half-width illustration is only a comparison key. Acceptance requires the source string verbatim on the candidate and server summary.

## UI PROCESS 1 — BOURDIEU

Applied by reference to the two signed deliveries, not a new design or aesthetic proposal.

|Question|Inherited decision and acceptance|
|---|---|
|Q1 Language/task|Buyer selects a pickup store, then explicitly confirms. Seller lookup/copy actions stay outside this card.|
|Q2 Recent experience|Novice/expert means recent store-finding experience, never class/device; search and city/district are both supported.|
|Q3 Consequence|Store choice affects destination; no query, paste, ranking or Enter action commits automatically.|
|Q4 Memory burden|Brand, canonical store name, string code and full address remain together through list/selection/summary.|
|Q5 Recovery/control|Keep no-result query; change clears choice; R7 shows missing code and separate candidate; close discards an unconfirmed choice.|
|Q6 Aesthetics|Use signed picker/matcher identity. Any unapproved aesthetic changes UNKNOWN; none authored in this RED commit.|

## UI PROCESS 2 — PROJECT UNDERSTANDING

|Read authority|Rule applied to this screen|
|---|---|
|架构 §0|Static/syntax proof cannot establish browser or production acceptance.|
|§1|Existing TypeScript strict/React/Next frontend and Go backend; no new service/package.|
|§2|Directory selection changes no merchant/payment/tenant boundary.|
|§26|One assigned worktree; no recursive delegation; author cannot provide independent review.|
|§27|One bounded card; CI acceptance followed by integrator handoff.|
|CVS §16.1 / CF-LOG|Client static search; final existing service/code validation through ensure and buyer BFF.|
|Buyer destination contract|Namespace-scoped string codes and server identity; client name/address is never authority.|
|Picker DELIVERY v2–v5|Two paths, mobile sheet, fixed confirmation, full recipient/contact in checkout summary, 422/429 recovery with no automatic retry.|
|Matcher DELIVERY fix2 / A1–A5|Selected-brand lookup, paste parsing, F-prefix, name affix comparison, code-miss candidates and explicit choice.|

## UI PROCESS 3 — MOBBIN → EAGLE

Inherited evidence: matcher DELIVERY's `fix2/eagle-readback.json`, module folder `MVQL9YPJVZ5FT` (18 items at design readback). This implementation lane has not re-imported or re-verified assets; no claim of a fresh Eagle read. Reference principles, never images/brands/store facts:

|Eagle item|Structural principle inherited|
|---|---|
|MVNM2RJ3M6VBW / Thrive|Selected name/code/address grouped before commitment.|
|MVOVHC45BTJIX / Selfridges|Preserve list-to-summary hierarchy on mobile.|
|MVRSAQWBW1O4A / ZARA|Address entry near choices, with alternate regional browsing.|
|MVRSAR4W5O6C1 / Subway|One row groups name and complete address.|
|MVNM2RJ5ULKUL / Ulta|Explicit tap and visible change action.|

No additional reference query is required for implementing the signed scope. No merchant/admin lookup or clipboard feature is added.

## UI PROCESS 4 — FIGMA OFFICIAL GUIDANCE

Applied by reference to both signed deliveries and `output/ux-audit-ref/FIGMA-GUIDE.md` (design retrieval 2026-10-28). No claim of a current online-document review or new Figma writes.

- G1 auto layout: preserve vertical hierarchy, mobile scrolling list and fixed confirm/footer.
- G2 variables: reuse semantic theme tokens; no arbitrary new visual system.
- G3 components/variants: maintain list, selected, pending, refused and cooldown states consistently.
- G4/G5 prototyping: implement the signed navigation as real controls; changing query/brand invalidates choice and no action navigates to itself.
- G6/G7: test actual UI, parity, locales and click ledger; static frames alone do not constitute acceptance.

## Design parity checklist — implementation/CI pending

|Required field/state|Signed desktop/mobile authority|RED assertion / remaining verification|Current|
|---|---|---|---|
|Brand + source date|Picker v2 two paths; matcher fix2 parity|UNIMART/FAMI controls, exact footer in all three languages|NOT_RUN|
|Name + string code + full address|Picker lists/selected/summary; matcher A/B|Exact 久旺; choose 建安 B instead of 樂隆 A; server summary full address|NOT_RUN|
|R1 paste exact|fix2 71405/71485|One unselected 231196, explicit click/confirm|NOT_RUN|
|R7 code-miss and candidate together|fix2 71443/71523|022305 notice + 024967 row, no automatic replacement, preserved paste|NOT_RUN|
|Recipient name + phone|Picker v5 14914/15634|Existing real-click recipient/error/checkout cases retained|NOT_RUN|
|Totals/payment timing|Picker checkout-summary states|Existing map/pay-at-pickup order assertions retained; implementation visual parity pending|NOT_RUN|
|Region path|Picker 13961/14020 through 14624/14688|Brand → 臺北市 → 大安區 → row, no backend write|NOT_RUN|
|Empty / exact-only miss|Matcher matching spec|Query preserved, no candidate/preselection, confirm disabled|NOT_RUN|
|Change / close / refresh|Signed wiring ledgers|A→B, query-change invalidation, close, same persisted pickup after refresh|NOT_RUN|
|Mobile density|Signed mobile frames + integrator density probe|Six measured mobile states; upper-middle paired median ≤12 blocks/100 chars; max ≤20|NOT_RUN|
|Three languages|Matcher three-language copy table + picker labels|Main screen/title/input/confirmation/source and paste miss in zh-TW/zh-CN/en|NOT_RUN|
|422 / 429 / pending|Picker refused/busy states|Existing server refusal assertions retained; implementation UI state verification still required|NOT_RUN|

## Tests, static checks and CI handoff

- `packages/format/tests/cvs-match.test.ts`: pure matcher contract, all real R1–R8, exact-only miss, chain formats, NFKC/臺/台/address numeric forms, affix stripping, label order, ignored unrelated public-store line, deterministic ≤8 list, source-string preservation and no mutation.
- `apps/storefront/tests/cvs-picker.test.mjs`: registers the matcher through Foundation's existing glob; generator existence + all-brand byte-identical committed JSON; only five UI fields, full record counts, canonical strings, region parsing/fallback.
- `tests/storefront/cvs-picker.mjs`: added to existing real browser runner; exact code/paste, explicit selection POST, no per-key API, fuzzy B≠A, query retention, county/district, close, refresh, paste miss and locales. Emits click/density ledgers only after successful assertions.
- `tests/storefront/cvs-buyer.mjs`: additive import and cases; **all pre-existing assertions remain verbatim and in order**. Implementation must preserve these server-boundary proofs while updating interaction where required; never delete or weaken a test to make it green.
- `rg -l 'CvsPickup' apps/storefront/tests tests` found source readers `tests/foundation/taiwan_cvs_guards_test.go` and `tests/ci/pr-modes.test.mjs`; no current component compilation harness found. Recheck at implementation import changes.
- Local `node --check` for both new `.mjs` files, new matcher `.ts` test and existing browser runner: exit 0. `git diff --check`: exit 0. These are syntax/whitespace checks only (E1).
- Browser ownership regenerated using `node scripts/dev/derive-browser-ownership.mjs` after merging latest `origin/r3/integration`; 53 modes. Generated file is the only current path outside the write allowlist, explicitly pre-approved as generated output and listed in the PR body. No Go changes; dependency map unchanged.
- RED run/job identifiers: `red.log` after push; pending at commit creation. Expected new assertion failures: matcher missing and static generator missing. Browser also lacks picker controls. Failure attribution must come from the raw CI logs, never this expectation.
- GREEN: NOT_RUN, no implementation push before confirmed RED.

NOT_RUN: every local test/build/server/browser; CI execution at this commit's creation; implementation, generated assets, three-locale source entries, full runtime UI parity, independent review, SANDBOX/LIVE and production. No standing process or local fixture was created. CI continuation/wakeup is owned by the dispatching integrator per the current card; return Foundation/Gates run IDs and end the session rather than polling longer than ten minutes.
