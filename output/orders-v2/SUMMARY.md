# orders-v2 — R5 A3 delivery

As of 2026-10-03 (Asia/Taipei). **PASS for the requested local unit gates**;
not a production release or provider acceptance.

## Ownership / scope

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/orders-v2`
- Branch: `unit/orders-v2`; dispatch integration base `b7a4add`; actual clean
  starting HEAD `19b3fad974ea38ad830dd8b064d23139e9f5df0d` (includes real-click gate).
- Only this worktree edited. No push, merge, deployment, credentials or production
  activity. No payment/amount/status-machine write implementations changed.
- Migration **0110** only; post_river 0022 not needed. All source commits carry the
  requested Codex co-author trailer.
- Root Codex owns all writes and test execution (runtime model identifier not
  exposed; inherited reasoning). Read-only audit agents `orders_contract` and
  `orders_tests`: gpt-6-luna/high, explorer. Fresh visual reviewer
  `orders_visual_final`: gpt-6-sol/medium. No delegated writes or recursive agents.
- Existing DAG reviewed; no pre-existing orders-v2 DAG node. Humaux task
  `58deaeee-4826-4a26-b313-d97847ece0cf`, agent `codex-orders-v2`, leased worktree.

## Delivered

| Item | Status / implementation |
|---|---|
| Seven columns | PASS — complete LC order reference, Taipei timestamp, recipient mask, money, independent payment/delivery states, source. |
| Seven task queues + All | PASS — SQL totals after common filters, before cursor; selected bucket affects `total`, not other queue counts. No client aggregation. |
| Five search forms | PASS — reference, last-four phone, recipient, current tracking, frozen SKU; actual browser controls tested. |
| Four filter families | PASS — payment including COD, delivery, actual live claim session, inclusive Taipei date. Filter-bound cursor resets correctly. |
| M20 and drafts | PASS — NOT_STARTED 未付款; MANUAL_UNASSIGNED 備貨中; MERCHANT_SHIPPED 已發貨; three locales; default excludes DRAFT, explicit inspection can include it. |
| Scope / privacy / read-only | PASS — fresh SQL auth fence, scoped RLS, masked list, no full phone, foreign stores cloaked with 404. No added read-side effects. |
| Performance / indexes | PASS — scoped last4/payment-date/tracking indexes; 10k real-PG search below 1s in every measured read. |
| Approved C | PASS — inline expansion retained; MOU01–06 safety/visual checks preserved. No bulk-result drawer. |
| Mobile / visual | PASS — 390px filters collapse, queues wrap, first order above fold; 44px new controls. 1586px ledger. Three locales. Independent fix-round disposition ship. |

API details and queue predicates: `contracts/merchant-orders-v2.md`.
Legacy unversioned GET list/detail remain available. V2 uses `?view=v2`.
Private search uses **read-only POST** `/orders/search?view=v2`, sole JSON `q`,
1KiB body bound, normal Origin/CSRF, no Idempotency-Key. It executes the same SQL
reader; it is not a command. Search/cursor text stays out of URLs and storage;
refresh deliberately clears it. Non-sensitive filters remain in the URL.

## Commands and exit codes (main agent, final relevant source)

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | 0 | `logs/test-node.log` (includes new 3-test v2 suite) |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `logs/admin-tsc.log` (silent success) |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | `logs/storefront-tsc.log` (silent success) |
| `bash scripts/dev/check-gates.sh` | 0 | `logs/check-gates.log`; 57 modes, tracked tests covered |
| `bash scripts/dev/depmap.sh --check` | 0 | `logs/depmap.log` |
| `bash scripts/dev/test-local.sh --browser-merchant-orders-ui` | 0 | `logs/browser-merchant-orders-ui.log`, `logs/playwright-cases.log`; **10/10** |
| `bash scripts/dev/test-focused.sh '^(TestMerchantOrders\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | `logs/focused-pg.log`; **12 PASS, 0 FAIL, 0 SKIP** |
| `go test ./internal/httpapi ./internal/merchantorders ./internal/pagination` | 0 | `logs/go-unit.log` |

