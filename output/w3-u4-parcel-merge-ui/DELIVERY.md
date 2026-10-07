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

---

## Finisher (Sonnet)

- Role/model: Claude Sonnet 5.5 (finisher), same worktree/branch `unit/w3-u4-parcel-merge-ui`, base = Kimi UI + trunk `0d6b4b5f` (`0adb2e8e`). Fixes the independent Opus review that BLOCKED the unit. Migration number 0164 was given by the integrator.
- Evidence class: REAL_PG (backend read, focused) + MOCK (stub upstream for the BFF route test) + BROWSER NOT_RUN (specs written, CI-only).
- Commits: `0a414f43` BFF, `17e0e58e` + docs commit backend (0164), `3b133aa1` UI + model tests, `b4177737` spec + Go harness, `2b181589` header, `1f857118` test-node, `0a5bd9e8` spec label; final SHA in the hand-off message.

### Findings -> change -> test -> result

| Finding | Change | Test (red -> green) | Exit |
|---|---|---|---|
| P1-1 BFF dissolve always 422 (`request.body !== null` on a DELETE; Next 16 gives every non-GET an empty body stream) | `route.ts` ~l.168: only a GET is judged by `request.body === null`; the DELETE keeps the content-length / transfer-encoding / Idempotency-Key fences (its body is never forwarded: `init` has none) | NEW `tests/admin/parcels-bff.test.ts` imports the REAL `[...resource]/route.ts` (node:module `registerHooks` resolves `@/`, extensionless libs, `server-only`, `next/*.js`; stub Go upstream) and sends a DELETE with an empty body stream like Next. Red: `red-finisher-bff.log` (422 !== 200). Green: `green-finisher-bff.log` | red 1 -> green 0 (5/5) |
| P2 CAS query only checked when the URL has `?` | `route.ts`: `parcel === "delete" ? !validParcelDeleteQuery(url) : parcel && url.includes("?")` -> 422 | same file, "every parcel DELETE must carry exactly one expected_version" (9 bad queries incl. none, `?`, 0, 03, dup, extra). Red after fixing only the body fence: `red-finisher-bff-2.log` (200 !== 422); green after | red 1 -> green 0 |
| P1-2 OPEN groups stranded after reload | **0164** `fulfillment.read_open_parcel_groups(p_hash,p_store)` (SECURITY DEFINER, owner commerce_checkout_writer, EXECUTE commerce_runtime only, REVOKE PUBLIC, COMMENT, `orders:read`, final access recheck like its siblings, <=200 OPEN groups newest first, members `{order_id, order_number, recipient_masked}`; mask = the 0110 list expression verbatim). Go `merchantorders.OpenParcelGroups` (+ `normalizeRecipientMask` extracted from `list_v2.go` so list and group read share ONE mask rule; any malformed mask degrades to "—") and route `GET /v1/admin/stores/{store_id}/parcel-groups` (orders:read) in `registerParcelRoutes`. BFF allowlist (exact grammar, no query), strict `parseOpenParcelGroups`, `readOpenParcelGroups`, pure `reconcileGroups`; `ParcelMerge` reads the groups on every load/tick (a read that started before a create is discarded) so ship/dissolve panels, row badges and the single-order hint exist after a reload; the server's version replaces the session's so the dissolve CAS is live | DB-free `TestParcelRoutesTransportRules` +4 cases (401 admitted, `?state=OPEN` 422, bare `?` 422, key 422): red with the route removed `red-finisher-open-route.log` (405), green. REAL_PG `TestParcelGroupOpenRead`: empty -> `"items":[]`; OPEN group listed with version 1 + masked members, exact keys, no name/phone/address/amount in the body; newest first; SHIPPED + DISSOLVED + other store absent (4 groups in PG, 2 listed); no token 401, `fulfillment:write` alone 403, foreign-store member 404, other store (own member) empty; mask identical to the v2 orders list row by row incl. leading U+3000 and a blank name; read writes no rows; EXECUTE limited to commerce_runtime. Red without the migration: `red-finisher-open-read.log` (503), green `green-finisher-open-read.log`. Node: `parcels-model.test.ts` parser + reconcile (red `red-finisher-model.log`: no such exports) | focused PG exit 0 |
| Pins | `r2_integration_upgrade_test.go`: 87 -> **88** with the comment line `0164 (W3-U4 open parcel-group read) adds one more: 87 -> 88` (integrator re-unions with 0162/0163 at merge); `manual_fulfilment_schema_test.go` MF02, `merchant_orders_v2_acl_test.go` PickListReadAuthority and `worker_authority_split_test.go` WAS02 gain the new definer (no function-count pin exists for the fulfillment schema; the T06 count is integration.* only) | `TestR2IntegrationUpgradeFromReleaseHead`, `TestManualFulfilmentMF02Schema`, `TestMerchantOrdersV2PickListReadAuthority`, `TestWAS02*` all PASS in the focused run | 0 |
| Contract | `contracts/manual-fulfilment-v1.md` Amendment W3-07B Routes line + Evidence line; `docs/delivery/units/w3-07b-parcel-merge.md` and `w3-u4-...md` notes | - | - |
| P2 banner shows the full recipient name | Chose the smaller change: **mask client-side at parse time** (`maskRecipient`, the 0110 rule: first non-blank char + `***`, "—" for blank/unprintable; the full name is dropped in `parseMergeSuggestions`, never kept in React state). Not changing `read_merge_suggestions` (0146 is applied and frozen; a CREATE OR REPLACE of a 40-line definer + Go struct + contract field + the 0146 tests would be a larger, riskier diff for the same visible result; the orders:read caller already sees the name in the order detail). Banner and panel members show `S***` | `parcels-model.test.ts` mask vectors (王小明, leading U+3000 / NBSP / zero-width, emoji initial, blank, control) + "the full name must not survive parsing"; spec step A asserts `S***` and no `Synthetic Buyer` | node 0 |
| P2 merge error vanishes when no suggestions remain | `ParcelGroup.tsx`: the banner section (rule + problem) renders while `problem` is set even with 0 suggestions; early return only when nothing to show | spec step "uncertain dissolve reconciled" asserts `parcel-merge-problem` with 0 suggestions of that pair | NOT_RUN (CI) |
| P2 uncertain dissolve | dissolve/ship failure `group_not_open` or `version_changed` -> `onMoved` -> refetch OPEN groups; a group no longer OPEN is dropped (`reconcileGroups`), `group_not_open` leaves its message in the banner slot (the dropped panel takes its own message with it); a changed version is refreshed | node: reconcile "vanished OPEN group is dropped / live version taken / history untouched"; spec: `page.route` fails the click's own DELETE AFTER it reached the server (labelled FAULT INJECTION), same-version retry -> 409 `group_not_open` -> panel gone + banner message | node 0; spec NOT_RUN |
| P2 ruling test only `length > 10` | `parcels-model.test.ts`: exact zh-TW / zh-CN / en ruling sentence; `blocked` and `errors.in_parcel_group` equal the orders-copy sentence in all 3 locales | node | 0 |
| P2 COD/CVS ruling untested in the harness | `browser_merchant_orders_ui_test.go`: three pairs + a COD and a CVS order of the SHIP pair's buyer (same owner and address; they would join its suggestion if mergeable). Disclosed owner-pool fixtures, **probed against the real list/detail/suggestions before use** (a paid card order flipped to COD makes the v2 list 503, so the COD order is an unpaid hold in COD/AWAITING_COLLECTION state; the CVS order is a paid order with destination kind `cvs_711`). Spec asserts banner count 3, the ship card has exactly 2 members and neither excluded id appears; Go asserts they never joined a group nor shipped. Helper names untouched (`parcelOrder`/`parcelPair` are closures) | `go vet -tags browser ./tests/foundation` | vet 0; browser NOT_RUN |
| P2 spec treated the reload dead end as expected | `parcel-merge.spec.ts`: merge -> **reload -> panel reappears** (masked members, live version) -> dissolve; re-merge -> reload -> badge + hint + panel -> **ship from the reloaded panel**; the stale in_parcel_group 409 copy is kept with a real second tab (tab B merges while tab A still shows the single-order form). Go server-state assertions updated (4 shipped members, 2 dissolved, stale pair OPEN and unshipped) | browser | NOT_RUN (CI) |

