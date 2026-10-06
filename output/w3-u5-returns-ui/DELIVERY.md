# W3-U5 退貨/商家取消 UI — delivery record

- Unit: `docs/delivery/units/w3-u5-returns-ui.md`; contract: `contracts/returns-v1.md` (backend W3-08B, migration 0155).
- Role/model: ui_worker (owner exception: Kimi K3 implements UI), actual model `kimi-k3` (Claude Agent SDK harness).
- Base SHA: `5862caf8c173bf6628b7871f9ac5799b4fde359f`; worktree `.worktrees/w3-u5-returns-ui`; branch `unit/w3-u5-returns-ui`.
- Write paths used: `apps/admin/components/OrderReturns.tsx` (new), `apps/admin/components/ReturnsList.tsx` (new),
  `apps/admin/components/OrderDetailPanel.tsx`, `apps/admin/components/OrderListFilters.tsx`,
  `apps/admin/components/order-actions.css`, `apps/admin/lib/returns-{model,copy,client}.ts` (new),
  `apps/admin/app/api/stores/[store]/[...resource]/route.ts` (additive), `apps/admin/app/[locale]/returns/page.tsx` (new),
  `apps/admin/lib/orders-request.ts` (additive grammar), `apps/admin/src/features/orders/routes.ts`,
  `apps/admin/src/shell-copy.ts`, `tests/admin/returns-{model,request}.test.ts`, `tests/admin/returns-ui.spec.ts` (new),
  `scripts/dev/test-local.sh`, `docs/delivery/GATES.md`, `output/w3-u5-returns-ui/*`.
  `MerchantOrders.tsx` untouched (W3-U4 parallel). No Go/SQL/migrations/contracts/go.mod/lockfiles touched.

## What was built (zh-TW first, en/zh-CN complete)

1. **訂單詳情「退貨」區** (`OrderReturns.tsx`, mounted when fulfillment is MERCHANT_SHIPPED/PROVIDER_LABEL_CREATED):
   登記退貨 (per-line qty ≤ shipped, reason enum + optional note composed into the 240-byte server reason),
   確認收貨 (per-line 0..registered, ≥1 unit), 驗貨與處置 (per-line 可再售+不可售=實收, client-enforced),
   關閉退貨單 (confirmation restating restock units), 撤銷登記 (REGISTERED only, release confirmation).
   One Idempotency-Key per distinct body, reused only for byte-identical retry after an unknown outcome (§4);
   every command carries `expected_version` CAS; refusals render in-dialog via the §7 closed error map;
   `version_changed`/`invalid_state`/`exceeds_shipped` re-read the list. After INSPECTED/CLOSED the panel shows
   「如需退款，請使用退款」 linking to `#order-refunds` (Integrator 裁决: returns never refunds).
   Disposition buttons (inspect/close) disable without `inventory:write` (session hint; Go re-authorizes).
2. **取消訂單** (`CancelOrderSection` in `OrderDetailPanel.tsx`): shown only while `commercial_state` ∈
   {DRAFT, AWAITING_PAYMENT, CONFIRMED, AWAITING_COLLECTION}; dialog warns 會釋放已保留的庫存, asks a reason
   (≤60 chars CJK-safe), CAS body `{expected_state, reason}`. Refusal copy in-dialog: `refund_first`
   (「請先完成全額退款，再取消訂單。」), `payment_in_flight`, `already_shipped`, `has_returns`, `not_cancellable`, …;
   never auto-refunds (I05/I13). `#order-refunds` anchor wraps OrderRefunds.
3. **退貨管理頁** `/{locale}/returns` (`ReturnsList.tsx` + route registry id `returns`, nav label 退貨/退货/Returns,
   permission `orders:read`, query grammar exactly `store` + one RMA `state`): RMA list with state-filter links
   (refresh-persistent), rows deep-link to `/{locale}/orders?store=…&state=all&order=…`; 「退款失敗待處理」 section
   from `GET /orders/cancel-refund-gaps` with captured/refunded/gap amounts and a 前往訂單退款 link.
