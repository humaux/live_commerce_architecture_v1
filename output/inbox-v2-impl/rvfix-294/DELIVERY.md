# RVFIX #294 — P1 display-window omission

Status: tests-only RED push prepared; implementation NOT_STARTED; author E0. No local tests or servers.

## Authority, ownership and rollback

Dispatched by the integrator on `unit/inbox-v2-ui`, in the assigned worktree only. Read `AGENTS.md`, `架构.md` §§0/1/2/10/26/27, `contracts/live-console-v1.md` §§3.3/3.6/4.4/11, original `PLAN.md`, current A9 producer/DTO/Thread consumers and the final findings of `cxf-rv-294.log`. Review head: `ae59f693e54f391b6e1d7753d6db94dad09b446a`. Merged `origin/r3/integration` at `e324e01a93219c099d6d2a9d35ed1f112595fe90` before edits; merge had no conflicts. Incoming integration files were not manually edited. No delegation, other worktree writes, merge to integration, auto-merge, deployment or statuses. Revert this lane's isolated repair commits through the integrator if rollback becomes necessary; no rollback executed.

P1 root: first-page load replaces delivery and deliveryLock from the displayed latest outbound even when A9 omits the unresolved operation because 50 newer inbound messages moved the display floor. Explicit false UNKNOWN authority does not establish delivery of that operation. Regression requires the observed queued/blocked result and no-fresh-send lock to survive repeated inbound-only refreshes.

## RED gate and click ledger

- Node: six new cases (queued/blocked × zh-TW/zh-CN/en) begin with an enabled captured form, observe the unresolved operation, then read inbound-only pages with explicit false UNKNOWN authority twice. Assert disabled sending, folded composer, retained result, no fresh receipt, and no POST from captured/current handlers.
- Browser: extend the existing three-locale state cases with queued and blocked omission fixtures, real Check status clicks twice, visible retained result, disabled sending and unchanged POST count. Existing reload/persistence, confirmed-sent transition, privacy and density checks remain.
- Click ledger (execution PENDING): messages → Check status → inbound-only page → original result retained/send disabled → second Check status → same visible result/no POST. MOCK fixture setup only; no DOM mutation, forced click or direct API substitutes for acceptance.
- CI must fail on these new assertions before implementation. Exact RED run/job/log goes in `red.log` after the integrator relays completion. Current GREEN is PENDING; older PR green does not cover this counterexample.
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

Humaux tool discovery returned no search/grep/get/enumerate/store/canvas/coord tools. Shared store/canvas/code index/link NOT_RUN; no local/platform memory accessed. This file and the PR relay preserve the handoff until tools return. Role: dispatched UI worker; model GPT-6, effort not exposed. No local process/fixture/temp directory started; evidence retained in the assigned output subtree. Integrator owns resumption when WAIT_RUN finishes; no unattended local process or polling loop remains.

NOT_RUN: local tests/build/tsc/browser/server/PG/container/LIVE; RED and GREEN conclusions pending; independent review pending; Humaux store/canvas/index/link unavailable. No completion claim.
