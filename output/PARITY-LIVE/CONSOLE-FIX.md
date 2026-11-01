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

## Registered implementation push (post-push local record; also PR comment)

Head7085179ec9af8e8b698972ba767a79e2c2fff69f. WAIT_RUN Foundation41281883816/job124775288450 queued; Gates41281883852 queued, actual browser jobs not yet created. Both headSha values verified. No evidence-only commit/push. PR body152 frame mappings updated; remote CI has timeout/conclusion and integrator fresh-session relay is the wakeup route. No local watcher or task-owned process remains.

## RELAY 2 — own-head failure readback and targeted repair 1

Authority: owner's RELAY 2 / CONSOLE-FIX card; current PR head verified as 7085179ec9af8e8b698972ba767a79e2c2fff69f. `gh pr checks 355`, `gh run view 41281883852 --log-failed --job 124776275936` and the completed job logs were read. Foundation41281883816/job124775288450 SUCCESS; original Console41281883852/job124776275824 SUCCESS; bundle:07/job124776276159 SUCCESS. Parity/job124776275936 FAILURE; receipts124784300783 and required124784354654 fail downstream. These are E3 observations of that exact head, not overall GREEN.

Evidence upload is repaired: artifact13074046827 (1063849 bytes), ZIP SHA25664219958de6d7e52648695f77e15944e789fe164900a68059bf628bd50f3cfee, uploaded successfully with hyphenated screenshot names. It contains 12 frame PNGs and nine error contexts from the first 21-case shard (12 passed / 9 failed); the shell stops after that failed shard, so remaining seven shards are NOT_RUN. Downloaded evidence is local only under console-fix-relay2/, excluded from the commit. Its playwright.log SHA2566264372470713e40ce265d4c0c1d3ca7b27b7fb190b970d3d71f89a7e69b951e.

All nine errors are existing exact-name textbox assertions at spec:967/1002: expected textbox named 回覆內容, element not found. They cover sent, pending and sold states on desktop/mobile. Source root: CommentReply's label wraps c.text, textarea AND the dynamic length counter; that counter participates in the implicit accessible name. The signed field name is therefore not stable as text changes. Repair: add only `aria-label={c.text}` to the textarea, reusing all three existing locale translations. This is an accessibility fix in the same Console module, not a selector/assertion/fixture relaxation. Existing exact-name assertions are the RED; all remain byte-for-byte unchanged. Visible copy, fields, controls, state, permissions, unknown fences, timers, viewport and screenshots remain unchanged. Mobile CSS keeps the sticky composer visible across all three tabs. No added imports; inspected comment-stream-hooks harness needs no allowlist change. Root-cause assessment is author E0 until own repair-head CI.

Fetched origin/r3/integration4b8c04c7213025c173e260db6f9c81cb5b818c61 and merged first: already up to date. Rollback point7085179ec9af8e8b698972ba767a79e2c2fff69f. Scope extension: apps/admin/components/CommentReply.tsx (same-module pre-approved); list in PR body. Generated browser ownership regenerated and --check exit0, byte-identical; no generated diff. Packet structure exit0 E1; timestamp-only packet-check.json restored to pre-run HEAD. git diff --check exit0. No Go dependency change, so dependency map unchanged. No local tests/servers or runtime verification. Impeccable runtime context/detector scripts omitted under owner's explicit local-check restriction; harden/craft-floor references read.

bundle:07 remains class(c): no authored returns/order/auth changes, no causal path identified. Base and head returns-ui.spec.ts SHA25614bd880a95d1fa9898fefd158f9a66d96cbae1b1d97016a110585d6d3135d67f match. Current own-head bundle passes; do not claim this PR repaired a former auth failure.

Humaux became available after initial discovery: task-specific search, own canvas/coord reads, code map/query and original development detail7bdb220b-0798-4b1d-aff5-16ee94cae75f read; rejected sample2/9 (coverage0.2222, not exhaustive). Dispatched-child exemption: no new claim or delegation. Store/canvas/index receipt follows in PR comment. Existing RULING-FIX edits and untracked prior evidence preserved. No source mutation/test servers/background processes beyond this bounded card.

