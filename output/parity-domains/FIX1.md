# PARITY-DOMAINS-FIX1

## Directed correction of PR #407 (run 41109244416)

Authority: Integrator card PARITY-DOMAINS-FIX1, contracts/storefront-v2.md §B, ADR-20261031 evidence retention, and Figma page 4:8829.

### Upstream sync
Merged `origin/r3/integration` at commit `3c8997eb3` into `unit/parity-domains` first. Resolved generated browser ownership conflict via `node scripts/dev/derive-browser-ownership.mjs`.

### Failure triage table

#### Relay 0 (Initial RED & Implementation gap)

| Failing job | Failing test | Evidence | Class / Root fix |
|---|---|---|---|
| 124250405991 (bundle:04, mode `--browser-design`) | `TestBrowserStoreDesign` (`tests/admin/editor-v2.spec.ts:282:3` >> `EDITOR-V2 main [zh-TW/zh-CN/en] [mobile/desktop]: four tasks, current versions, domain status`) | `playwright.log`: `Error: expect(locator).toHaveCount(expected) failed; Locator: getByTestId('design-domain-settings'); Expected: 0; Received: 1`. Legacy `<a href="/{locale}/settings?store=..." data-testid="design-domain-settings">` detour rendered instead of dedicated signed domain workspace. In addition, `tests/foundation/browser_store_design_test.go:167` omitted `domains-parity.spec.ts` from positional Playwright arguments. | (b) Product parity regression / implementation gap. Implemented the fourteen signed domain states, full DNS display (TXT/CNAME/A/AAAA), lifecycle actions (add, cancel, suspend, detach, reapply), recovery across reload with idempotent journal, and full value clipboard copying in `apps/admin/components/DesignDomains.tsx`. Removed legacy detour. Added signed domain dictionary in `packages/i18n/src/domains-parity.ts`. Added `tests/admin/domains-parity.spec.ts` to `browser_store_design_test.go:167`. |
| 124276376501 (receipts) | `matrix-receipts` | Receipt for member `--browser-design` was `FAIL` due to job 124250405991. | Downstream consequence of job 124250405991 failure. Resolves with root fix above. |
| required (aggregate) | All required checks | Job 124250405991 and receipts failure blocked aggregate check. | Downstream consequence. |

#### Relay 1 (CI Run 41205934303 on head f4ca0c80)

| Failing job | Failing test | Evidence | Class / Root fix |
|---|---|---|---|
| 124551071912 (build) | `Next builds (once per run)` | `components/DesignDomains.tsx`: TS2724/TS2459/TS7006/TS7053 type errors (`StorefrontDomains`/`StorefrontDomainRow`/`DomainRequestResult` imports and closure indexing). | (b) Re-exported domain DTO types in `apps/admin/lib/storefront-client.ts`, imported directly from `storefront-model.ts`, explicitly typed closure parameter `(d: StorefrontDomainRow)`, typed `stateMap` record, and destructured edge addresses cleanly. |
| 124551295763 (receipts) | `matrix-receipts` | Downstream failure caused by skipped runner jobs due to failed build. | Downstream consequence of job 124551071912. |
| 124551295809 (aggregate) | Click sweep / Visual lint | Downstream failure caused by missing build artifact. | Downstream consequence of job 124551071912. |
| 124551389764 (required) | Required aggregate | Downstream failure caused by upstream build/receipts/aggregate. | Downstream consequence of job 124551071912. |

#### Relay 2 (CI Run 41206762471 on head 13fb49b6)

| Failing job | Failing test | Evidence | Class / Root fix |
|---|---|---|---|
| 124554003084 (bundle:04, mode `--browser-design`) | `TestBrowserStoreDesign` (`tests/admin/domains-parity.spec.ts:132:3` >> `PARITY-DOMAINS [zh-TW/zh-CN/en] [mobile/desktop]: all signed states, DNS and persisted commands`) | `error-context.md`: `Error: expect(locator).toBeVisible() failed; Locator: getByTestId('design-domains').getByRole('heading', { name: '操作結果', exact: true }); Expected: visible; Error: element(s) not found`. Heading rendered as `先確認這一次操作` (`c.pending`). | (b) Synthetic response verification token off-by-one arithmetic error. `tests/admin/domains-parity.spec.ts:20` defined `proof = "domain-fixture-" + "a".repeat(29)`, which has 15 + 29 = 44 characters. However, `migrations/0106_store_domains.sql:162`, `contracts/invariants.json`, and `apps/admin/lib/storefront-model.ts:11` strictly enforce `tokenShape = /^[A-Za-z0-9_-]{43}$/` (32 bytes unpadded base64url). The 44-character token caused `parseDomainRequest` to throw `Error("storefront_shape")`, which `storefront-client.ts:94` handled as an uncertain write (`{ ok: false, uncertain: true }`), transitioning `DesignDomains.tsx` to `pending` ("先確認這一次操作") instead of `receipt` ("操作結果"). Corrected `proof` to `"domain-fixture-" + "a".repeat(28)` (exactly 43 characters). |
| receipts | `matrix-receipts` | Receipt for member `--browser-design` was `FAIL` due to job 124554003084. | Downstream consequence of job 124554003084. Resolves with root fix above. |
| aggregate | Required suite aggregate | Downstream failure caused by bundle:04 failure. | Downstream consequence. |
| required | Required checks | Downstream failure caused by bundle:04 failure. | Downstream consequence. |

#### Relay 3 (CI Run 41208736454 on head 4b3be53a)

| Failing job | Failing test | Evidence | Class / Root fix |
|---|---|---|---|
| 124559978416 (bundle:03, mode `--browser-design`) | `TestBrowserStoreDesign` (`tests/admin/domains-parity.spec.ts:155:36` >> `PARITY-DOMAINS [zh-TW/zh-CN/en] [mobile/desktop]: all signed states, DNS and persisted commands`) | `error-context.md` / `playwright.log:91`: `Error: expect(locator).toBeVisible() failed; Locator: getByTestId('design-domains').getByRole('heading', { name: '網域設定', exact: true }); Expected: visible; Timeout: 10000ms; Error: element(s) not found`. The snapshot confirmed the page returned to `tab === 'hub'` rendering `<button>網店網址</button>` rather than `<section data-testid="design-domains">`. | (b) Active tab state loss across page reload. In `apps/admin/components/Design.tsx`, `tab` was held only in non-persisted React component state `useState<Tab>("hub")`. When Playwright executed `await page.reload()` during domain reload persistence testing (lines 154, 187, 197), `Design` remounted with default `tab === "hub"`, unmounting `DesignDomains`. Persisted active `'domains'` tab and command journal recovery view in `sessionStorage` under `lc.design.tab.${store}` and `storefront-domains-cmd:${store}`. On component mount, restore `tab = "domains"` if either key is present; when navigating away from `'domains'`, remove `lc.design.tab.${store}`. |
| receipts | `matrix-receipts` | Downstream failure caused by failed member `--browser-design` in bundle:03. | Downstream consequence of job 124559978416. |
| aggregate | Required suite aggregate | Downstream failure caused by bundle:03 failure. | Downstream consequence. |
| required | Required checks | Downstream failure caused by bundle:03 failure. | Downstream consequence. |

### Verification & Artifact Integrity
- `node scripts/dev/derive-browser-ownership.mjs --check` exits 0 (54 browser modes registered).
- `gofmt -l tests/foundation/browser_store_design_test.go` clean (exits 0).
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` PASS (ADR-20261031 limits respected).
