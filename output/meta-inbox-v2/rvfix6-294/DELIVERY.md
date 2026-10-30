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