4. **訂單列表**: `OrderListFilters` gains a 「退貨與取消」 link to the returns page.
5. **BFF grammar** (additive): `POST orders/{id}/cancel`, `GET|POST orders/{id}/returns`, `GET returns`
   (the single exception to the no-query rule: exactly `?state=<RMA state>`), `GET orders/cancel-refund-gaps`,
   `POST returns/{id}/{receive|inspect|close|cancel}`; strict parsers fail closed as "unavailable" on drift
   (timestamptz offset form `+00:00`, per-line `restock+scrap=received≤registered`, min version per state,
   `refund_id` only when CLOSED, unique (warehouse,sku) lines, gap=captured−refunded≥1, reason `cancel_refund_failed`).

## Tests / gates (this worktree)

| Command | Exit | Evidence |
|---|---|---|
| `node --test --experimental-strip-types tests/admin/returns-model.test.ts tests/admin/returns-request.test.ts` (red, before implementation) | 1 (ERR_MODULE_NOT_FOUND) | `output/w3-u5-returns-ui/red.log` |
| `node --test --experimental-strip-types tests/admin/returns-model.test.ts tests/admin/returns-request.test.ts tests/admin/shell-registry.test.ts` | 0 (19 pass) | `output/w3-u5-returns-ui/green.log` |
| `bash scripts/dev/check-gates.sh` (GATES.md registry + G-UI3/G-UI5 architecture gate + check-headers ratchet) | 0 | `output/w3-u5-returns-ui/check-gates.log` |

New gate registered: `--browser-returns-ui` in `scripts/dev/test-local.sh` (usage line, mode guard, refuse-no-test
block running the two node suites first, admin+storefront build lists, run branch `LC_BROWSER_RETURNS_UI_ACCEPTANCE=1
go test -race -tags browser -run '^TestBrowserReturnsUI$'`) + `docs/delivery/GATES.md` mode row and coverage row.

## 點擊台帳 (click ledger)

Encoded as real-click Playwright steps in `tests/admin/returns-ui.spec.ts` (click/fill/selectOption/keyboard only;
`page.evaluate` used solely for the scrollWidth overflow measurement and one annotated forged-request negative probe;
refresh-persistence asserted after `page.reload()`): register→receive→inspect→close with per-step keyed-POST body
assertions, register→withdraw, `refund_first` and `payment_in_flight` cancel refusals (dialog stays open, order
unchanged), returns list + state filter + gap row click-through, restricted-member gating + server 401/403 probe,
zh-TW + en at 1586px and 390px with no-horizontal-overflow assertion. **Execution is NOT_RUN locally** (see BLOCKED).

## NOT_RUN

- `tsc --noEmit -p apps/admin` (CI G03 `typecheck:admin`): the dev-machine permission layer denied every tsc/pnpm
  invocation in this session; files were manually reviewed against the strict-typings they touch
  (`as const` tuple `.includes` casts, `Pending`/`WriteResult`/`Badge` tone signatures). Runs in CI.
