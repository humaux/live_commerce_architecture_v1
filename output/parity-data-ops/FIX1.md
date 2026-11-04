# PARITY-DATA-OPS FIX1 — PR #475

E3 failure evidence only. This batch is not GREEN; author correctness assessment remains E0. No local tests or servers.

## Authority and baseline

The dispatched FIX1 card is the change authority. Repository AGENTS.md, 架构.md §§0/1/2/26/27, contracts/invariants.json, migration-import-v1 §§3/4 and signed FRAME-COPY.md are the implementation boundaries. Current PR source and native Actions receipts determine status; earlier output reports are discovery pointers only.

Merged origin/r3/integration `c52b65283a5ac738d0c527a464d0cccc9523b1f4` into this branch as `5b428a0fe`. The sole conflict was generated browser-ownership.json; regenerated, never hand edited. Main checkout is read-only. Worker: Codex dispatched by Claude integrator; model/effort not exposed by runtime, UNKNOWN. No recursive delegation.

Latest base Gates41288903917 is success, but its native jobs do not include identity/bundle07. All rows below use actual failing assertions, not that workflow conclusion as a substitute for test coverage. Original tests-only RED and assertion receipts remain in red.log; this is one combined directed-fix push, not a new feature RED cycle.

## Every failing job → test → evidence → class

All PR Gates jobs below belong to run41288104896, execution merge `ea3d49b60282479c131e94ce5176a9921c1239a5`, head `8cd4f698f3fddeaf40901b84fb398a8f9b10c320`. Raw logs/artifacts are retained locally, uncommitted. Evidence filenames here are artifact members, downloadable with gh run download; hashes are in FIX1-evidence-manifest.json.

