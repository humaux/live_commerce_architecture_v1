# RVFIX6 / PR294 — independent B rejection after successful local A

Status: tests-only RED preparation; author runtime E0. Implementation NOT_STARTED, causal RED PENDING.
Role: assigned UI worker; Codex GPT-6 family, exact variant/effort unavailable. No delegation/duplicate claim.
Worktree: inbox-v2-ui; branch unit/inbox-v2-ui; PR294 targets r3/integration.
Base1772b71a1b84b5188e6eeb0fcb69241b305dd712. Authorized upstream merge a09262050d750895347e7333b8228194fec00cb2 ->18eed2963.
Existing dirty/untracked evidence preserved and excluded from staging. No PR merge/automerge/status/deploy.

## Authority and root

Read AGENTS.md; architecture §§0/1/2/26/27; docs/delivery/units/meta-inbox-v2.md;
contracts/live-console-v1.md A9/A12; current InboxThread/load/render/recovery and direct unit/native-click harnesses.
Specified cxf-rv-294.log final finding at1772b71a1: completed local A overwrites visible state on every refresh;
when independent B settles failed its obligation disappears but retained A still owns result/recovery selection.
No new API, backend or contract is needed. Historical output evidence is not current acceptance.
Humaux developer rules read7bdb220b-0798-4b1d-aff5-16ee94cae75f; rejected representative enumeration3/9, not exhaustive.

## RED batch and acceptance gate

Add six unit and six native-click cases: zh-TW/zh-CN/en × queued/blocked B -> failed, after actual local A=SUCCEEDED.
A/B have distinct authoritative outbound IDs and different originals. No inbound follows B.
Repeated native status clicks must show B rejection and Edit original must restore B text, with one POST only.
Reload/reopen then native Edit original must retain B failure and recovery text. Existing assertions are unchanged.
Fixture adds laterInbound=true default; only these new cases suppress that later buyer message.
No imports added/renamed; component harness allowlists unchanged. No source behavior changed in this RED batch.
PR-own Foundation, --browser-inbox, click sweep/visual/type gates and integrator independent acceptance are required.
Static node --check/diff checks are E1 only; all runtime/CI claims stay PENDING until exact-SHA receipts.

| Page/control | Operation | Expected | Actual |
| --- | --- | --- | --- |
| Thread/reply | fill + Send | A acknowledged once | CI PENDING |
| Thread/status | click after A success and B pending/failure twice | B failure replaces A success | CI PENDING |
| Thread/Edit original | click | B text, no resend | CI PENDING |
| Thread/status/recovery after reload | reload, click row, Edit original | persisted B failure + B text | CI PENDING |

## BOURDIEU

Q1 novice/expert means recent experience checking/recovering messages, never class/device.
Q2 task is recognize failed reply and edit the correct text. Q3 no operational-code knowledge needed.
Q4 server outbound IDs distinguish staff sends; success of A grants no authority over B.
Q5 identical safety/recovery in all locales and mobile layout. Q6 aesthetics UNKNOWN until owner; no design change.

## PROJECT UNDERSTANDING

Architecture §§0/1/2 distinguish MOCK/LIVE and server truth; §§26/27 require isolated writes and independent acceptance.
A9/A12 provide exact outbound identity and authoritative operation readback. Unit brief keeps recipient fixed,
UNKNOWN fail-closed and three locales. Owner dispatch mandates GitHub-only causal RED and no PR merge.
This tests-only correction adds no setting, visible string, route, API or design decision.

## MOBBIN → EAGLE

Existing approved inbox pattern only; no new structural principle, visual material or imported asset.
No Mobbin/Eagle access claimed. Needed Mobbin query: none for this behavior-only regression.

## FIGMA OFFICIAL GUIDANCE

No Figma write or layout/component/variable/prototype change. No new guide/lint/design acceptance claimed.
The existing guide entry output/ux-audit-ref/FIGMA-GUIDE.md is unchanged; no design deliverable in this card.

## Scope and relay

