<!-- Purpose: GATE-PORT scoped implementation and evidence ledger, pending actual gates.
Depends on: shared Next startup helper, unchanged readiness callbacks and synthetic real socket collision tests.
Used by: integrator review; no READY claim until required acceptance runs finish. -->
# GATE-PORT — automatic-port implementation E3; explicit-admin P1 boundary pending

- Branch: unit/gate-port-retry. Base: `9b738e0af64ebd08c25b39f8081d46d4dc5674fb`.
- Tested source: `6dcb4afbc3d408874e6a902d78806004045b61e2`. Author Codex (host does not expose exact parent model ID); read-only helper `gpt-6.1-sol`, medium, E1 only.
- No push/deploy, product Go/SQL/UI, migrations, dependencies or locks changed. `pnpm install --offline --frozen-lockfile` exit 0.

## Root cause and remaining boundary

The listen(0) → close → Next bind gap is real. `tests/helpers/next-startup.mjs` centralizes the recovery in eleven
storefront `*-gate.mjs` files (ten `startNext`, plus promotions `startStorefront`). `tests/admin` has no `*-gate.mjs`.
Existing readiness callbacks and caller-owned child/log teardown remain intact. Automatic ports only: an owned child
must have exited before readiness and its own bounded startup log must contain EADDRINUSE. Then pick another ephemeral
port, at most **three total launches**. Caller-specified ports, live-child timeouts, non-bind errors, pre-spawn errors
and missing/unreadable collision proof retain the original failure, without retry. First log path stays unchanged;
later exclusive `wx/0600` files use `.attempt-N`; intermediate failed log is closed before bounded 64 KiB inspection.

**Original PR #29 admin failure is not closed by this automatic-only implementation.**
The publish Go fixture passes `LC_JOINT_ADMIN_PORT`, pinned in `COMMERCE_PUBLIC_ORIGIN` and IdP callback.
Therefore the task's explicit-port no-retry rule deliberately leaves that path unchanged. This fact was reported
before implementation; a single async ruling request asks whether to keep the unit boundary and assign a Go-fixture
follow-up, or authorize that extension here. No Go fixture extension was assumed. All eleven healthy gate variants
passing does not prove that the explicit-admin collision was repaired. Overall GATE-PORT/PR #29 P1 is **pending this ruling**;
automatic-port feature alone is tested E3. Do not read this handoff as blanket merge/flake-resolution approval.

Other non-`*-gate.mjs` startup copies (UI sweep/audit, standalone buyer drivers and admin runners) were found read-only,
but not changed under the named write scope. In particular, detached admin process-group and localhost-specific
startup protocols must not be replaced mechanically.

## Red → green / invariants

- `red-next-startup.log`: real occupied loopback socket makes the old single-attempt policy fail 2/4 tests.
- `green-next-startup.log`: **8/8**, including the exact old publish-gate `startNext` loaded from Git and executed
  unchanged with process/network edges redirected to a synthetic real socket binder. It fails the same forced
  handoff, proving the old failure is not a missing-helper/import stand-in. New helper recovers after one collision.
- Negatives: explicit port stays one launch; non-bind crash, alive-child readiness rejection and pre-spawn error do
  not retry; persistent collision stops after exactly three. Fixtures use no PII/secret and clean their own children/ports.
- AST parity against the base proves **all eleven** complete gate assertion lists, readiness/env loops, spawn arguments
  and environment statements unchanged. Return shapes and browser restart remain; Host/status conditions and
  `100×50ms` / `400×100ms` waits are not widened. Node test is registered in `test-node.sh`.
- Mechanical adapter syntax/whitespace diagnostics were repaired before acceptance; no failed assertion was removed.

## Actual current gates

All commands ran on the frozen source above, no edits while executing. Batch session 84115, PID 23245, strictly
serial, 2026-10-09T22:45:33Z → 22:51:20Z, **BATCH_FAILED_MODES=0**. `LC_TEST_LOCK_WAIT=14400`; existing per-mode
Go timeouts unchanged (300–1700 seconds). No second test-local mode or another task's PID/lock was touched.

| Command | Exit | Evidence |
|---|---:|---|
| `node --test --test-reporter=tap tests/ci/next-startup.test.mjs` |0|8/8, `green-next-startup.log` |
| `bash scripts/dev/test-node.sh` |0|1331/1331 across 26 summaries, fail/skip 0; `node.log`, `node-counts.log` |
| `bash scripts/dev/check-gates.sh` |0|82 documented modes, 1256 inventory, headers; `gates.log` |
| `bash scripts/dev/test-local.sh --browser-storefront-publish` |0|23 cases; `browser-storefront-publish.log` |
| `bash scripts/dev/test-local.sh --browser-manual-order` |0|10 cases; `browser-manual-order.log` |
| `bash scripts/dev/test-local.sh --browser-buyer` |0|13 cases; `browser-buyer.log` |
| `bash scripts/dev/test-local.sh --browser-order` |0|33 cases; `browser-order.log` |
| `bash scripts/dev/test-local.sh --browser-buyer-comms` |0|MOCK mailbox/edge; `browser-buyer-comms.log` |
| `bash scripts/dev/test-local.sh --browser-merchant-buyer` |0|8 cases, three locales; `browser-merchant-buyer.log` |
| `bash scripts/dev/test-local.sh --browser-catalog-media` |0|20 cases/four cells; `browser-catalog-media.log` |
| `bash scripts/dev/test-local.sh --browser-store-domains` |0|22 cases; `browser-store-domains.log` |
| `bash scripts/dev/test-local.sh --browser-promotions` |0|32 cases/four cells; `browser-promotions.log` |
| `bash scripts/dev/test-local.sh --browser-storefront` |0|real PG/Go storefront shell; `browser-storefront.log` |
| `LC_SHOP_MOCK=1 bash scripts/dev/test-local.sh --browser-storefront` |0|12 cases + R5 checks; `browser-storefront-mock.log` |

Real browser runs are BROWSER with synthetic IdP/TLS/Graph/mailbox/API fixtures, not LIVE/provider/deploy evidence.
Existing screenshots and complete browser logs are at the `evidence=` paths inside those logs under this worktree's
`output/playwright/`; no source assertion, selector or threshold was weakened. Every changed gate has a representative run.
Playwright skill was used with the repository's explicitly requested test runner; no manual-click substitute.

## NOT_RUN / handoff / cleanup

Explicit-admin collision recovery / altered Go allocation handshake: **NOT_IMPLEMENTED, pending scope ruling**.
Independent K3/current PR CI, full foundation, global sweep/audit, other startup protocols, provider SANDBOX/LIVE,
payment/refund/message mutations and production deploy: **NOT_RUN**. No unrelated platform action occurred.
Owned batch, Node and gate sessions ended; every fixture handles its owned children/FDs/ports, synthetic unit dirs removed;
failed/green evidence retained. W3-U3 remains a separate WAITING_SCOPE404 task; this does not start it early.
