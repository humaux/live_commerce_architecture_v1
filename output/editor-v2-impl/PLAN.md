# EDITOR-V2-IMPL

Status: RED preparation; implementation NOT_RUN. Environment: MOCK only. Author assertions: E0.

Assignment: one UI card, branch `unit/editor-v2-ui`, worktree
`/Volumes/data/live_commerce_architecture_v1/.worktrees/editor-v2-ui`, PR base `r3/integration`.
Assigned base `aa4c5eb65724fe14afa7e10a9aaa9cf140a1517f`; fetched/fast-forwarded integration
to `d0e7331525fec046fbe9361906fa66367ae583c6` before adding a browser spec.
Role: dispatched UI implementation worker; runtime identifies Codex/GPT-6, exact model/effort UNKNOWN.
No delegation, production actions, local tests or servers.

## Authority and audit

Owner's current card authorizes the signed SIMPLIFY design. Sole visual authority:
`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/DELIVERY.md`,
SIMPLIFY section (lines 545 onward), SHA256
`b5a6e8da0df5befe676c516424b8c6ab7bf4015fef55b5998330e00b93be2a14`.
Its earlier fix-3 ruling additionally requires publish → view → back → cancel edit to retain the
server's live version and prevent publication of already-published changes.

Read repository AGENTS.md, 架构.md §§0,1,2,26,27, contracts/storefront-v2.md §B,
the existing Design component family, draft/publication clients/models and browser harness.
Existing UI opens all profile fields and five tabs. Signed UI has four task rows, mutually exclusive
disclosures, separate domain status, and compact review/results. Existing request identity, session
fence, CAS, media validation, preview token handling and unsaved guard remain authoritative.
No new API, setting, migration, transaction rule or readiness check.

Humaux search/get/canvas/coord/enumerate/code-index tools are unavailable in this runtime.
Therefore live ownership, rejected decisions and Humaux persistence are UNKNOWN, not fabricated.
This assignment provides the branch/worktree ownership; local git/rg supply code structure only.

## Exact planned write paths (declared before source edits)

|Path|Responsibility|
|---|---|
|`apps/admin/app/[locale]/design/page.tsx`|Existing route header only if CI requires it; no route/API shape change|
|`apps/admin/components/Design.tsx`|Task hub, editor navigation, save/cancel, review/results and current server versions|
|`apps/admin/components/DesignProfile.tsx`|Name/colour first; mutually exclusive optional groups|
|`apps/admin/components/DesignSections.tsx`|Name disclosure, secondary actions, five existing section types|
|`apps/admin/components/DesignNav.tsx`|Link disclosure, header/footer, secondary actions|
|`apps/admin/components/DesignPages.tsx`|One page at a time, title/slug/body and page actions|
|`apps/admin/components/DesignMedia.tsx`|Existing image picker presentation, if needed; existing validation preserved|
|`apps/admin/components/DesignField.tsx`|Field/header alignment if needed|
|`apps/admin/components/DesignSummary.tsx`|Read-only whole-document review from current draft responses|
|`apps/admin/components/DesignDomains.tsx`|Read-only existing domain responses, existing settings link|
|`apps/admin/components/design.css`|Signed responsive spacing, task rows and disclosure states using existing tokens|
|`apps/admin/lib/design-copy.ts`|Existing three-locale labels aligned with signed table; error coverage preserved|
|`packages/i18n/src/editor.ts`|Signed editor copy in zh-TW, zh-CN and en|
|`packages/i18n/src/index.ts`|Export editor copy and dependency header|
|`tests/admin/editor-v2.spec.ts`|New real-click MOCK browser acceptance, all card items in one RED push|
|`tests/admin/design.spec.ts`|Adapt existing click journeys to signed navigation without removing assertions|
|`tests/admin/oc/design-publish-pending.spec.ts`|Adapt existing recovery journeys to signed navigation; preserve original-key/body assertions|
|`tests/admin/timezone-ui.test.mjs`|Every new Design import injected into existing compiler harness, timestamp assertion preserved|
|`playwright.config.ts`|Add new spec to existing store-design suite; unchanged timeouts, retries and other specs|
|`output/ci-select-backend-browser/browser-ownership.json`|Regenerate with derive-browser-ownership.mjs, never hand-edit|
|`docs/engineering/dependency-map.md`|Regenerate via depmap.sh only if CI forces drift|
|`output/editor-v2-impl/**`|Plan, design readbacks, RED receipt, DELIVERY and handoff evidence|

Any further necessary in-module path will be listed in the PR body; prohibited shared/backend paths stop work.
Existing gate-adapter assertions, baseline.json, route-table-dispatchers.json and dependency locks untouched.

## Acceptance and sequence

1. One tests-only commit and PR push: baseline must reach the existing design page and fail new UI
   assertions, never a missing import/component or compilation failure. CI mode `--browser-design`.
2. Record exact RED head/run/job and assertion excerpt in red.log. Do not implement before causal RED.
3. One implementation push containing all UI, copy and harness adaptations plus generated ownership.
4. PR's own Foundation and Gates must pass at implementation head; independent review remains integrator-owned.
   Never merge or enable auto-merge. No evidence-only push after GREEN; use a PR comment.

New browser coverage: six locale/viewport hubs; home/profile/pages/navigation real edits with save and
refresh; preview save ordering; publish review/all four data groups/empty page; success→view→back→
cancel edit current version; definite refusal; uncertain same-identity recovery; system/custom domains;
desktop/mobile required-field parity; default and expanded density with screenshots and click ledger.
Mock response state survives reload in the fixture, and the pre-existing real Go/PG browser journeys
remain the separate persistence authority. User content is never translated or invented by UI code.

No local tests/servers. Allowed static checks only; ownership generation is expressly required by card.
CI monitoring limit about ten minutes: register Foundation/Gates workflow run IDs and hand off with WAIT_RUN
if unfinished. Integrator relays the completion in a fresh session. No silent background process.
