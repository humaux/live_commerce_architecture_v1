# RVFIX4 #294 — independently retain every observed outbound obligation

## Current CI continuation — 2026-10-30

The RED-stage statements below describe the previous tests-only handoff. Current authority is GitHub head `ae8a41fe37db6067571eea94a7267ee87c9306b6`, not that handoff. Fresh fetch/merge of integration `17328372dea91220050e9583259312e2db2e5d97` exited 0, already up to date. Read every failed job: Foundation `40929100216 / 123642440286` has 217 Inbox pass / 18 new assertion failures at `inbox-unknown.test.ts:207`; Gates `40929100369 / 123645010286` has six new native-click failures at `inbox-v2-ui.spec.ts:383`, 34 passed. Downloaded `gate-browser-inbox` artifact `12923022670` and inspected its original playwright.log. Receipt job `123659542487` and required job `123659727521` propagate Inbox failure. Both workflow head APIs match tests-only head; tested synthetic merge is `dae2426493bbd92628972abd8c7127a42b24b300`. No unrelated gate or apt/dpkg/mirror failure found. Causal RED is E3; implementation GREEN and independent acceptance remain PENDING.

Implementation attempt 1 changes only `apps/admin/components/InboxThread.tsx`: replace the single projected-row latch with a collection of unresolved projection identities and their states. Reconcile every retained record separately against explicit matching sent/failed evidence, preserve omitted records, and observe older queued/blocked records below a newer successful row. Remaining obligations take precedence over the latest terminal display row. Null-timestamp and conflicting matches fail closed. Existing exact local ACK reconciliation, refused/missing operation authority, original-text recovery, UNKNOWN latch, guards and privacy resets remain. No test, import, layout, copy, setting, API or dependency change. Current and captured handlers keep reading the same synchronous delivery fence. This is an implementation hypothesis until its own CI passes.

Static evidence: `node --check` on the two unchanged regression files and `git diff --check` exited 0. Required static browser-ownership generator exited 0 (53 modes), with no artifact diff because component-only edits are not discovery inputs. Local TypeScript compiler is absent, so component typecheck/compile is NOT_RUN locally; CI owns it. No Go dependency edge changed, so depmap generation is unnecessary. No new outside-card path this continuation. Preserve pre-existing rvfix3 changes and untracked handoffs; preserve the queued-run history in red.log. Source hashes and command receipts are in static.log. Raw downloaded browser artifacts remain local evidence and are not committed.

Humaux is available in this continuation: searched current card context, read development details `7bdb220b-0798-4b1d-aff5-16ee94cae75f`, checked rejected coverage (3/9 representative records; not a complete inventory), read focused code map, and stored the fresh RED record. Integrator retains coord ownership; no duplicate claim or delegation. Own child canvas `integrator-inbox-v2-impl-sub-rvfix4-ci` updated under an acquired resource lock. The previous unavailable-tool statement below is historical. No new visual production; the four UI-process sections and existing three-locale click ledger below still apply. Native click/runtime persistence/density execution for this implementation is PENDING; no author PASS is promoted above E0.

Acceptance: exact implementation-head PR Foundation plus Gates GREEN, followed by integrator independent review. Push once, report Foundation/Gates workflow IDs within ten minutes, and stop; integrator relays completion. Rollback point is tests-only head `ae8a41fe37db6067571eea94a7267ee87c9306b6`. No local tests/server/browser/build/PG/container/LIVE, no merge/automerge/status/deployment. No owned background process or temporary fixture. Preserve failed CI artifacts; no evidence-only push after GREEN.

claim_type: problem · project: live_commerce_architecture_v1 · module: inbox · date: 2026-10-30

## Authority, ownership and rollback

Dispatched RVFIX card for PR #294; integrator retains ownership and completion notification. Worktree `.worktrees/inbox-v2-ui`, branch `unit/inbox-v2-ui`, base/head before this RED: `ada3500f51adf8ee924ba02c1ad93258b6ed758d`. Role: dispatched implementation worker; runtime model variant/effort UNKNOWN, no delegation. Current source and GitHub receipts determine status; older output documents supply process provenance only.

Read repo AGENTS.md, architecture §§0/1/2/10/26/27, live-console-v1 §§3.3/4.4/11 (A9/A12 and history-authority amendment), InboxThread load/send/render and direct regression consumers. Independent P1 original: integrator log `output/integrator/tools/logs/cxf-rv-294.log`, final REQUEST_CHANGES at the above SHA. Fetch and merge of origin/r3/integration (`17328372dea91220050e9583259312e2db2e5d97`) reported already up to date.

Root: one projected-row identity and latest-row selection cannot represent multiple independently unresolved operations. Resolving A discards B on the same refresh; a newer sent B can also hide older unresolved A on entry. False UNKNOWN history authority does not settle queued/blocked sends. Required behavior: retain all observed unresolved obligations across omission until explicit matching settlement; exact local ACK reconciliation remains separate.

