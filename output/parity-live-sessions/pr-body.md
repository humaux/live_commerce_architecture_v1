Studio now opens the signed C4 session tasks, and Meta connections use the signed OAuth/pick/review/result/detail states on the existing scoped server contracts. All three locale catalogs are included. Copy-conflict details after a document reload remain deferred because the existing GET projection does not return them; this PR adds no backend contract. Required-fact, copy and UI-process evidence is in [DELIVERY.md](output/parity-live-sessions/DELIVERY.md).

FIX1 imports integration `da2cc7a3f` and repairs cancelled detail reads, SPA task navigation, selected no-op controls and the console Sessions destination. Shared browser setup follows the signed Create session → draft → claims → Advanced flow; all existing security, request-count, CAS, billing and persistence assertions remain.

### Relay 1: second directed fix

At head `1d77b89c`, Foundation `41267520054` failed solely on generated browser ownership drift. Gates `41267520139` tested merge `5b30adce3cab5d42a650a464cb9580c54f92e724`: all ten click-sweep shards, Studio, live-console and aggregate passed. Remaining failures:

| Job | Failure | Fix |
|---|---|---|
| bundle:05 `124734313790` | deal-loop and four live-tools cases select both Advanced and its nested This round summary | Click only the direct-child summary in the shared helper. |
| bundle:07 `124734313470` | CB11 hits the same shared-helper strict-mode error | Same shared fix; preserve 402→200 and all billing assertions. |
| bundle:06 `124734313755` | OAuth reconnect succeeds; token assertion runs after reload into collapsed settings | Reopen settings with the existing real-click helper; retain exact active-state and absent-banner assertions. |
| Foundation `124733309565` | Generated ownership hash is stale | Regenerate after final source edits and run `--check`. |
| receipts `124740395785` / required `124740808293` | The three failed browser receipts above | No workflow, receipt or status modifications. |

Full per-job classifications, original evidence pointers and SHA-256 digests are in [FIX1.md](output/parity-live-sessions/FIX1.md). Base Gates `41266607808` succeeded at `da2cc7a3f`; no class (c) exception claimed. This is directed attempt 2 for shared claim setup. An identical RED after this push must be reported BLOCKED; no third approach.

### Validation and boundaries

- Causal tests-only RED: head `395209e39`, run `41052979614`, job `124055728603`: four new C4 assertions failed, 23 existing workspace cases passed. [red.log](output/parity-live-sessions/red.log) also records the latest-head pre-fix failures.
- E1 only: syntax checks, packet structure, official ownership generation/check and dependency-map generation (unchanged). Evidence-retention check uses the committed head before push. No local tests or servers. Prior broad tsc was NOT_ACCEPTED due missing local dependencies/types; this relay adds no imports.
- Current-head GREEN and independent E4 review are PENDING. Integrator owns run-completion wake-up and fresh-session relay. Provider SANDBOX/LIVE and production acceptance are NOT_RUN.
- Relay changes only two admin test lines, generated ownership and card evidence. No test, fixture, timeout, threshold, exemption or gate-adapter assertion weakened. No new product/design/copy change in this relay.
- No deployment, merge, auto-merge, force-push or status posting.

Outside original literal write paths, disclosed for integrator approval: `apps/admin/components/LiveWorkspace.tsx` (same-module Sessions destination), `tests/admin/{parity-live-sessions.ts,studio-ui.spec.ts,claims-ui.spec.ts,live-console.spec.ts,meta-connect.spec.ts,meta-connect-gate.spec.ts,meta-health-ui.spec.ts,meta-health-model.test.ts,attribution.spec.ts,customers-billing.spec.ts}`, `tests/e2e/{deal-loop.spec.ts,live-tools.spec.ts}`, `tests/foundation/{browser_ads_attribution_test.go,browser_live_console_test.go}`, `packages/i18n/src/index.ts` (namespace export), generated `output/ci-select-backend-browser/browser-ownership.json`, and card evidence. Dependency map regenerated without a diff. No authored backend package, contract/OpenAPI, migration, R2 pin, dependency/lockfile, route-dispatcher or baseline change.

Raw CI downloads are ignored and uncommitted. Bulk Figma captures were removed from the index while local copies stayed; [figma-capture-manifest.json](output/parity-live-sessions/figma-capture-manifest.json) retains identities and hashes. No new Figma read/write or design acceptance claimed. Humaux/coord/canvas tools are unavailable in this relay; no shared-state update is asserted.

🤖 Generated with Codex (dispatched by Claude Code integrator).
