# PARITY-LIVE CONSOLE-FIX — implementation after assertion RED

PR #355 → r3/integration; branch unit/parity-live; worktree /Volumes/data/live_commerce_architecture_v1/.worktrees/parity-live. Integrator-dispatched UI implementer; no delegation. Runtime model/effort not exposed. Starting head f483929e1aec118f79ed9783a4ffbbdd8570c382 (the batched tests-only RED). Author verdict E0; static E1; implementation CI and independent acceptance PENDING.

## Authority and trail

Read AGENTS.md, 架构.md §§0/1/2/26/27, CONSOLE-FIX + relay1 owner card; read-only MAIN output/integrator/diag/live-console-20261101.md and output/integrator/tools/cards/parity-common.md. Business authority: primary actions/state may not be hidden by collapsed disclosures. contracts/live-console-v1.md §7.2 retains server permission, stock delta and expected_version/CAS authority; no API change. Current v3 section of ux-live/output/ux-live/DELIVERY.md and MAIN output/ui-parity/FIGMA-INVENTORY.md identify the signed family; old output status is not a CI verdict.

Fleet Bridge figma_status reported one connected document. Read get_design_context desktop1429:39502 and mobile-live1429:39782, inspected their returned PNGs. No Figma write, bridge restart, new reference or design. The signed product/lifecycle action area remains directly accessible; this patch removes only native disclosure barriers, adds no strings and retains existing controls/facts. Exact browser/frame comparison and density remain NOT_RUN pending CI. Existing DELIVERY.md contains required-field parity, Bourdieu, project understanding, Mobbin/Eagle limits, Figma guidance and the three-language copy table.

🧩 ponytail + impeccable applied to the bounded correction; local runtime setup/detector scripts not run under the owner's local-check restriction. Humaux/code-index/coord tools are absent this turn: search/store/canvas/claim NOT_RUN, no substitute local memory written. This is the integrator's existing assignment, not a separate claim. Prior stage recorded memory pointer357ed38d-e072-4232-b4cc-154edb36b2ed; not reverified here.

↩️ Rollback point f483929e1aec118f79ed9783a4ffbbdd8570c382. Fetched and merged origin/r3/integration4b8c04c7213025c173e260db6f9c81cb5b818c61 first (merge4f8cac59a); its seven frame-copy reports are inherited, not authored. Pre-existing RULING-FIX.md edits/untracked evidence preserved and excluded from this commit. No other worktree/branch modified. No merge/auto-merge/deployment from this lane.

## Confirmed RED — own tests fail on their new assertion

Tests-only head f483929e1aec118f79ed9783a4ffbbdd8570c382: Gates41277939407 / Console job124764486910 FAIL. Artifact13072770968 (gate-browser-live-console) has six error-context.md members, one for each zh-TW/zh-CN/en ×1586/390 case. All six fail the new tests/admin/live-console.spec.ts:303 assertion getByTestId('live-primary-action'): expected visible, received hidden, 10000ms. Thus this is assertion RED, not compile/infrastructure RED. Member hashes and exact assertion excerpts are in red.log. The downloaded archive is untracked local failure evidence, never committed. Its playwright.log member SHA256 is67836a26b8d9460b5743e5f1227a8d5e15f4e86808fb6151d7760c9aac4a8189.

Same-head Foundation41277939402 / job124763822904 PASS. Parity job124764486539 FAIL: upload rejected parity/1429:39502.png for Colon :. Its browser assertion remains UNKNOWN because no parity evidence artifact was uploaded. Receipts124774232755 and required124774377338 failed downstream. These are E3 failure observations at the RED head, not validation of the implementation.

## Root cause and implementation

1. LiveConsole lifecycle action/revenue/feed subtree used default-closed details.live-advanced; each OfferRow put its offer switch, price/facts, stock form, CAS feedback and recommend control inside default-closed details[data-live-tools]. Refresh remounts them closed. Replace the first wrapper with the same-class ordinary div and remove the offer disclosure wrapper/summary. All child controls, state, permissions, disabled predicates, handlers, version fences and uncertain-command behavior remain.
2. showLiveTools has an unconditional attachment assertion for disclosures. It now returns when no tools exist after the existing real-click mobile panel selection; when disclosures exist it retains attachment/open assertions and real summary clicks. Every helper call stays. No import changes, so the inspected SSR/workspace harness allowlists need no changes.
3. parityShot now writes parity/${id.replace(":", "-")}.png. IDs in ledgers stay raw. No parity assertion/viewport/threshold/timeout/fixture/exemption changed. PR body maps all152 signed IDs to hyphenated screenshot names and the existing DELIVERY exports.