### Verification run here (focused only)

- `LC_FOCUSED_TIMEOUT=2700s bash scripts/dev/test-focused.sh '^(TestParcel|TestMerchantOrders|TestR2IntegrationUpgradeFromReleaseHead|TestPicklist|TestPickList|TestCarrierExport|TestManualFulfilmentMF02Schema|TestWAS02)'` -> **exit 0**, 29 top-level PASS / 0 FAIL / 0 SKIP (TestParcelGroup, TestParcelGroupACL, TestParcelGroupOpenRead, TestMerchantOrdersV2PickListReadAuthority, TestR2IntegrationUpgradeFromReleaseHead, TestManualFulfilmentMF02Schema, TestWAS02, TestPickList*, TestCarrierExport*) — `green-finisher-focused.log`.
- `go vet ./...` -> 0; `go vet -tags browser ./tests/foundation` -> 0 (`vet-finisher.log` empty); `go test ./internal/httpapi ./internal/merchantorders ./internal/merchanttools ./internal/httperror` -> ok.
- `(cd apps/admin && npx tsc --noEmit -p .)` -> 0.
- `bash scripts/dev/check-gates.sh` -> 0 (headers ratchet OK; `TestParcelGroupOpenRead` runs in the catch-all shard g06, a note, not a failure) — `check-gates-finisher.log`.
- `node --test --experimental-strip-types tests/admin/parcels-model.test.ts parcels-request.test.ts parcels-bff.test.ts orders-model.test.ts orders-request.test.ts orders-v2.test.ts` -> 0, 47 pass / 0 fail — `node-finisher.log`. The three parcel suites are registered in `--browser-merchant-orders-bff`, `--browser-merchant-orders-ui` (test-local.sh) and `test-node.sh`.

