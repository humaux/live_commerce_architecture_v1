FIX1 replaces the previous controls and copy with the signed inbox/editor behavior. It implements the signed hub/preview/publication states and permits another DM after confirmed delivery within the existing server-authorized window. The shared inbox login helper opens the real mobile navigation menu.

## Evidence

- RED on this PR's tests-only head `deffd208590b89b7b1b794e06bc43f90b6f67ecb`: Gates **41057051192**, design job **124085434705** (14 failed/49 passed), inbox job **124085433146** (18 failed/70 passed). Native browser artifacts **12983057038** / **12980848918**. Every failing test/assertion is classified in `output/PARITY-INBOX-EDITOR/FIX1.md`; raw browser logs are retained there under `ci-red/`.
- Merged `origin/r3/integration` **2f61bf86673b3ebf4acbd3ee0ecacd54d37cf46f** before editing. No class(c) assertion is claimed as identical on base. Owner-provided base Gates 41078151280 success is context only.
- E1 only: node syntax checks on 9 changed .ts files; browser ownership generation + --check; depmap generation (no diff); check_packet; git diff --check all exit 0. No local tests/servers/browser/PG/build. Local tsc unavailable; TSX compilation belongs to CI.
- Final-head Foundation/Gates, all runtime click ledgers, density and independent screenshot comparison are **PENDING**. Exact new run IDs will be posted in a PR comment. Author verdict E0; observed RED is E3 failing evidence, never completion. One batched implementation push; integrator relays the CI conclusion. No merge or auto-merge.

## Current implementation relay

Foundation **41091867692** / job **124187532727** at implementation head `f18027d3e6debc949ed818e80d2636a1f96f1412` failed 12 login-harness cases with `ReferenceError: enterMessages is not defined`. The AST harness omitted the new shared-helper dependency. This relay extracts the real helper and models its native menu/visibility methods; all existing auth/store assertions stay, expanded to desktop and collapsed-mobile rails. No product code or fixtures change in this relay. Full per-case class(b) table: `output/PARITY-INBOX-EDITOR/FIX1.md`; filtered native receipt: `foundation-helper-failure.log`.

Same-module scope extension needing integrator review: `tests/admin/inbox-browser-source.test.ts`. Regenerated browser ownership is pre-approved. Syntax and ownership checks exit 0 (E1); new-head Foundation/Gates, browser parity and independent review remain PENDING. New run IDs will be posted in a PR comment. No local tests or servers; no merge or auto-merge.

## Behavior and retained coverage

- Inbox keeps the four signed chips and thread refresh; removes unsigned list refresh/overflow. Confirmed sent DM reopens a clean composer, retaining its receipt. No change to permission, capability, deadline, UNKNOWN, unresolved outbound, original key/body or private-reply quota checks.
- Editor has four direct task rows, separate Domains, compact signed copy with actual server versions, and a response-bound draft preview status alongside the existing token popup. No token is rendered or stored. The existing live-version text opens the retained history/rollback view, replacing the extra hub history row without adding a setting.
- Old navigation assertions migrate to signed controls: all SDB history/rollback/source/timezone/reload checks remain; exact clean version status replaces the removed saved badge; native chip clicks retain list re-reads, channel inclusion/exclusion, unread and bundle checks. The old sent-folding expectation now explicitly requires the follow-up composer. Queued/failed/blocked expectations and every adversarial obligation/UNKNOWN assertion remain. No tests deleted/skipped, fixtures rewritten or thresholds/timeouts/budgets/exemptions changed.
- Preview's existing browser tests now return via Back before publishing. Status-query helper uses the existing thread Refresh after confirmed sending reopens the composer. Component imports unchanged, so no injected harness allowlist changes.
- zh-TW/zh-CN/en entries are present for every new template, with literal three-locale hub assertions retained. UI process, required desktop/mobile facts and copy delta are in DELIVERY.md. No Figma write; fresh read-only hub/review context retained locally. Independent parity remains pending.

## Scope inventory for integrator review

FIX1 expressly authorizes root product fixes and migration of obsolete signed-frame assertions. Extensions beyond the original two-spec tests-only PR (same inbox/editor modules):