NOT_RUN: repair-head CI GREEN; full161 parity and screenshot review/density; independent E4; local tests/servers/tsc; SANDBOX/LIVE/deployment. Integrator relays a fresh session when registered Gates/Foundation conclude; no evidence-only push after GREEN.

## RELAY 2 registered push (local receipt; no evidence-only commit)

Repair1 head ecae57fa90e839339e5f9b67655613205b0c240a pushed to unit/parity-live; PR355 still targets r3/integration. WAIT_RUN Foundation41285438042/job124785656569 pending; WAIT_RUN Gates41285438087 in_progress (browser jobs not created at registration). Both runs' headSha values verified, no plan job used as acceptance. PR body updated with CommentReply.tsx same-module scope extension;152 frame mappings retain safe screenshot names. Humaux fix c4701383-0670-4f73-b50a-c2900d296e2d stored and linked to indexed CommentReply; own sub-canvas updated. No new local tests/servers/processes. Pre-existing RULING-FIX edits and untracked evidence remain untouched. Integrator fresh-session relay is the wakeup route; no CI polling beyond registration.


## RELAY 3 — own-head fixture lifecycle failure; targeted repair

Authority: current RELAY 3 / CONSOLE-FIX instructions and own-head CI raw receipts. Read AGENTS.md, 架构.md §§0/1/2/26/27, read-only MAIN diagnosis and parity-common.md. `gh pr checks 355` and `gh run view 41285438087 --job 124786481042 --log-failed` confirm head ecae57fa90e839339e5f9b67655613205b0c240a. Foundation41285438042/job124785656569 SUCCESS; Console/job124786480696 SUCCESS; bundle:07/job124786481003 SUCCESS. Parity/job124786481042 FAILURE; receipts124789810057 and required124789866407 fail downstream. E3 observations of that exact head; overall GREEN absent.

Parity shard1 passed all21 tests (33.29s Go test). Artifact13075545605 (1997988 bytes) uploaded successfully, including safe-name parity/1429-39502.png and playwright.log SHA256 54d8633aa76b05be97e1f3a0fde8d10980598952a3d5c74b84fe3c71b976e32f; the log reads `21 passed`. Shard2 failed in0.23s at browser_live_console_test.go:370 during fixture initialization: `create runtime login: ERROR: role "foundation_api" already exists (SQLSTATE 42710)`. Its browser assertions and shards3–8 are NOT_RUN. This is infrastructure failure, not a new assertion RED; the original tests-only assertion RED remains f483929e / Gates41277939407 / job124764486910.

Static causal chain: #355 introduced eight independent go-test processes in scripts/dev/test-local.sh's parity arm, all against one mode-level PG cluster. newFixture creates foundation_api once per process; TestMain closes pools but does not remove cluster roles. The shared fixture file is unchanged from integration (both Git blob855f267825bf5eeb2cacb383aded884252f22aaa). The PR's new caller owns this lifecycle mismatch. Reuse the existing fresh_pg helper before shards2–8, as the existing Stripe/WebKit modes already do. No shared fixture/backend/contract change, role reuse/drop workaround, removed test, fixture weakening, timeout/threshold change or arbitrary retry. The seven resets affect only the disposable, label-verified mode-owned CI container; its existing EXIT cleanup and lock remain in force. No local fixture was started.

Fetched and merged origin/r3/integration4b8c04c7213025c173e260db6f9c81cb5b818c61 first: already up to date. Rollback ecae57fa90e839339e5f9b67655613205b0c240a. Scope extension for integrator review: scripts/dev/test-local.sh, only #355's own parity arm (two added lines); output/ci-select-backend-browser/browser-ownership.json regenerated with the registry hash. All gate-adapter assertions remain untouched. No Go graph change, so depmap is unchanged.

Static E1: bash -n scripts/dev/test-local.sh exit0; ownership regeneration then --check exit0 (56 modes); python3 scripts/check_packet.py exit0 PASS_PACKET_STRUCTURE_ONLY; git diff --check exit0. Packet-check's timestamp-only output restored; pre-existing RULING-FIX edits and untracked evidence retained/excluded. No UI source, imports, visible strings, screenshot naming, frame mappings or click helpers changed this relay. No new UI/design claim; existing design/copy/process sections and all assertions remain required.