## bundle:07 — class (c), unchanged

Base4b8c04c and current PR share returns-ui.spec.ts blob7b7826dc726075f2237ad20385257ff7c398ee25 and browser_returns_ui_test.go blob e644991fde5feb6872834fbeeaf6c2e914f62276. Original diagnosis: restricted-session swap renders sign-in before returns assertions; exact auth root cause UNKNOWN. The PR authors no returns/order/auth path and the console helper has no returns caller. No causal path identified; no auth weakening or speculative fix. Fresh current-head evidence improves the earlier diagnosis: Gates41277939407 bundle:07 job124764487132 PASS. This is not a base-only reproduction and not a claim that a former auth failure was fixed by this PR.

## Checks and evidence level

|Command|Exit|Meaning|
|---|---|---|
|node scripts/dev/derive-browser-ownership.mjs|0|Regenerated56-mode ownership after merge and spec/helper change|
|node scripts/dev/derive-browser-ownership.mjs --check|0|Generated artifact matches current sources|
|python3 scripts/check_packet.py|0|PASS_PACKET_STRUCTURE_ONLY; E1 only; timestamp output restored, no generated packet diff|
|git diff --check|0|Whitespace only|

No local tests, servers, browsers, PG fixtures or Go test. tsc executable/dependencies unavailable, no install attempted. No Go imports changed; dependency-map regeneration unnecessary. No change to next-startup gate-adapter assertions, frozen baselines, dispatchers, contracts, migrations, R2 pin, go.mod or lockfiles.

|Current source|SHA256 (CI pending)|
|---|---|
|`apps/admin/components/LiveConsole.tsx`|`7991ccc2640a1ada18bec6a854c0d4f59fed0c3f1a8284eb286c09dc5aae4bd3`|
|`tests/admin/fixtures/live-console-controls.ts`|`9bd67dd503b2d75a2570b7fa2517672cce56ece378f61f36756c00c97cad544d`|
|`tests/admin/live-console.spec.ts`|`1d7fae39d1f3d6f64b461bf36bcf56098c767e2b1c245beba099b9fbdccbff00`|
|`output/ci-select-backend-browser/browser-ownership.json`|`5448fea15c2fffb073f8e85fa78434cd5ffb7b37127ce51678fb330db8661640`|

## Scope, gates and handoff

This implementation authors only LiveConsole.tsx, tests/admin/fixtures/live-console-controls.ts, tests/admin/live-console.spec.ts, regenerated output/ci-select-backend-browser/browser-ownership.json and task evidence CONSOLE-FIX.md/DELIVERY.md/red.log. The helper is a same-module scope extension explicitly permitted by CONSOLE-FIX; list for integrator review. Earlier PR source extensions remain listed in the PR body. No downloaded artifacts, traces, PNGs or bulk captures staged.

Acceptance gate: #355's OWN implementation-head Foundation/Gates checks, six new initial-visibility and post-reload stock23/offer-off assertions, all existing click/permission/CAS assertions and161 parity cases, with uploaded safe-name screenshots. Required checks and independent review must pass before integrator acceptance. At preparation, GREEN run/job IDs PENDING; register after push in a PR comment, never an evidence-only commit. Poll under10min; integrator fresh-session relay is the wakeup route. CI job timeouts/conclusions bound remote work. No local background process/watcher/fixture/container/port started.

NOT_RUN: implementation CI GREEN, all161 results/upload recovery, current implementation actual click ledger/density/parity review, independent E4, local runtime tests/servers, local tsc, SANDBOX/LIVE/deployment, Humaux store/canvas/coord/code index. No product-completion claim.

STATUS: WAIT_GREEN — implementation prepared after confirmed assertion RED; own CI and independent review pending.
