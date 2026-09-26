# Merchant order workspace: preflight, not UI acceptance

2026-09-27; inspected source `722bde44d4666b21fff38fe9a0c1283ff238cf24`.
This increment changes documentation only. No application/database/provider or
customer-system mutation, build, browser acceptance or production rollout.

## Verified gap and reuse

- `apps/admin/app/[locale]/page.tsx` renders `Ledger` or `Entry`;
  `components/WorkspaceFrame.tsx` has no orders navigation. There is no orders
  page. The existing settings wizard and purchase-entry controls do exist.
- `internal/httpapi/orders.go` registers authenticated list/detail reads;
  `internal/merchantorders/orders.go` owns the safe frozen order projections.
  [MOR acceptance](2026-09-25-merchant-orders-acceptance.md) and
  [MBT acceptance](2026-09-25-merchant-orders-bff-acceptance.md) remain prior
  backend/transport evidence, not evidence of this UI.
- The admin BFF, `lib/orders-request.ts`, server-cookie authentication,
  `WorkspaceFrame`, three locales and currency formatter are reusable.
  `workspaceData` is **not** an order admission helper: it chooses the first
  catalog store and requests warehouses. Use explicit authorized-store
  selection without inheriting catalog/inventory permissions or dev fixture.
- `tests/foundation/browser_merchant_orders_bff_test.go` already supplies signed
  mock OIDC, actual Next/Go/PG and business-created orders. Extend this test
  infrastructure at a distinct UI gate, not a mock-only replacement or a second
  large fixture runner. Existing transport tests must retain their own coverage.

## Design and behavior

Buyer **B 商品详情直接选购** stays approved. The merchant candidates A左右对照,
B逐单核对 and C表格原位展开 are separate and all three sidecars still say
`approved: false`. Existing [audit](2026-09-25-merchant-orders-ui-audit.md) and
`.impeccable/merchant-orders-options.json` are reused; no new image generation.
One filter/list/selected-detail composition, no duplicate KPIs or mutation CTA.

The new [MOU contract](../../contracts/merchant-orders-ui-v1.md) covers route/store
authority, stale-read suppression, PII lifecycle, honest payment/fulfillment
states, immutable totals/destination, and explicit real-browser gates. Visual
selection is pending; **MOU01–06 are NOT_RUN**.
Generated comp text such as refunded/shipped, channel or global counts does not
expand the API. Correct factual labels without inventing features.

## Verification and next execution boundary

Read-only static check passed: local contract/document links resolve; pending
selection, NOT_RUN and source SHA markers exist; all three merchant comp files exist and
their sidecars remain unapproved. This is document consistency, not product QA.

After merchant selection and contract review, assign isolated implementation
and test worktrees at the same frozen contract revision. Source author is not
the sole reviewer. Reuse existing `typecheck:admin`, `build:admin`,
`test:admin`, `test:i18n`; the new exact UI runner selector must be added and
recorded during implementation, not claimed to exist now. Relevant current
real-chain selectors are `--browser-merchant-orders-bff`, `--browser-identity`,
`--browser-merchant-buyer`, `--browser-order` and `--browser-payment` under
`scripts/dev/test-local.sh`. Scope repeat runs to changed dependencies and
retain independent authority/financial tests; do not relax passing gates.

Independent preflight memory `977f18dd-da68-4cfc-9817-cb412070f2e2`: the final
verdict scored both reported gaps **resolved**. These were synchronous
hide/pagehide PII clearing plus actual history/pageshow/cross-tab gates, and
explicitly labeling allocation-failure SQL injection separately from the pure
business path. Reviewer role `security_reviewer`, actual model `gpt-6-sol/high`,
read-only root source at `722bde4` plus these uncommitted docs; no source writes,
PG or browser calls. Root independently checked the injection source at
`tests/foundation/merchant_orders_test.go:595-617` and reran document checks.
No merchant implementation may infer visual approval from this document.
Remaining SaaS and production gates remain open.
