# RVFIX-370 — finding → change → RED run → GREEN run

Base: `c8e3de002` + merge of `origin/r3/integration` (`e8b735f10`). Push 1 = tests only. Implementation is NOT in this push (card order).

| Finding | Planned change (implementation push, not yet made) | RED test (this push) | RED run | GREEN run |
|---|---|---|---|---|
| P1-1 card state hidden in Advanced (`CardPayments.tsx:163,168`) | Move state badge, platform badge, not-open and blocked notes, saved/conflict/error notes and Enable/Disable actions out of `<details data-testid=card-advanced>`; keep limits and descriptor preview inside | `tests/admin/card-payments.spec.ts` CPU5, CPU6, CPU6b, CPU7b: assertions before any click on Advanced | PENDING (CI) | NOT_RUN |
| P1-2 reversed or >91-day range is a dead end (`Finance.tsx:63,71-72,85`) | `failure` computed only when `validQuery`; invalid range keeps the date button/editor and the range-invalid alert | `tests/admin/customers-billing.spec.ts` "RVFIX-370 finance: a reversed-range URL …" (real clicks, reload) | PENDING (CI) | NOT_RUN |
| P2-3 Settlements legend stub (`Settlements.tsx:141,147`) | Restore `Legend` (sign legend + net formula) from `origin/r3/integration`, rendered in List and Detail | `tests/admin/settlements.spec.ts` STL1 `settlements-legend`/`settlements-formula`, STL2 detail `settlements-formula` (restored verbatim) | PENDING (CI) | NOT_RUN |
| P2-7 `generated_at` required (`customers-model.ts:269`) | Optional in `parseFinanceSummary` (absent allowed, present must still be an instant); `FinanceSummary.generated_at?: string` | `tests/admin/customers-model.test.ts` "finance summary without generated_at parses …" | PENDING (CI) | NOT_RUN |

Other P2s (4, 5, 6, 8, 9, 10) go to an issue per the ruling; untouched.

No test deleted or weakened; no `force`/`dispatchEvent`/`evaluate`; money values untouched (server-owned).
Evidence label: all rows NOT_RUN locally (owner rule: tests run only in CI).

## Implementation (relay 1) — RED run: e3a7cd77 CI red (ids: integrator); GREEN run: PENDING (CI)
- P1-1: CardPayments.tsx:167-216 state/platform badges, not-open, blocked, saved/conflict/error notes and Enable/Disable moved outside the disclosure; Advanced keeps descriptor preview + limits.
- P1-2: Finance.tsx:71 `failure` is empty when `!validQuery`; range button, range-invalid alert and date editor stay reachable.
- P2-3: Settlements.tsx:146 `Legend` restored (legend + formula), also rendered in `Detail`.
- P2-7: customers-model.ts:81,268-276 `generated_at` optional; a present value must still be an instant.
