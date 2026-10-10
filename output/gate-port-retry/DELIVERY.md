<!-- Purpose: GATE-PORT current author handoff, required-gate evidence and retained failures.
Depends on: shared Next startup helper, Go-owned public admin relay, actual registry and socket/browser gates.
Used by: integrator independent review and PR; does not certify production or the extra WebKit mode. -->
# GATE-PORT — READY (required acceptance E3; independent review / CI pending)

## Current delivery — 2026-10-10

- Branch `unit/gate-port-retry`; final tested source `75233f5ad0537a8eb7f1c982d0578f7200fa3f13`.
- Original base `9b738e0a`; fetched trunk PR #33 `23568c39` merged in `b8fc7880` before tests.
- Source commits: `6dcb4afb` auto-port helper; `eb4de36d` explicit admin root fix; `75233f5a` CI regression wiring.
- Author: Codex (parent model ID not exposed). Read-only supplemental reviewer: `gpt-6.1-sol`, medium, E1.
  Its actual-CSRF-header P2 was fixed; both HTTP and existing TLS fronts verify `X-CSRF-Token`.
- Owner directly authorized Go **test fixture only** extension in reply to `call_5ZAIE4G8MqpxPZwS04RPGDw1`.
  No production Go/SQL/apps/DTO/routes/dependency/lockfile change, push or deploy.

### Root fix and red → green

Three Go fixtures (publish, domains, merchant-buyer) released the public admin port before other fixture
listeners and Node started. They now share `browserAdminRelay`: Go keeps the public listener continuously
owned, preserving the public IdP/CSRF origin. Existing `browserFront` is unchanged; its WebKit TLS front
targets the held listener. Next uses an automatic **private** port and, after readiness, attaches through
the existing authenticated fixture control (`POST /admin-upstream`). The target is loopback-only,
integer/body bounded, atomically set; self/invalid/different rebinding is refused. Host, inbound XFF,
HTTPS marker, Origin/CSRF, cookies, body/query and response redirects are preserved.

The shared Node helper still retries only an owned pre-readiness EADDRINUSE on an automatic port,
at most **three total launches**. A caller-specified port still never retries. Registration is one
bounded request; a lost ACK does not restart Next. Original **all eleven** assertion lists,
readiness loops, spawn arguments and environment statements remain AST-equal to the original baseline.

- `red-admin-origin.log`: exit **1**, a real rival socket steals the released public port. This RED
  used the extracted fixture with the original premature `Close`, SHA `c3536984…`; it is **not** a claim
  that the entire old Go browser source was run under this particular test.
- `green-admin-origin-final.log`: exit **0**, 3 top-level tests + 4 subtests, `-race`; held/released socket,
  pre-attach 503, invalid/self/rebinding refusal and HTTP/TLS header/CSRF/body/redirect parity.
- `explicit-admin/red-registry.log`: exit **1**, the actual registry omits the new regression;
  `explicit-admin/green-registry.log`: **10/10**, exit **0**. The single publish registry arm now selects
  `^TestBrowser(StorefrontPublish|AdminRelay.*)$`, guarded by the three required symbols.
- Final publish mode actually ran the 3 Go relay tests + 4 subtests **and** the original 23 UI cases.

### Final required gates — one immutable source

Full evidence: `final/evidence.json`. Batch **88166**, PID **89895**, strictly serial,
`2026-10-10T05:15:16Z` → `05:20:58Z`, `BATCH_FAILED_MODES=0`, exit **0**.
Every browser command has `LC_TEST_LOCK_WAIT=14400`; original mode timeouts are unchanged.
The explicit GATE-PORT request required representative local runs for every changed adapter;
this was not used as permission for full foundation or unrelated browser batches.

