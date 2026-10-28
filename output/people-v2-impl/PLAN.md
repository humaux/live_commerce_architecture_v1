# PEOPLE-V2-IMPL · scope before source edits

Status: RED preparation; implementation NOT_RUN. Evidence: E0 until CI executes.
Branch: `unit/people-v2-ui`; base after required integration merge:
`bf161bac96252c35337e07713c08a275d48e3e17` (fast-forward from `origin/r3/integration`).
Execution: Codex, UI implementation lane; actual runtime model/effort not exposed.
No delegation, no local tests or servers, no merge/auto-merge.

## Authority and audit

The assigned `ux-people/output/ux-people/DELIVERY.md`, latest simplify section,
owns the 24 signed frames and the 65-string zh-TW / zh-CN / en copy table.
Read architecture §§0/1/2/14/26/27, AGENTS.md, customers-billing-v1 CD1–CD8,
§§5/6/7, and the current customer components, routes, parsers and browser harness.
Live Fleet Bridge read: `1429:44842` and its result child `1429:44852`.
The latter is explicitly `Result / export / unknown`, `UNKNOWN read-only`.

Current list audit: refresh, tags, reports, imports, six data columns and explanatory
copy crowd the primary search/view task. Detail repeats facts and activity and has
an immediate export and typed-ERASE dialog instead of the signed confirmations.
These are findings from current source, not acceptance claims from output snapshots.

CD6 remains synchronous. Unknown transport results must never become invented server
queue records. Preserve the signed read-only unknown path using the actual client
outcome; successful export/erasure must reflect the real synchronous response and
subsequent server read. No backend/API/contract changes are authorized.

## Exact planned write paths

| Area | Paths |
| --- | --- |
| List and detail components | `apps/admin/components/Customers.tsx`, `apps/admin/components/CustomerDetail.tsx` |
| Scoped styling | `apps/admin/components/customers.css` |
| Three-locale entries | `packages/i18n/src/people.ts`, `packages/i18n/src/index.ts`, `apps/admin/lib/customers-copy.ts` |
| Browser acceptance (new) | `tests/admin/people-v2.spec.ts` |
| Existing browser harness registration | `tests/foundation/browser_customers_billing_test.go` |
| Every compiling harness found with `rg -rlE '(Customers\|CustomerDetail)\\.tsx' tests/` | `tests/admin/customer-tags-scope.test.mjs`, `tests/admin/import-integration.test.ts`, `tests/admin/w6-integration.test.ts` |
| Existing behavior regression coverage | `tests/admin/customers-billing.spec.ts` |
| Generated ownership | `output/ci-select-backend-browser/browser-ownership.json` (derive script only) |
| Generated dependency map, if forced | `docs/engineering/dependency-map.md` (depmap.sh only) |
| Card evidence | `output/people-v2-impl/PLAN.md`, `DELIVERY.md`, `red.log`, `pr-body.md`, `evidence/` beneath this directory |

Existing page entrypoints, read and retained: `apps/admin/app/[locale]/customers/page.tsx`
and `apps/admin/app/[locale]/customers/[customer]/page.tsx`. No new page/route needed.
Shared shell, other applications, backend packages, migrations, OpenAPI, contract,
dependency locks, route-table/baseline and startup gate-adapter assertions stay outside this card.
Anything else must be listed for the integrator instead of silently edited.

## CI RED → GREEN contract

One tests-only push covers all signed journeys: search/view/clear/back, desktop/mobile,
all three main-screen locales, profile facts/orders/consent/privacy, export confirmation,
name-plus-checkbox erasure, definitive success and reload, and unknown results with no
automatic POST replay. UI mutations use real clicks/fills/checks; route stubs are MOCK
fixture/fault preparation only. Every exercised control records visible expectations.
Assertions are independent of future component imports so RED reaches behavioral
assertions rather than missing components. Existing gate assertions remain unchanged.

After RED push: record PR, head SHA, Gates/Foundation run IDs. Observe at most ten
minutes, then hand back with WAIT_RUN; implementation waits for causal RED evidence.
Only then make one implementation push, update all compiling harness imports, and
regenerate ownership. Do not push a final evidence-only commit after GREEN.

## UI PROCESS handoff

- BOURDIEU: Q1 task terms use signed seller copy; Q2 recent task experience governs
  shortcuts; Q3 identity stays explicit; Q4 destructive effects and retention stay visible;
  Q5 no class/device assumptions; Q6 aesthetics separate, owner signed the supplied frames.
- PROJECT UNDERSTANDING: architecture §14 forbids inferred identity/consent;
  CD6 requires synchronous truth; CD7 retains financial records and blocks open payments;
  §§5/6/7 require server permissions, idempotency and bounded exports.
- MOBBIN → EAGLE: preserve the approved design's directory/profile/consent structure;
  reference provenance is in the assigned DELIVERY (Square `MVP1C226OIGO0`, Shopify
  `MVP1C22DSX0X6`, Klaviyo `MVRS5VC9CEFOF`). No new reference import or design is made.
- FIGMA OFFICIAL GUIDANCE: approved delivery applies FIGMA-GUIDE G1/G2/G3/G4/G6/G7;
  preserve layout/variables/component semantics and navigation. This lane reads only.

Click ledger, desktop/mobile required-fact parity, three-language copy coverage and
mobile density must be reported from actual CI product evidence at implementation
handoff; Figma static counts are not runtime acceptance.
