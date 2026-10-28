# PEOPLE-V2-IMPL · tests-only RED handoff

State: **implementation NOT_RUN; causal RED PENDING; GREEN NOT_RUN**.
Author assessment E0. No UI acceptance or completion claimed.
Branch `unit/people-v2-ui`; base `bf161bac96252c35337e07713c08a275d48e3e17`.

## Authority and exact scope

Latest signed simplify DELIVERY SHA256:
`0536aab8723ca87ea28268d9d2abf76b5d99a34700affff42d83a0a0adb1e2e7`.
Live Fleet Bridge inspected mobile `1429:44842` and result `1429:44852`;
their screenshots are retained in this card's evidence directory. No Figma writes.
Exact implementation and test paths were recorded before source edits in PLAN.md.
Only tests, generated ownership and this card's evidence changed in this stage.

Current remote contract blob `e618c69c1e637022b3a7385f741975e424259004`
matches local `contracts/customers-billing-v1.md`. CD6 is synchronous. The
live design's unknown/read-only result does not establish a backend queue.
Keep synchronous success distinct from uncertain transport outcomes.

## RED assertions and click ledger

`tests/admin/people-v2.spec.ts` is appended to the real CB11 browser harness.
No prior test or assertion was removed or weakened. It imports no future component
or future locale file, so the initial behavioral assertions run against existing UI.
The signed prompt (`PEOPLE-V2-SIGNED-SEARCH`), review-before-export
(`PEOPLE-V2-EXPORT-CONFIRM`) and mandatory scope checkbox
(`PEOPLE-V2-ERASE-SCOPE`) are expected RED assertions, not observed failures yet.

The signed-identity login uses the existing MOCK IdP, production Next harness and
isolated CB11 fixture. Scoped page.route responses are explicitly MOCK existing
customer DTOs; route.abort prepares uncertain-result faults. No mock introduces
a backend pending-request field. UI actions use click/fill/check/uncheck/reload;
page.evaluate reads layout only. Each test emits a per-control JSON click ledger
into the CI harness evidence directory. Synthetic fixture writes and persistence
prove mocked UI behavior only, never Go/PG or production privacy execution.

| Page/state | Control/operation | Expected visible result | Actual |
| --- | --- | --- | --- |
| Main, three locales × 1440/390 | search fill, submit, refresh | signed prompt and persistent query | PENDING CI |
| Search results | customer link, return, clear, phone search | customer facts and preserved context | PENDING CI |
| Detail | orders/consent/privacy anchors | respective server facts visible | PENDING CI |
| Export review | download, confirm | protected scope and permissions before one POST/file | PENDING CI |
| Erasure review/ready | wrong/right name, checkbox check/uncheck | name plus scope required; safety facts retained | PENDING CI |
| Erasure completed/refused | confirm, refresh | server deactivation or refusal; three orders retained | PENDING CI |
| Export/erasure unconfirmed | submit, pending entry, view, refresh, return | read-only lock, identity retained, no replay | PENDING CI |

## UI PROCESS · BOURDIEU

Q1 use signed seller terms; Q2 distinguish recent task experience, never class/device;
Q3 keep identity explicit through confirmation; Q4 retain irreversible-effect and
legal-retention facts; Q5 no social/class inference from customer metadata;
Q6 aesthetics are separate from task success. Owner signed the supplied design;
this lane has no new aesthetics or user-comprehension evidence.

## UI PROCESS · PROJECT UNDERSTANDING

Read AGENTS.md and architecture §§0/1/2/14/26/27: scoped frontend with PG/Go authority,
separate identity/transaction/touchpoint facts, consent cannot be inferred from purchase,
and author cannot be sole acceptance reviewer. Read customers-billing-v1 CD1–CD8,
§§5/6/7: privacy permissions, synchronous responses, idempotent commands, 200-order
export bound, financial/legal retention and open-payment erasure refusal remain.
Actual customer pages, components, parsers, client and CB11 browser harness were read.

## UI PROCESS · MOBBIN → EAGLE

No new design/import in this stage. The signed authority's existing customer folder
`MVQL9YUYU0QJ0` records Square `MVP1C226OIGO0` (search/result hierarchy), Shopify
`MVP1C22DSX0X6` (identity/order context) and Klaviyo `MVRS5VC9CEFOF` (consent by
channel/purpose). These are provenance pointers, not a claim of a fresh Eagle
visual audit by this implementation lane. No third-party images or facts copied.

## UI PROCESS · FIGMA OFFICIAL GUIDANCE

The signed authority maps FIGMA-GUIDE G1 auto layout, G2 variables, G3 components /
variants, G4 prototype navigation, G6 annotations and G7 design world. This stage
only reads signed geometry/state and prepares CI clicks. Applied browser layout,
focus behavior and component parity remain NOT_RUN until implementation and CI.

## Required-fact and locale parity

The authority's desktop/mobile checklist and 65-string zh-TW/zh-CN/en table remain
the implementation requirements. Test fixture facts include actual response-shaped
customer ID/name/phone suffix, first-recorded date, three orders, monetary aggregate,
consent dates/sources and completed privacy history. Runtime parity, all visible
strings and mobile text density are NOT_RUN; Figma counts do not prove product counts.

## Verification and continuation

- `node --check tests/admin/people-v2.spec.ts`: exit 0 (syntax only, E1).
- `node scripts/dev/derive-browser-ownership.mjs`: exit 0, 53 modes; static generator.
- `git diff --check`: exit 0 (whitespace only).
- Local tsc: NOT_RUN, no installed workspace compiler/dependencies; no installation done.
- Local tests/builds/servers/containers/PG/browser: NOT_RUN by owner instruction.
- RED/GREEN run and job receipts: see red.log; causal RED still PENDING at preparation.
- After RED push, observe CI at most ten minutes, record Gates/Foundation WAIT_RUN,
  and return to integrator. Implementation must wait for an assertion-level RED.
- No merge, auto-merge, production action, live key, real buyer data or external message.
- No source changes outside PLAN. No processes/ports/containers started to clean up;
  the two bridge-created screenshots were moved from worktree root into this card.

Humaux scope decision: `9fb050af-4918-4f9a-8a72-b78c96aecaaa`.
Own canvas element: `integrator-sub-people-v2-impl` (RED preparation); parent untouched.