bundle:07 remains class(c), unchanged: returns spec and harness are identical to integration (Git blobs7b7826dc726075f2237ad20385257ff7c398ee25 / e644991fde5feb6872834fbeeaf6c2e914f62276), and this own-head bundle passes. No evidence that this PR repaired the earlier session-swap failure.

Humaux/coord/code-index tools are absent in this session; shared search/store/canvas/index NOT_RUN. Existing integrator assignment retained; no new claim/delegation or substitute local memory. Installed ponytail/impeccable instructions read; no design work or local runtime helpers run. Existing failure evidence preserved locally and excluded from commits.

Acceptance: own repair-head Foundation/Gates all green, all8 shards and161 cases (152 signed frames plus9 controls/interaction cases), safe screenshot upload and independent visual/click acceptance. Existing CI RED at shard2 is the comparator for this lifecycle repair. Author root-cause verdict E0; static E1. NOT_RUN: repair-head CI GREEN, remaining parity cases/full screenshot and density review, E4, all local tests/servers, SANDBOX/LIVE/deployment, Humaux shared writes. New run IDs are registered in a PR comment after the single implementation push; integrator fresh-session relay is wakeup, job timeout/conclusion bounds remote waiting. No local watcher/process/port/container started.

## RELAY 3 registered implementation push (local receipt; no additional commit)

Head42d318e28c1ec909f6e46450b75822a233907a2e pushed to unit/parity-live; PR355 targets r3/integration. WAIT_RUN Gates41287198479 in_progress (browser jobs not yet created) and Foundation41287198492/job124790794893 in_progress at readback; both headSha values verified. No plan job substituted for test evidence. PR body lists the parity-runner scope extension and retains152 safe-name frame mappings. Integrator fresh-session relay is wakeup after workflow conclusion; no further polling/local process. No shared-memory write or completion claim.

STATUS: WAIT_GREEN — 42d318e28c1ec909f6e46450b75822a233907a2e; WAIT_RUN Gates41287198479 / Foundation41287198492. AuthorE0 / staticE1; full CI acceptance pending.


## RELAY 4 — signed public-reply label; targeted repair 1 on this blocker

📜 Authority: current RELAY4 card and PR355 current-head CI, AGENTS.md / 架构.md §§0/1/2/26/27, MAIN read-only diagnosis and parity-common.md, contracts/live-console-v1.md §7.2. Current v3 UX-LIVE handoff and FIGMA-INVENTORY identify the frame family. Reports under output are leads only; the diagnosis below uses raw CI and current source/frame readback.

📋 Assignment: integrator-dispatched CONSOLE-FIX, existing unit/parity-live worktree only, no separate claim or delegation. Humaux/coord/code tools absent: search/store/canvas/lock/index NOT_RUN; no local/platform memory substitute. 🧩 ponytail and impeccable harden/craft-floor read; optional runtime skill scripts omitted under the owner's static-only local restriction.

↩️ Rollback point42d318e28c1ec909f6e46450b75822a233907a2e. Fetched and merged origin/r3/integration c52b65283a5ac738d0c527a464d0cccc9523b1f4 first, merge31478e230a5d318f30f57796dafbf72434b8a933. Incoming contracts/migration/pins/other modules belong to integration, not authored changes. Prior RULING-FIX.md edits and all untracked evidence remain excluded. No other worktree/branch touched.

E3 failure observations at42d318e28: Foundation41287198492/job124790794893 SUCCESS; Gates41287198479 original Console124791707826 and bundle:07/job124791707701 SUCCESS. Parity124791708132 FAILURE; receipts124797025364 and required124797255864 fail downstream. First shard21 passed; second shard17 passed/3 failed; six later shards NOT_RUN. The fresh-PG fixture fix now reaches browser assertions in shard2; no repeated role error observed. Artifact13076252624 uploaded3739351 bytes /38 safe-name frame PNGs; ZIP SHA2563655184e529d25ae1782ee633378e9c29f4fe02c551dfa4e2ef35ff978f55dc9. Shard2 playwright.log SHA2565eae1e73d8d461d99a2a8858a0c8d67b011a0ebd3f622e5ecbbbcf5d678bdb2c.