RED stage only: add regressions before implementation. Existing product code is unchanged. Rollback point: pre-RED SHA above; isolated tests-only commit can be reverted by the integrator. Pre-existing dirty `rvfix3-294/DELIVERY.md` and untracked handoffs are preserved and excluded. No destructive operation or production action.

Humaux search/get/rejected/store/canvas/index/link/coord are unavailable in this session (tool inventory has no Humaux tools). No shared-state write or claim is fabricated. Read AGENTS-provided skill catalog and ponytail SKILL.md; reused existing hook/browser harnesses, no new dependency, tool or helper script.

## Regression and acceptance gate

18 new Node cases: zh-TW/zh-CN/en × queued/blocked × (A-to-B handoff, older pending under sent on entry, two observed then one omitted). Each checks folded form, disabled Send, correct seller notice, current and captured submit handlers, zero POSTs, omission retention, reappearance and explicit all-settled recovery. No operation ID is invented for another staff member's projection. These are MOCK component regressions, not browser evidence.

Extend the existing six native-click completed-ACK cases (no discovery names/count changes): additionally reload into projection-only A, click thread Refresh to observe A sent plus pending B, omit/reintroduce B, reload from visible server facts, click Refresh to observe B sent and fill a new draft. Also reload with two pending rows, settle B while omitting A, and reload into older A beneath newer sent B. Assert visible lock/result, hidden composer, disabled Send and unchanged POST count. Fixture setup changes transport facts only; no DOM mutation, force click, dispatchEvent or API substitute for UI acceptance.

Gate: PR-own CI Foundation must reach the new behavioral assertions for causal RED; then one implementation batch followed by exact-head Foundation/Gates GREEN and independent review. Original assertions, timeouts, fixtures, exemptions, discovery counts and next-startup gate parity remain intact. Implementation, GREEN and independent acceptance PENDING. Author assessment E0; syntax only E1 after checks. No E3 completion claim before receipts.

## BOURDIEU

Q1 novice/expert means recent experience with this reply/status task. Q2 preserve choose buyer → reply or check status. Q3 another staff member's completed send cannot erase pending work. Q4 no invented identity/delivery. Q5 identical safe behavior and existing seller strings across zh-TW/zh-CN/en. Q6 aesthetics separate; no new owner verdict inferred, no visual change.

## PROJECT UNDERSTANDING

Architecture §§0/1/2 require distinct evidence environments and TypeScript UI consuming server facts. §10 requires channel/window eligibility and human takeover safety. §§26/27 require isolated ownership and independent acceptance. Live-console-v1 §3.3 leaves planning/check permission/window rules intact; §4.4 defines queued/sent/failed/blocked/unknown; §11 A9 omission/history authority cannot prove individual delivery, A12 ACK owns an exact operation. No new setting, API, locale string or backend state.

## MOBBIN → EAGLE

No new visual pattern, reference, asset or import for this behavioral repair. Existing signed layout/reference provenance remains in `../DELIVERY.md`; no live Mobbin/Eagle lookup or import claimed. No needed pattern query added.

## FIGMA OFFICIAL GUIDANCE

No Figma/design write, component/variable/prototype change or new visual delivery. Preserve the signed component/layout and seller-copy behavior. Existing guide provenance is `../DELIVERY.md` G1 auto layout, G2 variables, G3 components, G4/G5 prototyping; project guide absent in this worktree, so no fresh guide read or upstream re-verification claimed. New Figma export, copy lint, desktop/mobile fact and copy re-audit NOT_RUN for this behavior-only repair.

## Scope, evidence and handoff

Changed tests: `tests/admin/inbox-unknown.test.ts` (outside initial path list, existing in-module direct consumer allowed by scope pre-approval), `tests/admin/inbox-v2-ui.spec.ts` (in card). Generated ownership artifact is regenerated last, never hand-edited. Evidence under this directory is in card. Product implementation NOT_YET_WRITTEN; no migration, R2 pin, contract, shared package, lockfile, next-startup, exemption or baseline edits. Dependency-map regeneration unnecessary: no source/import/dependency change.

Click ledger: new rows emitted by `record()` when the existing CI browser cases execute. Current runtime actual/exit code PENDING; no click PASS claimed. Local node syntax and whitespace receipts plus file hashes are in `static.log`; CI receipts follow in `red.log`/PR comment. No tests, servers, browsers, containers or PG run locally. No background process, downloaded fixture or temporary worktree created.

After pushing, record Foundation/Gates workflow run IDs and hand off within ten minutes. Integrator relays completion and wakes the continuation; no silent local polling task. Do not implement until the new RED reaches its own behavioral assertions. Stop for unrelated gates or two targeted failures on the same blocker. No merge/auto-merge/status write.