| Job | Failing test | Native evidence | Class | Directed repair / disposition |
|---|---|---|---|---|
| 124794932634 | TestBrowserClickSweep shard 1 | gate-browser-click-sweep_1_10 / ledger.md; aggregate click.md: customer import mobile/zh-TW 更換檔案 | b | Selected filter is disabled; real native filechooser is recorded as an effect, alongside errors. No exemption or timing change. |
| 124794932733 | TestBrowserClickSweep shard 2 | gate-browser-click-sweep_2_10 / ledger.md; aggregate click.md: customer import desktop/en Change file | b | Selected filter is disabled; real native filechooser is recorded as an effect, alongside errors. No exemption or timing change. |
| 124794932776 | TestBrowserClickSweep shard 3 | gate-browser-click-sweep_3_10 / ledger.md; aggregate click.md: inventory mobile/zh-TW 全部; customer import desktop/zh-TW 更換檔案 | b | Selected filter is disabled; real native filechooser is recorded as an effect, alongside errors. No exemption or timing change. |
| 124794932768 | TestBrowserClickSweep shard 5 | gate-browser-click-sweep_5_10 / ledger.md; aggregate click.md: inventory desktop/zh-TW 全部 | b | Selected filter is disabled; real native filechooser is recorded as an effect, alongside errors. No exemption or timing change. |
| 124794932607 | TestBrowserClickSweep shard 6 | gate-browser-click-sweep_6_10 / ledger.md; aggregate click.md: inventory desktop/en All | b | Selected filter is disabled; real native filechooser is recorded as an effect, alongside errors. No exemption or timing change. |
| 124794932945 | TestBrowserClickSweep shard 10 | gate-browser-click-sweep_10_10 / ledger.md; aggregate click.md: J1 opening stock 9/7 | a+b | J1: click signed Search SKU → textbox → Search; original 9/7 assertions unchanged. |
| 124794932630 | TestBrowserClickSweep / visual-lint shard 1 | gate-browser-visual-lint_1_4 / ui-visual-audit/*/lint.json: inventory en: R3 first cell 63.6px/content107px + R6 header overlap; zh-TW: R3 | b | Scope inventory intrinsic column sizing and scrollport; hide the native input with an unambiguous scoped display rule. Thresholds unchanged. |
| 124794932929 | TestBrowserClickSweep / visual-lint shard 3 | gate-browser-visual-lint_3_4 / ui-visual-audit/*/lint.json: customer import three desktop locales: R1 +293px and R6 input/button overlap; inventory zh-CN: R3 | b | Scope inventory intrinsic column sizing and scrollport; hide the native input with an unambiguous scoped display rule. Thresholds unchanged. |
| 124794932904 | TestBrowserClickSweep / visual-lint shard 4 | gate-browser-visual-lint_4_4 / ui-visual-audit/*/lint.json: customer import three mobile locales: R1 +33px and R6 input/button overlap | b | Scope inventory intrinsic column sizing and scrollport; hide the native input with an unambiguous scoped display rule. Thresholds unchanged. |
| 124794932933 | bundle:01 --browser-ui-gallery / ds-hex-ratchet.test.mjs:87 | job log: total hex colours414 > baseline386; data-ops.css28 > baseline0 | b | Replace owned literal colors with existing shared semantic tokens. Baseline unchanged. |
| 124794932874 | bundle:03 TestBrowserCatalogCore/PE12-17-TWD | gate-browser-product-editor / playwright.log: product-editor.acceptance.ts:273 waits for collection-title after old collection-new | a | Click signed Manage manual collection control before the same title/save/member assertions. |
| 124794932874 | bundle:03 TestBrowserCatalogCore/CC12-frozen (catalog-core + product-editor) | gate-browser-catalog-core + gate-browser-product-editor / playwright.log: 16 warehouse fixture reads fail; two collection-title visibility failures | b | Browser-session fetch for fixture setup only, same status/receipt assertions; separate visible collection title from its screen-reader-only slug. No fixture data or business oracle changed. |
| 124794932977 | bundle:04 TestBrowserAdminLedgerFixtureChain | gate-browser-admin-legacy / playwright.log: ledger.spec.ts:248 product-new absent (legacy fixture has no identity); visual-states.spec.ts:48 old radio absent | a | Signed Back to products navigation keeps scoped href/URL and no-inline-panel assertions; full editor journey retained in signed-session shared helper and catalog-core. Selection/caret/scroll assertions use signed Search SKU and stock scrollport. |
| 124794932977 | bundle:04 TestBrowserMerchantBuyerRealChain | gate-browser-merchant-buyer / browser.log: merchant-buyer-gate.mjs:129 missing Add product link | a | Shared createProductInEditor clicks Back to products → New product; selectLedgerRow opens signed SKU/disclosure in every locale. Price, zero-stock, purchase URL, PG facts and buyer checks preserved. |
| 124794932611 | --browser-webkit merchant-buyer member | gate-browser-webkit / merchant-buyer-real-1409030527/browser.log: same Add product assertion | a | Same shared helper and signed entry mapping; no engine skip or exception added. |
| 124794932547 | bundle:05 TestBrowserStorefrontPublish | gate-browser-storefront-publish / browser.log: storefront-publish-gate.mjs:146 missing Add product link | a | Same shared helper and signed entry mapping; publication and buyer assertions unchanged. |
| 124794932562 | bundle:07 TestBrowserSettingsWizardRealChain | PR artifact gate-browser-identity / settings-real-20261101T083542.484080454/playwright.log:11–12,28,32,37; exact same assertion in BASE proof below | c | Leave unchanged. The latest green base omitted identity, so it cannot establish that this is a PR regression. |
| 124794932562 + Foundation124793465579 (run41288104889) | bundle:07 --browser-migration-import + Node unit suites / import-coordinator.test.mjs:81,112 | job log assertions at test lines90/122: actual undefined, expected false for import-restart.disabled after acknowledged or terminal receipt | b | Set explicit disabled={locked} on result restart; original lock/replay/mismatch assertions unchanged. |
| 124799649265 / 124799649295 / 124800307357 | receipts / aggregate / required | native jobs plus aggregate click.md and visual.md show failed members above | cascade | No receipts/gate changes. Shard1 failure also leaves its platform-runner.pass marker absent; not credited as a platform pass. |

## Class (c): exact baseline proof, no settings edit

Base integration run41256591992 at `e8b735f103a5488b6e3bcca9c8d4534c77007d98`, job124701685425, artifact13063513565 (`gate-browser-identity`). Its `playwright/settings-real-20261101T004621.374573085/playwright.log` lines11–12 resolve `getByTestId('settings-policy-form').getByRole('button', { name: 'Save pricing policy', exact: true })` to a disabled submit button; lines28/32/37 identify `settings-v2.spec.ts:1085`, the same timeout as this PR. Native base job log line3718 reports `--- FAIL: TestBrowserSettingsWizardRealChain (26.23s)`. Both the helper and SettingsWizard source are unchanged between this base and the PR. This proves the identical failure on a base run; it does not claim the current PR is green.

## Scope and preserved gates

No fixture values, exemption lists, thresholds, dispatcher/baseline files, contracts, OpenAPI, migrations, R2 pin, dependencies or locks changed. Existing gate-adapter assertions remain untouched. The monitor accepts a file picker only when the browser actually emits filechooser; a label/click alone earns nothing, and page errors still fail. An additive deterministic protocol regression covers existing/new-page listeners and error preservation. CI provides runtime verification.

Signed inventory is now a stock list → SKU edit, rather than a permanently selected radio/tray. Stock assertions read the same server balance from the selected-SKU panel. New product is reached through Back to products and the existing New product link; merchant-product is the shared owner of that setup. The legacy bearer fixture lacks server identity for the product editor, so its inventory test verifies the signed Back to products href/navigation and absence of inline creation. The catalog real-session suite and both merchant/buyer chains retain full editor-creation coverage; no fixture was rewritten.

Outside literal C6 paths, all explicitly listed in the PR: tests/admin/{ledger.spec.ts,visual-states.spec.ts,product-editor.acceptance.ts}, tests/storefront/{merchant-product.mjs,merchant-buyer-gate.mjs,storefront-publish-gate.mjs}, tests/ui/{click-sweep.mjs,click-sweep-lib.mjs,click-sweep-lib.test.mjs}; generated browser ownership and dependency map as applicable. These are directed failed callers/helper repairs under FIX1; integrator must review their scope.

## Static verification and remaining boundary

- node --check: six changed .mjs sources, exit0. No node --test run.
- Targeted strict tsc: changed C6 components, parity and ledger specs plus visual-states, exit0. Additional product-editor.acceptance compile closure needs locally uninstalled axe-core; full closure NOT_RUN locally and delegated to CI. No dependency added or source weakened.
- Static design detector: [] findings, exit0. This is source-level only.
- Browser ownership generation + --check, depmap generation, packet and evidence-retention receipts are recorded in DELIVERY.md after execution.
- Customer source-specific required-field error remains a pre-existing downstream limitation: the frozen preview row has code=required but no field; Go uses required for both source id and name. Its parity assertions remain unchanged, not waived. No backend/contract expansion in this card. If newly reached CI fails there, STOP with the exact assertion; do not fabricate a field or try a third approach.
- New-head GREEN, click ledgers, screenshot parity, independent review, SANDBOX/LIVE: NOT_RUN pending own CI. On any repeated assertion after this batch, report BLOCKED, do not push a third approach.
- Integrator resumes this lane when registered Gates/Foundation runs finish; no silent background local process or long CI polling.