- `tests/admin/returns-ui.spec.ts` browser run: needs the Go harness below; Playwright is not installable here.
- `bash scripts/dev/test-local.sh --browser-returns-ui` end-to-end: same blocker (fails closed by design until the
  harness lands — the mode's refuse-no-test block `test -f tests/foundation/browser_returns_ui_test.go`).

## BLOCKED / integrator to-do

1. **Go browser harness** (integrator): `tests/foundation/browser_returns_ui_test.go` `TestBrowserReturnsUI` driving
   the spec; fixture contract is documented in the spec header (env: `LC_BROWSER_PUBLIC_ORIGIN`, `LC_BROWSER_EVIDENCE`,
   `LC_BROWSER_STORE`, `LC_BROWSER_ORDER_SHIPPED` (MERCHANT_SHIPPED, CAPTURED, one SKU qty≥2, no live RMA),
   `LC_BROWSER_ORDER_PAID` (CONFIRMED+CAPTURED → refund_first), `LC_BROWSER_ORDER_IN_FLIGHT` (payment attempt in
   flight → payment_in_flight), `LC_BROWSER_GAP_ORDER` (cancel_refund_failed gap), `LC_BROWSER_RESTRICTED_TOKEN`).
2. **訂單列表「有退貨」篩選** (brief item 3): **BLOCKED by backend** — W3-08B explicitly ships no order-list
   has-returns filter ("`GET /returns` is the list"). Delivered substitute: the 退貨與取消 link in OrderListFilters
   plus the /returns page. Re-adding the filter needs a Go/SQL change outside this unit's write paths.

## Risks

- `OrderReturns.tsx` is 526 lines (>500 G-UI5 WARN, far under the 800 hard gate); the dialog is one native
  `<dialog>` shared by five kinds, mirroring OrderRefunds.
- The returns panel self-fetches `readWorkspace` for the `inventory:write` hint; unknown → enabled, server decides
  (a 403 surfaces as copy, never a silent no-op).
- Register bodies never send `warehouse_id` (the order detail does not expose it); a multi-warehouse SKU answers
  422 `ambiguous_line`, mapped to merchant copy.
- No success is rendered from a write response; every step re-GETs (`setTick`/`onChanged`) before showing state.

## Integrator follow-up (Sonnet)

- Trunk merged: `git merge r3/integration` (93233a00, browser-tag build fix) into the branch; no conflicts.
- TS2741 fixed: `ReturnsList.tsx` passes `scrollHint={c.scrollHint}` to both `TableFrame`s; `scrollHint` added to `returns-copy.ts` in zh-TW, zh-CN, en
  (same wording as `presentation-copy.ts`). `pnpm exec tsc --noEmit -p apps/admin` -> exit 0.
- Harness written: `tests/foundation/browser_returns_ui_test.go` `TestBrowserReturnsUI` (tag `browser`, helpers prefixed `bru`; reuses `brfStartAdmin`,
  `brfPlaywright`, `brfEvidence`, `rt*`). Seeds only through real paths on `tcvEnv{stripe:true}`: `LC_BROWSER_ORDER_SHIPPED` = paid card order + `PUT /shipment`;
  `LC_BROWSER_ORDER_PAID` = CONFIRMED+CAPTURED; `LC_BROWSER_ORDER_IN_FLIGHT` = `rtPayStarted`; `LC_BROWSER_GAP_ORDER` = paid, refunded, cancelled,
  provider `refund.failed` webhook (as `TestMerchantCancelRefundGap`); `LC_BROWSER_RESTRICTED_TOKEN` = member with `orders:read` only. After Playwright it
  asserts PG facts: shipped order has exactly 1 CLOSED + 1 CANCELLED RMA, exactly 1 `returns.rma.restock` ledger row, CAPTURED facts and refund count unchanged,
  refused orders still CONFIRMED / AWAITING_PAYMENT, no new `orders.merchant_cancelled` audit row, screenshot manifest has zh-TW+en x desktop+mobile.
- Verified locally (E2, not the browser): `go vet -tags browser ./tests/foundation` -> 0; `gofmt -l tests/foundation` -> empty;
  `bash scripts/dev/check-gates.sh` -> 0; `node --test --experimental-strip-types tests/admin/returns-*.test.ts` -> 10 pass;
  seeding path run once through `LC_FOCUSED_TAGS=browser scripts/dev/test-focused.sh` with a throwaway test (deleted): 4 orders in the expected states, gap listed.
- CI gates (not run locally, heavy): `--browser-returns-ui`, `--browser-click-sweep`, `--browser-visual-lint`.
- NOT_RUN: the Playwright spec itself; the harness has never driven it. Spec-vs-UI mismatches (selectors, test ordering on `LC_BROWSER_ORDER_SHIPPED`) can only show up on CI.