| Command / mode | Exit | Actual evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` |0|1333/1333, 26 summaries, fail/skip/cancel 0; `final/node.log` |
| `bash scripts/dev/check-gates.sh` |0|82 documented modes, inventory 1256, headers; `final/gates.log` |
| `GOTOOLCHAIN=go1.27.2 go vet -tags browser -p 2 ./tests/foundation` |0|`final/vet.log` |
| `gofmt -l` (4 changed Go test files) |0|empty output; `final/gofmt.log` |
| `bash scripts/dev/test-local.sh --list` |0|83 entries including foundation; `final/modes-list.txt` |
| `go run ./scripts/dev/contractdrift` |0|errors 0, 362 existing baseline warnings; `final/contractdrift.log` |
| `--browser-storefront-publish` |0|3 relay Go tests + 4 subtests, 23 UI cases; `final/browser-storefront-publish.log` |
| `--browser-store-domains` |0|22 cases; `final/browser-store-domains.log` |
| `--browser-merchant-buyer` |0|8 cases, three locales; `final/browser-merchant-buyer.log` |
| `--browser-manual-order` |0|10 cases; `final/browser-manual-order.log` |
| `--browser-buyer` |0|13 cases; `final/browser-buyer.log` |
| `--browser-order` |0|33 cases; `final/browser-order.log` |
| `--browser-buyer-comms` |0|MOCK mailbox/edge; `final/browser-buyer-comms.log` |
| `--browser-catalog-media` |0|20 cases; `final/browser-catalog-media.log` |
| `--browser-promotions` |0|32 cases; `final/browser-promotions.log` |
| `--browser-storefront` |0|REAL stack, 28 products/2 collections; `final/browser-storefront.log` |
| `LC_SHOP_MOCK=1 … --browser-storefront` |0|12 SF cases + R5 extras; `final/browser-storefront-mock.log` |

Each abbreviated browser row means `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh <mode>`.
Evidence class: **E3 author environment, BROWSER / REAL_PG with MOCK IdP / synthetic edge**, not LIVE.

### Extra WebKit run — FAIL, baseline reproduced (not PASS)

Extra `LC_BROWSER_ENGINE=webkit … --browser-storefront-publish` on `eb4de36d` exited **1**:
all 22 workflow steps passed, then the unchanged `uiErrors=[]` assertion saw five
`Fetch API cannot load https` errors. Raw driver: `explicit-admin/webkit-current-driver.log`,
SHA256 `8ad1368f…`; original artifacts `storefront-publish-3646156619`.

Controlled comparison restored **only** the old `9b738e0a` publish Go/JS pair in this worktree,
with product/runtime/current runner unchanged. It also exited **1** after the same 22 workflow
steps with the same five errors. Raw driver `explicit-admin/baseline-webkit/driver.log`,
SHA256 `1fb6fe9c…`; artifacts `storefront-publish-4253993085`. Exact old file hashes and commands
are in the evidence manifest. Both files were restored to final source before final gates.
Thus this extra failure predates the retained relay; no cancellation filter, assertion deletion,
retry or timeout adjustment was added. It remains an explicit baseline follow-up, **not fixed / not PASS**.

### CI gates / handoff / cleanup

Required modes for this diff: `--browser-storefront-publish`, `--browser-store-domains`,
`--browser-merchant-buyer`, `--browser-manual-order`, `--browser-buyer`, `--browser-order`,
`--browser-buyer-comms`, `--browser-catalog-media`, `--browser-promotions`, `--browser-storefront`;
also `LC_SHOP_MOCK=1 --browser-storefront`. Actual PR selection remains registry-owned.
Integrator owns push / PR / independent K3; author only commits.

Independent K3 on final candidate, required PR CI, full foundation/G07, global sweep/visual-lint,
provider SANDBOX/LIVE, production deployment and other non-`*-gate.mjs` startup protocols:
**NOT_RUN**. Extra WebKit is **FAIL_BASELINE_REPRODUCED**, not NOT_RUN.
Owned batch/static/baseline sessions ended; fixtures cleaned their own processes/listeners/containers.
No other PID or lock was killed/deleted. Temporary baseline files restored exactly; no source edits
during the final batch. Old failures and the previous partial handoff below are retained as history.
Go graph indexed 4 files (18 + 73 entities); root-memory links to the relay and all 3 callers succeeded.
MJS graph support is unavailable; actual Git/AST regression evidence is used instead, not a fake graph claim.

## Previous automatic-only handoff — HISTORICAL, superseded above

The pending-ruling statements below describe the earlier `0c1a4e05` delivery, not current status.

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