- `apps/admin/components/Design.tsx`, `Inbox.tsx`, `InboxThread.tsx`, `design.css`.
- `apps/admin/lib/design-copy.ts`; required module dictionary `packages/i18n/src/editor.ts`.
- `tests/admin/design.spec.ts`, `inbox-browser-support.ts`, `inbox-bundle-ui.spec.ts`, `inbox-ui.spec.ts`, `inbox-unknown.test.ts`.
- Original `tests/admin/editor-v2.spec.ts` / `inbox-v2-ui.spec.ts` helpers migrate real navigation. Existing tests-only exception `tests/admin/inbox-discovery.test.ts` remains from the earlier relay.
- Generated `output/ci-select-backend-browser/browser-ownership.json` is pre-approved. `scripts/dev/depmap.sh` produced no diff. Task receipts use the existing case-preserved `output/PARITY-INBOX-EDITOR/` directory (the requested lowercase path resolves there on this volume).

No migrations, contracts/OpenAPI, Go modules, lockfiles, dispatcher/baseline lists or gate-adapter assertions changed. Humaux tools unavailable: no store/canvas/coord/code-index update claimed; integrator must attach this handoff to the existing task.

DEFERRED → **INBOX-PREVIEW**: list-row decrypted last-message preview only, per prior integrator ruling; no API field or fake preview is introduced. All other signed coverage remains required.

## Signed frame artifact map

The existing 75-frame map is retained. Physical artifact names encode `:` as `-` for GitHub upload compatibility; attachment labels retain signed node IDs. Browser screenshots and comparison are pending the new exact-head CI.