The actual focused shell regex uses `|` (without Markdown table escaping).
Native hidden→visible events were trusted: no reads while hidden, PII cleared,
fresh read on return; see `native-visibility.json`. Browser uses real
OIDC→Next→BFF→Go→isolated PG, **MOCK** identity/payment providers, not LIVE.
`click-ledger.json` records controls, visible SQL totals, refresh behavior and
mobile expand/select/apply/reopen/reset in all three languages.

## 10k / red-green evidence

- Genuine initial RED: new `TestMerchantOrdersV2ReadContract` hit the unimplemented
  v2 route and got 422, exit 1 (`red/pg-contract.log`). Same boundary GREEN exit 0
  (`logs/pg-contract-green.log`), also included in final focused pass.
- Final 10k last4 search: n=10, p50 **109.286125ms**, max **142.518209ms**.
- Final current-tracking search: n=10, p50 **78.651166ms**, max **100.571167ms**.
  Bound is <1000ms for every HTTP read, with setup excluded; not a LIVE SLA.
- Fixture: 10k synthetic archived orders paired with released reservations to
  preserve the aggregate FK; plus real captured/shipped and ready-to-ship orders.
  No invented transition used to fake shipping acceptance.
- `ready_to_ship` agrees with an independent SQL count; totals stable over cursor
  pages; same-tenant other-store and cross-tenant isolation; invalid filters,
  mismatched cursor and list-phone leakage negatives; read-only table counts.
- Existing observed-lock revocation adversary retained and extended with v2
  revoked token, missing order grant, and empty result after revoke cases.
- Initial 10k fixture FK failure retained separately in
  `red/pg-v2-initial-projection.log`; it is not the product RED→GREEN proof.

## Evidence / review

Final browser run directory:
`output/playwright/merchant-orders-c-browser/20261002T180311.730337000/`.
Within this delivery directory: `orders-{locale}-{width}x{height}.png` are
document-top captures; `ledger-*` are deliberately scrolled ledger views;
`inline/` contains copied final MOU05 open-detail desktop/mobile captures.
All locales are zh-TW/zh-CN/en; widths 390 and 1586.

`REVIEW.md` records independent SQL/privacy and visual findings and resolution.
Scoped design truth is recorded in `.impeccable/merchant-orders-v2-brief.md`;
global DESIGN.md is preserved. No old assertions were removed or thresholds
loosened. Two precise pre-existing baseline expectations were aligned with the
already-integrated COD DTO (0107's two amount fields) and migration count 0110;
the exact-field privacy assertions remain strict. Legacy draft-inspection cases
explicitly select `all`; the new default-hide-draft assertion is independent.

## Decisions / limits / NOT_RUN

- Completion is conservative: paid picked-up CVS, or recorded COD/PAP collection.
  Manual home shipment alone is not inferred delivered. This is documented and
  shown in UI; no new delivery state machine. Owner semantic confirmation was
  requested but not received, so this remains an explicit integration assumption.
- Queues overlap (e.g. COD awaiting collection and shipping); they must not be
  summed. Live attribution means durable claim purchase, not viewing history.
- Session picker lists latest 100 order-linked sessions, not every unused session.
- Order reference uses the complete `LC-` + UUID hex to avoid collisions; it does
  not fabricate a short database sequence.
- NOT_RUN: full repo race/all-browser release suite, live or sandbox Meta/payment/
  logistics providers, production performance/traffic, bulk-operation M05.
- test-node reports existing R04 binary test NOT_RUN because
  `COMMERCE_R04_LIVEKIT_BINARY` is unset; no LiveKit service/config altered.
- Runtime syntax compile smoke log is not acceptance (`go-compile.log`). Actual
  focused PG and browser cases above supply acceptance.

## Commits

- `223c08d3` — read API/SQL/indexes/cursor and real-PG tests.
- `6cec5db2` — private BFF/UI, three-language responsive ledger, C regression and
  first/cursor-page privacy browser cases, scoped visual documentation.
- Evidence and this summary are in the final documentation commit (its SHA is
  reported in the handoff rather than self-referenced inside its own tree).
