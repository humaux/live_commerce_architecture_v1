# W3-U4 合并出货 UI (parcel-merge UI) — delivery record

- Unit: `docs/delivery/units/w3-u4-parcel-merge-ui.md` (W3-07B amendment, manual-fulfilment-v1)
- Branch: `unit/w3-u4-parcel-merge-ui`, worktree `.worktrees/w3-u4-parcel-merge-ui`, base SHA `49859ae2` (trunk)
- Role/model: UI implementer (ui_worker scope), Kimi K3 (Claude Code harness)
- Allowed paths used: `apps/admin/**`, `tests/admin/**`, `tests/foundation/browser_merchant_orders_ui_test.go`, `scripts/dev/test-local.sh`, `output/w3-u4-parcel-merge-ui/**`. No Go product code, SQL, contracts, go.mod/go.sum, OpenAPI, or lockfiles touched.

## What was delivered

1. **Merge banner** (`apps/admin/components/ParcelGroup.tsx`): when `GET orders/merge-suggestions` returns items, the orders page shows 「N 組訂單可合併出貨」 with the owner-ruling sentence 「只合併包裹，不合併付款或金額；每張訂單仍各自計價、各自收到出貨通知。」 (3 locales), an expand toggle, and suggestion cards (recipient, order short-ids, count, 合併 button → keyed POST create).
2. **Group panels**: members, state badge, 「填寫運單」 form reusing the extracted `ShipmentFields`/`shipmentFieldsError` from `OrderShipment.tsx` (testid prefix `parcel-ship-*`), keyed PUT to the group-shipment route; 「解除合併」 (OPEN only) with confirm pair → CAS DELETE `?expected_version=N`; close for terminal states.
3. **Row badge + single-order block** (`MerchantOrders.tsx`): `parcel-badge-{order_id}` on grouped rows; `OrderDetailPanel` sections gain `parcelBlockText`, so an OPEN-group member shows 「此訂單在合包中，請在合包填寫運單」 instead of the single-order shipment form. Server `in_parcel_group` (409) maps to the identical sentence in `orders-copy` (3 locales).
4. **Libs**: `parcels-model.ts` (strict exact-key parsers, 2..20 caps), `parcels-request.ts` (route grammar + dissolve query validator), `parcels-copy.ts` (3-locale copy incl. every refusal code), `parcels-client.ts` (read + keyed commands + keyless CAS delete; uncertain outcomes never blind-retried, parse failure after commit → `uncertain`).
5. **BFF** (`route.ts`): parcel-aware `DELETE` wrapper (everything else keeps 405/404), exact query rules (only the dissolve DELETE may carry exactly `?expected_version=N`, raw-URL checked), keyless/bodyless GET+DELETE rules, `private, no-store` on parcel responses, parcel routes require real `authConfig` like the other order routes.
6. **Tests**: node unit tests (parsers, grammar, copy parity incl. ruling sentence + in_parcel_group sentence equality); real-click Playwright `tests/admin/parcel-merge.spec.ts`; Go harness fixtures + post-click PG assertions; `test-local.sh` wiring.

## Tests run (this sandbox)

| Test | Command | Result |
|---|---|---|
| parcels-model + parcels-request unit (red) | `node --test --experimental-strip-types …` | FAIL as expected (module not found) — `output/w3-u4-parcel-merge-ui/red.log`, `red-run.txt` |
| parcels-model + parcels-request unit (green) | same | **7 pass / 0 fail** — `output/w3-u4-parcel-merge-ui/green.log` |
| orders-model regression | `node --test --experimental-strip-types tests/admin/orders-model.test.ts` (+ the 2 parcel files) | **23 pass / 0 fail** total |
| gofmt syntax | `gofmt -l tests/foundation/browser_merchant_orders_ui_test.go` | clean (no output) |
| Header ratchet | manual emulation of `scripts/dev/check-headers.sh` against base 49859ae2 | all touched files carry Purpose/Depends on/Used by (orders-copy keeps its pre-existing leading comment, allowed for modified files) |

## NOT_RUN (blocked by sandbox permissions; CI gates must run them)

- `bash scripts/dev/test-local.sh --browser-merchant-orders-ui` — the real browser gate: orders-ui.spec + **parcel-merge.spec.ts real clicks** (banner → merge → group waybill → both members MERCHANT_SHIPPED → reload persistence → dissolve confirm → re-merge → single-order blocked → stale submit 409 `in_parcel_group` copy) + PG post-assertions (1 SHIPPED / 1 OPEN / 1 DISSOLVED group). Go execution, Playwright and pnpm are permission-blocked here.
- `go vet -tags browser ./tests/foundation` (go binary blocked; gofmt parse check passed instead).
- `tsc --noEmit -p apps/admin/tsconfig.json` (tsc blocked; imports/types were cross-checked by hand against settings-client/orders-client/orders-model signatures).
- `bash scripts/dev/check-headers.sh` (script blocked; emulated manually, see above).
- `--browser-merchant-orders-bff` gate re-run (parcels-request.test.ts wired into it; not executed here).
- The click ledger for the UI acceptance is therefore the spec's `test.step` list above — each step is real clicks/fills/selectOptions with user-visible assertions plus a reload-persistence check; it has **not executed** in this sandbox and must go green in CI before merge.

## BLOCKED

- Preamble asked to merge current `r3/integration` (`0d6b4b5f`) before the first gate run: `git merge` is permission-denied in this sandbox. The delta touches only deploy-prep docs/tests/scripts — zero file overlap with this unit, so the merge is trivial for the integrator.

## Design notes / risks

- No group-list route exists (frozen API): group membership is session knowledge learned from create/ship responses; a reload drops the panels and badges. The server guards (`in_parcel_group`, `already_in_group`, CAS) stay the authority — the spec's last step proves the stale-path 409 copy through real clicks.
- The banner/panels render only with a `fulfillment:write` probe (like TrackingImport); Go re-authorizes every write.
- `OrderShipment.tsx` change is extraction-only: same validation rules (`shipmentFieldsError`), same DOM testids (prefix `ship-*` default), one added optional `parcelBlock` prop.
- The Go harness runs the parcel spec **after** the existing read-only row-count assertions, so the MOU read-only guarantee is unchanged; the harness ctx was raised 270s → 540s and the gate `-timeout` 300s → 600s for the second browser run.
