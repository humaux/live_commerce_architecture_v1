## FIX #303 relay 2 — prevent navigation from submitting the next step

Current-head CI **40846464956 / job 123357007118**, artifact **12886400573**, failed `settings-v2.spec.ts:154`: after filled product fields, Back: delivery → Next: product unexpectedly opened Review settings. React reused the clicked navigation button as a submit button before its native default action. Key the form by screen so navigation removes its original target while retaining the controlled draft. Existing assertions, fixtures, waits and gates stay intact. First attempt for this browser blocker.

Fetched/merged origin/r3/integration first; already up to date at `d8093bc43`. Strict static TypeScript and diff checks exit 0 (E1); no local tests/servers/builds. Original failure, snapshot, screenshot and source hash are in `output/settings-v2-impl/fix303-relay2-*`; DELIVERY.md has the root-cause record. No new outside-plan path or import; historical playwright.config.ts registration remains for integrator approval. No generated map change required: no browser spec/page/route or Go dependency changed. Current repair GREEN/E4 pending. Revert this repair commit on the assigned branch to undo it.

## Problem and change

The tests-only head fails because the signed simplified settings surface is absent. Implement the three-row overview, profile editor/review/save/recovery and optional three-step setup in zh-TW, zh-CN and en. Existing advanced configuration remains reachable by real clicks. Reuse existing profile CAS, quick-start marker, unique MANUAL delivery services and catalog commands; preserve original command bytes/key after uncertain results. No market, policy, payment or carrier defaults are invented.

Merged origin/r3/integration `d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e` before implementation. This imports the existing OC signup navigation fence; the original browser artifact proves the corresponding `createStore` ERR_ABORTED race. No OC test change authored in this card.

## Evidence

- Causal RED: test head `696629d92e786e70bc70545089bc114d1b7cb7e4`; Gates run **40828407097**, job **123300694776**, artifact **12878213371**: `settings-v2.spec.ts:22`, visible signed overview missing. Fresh original receipt: `output/settings-v2-impl/fix303-red-playwright.log` and `red.log`.
- Original bundle:08: job **123300694974**, artifact **12879157189**, signup/createStore navigation race; receipt in `fix303-oc-playwright.log`.
- Prior Foundation **40828407094 / 123296691356** passed the tests-only head. Implementation GREEN pending this PR's own CI; author E0, static E1, independent E4 NOT_RUN.
- Allowed static checks: targeted strict TypeScript, node syntax, bash syntax, packet structure and diff whitespace all exit 0. No local tests/builds/servers. New retry assertion adds coverage; no existing assertion, fixture, threshold, exemption or timeout weakened.

## Scope and handoff

All implementation paths are in `output/settings-v2-impl/PLAN.md`. Historical RED registration additionally changed **playwright.config.ts** outside that original plan, for integrator approval; it registers the calling spec and imported helper. No new outside-plan path in FIX #303. Browser ownership regenerated after the integration merge. No contract, OpenAPI, migration, Go dependency, lockfile, baseline or dispatcher change. Dependency map unchanged because no Go dependency changed.

UI process, desktop/mobile fact parity, three-language copy and authored click ledger are in DELIVERY.md. Actual implementation click/persistence acceptance awaits CI. Humaux store/canvas/coord unavailable; local handoff retained. NOT_RUN: implementation CI GREEN, independent review, production/provider/SANDBOX/LIVE acceptance. Integrator relays completed workflow results; no merge, auto-merge or deployment.

🤖 Generated with Codex (dispatched by Claude Code integrator).
