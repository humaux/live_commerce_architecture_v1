# RVFIX #294 — P1 display-window omission

Status: causal RED VERIFIED at tests-only head fee26469; P1 implementation prepared for one push; exact implementation-head GREEN and independent acceptance PENDING. Author assessment E0. No local tests or servers.

## Authority, ownership and rollback

Dispatched by the integrator on `unit/inbox-v2-ui`, in the assigned worktree only. Read `AGENTS.md`, `架构.md` §§0/1/2/10/26/27, `contracts/live-console-v1.md` §§3.3/3.6/4.4/11, original `PLAN.md`, current A9 producer/DTO/Thread consumers and the final findings of `cxf-rv-294.log`. Review head: `ae59f693e54f391b6e1d7753d6db94dad09b446a`. Merged `origin/r3/integration` at `e324e01a93219c099d6d2a9d35ed1f112595fe90` before edits; merge had no conflicts. Incoming integration files were not manually edited. No delegation, other worktree writes, merge to integration, auto-merge, deployment or statuses. Revert this lane's isolated repair commits through the integrator if rollback becomes necessary; no rollback executed.

P1 root: first-page load replaces delivery and deliveryLock from the displayed latest outbound even when A9 omits the unresolved operation because 50 newer inbound messages moved the display floor. Explicit false UNKNOWN authority does not establish delivery of that operation. Regression requires the observed queued/blocked result and no-fresh-send lock to survive repeated inbound-only refreshes.

## RED gate and click ledger

- Node: six new cases (queued/blocked × zh-TW/zh-CN/en) begin with an enabled captured form, observe the unresolved operation, then read inbound-only pages with explicit false UNKNOWN authority twice. Assert disabled sending, folded composer, retained result, no fresh receipt, and no POST from captured/current handlers.
- Browser: extend the existing three-locale state cases with queued and blocked omission fixtures, real Check status clicks twice, visible retained result, disabled sending and unchanged POST count. Existing reload/persistence, confirmed-sent transition, privacy and density checks remain.
- Click ledger (execution PENDING): messages → Check status → inbound-only page → original result retained/send disabled → second Check status → same visible result/no POST. MOCK fixture setup only; no DOM mutation, forced click or direct API substitutes for acceptance.
- E3 causal RED: Foundation `40827000050` / job `123292220975`, exact tests-only head `fee26469298672ca2c1e7537c121f1b3abc5ee43`: six new omission assertions fail, 185 passed / 6 failed. See `red.log` and filtered original `red-assertions.log`. Current implementation GREEN remains PENDING; prior green is historical.
- Static checks: `node --check` on both changed test files, required `node scripts/dev/derive-browser-ownership.mjs`, and `git diff --check` exited 0 (E1 syntax/generation only). No local runtime verification.

## BOURDIEU

Q1 recent task experience only; Q2 preserve reply/status recovery sequence; Q3 retain unresolved result and safe status query; Q4 use existing server facts; Q5 same three-locale desktop/mobile controls, no new layout/copy; Q6 aesthetics unchanged and no new owner verdict inferred. Original design process/parity/copy table remains in `../DELIVERY.md`; this repair adds no visible string or design frame.

## PROJECT UNDERSTANDING

Architecture §§0/1/2 separate MOCK/CI from LIVE and retain Go/PostgreSQL authority; §10 separates eligibility/delivery/read/takeover; §§26/27 require isolated ownership and independent acceptance. Contract §§3.3/3.6 preserve window, permission, CAS and no-blind-resend; §4.4 keeps queued/blocked distinct; §11 states that A9 display omission is not reconciliation. Current `internal/inbox/send_read.go` implements that display floor. Review P1 and this dispatch define this repair; P2 timestamp ordering is deferred to an integrator issue as requested.

## MOBBIN → EAGLE

No new design/pattern selection or import in this behavior repair. Original signed handoff's list/thread/recovery structure remains; original reference process is documented in `../DELIVERY.md`. No new Mobbin query needed. No fresh Eagle or Figma observation claimed.

## FIGMA OFFICIAL GUIDANCE

No Figma canvas, component, variable, prototype or layout change. Original applied guidance and reference file remain documented in `../DELIVERY.md`; this repair preserves those frames and controls. Copy lint/design parity re-export NOT_RUN (no design change).

## Scope, risks and handoff

Tests-only changes: `tests/admin/inbox-unknown.test.ts`, `tests/admin/inbox-v2-ui.spec.ts`, required generated `output/ci-select-backend-browser/browser-ownership.json`, and this task evidence. The unknown-suite path and generated artifact were already declared outside the initial write list; list them again in the PR. No imports added/renamed, no new spec/page/route, no dependency-map input change, no migration/contract/lockfile/baseline/pin or exemption change.

P2 for issue: compare RFC3339 instants numerically for nextReply/latest-outbound selection; mixed fractional precision currently misorders them. Integrator to open issue; not fixed by this P1 card.

Humaux is available in relay1: directed search/get and rejected enumeration (9/9, coverage 1), code map/search, locked fix store and incremental code index used. The store is `[fix] RVFIX294 causal omission RED verified; P1 implementation pending CI`; receipt `57545b32-16d8-40e8-8ade-371fe0d2368b`. Own relay canvas and link receipts are in the final PR handoff. Parent task remains with the integrator; no new queue claim or delegation. Role: dispatched UI worker; model GPT-6, effort not exposed. No local process/fixture/temp directory started; evidence retained in the assigned output subtree. Integrator owns resumption when WAIT_RUN finishes; no unattended local process or polling loop remains.

NOT_RUN: local tests/build/tsc/browser/server/PG/container/LIVE; exact implementation-head CI GREEN/browser click ledger/density/persistence; independent review. `tsc` is not installed in this worktree. No completion claim.

## Relay 1 implementation and evidence

Before edits, fetched and merged `origin/r3/integration`; it remains `e324e01a93219c099d6d2a9d35ed1f112595fe90` and merge reported already up to date. Rollback point is tests-only `fee26469298672ca2c1e7537c121f1b3abc5ee43`; existing task handoff files were preserved.

`InboxThread` now remembers only the observed outbound projection metadata (at/kind/principal_id; no duplicate message body). An unresolved queued/blocked lock and result survive inbound-only pages or another outbound. A visible matching projection with a known result updates the lock; absent/missing-state projections cannot unlock it. The ref is cleared on the privacy reset and a new acknowledged A12. No draft persistence, new copy, layout or import was introduced. The existing six RED cases retain their original omission/captured-handler assertions and additionally cover an unrelated sent projection and the observed projection's confirmed sent result. A9 has no operation id in its frozen browser DTO: this repair uses existing projection fields, adds no backend/contract changes, and remains conservative when the observed row stays outside the display window.

`node --check` on changed Node tests and existing browser spec, required ownership regeneration (53 modes), and `git diff --check` exit 0 (E1 only). No Go graph/import change: dependency-map generation unnecessary. No harness import change. Exact source hashes are in `source-sha256.txt`. First implementation attempt for this P1 blocker; no weakened test/baseline/threshold/exemption. Additional outside-initial-list paths remain `tests/admin/inbox-unknown.test.ts` and the already-declared generated browser ownership artifact. P2 is already posted under “P2 for issue” at PR comment 6489297640; integrator owns issue creation.
