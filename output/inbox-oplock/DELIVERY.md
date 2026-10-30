# INBOX-OPLOCK — implementation awaiting CI

Refs: integrator cross-vendor review of RC `c2173f5fc`, finding P1-2 (2026-10-30).

## Assignment and evidence

- Actor: Codex, dispatched by Claude integrator; no recursive delegation.
- Base: `c2173f5fcc6b4031ccbba7727c0ae088ff3996a0`.
- Branch: `unit/inbox-oplock`; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/inbox-oplock`.
- Model/effort: runtime did not expose a verified exact model/effort identifier.
- Allowed module: inbox component and its named browser/Node assertions, card evidence, generated browser ownership.
- Current stage: causal RED confirmed from RELAY 1 and raw CI log; one root fix implemented in InboxThread.tsx. Author assessment is E0; GREEN and independent acceptance remain pending.
- Static syntax checks: `node --check tests/admin/inbox-v2-ui.spec.ts` and `node --check tests/admin/inbox-unknown.test.ts`, exit 0 (E1 only).
- CI receipts: `red.log`; RED head `dac568eb556b3c7d0d0f71fcd2960b2978c784c1`, Foundation run `41017289344`, job `123936404324`, Node unit step exit 1. Own-row sent form-unlock and sent/failed status assertions fail causally. Relay 1 supplied the completed job and the conditional authorization to implement.
- Local tests, servers, browsers, Go/PG, LIVE effects, independent review, GREEN: NOT_RUN.
- Humaux lookup/store/canvas/coord unavailable in this runtime; this file is a worktree handoff, not a substitute shared-state update.

## Acceptance gate

The PR's own GitHub Gates/Foundation must first demonstrate that own A9 sent evidence fails the migrated unlock assertion on the unchanged component. After integrator confirmation, one component fix must make the same assertions green. With integration:read the exact-operation assertions remain unchanged. Without it, only the ACK's outbound_id and terminal sent/failed ledger row with has_unknown_outbound exactly false may settle the ACK. No reconciliation may create a second POST.

Browser collection titles/count remain unchanged: the existing unpermitted case covers sent/failed through Check status and thread refresh, missing/queued/blocked own rows, an unrelated sent row, rejected-text editing, and reload after sent. Node cases additionally cover unknown and true/absent/null/malformed history authority, current/captured submit callbacks, and all three locales. Discovery assertions are unchanged.

## Click ledger plan

Actual results are PENDING CI; the existing browser ledger writer records each scenario.

| Page | Control/action | Expected |
| --- | --- | --- |
| Messages, each locale | Fill draft; click Send | Exactly one POST, queued display lock |
| Selected thread | Check status / Refresh with own row missing, queued, blocked | Composer folded; Send disabled; zero operation reads |
| Selected thread | Check status / Refresh with own sent row | Empty next composer usable after a new draft; one POST |
| Selected thread | Check status / Refresh with own failed row; Edit original text | ACK's own draft restored; no POST |
| Selected thread | Reload after sent; reopen; fill | Server facts still permit next reply; one POST |

## Project understanding

Read AGENTS.md and 架构.md §0, §1, §2, §10, §26, §27: preserve evidence boundaries, server authority, message eligibility, worktree ownership and independent acceptance. Read contracts/live-console-v1.md:640: pending/unknown never authorizes another send; A9 history authority and A4/A5/A12 server checks remain controlling. The current card is authority for the narrowly authorized unpermitted oracle migration. No grants, migrations, contracts or server checks change.

## Bourdieu

Q1–Q5: NOT_ASSESSED; this behavioral correction preserves the existing screen and changes no information hierarchy or settings. Novice/expert refers to experience replying in this inbox, never class or device. Q6 aesthetics: UNKNOWN until owner; no aesthetic claim or new design.

## Mobbin → Eagle

No new visual pattern or reference used; no Mobbin query or Eagle import is required for this existing-composer correction. External design research NOT_RUN.

## Figma official guidance

No Figma change, frame, component, variable or prototype is delivered; no Figma guidance application is claimed. Figma tools/copy lint/density measurement NOT_RUN (no visual change).

## Field parity and copy

No new desktop/mobile frame or summary/review state. Existing browser assertions exercise the same composer/status/edit controls at 390px in zh-TW, zh-CN and en. No new visible product strings; the existing three-locale copy remains the oracle.

| zh-TW | zh-CN | en |
| --- | --- | --- |
| 查詢狀態 | 查询状态 | Check status |
| 重新整理 | 刷新 | Refresh |
| 編輯原文 | 编辑原文 | Edit original text |
| 已送出，未代表已讀。 | 已发送，不代表已读。 | Sent does not mean read. |
| 傳送遭拒；原文保留，可編輯。 | 发送被拒；原文已保留，可以编辑。 | Send rejected; original text saved for editing. |

## Handoff

The root cause was requiring integration:read to settle every queued ACK, while the reply-capable default role lacks that permission. The fix adds only the unpermitted A9 branch: explicit clear history plus exactly one outbound row matching the ACK outbound_id and terminal sent/failed. Failed restores the original ACK draft through existing recovery; sent uses existing nextReply. Existing operation reads remain authoritative for permitted staff, and unresolved outbounds/UNKNOWN still block sends. No POST is added.

Local implementation typecheck NOT_RUN: tsc/dependencies are unavailable in this worktree. No imports, browser titles/counts, routes, grants, migrations, contracts, server checks or other assertions changed in this implementation push. Browser ownership was regenerated last (53 modes), and `node scripts/dev/derive-browser-ownership.mjs --check` exited 0; generated content was unchanged. Both named test files passed `node --check` (exit 0). No Go dependency graph change; depmap regeneration not required.

Await the implementation head's own Foundation/Gates run IDs via the final relay/PR comment; stop polling within ten minutes. Integrator relays completion into a fresh session. Never merge, auto-merge or deploy from this lane. Generated ownership (from RED) is the only file outside named inbox source/tests/evidence; integrator review required. Preserve failed CI evidence.

## RELAY 2 — first targeted correction

Current implementation head `b0ad9de375743a068779f9a0367906b6f8717f3e` failed Foundation run `41018599404`, job `123940483680`, in Node unit suites (exit 1; 301 tests, 295 passed, six failed). All six failures were the new own-row sent cases (three locales × status/refresh), at `inbox-unknown.test.ts:440`: the status VNode disappeared when nextReply correctly unfolded the composer. Failed recovery and conservative-lock cases passed in that job; this is not overall card acceptance.

Correction attempt 1 preserves the existing sent message label as the accessible delivery receipt for reply-only staff when nextReply opens the composer. It adds no text block, string, request, import or setting. The integration:read branch and every test assertion remain unchanged. The automated gate must now find the sent receipt and continue through the existing usable-composer and exactly-one-POST assertions. Browser click ledger results and current-head GREEN remain PENDING CI; author assessment E0. This relay ran no local tests, servers or browsers. `node --check` on both named test files and `git diff --check` exited 0 (E1 only). Typecheck remains NOT_RUN: no tsc or node_modules available. No new temporary files or processes require cleanup.


## RELAY 3 — browser scenario preparation, targeted attempt 1

Current head `a68c01171c8fde78cde096cec82d1df9c4dda506`: Foundation run `41019899283`, job `123944583174` passed. Gates run `41019899239`, bundle:06 job `123946344555` failed: Inbox browser 55 passed / 3 failed. `gh run view --log-failed --job` could not expose logs while the whole run remained in progress; the completed job logs were read through the GitHub job-logs API, and artifact `12962420154` (`gate-browser-inbox`) supplied the assertion details. Filtered raw failure evidence: `relay3-browser-failure.log`.

All three locale failures occur at `inbox-v2-ui.spec.ts:523` → `enter` → `inbox-browser-support.ts:76`, waiting for the sign-in button. The first sent/status variant finishes; the next loop iteration reuses its signed-in cookie and page transport routes. The existing login helper unconditionally expects a signed-out page, and previous mock routes would also replace the real navigation fixture it verifies.

Attempt 1 for this distinct preparation blocker clears this test page's route interceptors and authentication cookies before each variant, then uses the unchanged real signed login and installs that variant's fresh transport. Every assertion, fixture response, case title/count, wait, timeout, shared helper and product source remains unchanged. This preserves mobile clicks, status/refresh, sent reload, failed recovery and exactly-one-POST checks for all four variants in each locale. No browser spec, page or route is added/renamed/deleted; browser ownership is regenerated last because the existing spec hash changed. No integration merge is needed for this edit. No dependency-map inputs changed.

Static `node --check tests/admin/inbox-v2-ui.spec.ts` exited 0 (E1). CI execution after this repair, current-head GREEN and independent acceptance remain PENDING; author E0. No local tests, servers or browsers were run. Humaux tools became discoverable in RELAY 3; earlier unavailability entries are historical. Targeted lookup, rejected subset (4/9, coverage 0.4444), development detail `7bdb220b`, code map and shared-state lock were read/acquired; parent retains task ownership. No recursive delegation. The integrator will relay the new exact-head CI conclusions into a fresh session.

Final static ownership regeneration: 53 modes; `node scripts/dev/derive-browser-ownership.mjs --check` and `git diff --check` exited 0. Only the existing spec source hash changed in the generated artifact. Raw failure log preserved; this relay's downloaded artifact extraction was removed after retaining the failure excerpt. No task processes were started.


## RELAY 4 — browser scenario preparation, targeted attempt 2 (final)

Head `ae43f5924c5d2504571385bd79dde761e3a9c8cf`: Foundation run `41022274830`, job `123952099090` passed. Gates run `41022274820`, bundle:06 job `123953414679` failed: Inbox browser 55 passed / 3 failed. The whole Gates run was still in progress, so the completed job was read through the GitHub job-logs API. Artifact `12961943679` supplies the raw assertion excerpts in `relay4-browser-failure.log`. Run metadata binds the PR run to this head; the job log reports synthetic merge SHA `294fb9ce9493d3600aa6105530d0658a2cf80011`.

The three locale failures now occur at `inbox-browser-support.ts:78`: signed login succeeds, but `nav-group-messages` is hidden. The artifact's visible shell offers Open navigation. The previous variant left the page at 390px; the shared login helper requires visible desktop navigation. The first preparation repair cleared authentication/routes but retained that viewport.

Final targeted preparation fix restores a 1280x900 viewport before each existing signed login. Each variant still switches to 390x844 immediately after entry and before opening the thread, sending, status/refresh, rejection editing or reload acceptance. No assertion, timeout, fixture response, case title/count, shared helper or product source changes. If this preparation blocker fails again, stop and escalate; no third fix attempt. No spec, page or route is added/renamed/deleted, so the card requires regeneration of the existing ownership hash without an integration merge. No dependency-map inputs change.

Current-head GREEN and independent acceptance remain PENDING. Author E0; local tests, servers, browsers and LIVE NOT_RUN. Humaux/coord/canvas tools are not exposed in RELAY 4; lookup/store/update cannot be claimed. This worktree delivery records the handoff only. The integrator will relay the new exact-head Gates/Foundation conclusions into a fresh session. Downloaded artifact ZIP removed after preserving the raw failure excerpt; no processes were started.

RELAY 4 static E1: `node --check tests/admin/inbox-v2-ui.spec.ts`, browser ownership regeneration (53 modes), `node scripts/dev/derive-browser-ownership.mjs --check` and `git diff --check` exited 0. Only the existing spec hash changed in generated ownership; collected case titles/counts unchanged. No local test execution.