### Click ledger delta (AGENTS.md UI rule; all steps are real click/fill/selectOption; the only non-click helpers are the [READ/MEASURE] viewport check and the [FAULT INJECTION] page.route; NOT_RUN here, CI)

| Step | Control / action | Expected |
|---|---|---|
| A banner | read banner, click `parcel-suggestions-toggle` | "3 order groups ..." (COD/CVS excluded), ship card 2 members, `S***`, no full name |
| B merge ship pair | click `parcel-merge-<id>` | panel Open, count "2 ...", member `LC-... · S***` |
| C waybill | `parcel-ship-open` / carrier / tracking / submit | group Shipped, every member MERCHANT_SHIPPED |
| D reload | reload | shipments persist, no OPEN panel (SHIPPED is history) |
| E merge + reload + dissolve | merge, reload, `parcel-dissolve` + `-confirm`, `parcel-close` | panel reappears Open with `LC-... · S***`, dissolves, closes |
| F uncertain dissolve | merge, confirm (request failed after commit), confirm again | "Temporarily unavailable" then panel gone + banner "already been shipped or dissolved" |
| G re-merge + reload + ship | merge, expand, reload, badge + hint, `parcel-ship-*` | panel + badge + hint after reload, both members shipped |
| H stale second tab | tab B merges the stale pair, tab A submits `shipment-submit` | 409 `in_parcel_group` sentence, no shipment record |

### CI gates (integrator; RAM-heavy, NOT run locally)

`--browser-merchant-orders-ui` (orders-ui.spec + parcel-merge.spec + PG assertions), `--browser-merchant-orders-bff`, `--browser-home-cod`, `--browser-picklist`, `--browser-click-sweep`, `--browser-visual-lint`, foundation shards (incl. `TestParcelGroupOpenRead` in g06; full `TestR2IntegrationUpgrade*` and legacy upgrade shards because 0164 adds a function).

### NOT_RUN / BLOCKED

- All browser modes above (parcel-merge.spec steps A-H were written, `tsc`-checked ad hoc and the Go harness `go vet -tags browser`-checked, but never executed). Highest-risk bits for the first CI run: the `route.fetch()` + `abort` fault injection (step F), the second-tab navigation (step H), the COD/CVS owner-SQL fixtures inside the harness (probed in a REAL_PG scratch run against the v2 list, detail and suggestions, but not inside the browser harness), and the harness timeout (still 540s/600s for three pairs plus two extra orders).
- The CVS-flipped fixture order answers 503 on its DETAIL route (a bare `destination.kind` flip has no consistent pickup/shipment rows); nothing in the spec expands it, and the list/suggestions are verified. Left as is rather than faking a CVS shipment.
- `docs/architecture/DEPENDENCIES.md` not regenerated (generated file; the integrator runs `scripts/dev/gen-deps.sh` after merge).
- OpenAPI shared schema not touched (integrator-owned): add `GET .../parcel-groups` next to the four W3-07B routes.

### Integrator to-do

- Migration `0164_open_parcel_groups.sql` (given number); re-union the R2 count (87 -> 88 here) with 0162/0163; privilege pins updated in MF02, PickListReadAuthority, WAS02 (EXECUTE commerce_runtime only).
- Regenerate `docs/architecture/DEPENDENCIES.md`; optionally `node scripts/dev/shard-plan.mjs --write` so `TestParcelGroupOpenRead` leaves the catch-all shard.
- Risk accepted: the open-groups read lists the 200 newest OPEN groups (ponytail comment in the migration); it fails closed (503 at the Go validator) on an OPEN group with <2 or >20 members, which W3-08B's cancel keeps from existing.