Card paths: tests/admin/inbox-unknown.test.ts, tests/admin/inbox-v2-ui.spec.ts, this output/meta-inbox-v2 directory.
Outside card: packages/i18n/src/index.ts merge-conflict header union only; preserve catalog/editor/inbox dependencies.
Mandatory generated output/ci-select-backend-browser/browser-ownership.json regenerated last; no hand edits.
Upstream merge carries integrator-owned changes, not new lane edits. No Go dependency edge, so depmap unchanged.
Rollback reference1772b71a1; no destructive restore. Native run IDs go to PR comment after push.
Integrator completion relay wakes next session; stop within ~10min CI polling, no silent background work.
P2 for issue: no new P2 in this review; existing timestamp display-order P2 remains separately recorded.
NOT_RUN: local tests/server/browser/PG/container/LIVE/tsc, causal RED, implementation GREEN, independent acceptance.

## Implementation relay after causal RED (2026-10-30)

Current authority: integrator's IMPL AFTER CAUSAL RED dispatch; repository source at
0f13bd50fe9928c1c52e6862eff44c16e2b44a6c; docs/delivery/units/meta-inbox-v2.md;
contracts/live-console-v1.md A9/A12 and nullable delivery guard. Architecture sections listed above reread.
Humaux/coord/code-index tools unavailable in this session: lookup/store/canvas/index/link NOT_RUN.
This continues the assigned card without delegation or claiming another task. Existing unrelated dirty files preserved.

Root cause: each exact-operation read repainted successful A before projection selection, while Edit original
always preferred A's retained draft. Retain A's operation/outbound identity and draft for exact reconciliation;
select a distinct projected result after A succeeds and keep a separate private recoveryText for that result.
Rejected A still takes priority after independent obligations settle. Independent queued/blocked tracking,
UNKNOWN latch, authorization, send guards, copy, assertions, fixture data and timings remain unchanged.
Recovery text clears with conversation erasure and each new successful submission receipt.
No imports changed; the three direct InboxThread harnesses therefore need no new dependency allowlist.

RED: Foundation40983874335/job123830148198 and Gates40983874479/bundle:06 job123831384255;
exact RED head0f13bd50f, six locale/state failures each. Foundation raw assertions read in this session;
browser artifact six failures/52 passes is the integrator's independent receipt, not a local browser run.
See red.log for provenance and the explicitly ignored shard:g10 P2 pg_restore flake.
Additional known GREEN blocker: inbox-discovery.test.ts:72 expects33 but discovers39 after this card's six
new browser cases. Left unchanged under the explicit no-assertion-edit rule and relayed for integrator disposition.

No new design, visible string, setting, page, route or API. BOURDIEU/PROJECT UNDERSTANDING/MOBBIN→EAGLE/
FIGMA OFFICIAL GUIDANCE sections above continue to apply to this behavior-only fix. All three locale
entries and existing parity facts are unchanged. Native click/reload recovery acceptance remains CI PENDING;
the click ledger above is not marked PASS. No new Mobbin query or Figma work is needed.

Pulled assigned branch --ff-only; origin/r3/integration a09262050d750895347e7333b8228194fec00cb2 already
ancestor of current head (HEAD...origin/r3/integration=34/0), so no merge required this relay.
Source edit: apps/admin/components/InboxThread.tsx only. Evidence stays in this directory.
Browser ownership must be regenerated last and checked before the single implementation push; no hand edits.
No Go dependency edge changed; dependency-map.md unchanged. Static command exits go in implementation-static.log.
Rollback reference: 0f13bd50fe9928c1c52e6862eff44c16e2b44a6c, source diff only; no destructive rollback performed.
Out-of-card paths this relay: only mandatory browser-ownership generated output if its content changes.
No merge/automerge/status/deploy. No evidence-only follow-up push. Final head/WAIT_RUNs go in PR comment;
integrator completion relay supplies the wake-up after this session, within the ten-minute CI polling bound.
Evidence: author runtime E0; local static checks E1 only; implementation GREEN and final independent review PENDING.
NOT_RUN: local tests/servers/browsers/builds/PG/containers/LIVE, tsc (compiler/dependencies absent),
current-head CI runtime acceptance, native click/persistence acceptance, independent acceptance, Humaux operations.