|Frame|Real-route browser artifact|Current DELIVERY PNG|
|---|---|---|
|1429:37361|`parity/1429-37361.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-37361-1793201291248-0.png`|
|1429:37452|`parity/1429-37452.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-37452-1793204688746-0.png`|
|1429:37543|`parity/1429-37543.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-37543-1793204688888-0.png`|
|1429:37634|`parity/1429-37634.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-37634-1793204689022-0.png`|
|1429:37725|`parity/1429-37725.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-37725-1793204689206-0.png`|
|1429:37816|`parity/1429-37816.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-37816-1793201292448-0.png`|
|1429:37907|`parity/1429-37907.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-37907-1793201292673-0.png`|
|1429:37998|`parity/1429-37998.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-37998-1793201292910-0.png`|
|1429:38090|`parity/1429-38090.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38090-1793201293146-0.png`|
|1429:38182|`parity/1429-38182.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38182-1793201293392-0.png`|
|1429:38274|`parity/1429-38274.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38274-1793201293638-0.png`|
|1429:38366|`parity/1429-38366.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38366-1793201293854-0.png`|
|1429:38458|`parity/1429-38458.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38458-1793201294258-0.png`|
|1429:38550|`parity/1429-38550.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38550-1793201294486-0.png`|
|1429:38642|`parity/1429-38642.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38642-1793201294691-0.png`|
|1429:38826|`parity/1429-38826.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38826-1793201294846-0.png`|
|1429:38876|`parity/1429-38876.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-38876-1793204689348-0.png`|
|1429:38926|`parity/1429-38926.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-38926-1793204689433-0.png`|
|1429:38976|`parity/1429-38976.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-38976-1793204689530-0.png`|
|1429:39026|`parity/1429-39026.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39026-1793201295525-0.png`|
|1429:39076|`parity/1429-39076.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39076-1793201295683-0.png`|
|1429:39114|`parity/1429-39114.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39114-1793201295839-0.png`|
|1429:39152|`parity/1429-39152.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39152-1793201296040-0.png`|
|1429:39190|`parity/1429-39190.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39190-1793201296229-0.png`|
|1429:39229|`parity/1429-39229.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39229-1793201296395-0.png`|
|1429:39268|`parity/1429-39268.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39268-1793201296548-0.png`|
|1429:39307|`parity/1429-39307.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39307-1793201296722-0.png`|
|1429:39346|`parity/1429-39346.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39346-1793201296894-0.png`|
|1429:39385|`parity/1429-39385.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39385-1793201297061-0.png`|
|1429:39424|`parity/1429-39424.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39424-1793201297244-0.png`|
|1429:39463|`parity/1429-39463.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39463-1793201297398-0.png`|
|1429:40688|`parity/1429-40688.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-40688-1793201297634-0.png`|
|1429:40780|`parity/1429-40780.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-40780-1793201297872-0.png`|
|1429:40872|`parity/1429-40872.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-40872-1793201298299-0.png`|
|1429:41042|`parity/1429-41042.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-41042-1793201298555-0.png`|
|1429:41081|`parity/1429-41081.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-41081-1793201298753-0.png`|
|1429:41120|`parity/1429-41120.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-41120-1793201298915-0.png`|
|1429:58696|`parity/1429-58696.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-58696-1793201299400-0.png`|
|1429:58822|`parity/1429-58822.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-58822-1793201299608-0.png`|
|1429:61092|`parity/1429-61092.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-61092-1793201299842-0.png`|
|1429:61183|`parity/1429-61183.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-61183-1793201300071-0.png`|
|1429:61312|`parity/1429-61312.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-61312-1793201300219-0.png`|
|1429:61350|`parity/1429-61350.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-61350-1793201300370-0.png`|
|1429:63114|`parity/1429-63114.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-63114-1793196701844-0.png`|
|1429:63170|`parity/1429-63170.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-63170-1793196703459-0.png`|
|1429:64602|`parity/1429-64602.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64602-1793196709928-0.png`|
|1429:64684|`parity/1429-64684.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64684-1793196710010-0.png`|
|1429:64758|`parity/1429-64758.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64758-1793196710144-0.png`|
|1429:64840|`parity/1429-64840.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64840-1793196710221-0.png`|
|1429:64914|`parity/1429-64914.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64914-1793197063522-0.png`|
|1429:65008|`parity/1429-65008.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65008-1793197062030-0.png`|
|1429:65094|`parity/1429-65094.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65094-1793196710551-0.png`|
|1429:65151|`parity/1429-65151.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65151-1793196710628-0.png`|
|1429:65280|`parity/1429-65280.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65280-1793196710747-0.png`|
|1429:65337|`parity/1429-65337.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65337-1793196710822-0.png`|
|1429:65386|`parity/1429-65386.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65386-1793197063807-0.png`|
|1429:65477|`parity/1429-65477.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65477-1793197062635-0.png`|
|1429:65560|`parity/1429-65560.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65560-1793196711158-0.png`|
|1429:65613|`parity/1429-65613.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65613-1793196711240-0.png`|
|1429:65658|`parity/1429-65658.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65658-1793196711369-0.png`|
|1429:65714|`parity/1429-65714.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65714-1793196711450-0.png`|
|1429:65762|`parity/1429-65762.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65762-1793196711577-0.png`|
|1429:65800|`parity/1429-65800.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65800-1793197063175-0.png`|
|1429:65830|`parity/1429-65830.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65830-1793196711777-0.png`|
|1429:65868|`parity/1429-65868.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65868-1793196711855-0.png`|
|1429:65898|`parity/1429-65898.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65898-1793196711979-0.png`|
|1429:65934|`parity/1429-65934.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65934-1793196712060-0.png`|
|1429:65962|`parity/1429-65962.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65962-1793196712183-0.png`|
|1429:66000|`parity/1429-66000.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66000-1793196712266-0.png`|
|1429:66030|`parity/1429-66030.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66030-1793196712392-0.png`|
|1429:66073|`parity/1429-66073.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66073-1793196712469-0.png`|
|1429:66108|`parity/1429-66108.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66108-1793196712592-0.png`|
|1429:66154|`parity/1429-66154.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66154-1793196712669-0.png`|
|1430:68109|`parity/1430-68109.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1430-68109-1793196712796-0.png`|
|1430:68181|`parity/1430-68181.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1430-68181-1793196712876-0.png`|

🤖 Generated with Codex (dispatched by Claude Code integrator).
