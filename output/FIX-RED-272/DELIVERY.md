# FIX-RED-272 handoff

Authority: owner-dispatched FIX-RED #272 and issue #282, current PR head 3797df6a0fc6ae2555c06edc8856ec312618e15b; current-head Gates 40746205385. Read AGENTS.md, 架构.md §§0/1/2/26/27, contracts/tasks.json, merchant-orders-v2.md and relevant manual-fulfilment-v1.md sections. Humaux/coord/store/canvas tools unavailable; no persistence or independent review receipt claimed.

## Root causes and fixes

1. Existing shipment browser callers stopped at the new review step. Add only the real shipment-confirm click in home-cod, manual-fulfilment (record and re-ship), parcel-merge (409 race), deal-loop and click-sweep. All original request/state/history/CAS assertions remain verbatim and in order.
2. The orders-ui MOCK command succeeded, but its recorded_at used seconds precision. Both strict detail and history decoders require the canonical six-digit microsecond wire format (orders-model.ts timestamp; internal/merchantorders/orders.go timestampLayout). Correct only that MOCK serialization and validate it with the existing strict parser before use. Do not broaden the parser. Three locale cases and all original assertions remain.
3. Merge origin/r3/integration first; resolve only generated browser ownership with the official generator. Regenerate again for changed specs. No Go dependency imports changed, so depmap is unchanged.

## Verification and evidence

Current-head RED: red.log, including all six failed jobs and their causal downstream aggregators. Original implementation RED remains in PR history: 40664077412 / 122784723101.
Local static checks: node --check tests/ui/click-sweep.mjs = exit 0; git diff --check = exit 0; ownership generator = exit 0. No local tests/builds/servers/PG/browser execution. These checks establish E1 syntax/structure only, not TypeScript compilation or product acceptance.
GREEN: PENDING new PR CI. Author assessment E0; independent E4 review NOT_RUN. Integrator relays the Gates/Foundation completion for a fresh session. Do not merge or auto-merge.

## Click ledger

| Page | Controls / operation | Expected result | Current RED | Repair CI |
|---|---|---|---|---|
| Orders home-COD | submit, confirm | saved shipment, then collection | no saved record | PENDING |
| Orders manual | record/confirm, correct, void, re-ship/confirm | keyed writes and version/history assertions | no shipped state; downstream CSV fails | PENDING |
| Orders parcel | submit, confirm | server 409 and group refresh | latent #282 path, blocked by earlier orders-ui phase | PENDING |
| Deal loop | submit, confirm | merchant/buyer shipment then refund | no shipped state | PENDING |
| Click sweep J3 | submit, confirm | shipped and collected, buyer readback | shipment record absent; buyer PENDING | PENDING |
| Orders MOCK zh-TW/zh-CN/en | cancel, confirm, refresh | one command; canonical DTO readback | all 3 missing shipment-record | PENDING |

## UI process applicability

No product UI, visible copy, layout or Figma changes in this repair. Bourdieu Q1–Q6, project screen rules, Mobbin→Eagle references and Figma official guidance are not re-designed here. Existing three-language confirmation coverage is retained. This handoff makes no new visual parity, density, Figma lint, browser PASS or production claim.

## Scope

Changed outside original implementation paths under FIX-RED pre-approval: tests/admin/home-cod.spec.ts, tests/admin/manual-fulfilment.spec.ts, tests/admin/parcel-merge.spec.ts, tests/e2e/deal-loop.spec.ts, tests/ui/click-sweep.mjs; generated output/ci-select-backend-browser/browser-ownership.json; output/FIX-RED-272/{red.log,DELIVERY.md}. These belong to the same orders flow and must be listed for integrator review. Integration merge includes upstream changes, not lane-authored edits to those modules/contracts. Existing untracked output/ORDERS-V2-IMPL remains untouched.
