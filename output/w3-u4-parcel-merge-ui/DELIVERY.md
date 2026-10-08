# W3-U4 合并出货 UI (parcel-merge UI) — delivery record

## Current round at 44bb155e — CTUI read-fault race repaired

Merged `origin/r3/integration` **97e34a4c7646de9d0753fb83ed88d56c1b43f01c** (PR #3) with `git fetch origin && git merge origin/r3/integration`, exit **0**, as **5ca3b82cede63d2e691b3b3b88e0da80c945ed20**. No conflicts. The integrator confirmed picklist, Studio and CVS passed Linux CI at 44bb155e; the older Studio/CVS open findings below are historical and closed.

**Root and minimal correction.** CI run **37729726011**, job **113155952226**, failed the `tag-read-503 Retry` assertion before any DELETE. The trace shows the initial catalogue GET starting at 15397.669 ms and receiving 503; `page.reload` starts at 15407.958 ms. The fresh catalogue GET at 15551.698 ms returns 200. `detail()` waited only for the notes container, so an unfinished initial read consumed the next-request fault. The fresh page correctly displayed successful data. This is a fault-injection preparation race, not a BFF or product failure.

The spec now waits for the genuine Seed01 catalogue checkbox and the enabled notes pagination control before arming **each** one-shot read fault. An enabled pagination control proves the independent notes read completed; prepopulated detail rows alone cannot prove that. All original reload, unavailable-copy, explicit Retry, catalogue, cap, pagination and persistence assertions remain. No sleep, timeout, retry budget, product behavior or fixture response was changed. Independent AST review found all **98 original expect chains in their original order**, plus exactly two readiness assertions.

The merged customer-tag product and exact BFF leaves equal trunk; the parcel/inbox catch-all equals the previous PR head. Both route families retain their method/body/permission behavior. `w6-round/source-union-proof.json` records the hashes and comparisons.

**Evidence level E3, real Go/PG with signed MOCK identity and synthetic data.** Source is merge 5ca3b82c plus `tests/admin/customer-tags.spec.ts` SHA-256 **100557a7df9255e42efad7b56d767bbf9172cf43fe53b309dbd067bee4b60e52**. Each result JSON records this source/hash and verifies the file remained unchanged throughout its gate. RED is the original CI failure (`ci-red-playwright.log`, `ci-trace-facts.json`, original trace hash/path). The unchanged local baseline passed: no local RED is claimed. The fixed full mode passes, including both injected 503/Retry cases and **101/101 click-ledger entries**.

| Command | Exit | Evidence under w6-round/ |
|---|---:|---|
| `bash scripts/dev/test-local.sh --browser-customers-billing` (unchanged baseline) | 0 | customers-before.json/log |
| `bash scripts/dev/test-local.sh --browser-customers-billing` (fixed) | 0 | customers-green.json/log; base suite 12 + CTUI 9; green-playwright.log; green-w6ui-click-ledger.json |
| `bash scripts/dev/test-local.sh --browser-picklist` | 0 | picklist.json/log; original real-click case passed |
| `bash scripts/dev/test-local.sh --browser-cvs` | 0 | cvs.json/log; MOCK Chromium and WebKit passed |
| `node --test --experimental-strip-types tests/admin/parcels-bff.test.ts tests/admin/inbox-bff.test.ts tests/admin/customer-tags-bff.test.ts tests/admin/customer-tags-proxy.test.ts tests/admin/w6-route-seam.test.mjs` | 0 | bff-seams.json/log; 46 tests including real Request/handler/auth seam cases |
| `bash scripts/dev/test-node.sh` | 0 | node.json/log; 910 tests across 18 Node test invocations |
| `bash scripts/dev/check-gates.sh` | 0 | check-gates.json/log; **80 modes, all documented; every tracked test file is run** |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | 0 | typecheck.json/log |
| `git diff --check` | 0 | final source and evidence staging check |

Browser modes ran sequentially through the existing shared lock. Test processes ended normally. Own generated CVS prints and gate output were moved to the primary checkout `output/w3-u4-parcel-merge-ui/w6-round/generated/`; no other task files were removed. Raw logs remain in that primary checkout; any normalized committed log view has a lossless `.raw.gz` and hash mapping.

**NOT_RUN / handoff:** New-head Linux CI and K3 pre-push review belong to the integrator. Studio was not rerun in this batch (the integrator confirmed its previous head passed; this change touches only CTUI). CVS SANDBOX, real providers, billing SANDBOX/LIVE and deployment are NOT_RUN. No push, rebase, force-push, production operation or deferred-review scope change. No known remaining P0/P1 in this batch; independent source review is E1 and does not substitute for the integrator's acceptance.

---

## Historical: Inbox trunk union merge (superseded by the current round below)

Merge commit **`4c51ddff25e4cb43590fedcdf565429a00b12d58`** combines local `1ee8afaa12082c66ac6a13b0a8ed2226686e5ff0` with fetched `origin/r3/integration` **`f2ac619f15be832475b37ffabd4bbd497e5c4bb3`**, which contains merged PR #8. No rebase, force-push or push.

Actual conflicts were the shared BFF route-admission condition and Playwright suite table. Both were resolved as a union: retain parcel AND inbox admission, `parcel-merge-ui` with parcel-merge.spec.ts AND `inbox` with inbox-ui.spec.ts/inbox-bundle-ui.spec.ts. The script usage/allow lists, GATES rows and contract merged automatically. Both parents'79 allowed modes and79 documented mode identifiers remain; inbox calibration guidance and the existing merchant-orders/parcel invocation remain intact. No product-choice conflict occurred.

**E3 validation on merge source4c51ddff:**

| Command / check | Exit | Evidence in inbox-union/ |
|---|---:|---|
| `git fetch origin && git merge origin/r3/integration` | initial merge1 (two conflicts), resolution/commit0 | parents and merge SHA in union-proof.json |
| Automated comparison against BOTH parents' mode/usage/GATES sets, both suite mappings and route union | 0 | union-proof.json, source hashes verified |
| `bash -n scripts/dev/test-local.sh` | 0 | shell syntax checked |
| `bash scripts/dev/check-gates.sh` | 0 | **79 modes, all documented; every tracked test file is run**; check-gates.json/log |
| `bash scripts/dev/test-node.sh` | 0 | 792 tests, including both real-route families; node.json/log |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | 0 | typecheck.json/log |
| Diff whitespace check of the resolved/registry files against incoming trunk | 0 | resolution scope only |

The pre-merge whole-index whitespace check returned2 only for incoming historical PR8 raw logs (`cvs-addendum-baseline.log` progress carriage returns and `protocol-private-evidence-typecheck.log` trailing blank line). Those already-merged evidence files were preserved. Resolution source passes its scoped check; no ignored failure is represented as a green whole-index check.

This merge does **not** resolve the previous round's unproven Studio native-picker root. That remains the explicit open item in the following section. Browser/PG reruns after this registry merge are NOT_RUN; this turn's requested mode-documentation gate and the additional Node/type checks are green. Authoritative logs are retained in the primary checkout at `output/w3-u4-parcel-merge-ui/inbox-union/`.

---

## Round packet 973ed2d0 — picklist repaired; Studio root UNRESOLVED

**Partial handoff, not a claim that both CI failures are fixed.** Picklist is repaired with E3 red→green evidence. Studio's first failed operation is identified, but a product root cause has not been proven; no speculative Studio product or acceptance-test change is included. CVS passed Linux CI on973 per integrator and is CLOSED; this round's requested local CVS regression also passed.

### Verified picklist root and repair

The new parcel reads on MerchantOrders reached an incomplete W3-U1b MOCK fixture. It returned404 for `orders/merge-suggestions` and `parcel-groups`; the correct privacy teardown then removed the order row while the first checkbox click was waiting. CI log: round973/picklist/ci-gates/--browser-picklist.log. The fixture now returns contract-valid empty collections for those two GETs, after its existing token and exact-store checks. Missing token remains401, wrong store and unsupported POST remain404. No product privacy behavior changed.

An actual fixture HTTP regression test passes each response through the two production parcel parsers and checks the denial cases. RED404≠200 → GREEN. All52 original Node assertion calls/order and the entire picklist browser spec are unchanged; independent source review receipt `a4f80bac-d039-4162-bfbb-a806bfab38c0`. The original full picklist browser gate now passes all its locale/viewport interactions.

### Studio: precise boundary and remaining gap

CI run37713621244/job113105067253 first fails `studio-ui.spec.ts:198`: the native date picker leaves the date unchanged after click→ArrowRight→Enter. Initial create/PATCH/reload already returned200, and main has not armed lost-ACK or started the worker. The Go errors quoted in the packet are postflight checks of an incomplete scenario, not proof of a changed retry key or bad migration. `authority=0` means no unauthorized forwarded headers. This is not the October5 readiness diagnosis.

Untouched973 passed the complete local Studio gate. Relevant Studio/CSS/spec/harness files are byte-identical to trunk18f82636; source hashes and precise CI coordinates are retained. macOS native-control experiments with/without tracing gave existing handler30/30, preventDefault30/30; native Linux ARM gave existing handler15/15 and preventDefault14/15. The native-only negative controls did not select a date. These results do not justify a handler change. Linux amd64 under local ARM emulation did not start Node/Chromium, so it is NOT_RUN; the native ARM diagnostic required releasing the isolated xvfb-run PID1 readiness wait after its X socket existed and is not equivalent to GitHub x64. No experiment changed Studio assertions or waits.

See `round973/STUDIO-DIAGNOSIS.md`. A reproduced Linux failure still needs native popup/key-routing and input/change/value-write chronology to distinguish focus/activation from controlled-state overwrite. Existing CI artifacts do not capture that boundary. **Local PASS does not close the unproven Studio root; this batch should not be represented as both findings fixed.**

### Commands and exit codes

| Command | Exit | Evidence under round973/ |
|---|---:|---|
| `node --test --experimental-strip-types --test-name-pattern='pick fixture serves the scoped parcel' tests/admin/picklist-model.test.ts` | 1 RED → 0 GREEN | pick-fixture-red.log, pick-fixture-green.log, pick-fixture-red-green.json |
| `bash scripts/dev/test-local.sh --browser-picklist` | 0 | picklist-green.json/log, local-picklist/ |
| `bash scripts/dev/test-local.sh --browser-studio-ui` (untouched973 baseline) | 0 | studio-baseline.json/log, local-studio/; not a repaired Studio claim |
| `bash scripts/dev/test-local.sh --browser-cvs` | 0 | cvs-green.json/log, local-cvs/; MOCK+WebKit PASS, SANDBOX NOT_RUN |
| `bash scripts/dev/test-node.sh` | 0 | node.json/log; 649 tests |
| `bash scripts/dev/check-gates.sh` | 0 | check-gates.json/log |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | 0 | typecheck.json/log |
| Strict changed-test TypeScript check, with allowJs to resolve the existing .mjs fixture | 0 | exact argv in test-typecheck-with-js.json; initial helper configuration without allowJs failed existing imports (test-typecheck.json/log), preserved |
| Native picker diagnostic commands | 0 completion | native-picker-*.json and Linux command JSONs; completion is not every variant passing |
| Linux amd64 diagnostic before Node startup | 137 / NOT_RUN | linux-native-not-run.txt; removed only the owned labelled container |
| `git diff --check` | 0 | final source/evidence check |

Raw build logs contained carriage-return progress lines and trailing whitespace, so the initial staged diff check exited2. Committed text views are whitespace-normalized; original bytes remain in the primary archive and lossless `.raw.gz` files, mapped by `round973/normalized-log-views.json`. The final staged check exits0.

All browser modes were run serially. Only `tests/admin/picklist-fixture.mjs` and `tests/admin/picklist-model.test.ts` change executable repository test code. Studio/product/migrations/CVS code is unchanged. Gate manifests bind973 plus exact two-file hashes. MAIN `/Volumes/data/live_commerce_architecture_v1/output/w3-u4-parcel-merge-ui/round973/` retains287 evidence files in archive-hashes.json, including downloaded CI and local screenshots/traces; committed files are the top-level logs/manifests/diagnostic sources, not bulk archives.

No push. Owned processes and labelled probe containers ended; generated picklist/CVS files moved to the retained evidence tree. Pinned public Playwright images remain as dependency cache; other containers and caches were not cleaned. Optional media R04, provider SANDBOX/LIVE, and a new Linux CI run are NOT_RUN. Await integrator review/ruling for the unproven Studio item.

---

Migration references in historical prose below are normalized to the current allocation **0166**. Original command outputs and their historical counts/checksums remain unchanged; current merged R2 pin is **91** (0165 already merged).

## Codex-4 migration addendum 2 — 2026-10-08 (current)

**E3: requested migration-sensitive REAL_PG set passed, 9 PASS / 0 FAIL / 0 SKIP.** This addendum supersedes the earlier migration allocation and R2 pin. The separate CVS test-synchronization scope question remains pending; no CVS product/test changes are included here.

- `git fetch origin && git merge origin/r3/integration`: fetched `18f82636efcda03351e141d9bbd14b523161dd26`, which includes PR #12 `9dcd6c54`; merge commit `28a154bff04944eca03b19040202faadea1b3f68`. The only conflict was the R2 comment/pin; retained trunk 90 first to capture the actual RED count mismatch before correcting it.
- Renamed this unmerged parcel migration to `migrations/0166_open_parcel_groups.sql`; current code, SQL version labels, test comments, contract and unit-doc references now use 0166. SQL logic is unchanged. R2 pin is exactly **91**, after merged 0165's 90.
- CRP02 manual hold-back now records 0166 with its real checksum, removes it from the ledger for the ordered populated upgrade, applies it, and asserts exactly one checksum-matching ledger row. KC03 and MCI02 derive held migrations by numeric glob, automatically including 0166.
- All **155 files under merged trunk's migrations directory** compare byte-for-byte equal (including 0165); relative to trunk the only migration addition is 0166. `codex4-migration-integrity.json` records trunk/base hashes, every verified migration path, changed-source hashes, and all nine selected test names. Historical raw logs/hash evidence remain original.

| Command | Exit | Evidence |
|---|---:|---|
| `git fetch origin && git merge origin/r3/integration` | fetch 0; initial merge 1 (one conflict), resolution/commit 0 | `28a154bf`; ancestor check for 9dcd6c54 exit 0 |
| `bash scripts/dev/test-focused.sh '^TestR2IntegrationUpgradeFromReleaseHead$'` (before correction) | **1 RED** | actual migration count 91, expected 90; `codex4-migration-pin-red.*` |
| `bash scripts/dev/test-focused.sh '^(TestR2IntegrationUpgradeFromReleaseHead\|TestLiveClaimsKC03Schema\|TestClaimsRetentionCRP02Schema\|TestParcel.*)$'` | **0 GREEN** | 9 PASS, 0 FAIL, 0 SKIP; `codex4-migration-focused-green.*` (exact argv in JSON) |
| `bash scripts/dev/test-node.sh` | 0 | 648 tests; `codex4-migration-node.*` |
| `bash scripts/dev/check-gates.sh` | 0 | `codex4-migration-check-gates.*` |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | 0 | `codex4-migration-typecheck.*` |
| `git diff --check` and source-hash verification | 0 | Source bytes match focused-gate manifest |

NOT_RUN after this merge/renumber: browser modes and Linux CI (prior browser results remain bound to their recorded earlier source). The user's migration-sensitive REAL_PG set and local Node/structural/type gates above were rerun. No push, deployed/shared database mutation or modification of a merged migration.

---

## Codex-4 owner batch — 2026-10-08 (current handoff)

**Status: local requested gates GREEN; privacy P1 fixed (E3, MOCK/REAL_PG). Original Linux CVS close-stall root cause remains UNKNOWN.** This record does not claim that the historical CI failure has a proven product fix. Integrator K3 review and same-head Linux CI remain required; no push or production action performed.

- Owner/worktree/branch: Codex-4, `.worktrees/w3-u4-parcel-merge-ui`, `unit/w3-u4-parcel-merge-ui`; fetched and merged remote `b4495ba2` before changes. Primary model/effort is not exposed by this runtime; independent scoped roles: test_worker, platform_explorer, security_reviewer. Current tested source is `a6bd46d8154fb2b05f3c80adc1c1ba7ab099e7e6`; exact changed-source hashes are in `codex4-source-hashes.json` (this final commit adds evidence/docs only).
- Commits: `75b9de32` private-scope teardown; `cf84032f` fresh-checkout fixture isolation; `e27728c5` actual parcel suite registration; `a6bd46d8` deterministic observation of real refusal before its real group refresh.
- Product change: either parcel read returning signed-out/forbidden/not-found aborts its sibling and synchronously revokes parent order scope. Order list/detail, selections, search, groups, capabilities and session boundary clear; delayed siblings, order polls and capability reads cannot repaint. Ordinary 503 remains a local Retry. Only `MerchantOrders.tsx` and `ParcelGroup.tsx` change product behavior.
- Privacy proof: independent test uses the actual lifecycle source, actual clients and real `OrderReadError` (MOCK network): RED 7 failures + 2 controls, GREEN 9/9. Real browser adds six denial cases (both reads × 401/403/404) with actual Go sibling responses and the real order poll held until private UI is gone, then checks again after completion. Complete MOU/parcel browser scenario now passes.
- Required gate repairs uncovered on execution: original MOU proof deliberately mutates catalogue price to 99999; later parcel checkout incorrectly reused it. Restore only fresh-checkout catalogue/service inputs after original frozen-order/read-only proofs; focused real PG test reproduces contamination and checks old snapshot unchanged. The Go-selected `parcel-merge-ui` suite was absent from Playwright config; register it and test actual CLI collection. Finally, an actual 409 followed by a group 200 about 19 ms later replaces the form containing the old error. The test holds that unchanged response, observes the original error first, then releases it for original group/hint assertions. All 121 previous assertion chains remain, with three actual-response checks added. These repairs are test fixture/configuration changes, not explanations of the CVS CI failure.
- Deferred comments `4212524352`, `4212524369`, `4212524378` untouched. No Go product, migration, API, OpenAPI or dependency/lockfile changes.

### Commands and exit codes

| Command | Exit | Result / evidence |
|---|---:|---|
| `git fetch origin && git merge origin/unit/w3-u4-parcel-merge-ui` | 0 | Fast-forward to b4495ba2; no push |
| `node --test --experimental-strip-types tests/admin/parcels-scope.test.ts` | 1 → 0 | 7 RED + 2 controls → 9 GREEN; `codex4-scope-test-red.log`, `codex4-scope-parent-green.log` |
| `bash scripts/dev/test-focused.sh '^TestParcelFixtureSameBuyerTwoOrders$'` | 1 → 0 | REAL_PG polluted-price reproduction; unchanged final Go file hashes in `codex4-fixture-price-green.json` |
| `node --test --experimental-strip-types tests/admin/parcels-discovery.test.ts` | 1 → 0 | Actual Playwright CLI rejects missing suite, then collects exact parcel case; discovery logs |
| `bash scripts/dev/test-local.sh --browser-cvs` | 0 | Untouched b449 baseline and e277 product-final run both pass MOCK + WebKit; `codex4-cvs-baseline.*`, `codex4-cvs-final.*` |
| `bash scripts/dev/test-local.sh --browser-merchant-orders-ui` | 0 | MOU r4: 10 original cases + complete parcel case including six scope denials; `codex4-mou-r4.*` binds final spec hash. Earlier r1/r2/r3 are retained RED evidence |
| `bash scripts/dev/test-local.sh --browser-order` | 0 | 23 UI/history cases and real PG exact counts; `codex4-order-final.*` |
| `bash scripts/dev/test-node.sh` | 0 | 648 tests; `codex4-node-final.*` |
| `bash scripts/dev/check-gates.sh` | 0 | Mode/test/header coverage; `codex4-check-gates-final.*` |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | 0 | `codex4-typecheck-final.*` |
| Strict TypeScript API check of three new/changed test files using admin tsconfig | 0 | Exact executable argv: `codex4-tests-typecheck-final.json`; log alongside |
| `git diff --check` | 0 | No whitespace errors |

### Evidence and unresolved boundary

- **Original CVS CI:** preserved trace shows popup `Page.close` waiting about 172 seconds. The later `goto` starts after the 180-second deadline; no orders render loop is proven. Original local b449 and final product source both pass. No CVS test wait or print behavior changed. See `codex4-cvs-diagnosis.md` and `.json`; integrator must classify/retest the original Linux close stall. This item is UNKNOWN, not silently CLOSED.
- Authoritative retained evidence root: `/Volumes/data/live_commerce_architecture_v1/output/w3-u4-parcel-merge-ui/`. Browser archives: `/Volumes/data/live_commerce_architecture_v1/output/playwright/pr2-local-runs/`; complete file hashes: `codex4-evidence-archive.json`. Original CI trace: `/Volumes/data/live_commerce_architecture_v1/output/playwright/pr2-ci-37654972345/trace.zip`.
- MOU GREEN archive: `merchant-orders-c-browser/20261007T235521.914375000/` (Playwright logs + native visibility). CVS GREEN: `taiwan-cvs/20261007T235324.193982000/`; order: `buyer-order-4227139714/`. RED MOU runs also retained. All fixture identities are synthetic.
- Independent source-only review: no scoped P0/P1; AST comparison confirms 121 existing assertion chains retained (E1, not a second runtime acceptance). Humaux receipt `1c0c55a1-1e67-47c6-bc3c-e8f85a2da679`.
- NOT_RUN: CVS SANDBOX/live providers (owner test keys absent), optional media R04 binary suite, post-handoff Linux CI/K3 review. No production/live customer messages or deployment.
- Cleanup: test scripts ended; own generated visuals archived with hashes, then only 22 tracked generated-output paths restored. Own untracked CVS print output moved to archive. Other worktrees/processes/caches untouched.

---

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

- Role/model: Claude Sonnet 5.5 (finisher), same worktree/branch `unit/w3-u4-parcel-merge-ui`, base = Kimi UI + trunk `0d6b4b5f` (`0adb2e8e`). Fixes the independent Opus review that BLOCKED the unit. Migration number 0166 was given by the integrator.
- Evidence class: REAL_PG (backend read, focused) + MOCK (stub upstream for the BFF route test) + BROWSER NOT_RUN (specs written, CI-only).
- Commits: `0a414f43` BFF, `17e0e58e` + docs commit backend (0166), `3b133aa1` UI + model tests, `b4177737` spec + Go harness, `2b181589` header, `1f857118` test-node, `0a5bd9e8` spec label; final SHA in the hand-off message.

### Findings -> change -> test -> result

| Finding | Change | Test (red -> green) | Exit |
|---|---|---|---|
| P1-1 BFF dissolve always 422 (`request.body !== null` on a DELETE; Next 16 gives every non-GET an empty body stream) | `route.ts` ~l.168: only a GET is judged by `request.body === null`; the DELETE keeps the content-length / transfer-encoding / Idempotency-Key fences (its body is never forwarded: `init` has none) | NEW `tests/admin/parcels-bff.test.ts` imports the REAL `[...resource]/route.ts` (node:module `registerHooks` resolves `@/`, extensionless libs, `server-only`, `next/*.js`; stub Go upstream) and sends a DELETE with an empty body stream like Next. Red: `red-finisher-bff.log` (422 !== 200). Green: `green-finisher-bff.log` | red 1 -> green 0 (5/5) |
| P2 CAS query only checked when the URL has `?` | `route.ts`: `parcel === "delete" ? !validParcelDeleteQuery(url) : parcel && url.includes("?")` -> 422 | same file, "every parcel DELETE must carry exactly one expected_version" (9 bad queries incl. none, `?`, 0, 03, dup, extra). Red after fixing only the body fence: `red-finisher-bff-2.log` (200 !== 422); green after | red 1 -> green 0 |
| P1-2 OPEN groups stranded after reload | **0166** `fulfillment.read_open_parcel_groups(p_hash,p_store)` (SECURITY DEFINER, owner commerce_checkout_writer, EXECUTE commerce_runtime only, REVOKE PUBLIC, COMMENT, `orders:read`, final access recheck like its siblings, <=200 OPEN groups newest first, members `{order_id, order_number, recipient_masked}`; mask = the 0110 list expression verbatim). Go `merchantorders.OpenParcelGroups` (+ `normalizeRecipientMask` extracted from `list_v2.go` so list and group read share ONE mask rule; any malformed mask degrades to "—") and route `GET /v1/admin/stores/{store_id}/parcel-groups` (orders:read) in `registerParcelRoutes`. BFF allowlist (exact grammar, no query), strict `parseOpenParcelGroups`, `readOpenParcelGroups`, pure `reconcileGroups`; `ParcelMerge` reads the groups on every load/tick (a read that started before a create is discarded) so ship/dissolve panels, row badges and the single-order hint exist after a reload; the server's version replaces the session's so the dissolve CAS is live | DB-free `TestParcelRoutesTransportRules` +4 cases (401 admitted, `?state=OPEN` 422, bare `?` 422, key 422): red with the route removed `red-finisher-open-route.log` (405), green. REAL_PG `TestParcelGroupOpenRead`: empty -> `"items":[]`; OPEN group listed with version 1 + masked members, exact keys, no name/phone/address/amount in the body; newest first; SHIPPED + DISSOLVED + other store absent (4 groups in PG, 2 listed); no token 401, `fulfillment:write` alone 403, foreign-store member 404, other store (own member) empty; mask identical to the v2 orders list row by row incl. leading U+3000 and a blank name; read writes no rows; EXECUTE limited to commerce_runtime. Red without the migration: `red-finisher-open-read.log` (503), green `green-finisher-open-read.log`. Node: `parcels-model.test.ts` parser + reconcile (red `red-finisher-model.log`: no such exports) | focused PG exit 0 |
| Pins | `r2_integration_upgrade_test.go`: 87 -> **88** with the comment line `0166 (W3-U4 open parcel-group read) adds one more: 87 -> 88` (integrator re-unions with 0162/0163 at merge); `manual_fulfilment_schema_test.go` MF02, `merchant_orders_v2_acl_test.go` PickListReadAuthority and `worker_authority_split_test.go` WAS02 gain the new definer (no function-count pin exists for the fulfillment schema; the T06 count is integration.* only) | `TestR2IntegrationUpgradeFromReleaseHead`, `TestManualFulfilmentMF02Schema`, `TestMerchantOrdersV2PickListReadAuthority`, `TestWAS02*` all PASS in the focused run | 0 |
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

`--browser-merchant-orders-ui` (orders-ui.spec + parcel-merge.spec + PG assertions), `--browser-merchant-orders-bff`, `--browser-home-cod`, `--browser-picklist`, `--browser-click-sweep`, `--browser-visual-lint`, foundation shards (incl. `TestParcelGroupOpenRead` in g06; full `TestR2IntegrationUpgrade*` and legacy upgrade shards because 0166 adds a function).

### NOT_RUN / BLOCKED

- All browser modes above (parcel-merge.spec steps A-H were written, `tsc`-checked ad hoc and the Go harness `go vet -tags browser`-checked, but never executed). Highest-risk bits for the first CI run: the `route.fetch()` + `abort` fault injection (step F), the second-tab navigation (step H), the COD/CVS owner-SQL fixtures inside the harness (probed in a REAL_PG scratch run against the v2 list, detail and suggestions, but not inside the browser harness), and the harness timeout (still 540s/600s for three pairs plus two extra orders).
- The CVS-flipped fixture order answers 503 on its DETAIL route (a bare `destination.kind` flip has no consistent pickup/shipment rows); nothing in the spec expands it, and the list/suggestions are verified. Left as is rather than faking a CVS shipment.
- `docs/architecture/DEPENDENCIES.md` not regenerated (generated file; the integrator runs `scripts/dev/gen-deps.sh` after merge).
- OpenAPI shared schema not touched (integrator-owned): add `GET .../parcel-groups` next to the four W3-07B routes.

### Integrator to-do

- Migration `0166_open_parcel_groups.sql` (given number); re-union the R2 count (87 -> 88 here) with 0162/0163; privilege pins updated in MF02, PickListReadAuthority, WAS02 (EXECUTE commerce_runtime only).
- Regenerate `docs/architecture/DEPENDENCIES.md`; optionally `node scripts/dev/shard-plan.mjs --write` so `TestParcelGroupOpenRead` leaves the catch-all shard.
- Risk accepted: the open-groups read lists the 200 newest OPEN groups (ponytail comment in the migration); it fails closed (503 at the Go validator) on an OPEN group with <2 or >20 members, which W3-08B's cancel keeps from existing.

---

## Finisher 2 (Sonnet) — P1-A: merge-suggestions leaked the full recipient name

- Role/model: Claude Sonnet 5.5, same worktree/branch, HEAD before this round `03277408`. Fixes the independent Opus review (MERGE-AFTER-FIXES) P1-A and the MOU browser-gate failure (CI run 37609774297 job 112754029498: all 10 orders-ui.spec cases fail at `expect(detailCalls).toBe(before)` 0 -> 1).
- Evidence class: REAL_PG (backend) + node (UI model) + BROWSER NOT_RUN (CI).
- Root cause: `fulfillment.read_merge_suggestions` (0146) returned the FULL `recipient_name`; the orders page fetches it on every load, so a detail-level field reached the browser without an expand. The previous finisher's client-side `maskRecipient` only hid it on screen (the 0146 "frozen" argument is void: 0166 is unmerged and applied to no shared DB, so CREATE OR REPLACE inside 0166 is safe). The harness also counted the suggestions path as a detail read (any `/orders/<x>`).

| Change | File | Test (red -> green) |
|---|---|---|
| 0166 `CREATE OR REPLACE FUNCTION fulfillment.read_merge_suggestions(bytea,uuid)` emits `{recipient_masked, order_ids}`; body = 0146's, mask applied AFTER the GROUP BY on `min(c.recipient_name)` with the 0110 expression and escapes verbatim; `SECURITY DEFINER SET search_path=pg_catalog` repeated, then OWNER/REVOKE/GRANT/COMMENT re-run (COMMENT says recipient_masked) | `migrations/0166_open_parcel_groups.sql` | NEW REAL_PG `TestParcelGroupMergeSuggestionsMasked`: 3 buyers (plain, leading U+3000, blank name) -> exactly `{recipient_masked, order_ids}`, masks `王***` / `王***` / `—`, equal to the v2 orders-list mask per order, raw body free of the full name, phone, `recipient_name`. Red on the old function: `red-finisher2-suggestions-mask.log` (leaks 王小明, exit 1). Green: `green-finisher2-focused.log` |
| P2: the open-groups function's regex used invisible literal U+200B..U+200F/U+2028/U+FEFF characters -> the 0110 escaped form (behaviour identical, `TestParcelGroupOpenRead` still green incl. U+3000/blank rows) | same | `TestParcelGroupOpenRead` |
| `MergeSuggestion.RecipientMasked` (`json:"recipient_masked"`), each item validated with `normalizeRecipientMask` like `OpenParcelGroups` | `internal/merchantorders/parcels.go` | REAL_PG test above (through the real HTTP handler) |
| UI: `parseMergeSuggestions` takes `{recipient_masked, order_ids}` with the same mask-shape check as `parseOpenParcelGroups` (shared `maskedRecipient`); the pre-0166 `{recipient_name}` shape and a full name in the mask slot are refused; `maskRecipient`, `leadingBlank`, `recipient()` and their test vectors DELETED | `apps/admin/lib/parcels-model.ts`, `tests/admin/parcels-model.test.ts` | red against the old model `red-finisher2-model.log` (1 fail), green 9/9 |
| Contract: suggestions DTO is `{items:[{recipient_masked, order_ids}]}` | `contracts/manual-fulfilment-v1.md` (Routes line) | - |
| Harness: only the exact `/orders/merge-suggestions` suffix is exempt from `detailCalls` (list-level, server-masked read); the wrapper records that endpoint's response bodies and fails the run (`t.Errorf` in the handler, `t.Fatalf` after the parcel run) if one contains `Synthetic Buyer`, `900000001` or `recipient_name`, or if none was observed (non-vacuous). `orderCalls` / `stripped` unchanged | `tests/foundation/browser_merchant_orders_ui_test.go` | `go vet -tags browser ./tests/foundation` exit 0; browser NOT_RUN |

### Commands (this sandbox) and exit codes

- `go build ./...` -> 0; `go vet ./internal/merchantorders ./internal/httpapi` -> 0; `go vet -tags browser ./tests/foundation` -> 0; `gofmt -l` clean.
- `go test ./internal/httpapi -run TestParcel -count=1` -> ok; `go test ./internal/merchantorders -count=1` -> ok.
- `LC_FOCUSED_TIMEOUT=2700s bash scripts/dev/test-focused.sh '^(TestParcel|TestManualFulfilmentMF02Schema|TestMerchantOrdersV2PickListReadAuthority|TestWAS02|TestR2IntegrationUpgrade|TestT06)'` -> **exit 0**, 33 top-level PASS / 0 FAIL / 0 SKIP (new test, TestParcelGroupOpenRead, TestParcelGroup, TestParcelGroupACL, MF02 unchanged, PickListReadAuthority, WAS02, TestR2IntegrationUpgradeFromReleaseHead (pin stays 89), all TestT06* (function-count pin untouched)) — `green-finisher2-focused.log`.
- `node --test --experimental-strip-types tests/admin/parcels-model.test.ts parcels-request.test.ts parcels-bff.test.ts orders-model.test.ts` -> 0, 32 pass / 0 fail (`node-finisher2.log`); `(cd apps/admin && npx tsc --noEmit -p .)` -> 0.
- `bash scripts/dev/check-headers.sh` -> 0; `bash scripts/dev/check-gates.sh` -> 0 (`TestParcelGroupMergeSuggestionsMasked` lands in catch-all shard g06, a note).

### NOT_RUN

- `bash scripts/dev/test-local.sh --browser-merchant-orders-ui` (orders-ui.spec x10 + parcel-merge.spec A-H + the new response-leak guard): owner rule 2026-10-06 (AGENT-PREAMBLE §2) puts every `--browser-*` mode on GitHub, not the dev Mac. It must go green in CI; confirm `playwright-parcel.log` lists parcel spec steps A-H. The spec itself needed no change (it asserts `S***`, which the server mask produces for the fixture's `Synthetic Buyer`).

### Integrator to-do

- No new migration file (R2 pin stays 89; T06 function count unchanged: the suggestions function is replaced, not added). 0166 now also carries the masked `read_merge_suggestions`; if 0166 was ever applied to a shared DB, ship the replace as a new migration instead.

---

## Finisher 3 (Sonnet) — parcel fixture `conflicting request or version` (CI 37612062730 at 5bf9a747 and 44b89695)

Two independent fixture bugs, both in the browser harness' `parcelOrder` (product code is correct; no assertion changed). The `t.Helper` chain hid the failing step; the new REAL_PG test `TestParcelFixtureSameBuyerTwoOrders` (`tests/foundation/parcel_fixture_test.go`, no browser) gives every step its own message.

1. **One-unit orders cannot start payment.** A unit costs 1250 minor; `checkout.validPaymentResult` requires `AmountMinor%100==0` (whole TWD), so `StartPayment` on a quantity-1 order returns `command.ErrConflict`. Red: `red-finisher3-fixture.log` (`pf[a] start: conflicting request or version`, the FIRST order). The MOU `newDraft(1)` drafts never start payment, which is why the quantity-1 parcel orders were the first to hit it. Fix: parcel orders have quantity 2 (as the MOU `authorized/captured/...` orders do).
2. **`bcHarness.prepare` is single-use per capability.** The cart and destination are created at version 0, so a second `prepare` for the same buyer conflicts; `clone := q` also dropped the cart/destination versions between calls. Red (after fixing 1): `red-finisher3-fixture-2.log` (second order). Fix: `pfBuyer` keeps one buyer's state; the 2nd+ order bumps the cart and re-selects the same `bdHome` destination at the live versions (as `hcodRehome` does), then re-quotes. The distinct-tag change of 44b89695 was not the cause (keys are random per call).

`parcelOrder` in `browser_merchant_orders_ui_test.go` now delegates to `pfBuyer.order` (one buyer per capability token), so the browser harness and the focused test run the same code.

Commands: `bash scripts/dev/test-focused.sh '^(TestParcel|TestManualFulfilmentMF02Schema|TestMerchantOrdersV2PickListReadAuthority|TestWAS02|TestR2IntegrationUpgrade|TestT06)'` -> exit 0, 34 PASS (33 earlier + the new test; `green-finisher3-focused.log`); `go vet -tags browser ./tests/foundation` -> 0; `go vet ./tests/foundation` -> 0; `check-gates.sh` -> 0. NOT_RUN: the browser gate (CI).

---

## Finisher 4 (Sonnet) — PR #2 round: MOU root cause 2, Codex P2s, trunk merge

- Trunk merged (`origin/r3/integration` 1aad42d0, PR #6 + PR #4 0163): clean, R2 pin is 90 (0163 + 0166).
- **MOU run 37623124987 root cause** (log: `pf[ship-a] begin: conflicting request or version`, the FIRST parcel order): the harness flips the shared delivery service to `cvs_familymart` (version 2) for its pickup fixture before the parcel stage, and `bcHarness.prepare` hard-codes `ServiceVersion: 1`; `checkout.go:315` refuses a stale service version (and a home destination on a CVS service). Reproduced REAL_PG in `TestParcelFixtureSameBuyerTwoOrders` (flip the service first): red `red-finisher4-service-flip.log`. Fix: `pfRestoreHome` flips the service back to home (expected version 2 -> 3) and `pfBuyer` stamps every checkout input with that live version; the harness calls it before the parcel stage. parcel-merge.spec.ts never ran in that CI (setup failed first), so there is still no browser evidence for steps A-H.
- Codex P2 4207192912: `groupOfOrder` (parcels-model.ts) prefers the OPEN group over retained history; MerchantOrders `groupOf` uses it. Red: `red-finisher4-groupof.log` (missing export), green in `node-finisher4.log`.
- Codex P2 4207192932: `ParcelMerge.commit()` moves `groupsRef.current` before `onGroups`; `replace`/`dismiss`/create/reconcile all start from the ref, never from the render-time `groups` closure.
- Codex P2 4207192949: a failed suggestions read keeps a visible `suggestionsUnavailable` alert (3 locales) plus a `parcel-reload-retry` button (also for the groups-load error); a later successful read clears only its own error. Copy parity asserted in `parcels-model.test.ts`.
- Not covered by a red test: the two ParcelGroup.tsx fixes are component state logic with no React test harness in the repo; they are type-checked and rely on the browser gate. No spec step was added for them (cannot be run locally).
- Exit codes: `go vet -tags browser ./tests/foundation` 0; `go build ./...` 0; node (parcels-model/request/bff, orders-model, orders-v2) 37 pass 0; `tsc --noEmit` 0; `check-gates.sh` 0; `check-pkgdocs.sh` 0; `depmap.sh --check` 0 (up to date); focused PG set (`TestParcel*`, MF02, PickListReadAuthority, WAS02, `TestR2IntegrationUpgrade*`, `TestT06*`) 0, 34 PASS.
- NOT_RUN: `--browser-merchant-orders-ui` (CI).

---

## Finisher 5 (Sonnet) — PR #2 round 3: foundation red + three more Codex comments

- **Foundation red (run 37633660814, "Node unit suites"):** `tests/admin/card-payments-request.test.ts` pins the BFF route source text. My parcel auth fence appended `|| parcel` right after `discoveryRoute.test(path)` and the cache-header chain put `parcel` before `storefront`, so two pinned regexes stopped matching. Reproduced locally (red), fixed by moving `parcel` to the front of the auth chain and after `storefront` in the Cache-Control chain; the test is unchanged. I had not run `test-node.sh` before; it is now in the push gate.
- P1 4207932348: `parcel-merge.spec.ts` dissolve step now clicks Dissolve, sees the confirmation, clicks `parcel-dissolve-back`, asserts the confirmation closed, the group Open and its ship/dissolve actions enabled, then dissolves for real. Browser step: NOT_RUN locally (CI); no red run is possible without the browser.
- P2 4207932360: `MerchantOrders` passes `refreshGen = refresh + pollGen` (Refresh button/retry/writes bump `refresh`, every successful page-one poll bumps `pollGen`) into `ParcelMerge`, which includes it in the read effect dependencies.
- P2 4207932374: suggestion cards show `parcelOrderNumber(id)` (`LC-<32 uppercase hex>`), the same function the OPEN-group parser now uses. Red: `red-finisher5-ordernumber.log`; green `node-finisher5.log`.

---

## Finisher 6 (Sonnet) — Codex P1 4208064711: every group-waybill control in the spec

- `parcel-merge.spec.ts`: both shipped groups now type every control with real input. SHIP group = the `other` branch (carrier name, tracking, https link, note), preceded by two visible refusals (`other` without a name; a non-https link) with the group still Open. Re-merged DISSOLVE-pair group = a known carrier (black_cat, no name) with link and note. After a reload each member's persisted values are read back: record (carrier label, tracking, link href/host), history (note) and the correction form prefilled from the stored head (all five fields); nothing is submitted.
- `browser_merchant_orders_ui_test.go`: PG assertion that both members of each shipped group carry the typed carrier, name, tracking, link and note on their head version.
- NEW REAL_PG `TestParcelGroupWaybillFieldsPersist` (no browser): group ship with `other` + name + link + note and with a known carrier persists on every member; a non-https link is refused (4xx) and the group stays OPEN. It passed first time (`green-finisher6-waybill.log`): the product code for these controls was not broken, so no product change was needed.
- NOT_RUN: the browser gate (spec steps never executed here).

---

## Finisher 7 (Sonnet) — Codex P1 4208172280: `parcel-reload-retry` is clicked in the spec

`parcel-merge.spec.ts` gains two steps, each with ONE labelled `page.route` fault injection (one 503 on a page load), then the route is removed and the real Retry click reaches Go/PG: (1) GET `orders/merge-suggestions` 503 -> error copy + `parcel-reload-retry`, no cards; Retry -> banner count 1 and the stale pair's card; (2) GET `parcel-groups` 503 (after tab B left the stale pair in an OPEN group) -> error copy + Retry, no panel; Retry -> the OPEN panel with its member number/mask, the row badge and the single-order block. Both wait for a new 200 response and a request-counter increase (not a cache hit). Browser steps: NOT_RUN locally (CI).

---

## Finisher 8 (Sonnet) — Codex P2s 4208254249 and 4208254260

Pure helpers in `parcels-model.ts` (`parcelGenAfterPoll`, `parcelGenAfterShipmentRefusal`) with node tests (red `red-finisher8-gen.log`: missing exports). `MerchantOrders` bumps the parcel generation on every successful poll before the `if (cursor) return` (list pagination unchanged); `OrderShipment` reports the refusal code via `onRefused` -> `sections.onShipmentRefused`, and an `in_parcel_group` refusal bumps the same generation so the OPEN panel appears and the single-order form is replaced by the group hint. No browser step added (NOT_RUN).

Finisher 9: parcel-merge.spec.ts asserts, without a reload after the real-click in_parcel_group refusal, the OPEN panel, the row badge and the group hint replacing the single-order form (proves onShipmentRefused -> pollGen -> parcel reads). Browser: NOT_RUN.