All3 failures are publicSent frames1429:42205/42310/42362: real locator.click waits for exact button name「送出公開回覆」until the unchanged110000ms test timeout. apps/admin/src/features/live/comment-copy.ts instead supplies「送出回覆」. CommentReply uses c.publicSend directly. This is a product copy mismatch in #355's own locale module; do not alter the test oracle. Original tests-only visibility assertion RED remains f483929e / Gates41277939407 / Console124764486910. This later exact-name click failure is a regression comparator, not a newly pushed tests-only assertion RED.

Fleet Bridge figma_status returned one connected document; get_metadata depth8 read desktop1429:42507 and mobile1429:42886 separately (134490/61148 bytes; each <200KB). Visible descendants I1429:42605;1429:30527;4:12713 and I1429:42916;1429:30527;4:12713 both read「送出公開回覆」. Readback SHA256 desktop9e146e507819d48a7784a946b0204b657591ff029e20830fdd4e2a853ca98b6e; mobilec0fb06b003b78e7a75249a2f2fe94f7ea753d3fb14b29ea9c2ff5eaf23d2ade5. No Figma write or restart. Full raw metadata/archive retained locally, excluded from commits per evidence retention.

Repair: one zh-TW publicSend value aligned to that signed copy; DELIVERY three-language row aligned. zh-CN/en entries already exist and remain「发送回复」/「Send reply」; main-screen three-language browser cases remain required. No new locale key/import/component/handler/permission change, no harness allowlist change. Existing Bourdieu Q1–Q6, project rules, Mobbin→Eagle, Figma official guidance, required-facts/mobile-summary parity and three-language sections remain in DELIVERY. No redesign, new fact/control, or new visual/click acceptance claim. Every browser assertion, helper call, timeout, threshold, fixture and exemption stays unchanged; gate-adapter assertions unchanged.

bundle:07 remains class(c): current integration and HEAD share returns-ui.spec.ts blob7b7826dc726075f2237ad20385257ff7c398ee25 and browser_returns_ui_test.go blob e644991fde5feb6872834fbeeaf6c2e914f62276. No authored returns/order/auth path or identified causal reach; current own-head bundle passes. No claim that this PR repaired that earlier session-swap defect or that a base-only reproduction was run.

Static E1: node scripts/dev/derive-browser-ownership.mjs then --check exit0 (56 modes, generated file byte-identical after merge); python3 scripts/check_packet.py exit0 (structure only; exact pre-run packet-check output bytes restored); git diff --check exit0. No import/dependency change, so depmap is unchanged. No tsc executable available. No local tests/servers/browser/PG/container/install started. Author fix verdict E0 until own-head CI.

Scope extension for integrator review: apps/admin/src/features/live/comment-copy.ts, same Console module already present in PR; only1 value. Task evidence CONSOLE-FIX.md/DELIVERY.md/red.log updated. Generated browser ownership has no new diff. Inherited merge changes are not authored scope. No forbidden baseline/dispatcher/contract/OpenAPI/migration/pin/lockfile edit.

Acceptance: own repair-head Foundation/Gates GREEN, all8 shards/161 cases and safe uploads, then independent frame/click/density review. First targeted attempt for this copy blocker; prior repairs addressed distinct accessible-name and PG lifecycle defects. New run IDs go in a PR comment after one implementation push; stop polling under10min. Integrator fresh-session relay is the wakeup route, existing CI timeouts/conclusions bound waiting. No evidence-only commit after GREEN.

NOT_RUN: repair-head CI GREEN/full161; independent E4 / visual/density and complete actual click ledger; local runtime tests/servers/tsc; provider SANDBOX/LIVE/deployment; Humaux shared reads/writes/index. No completion claim.

STATUS: WAIT_PUSH — RELAY4 signed-copy repair prepared; authorE0/staticE1, CI acceptance pending.
