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

### Verification & Artifact Integrity
- `node scripts/dev/derive-browser-ownership.mjs --check` exits 0 (54 browser modes registered).
- `gofmt -l tests/foundation/browser_store_design_test.go` clean (exits 0).
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` PASS (ADR-20261031 limits respected).
