# PARITY-DOMAINS-FIX1

## Directed correction of PR #407 (run 41109244416)

Authority: Integrator card PARITY-DOMAINS-FIX1, contracts/storefront-v2.md §B, ADR-20261031 evidence retention, and Figma page 4:8829.

### Upstream sync
Merged `origin/r3/integration` at commit `3c8997eb3` into `unit/parity-domains` first. Resolved generated browser ownership conflict via `node scripts/dev/derive-browser-ownership.mjs`.

### Failure triage table

| Failing job | Failing test | Evidence | Class / Root fix |
|---|---|---|---|
| 124250405991 (bundle:04, mode `--browser-design`) | `TestBrowserStoreDesign` (`tests/admin/editor-v2.spec.ts:282:3` >> `EDITOR-V2 main [zh-TW/zh-CN/en] [mobile/desktop]: four tasks, current versions, domain status`) | `playwright.log`: `Error: expect(locator).toHaveCount(expected) failed; Locator: getByTestId('design-domain-settings'); Expected: 0; Received: 1`. Legacy `<a href="/{locale}/settings?store=..." data-testid="design-domain-settings">` detour rendered instead of dedicated signed domain workspace. In addition, `tests/foundation/browser_store_design_test.go:167` omitted `domains-parity.spec.ts` from positional Playwright arguments. | (b) Product parity regression / implementation gap. Implemented the fourteen signed domain states, full DNS display (TXT/CNAME/A/AAAA), lifecycle actions (add, cancel, suspend, detach, reapply), recovery across reload with idempotent journal, and full value clipboard copying in `apps/admin/components/DesignDomains.tsx`. Removed legacy detour. Added signed domain dictionary in `packages/i18n/src/domains-parity.ts`. Added `tests/admin/domains-parity.spec.ts` to `browser_store_design_test.go:167`. |
| 124276376501 (receipts) | `matrix-receipts` | Receipt for member `--browser-design` was `FAIL` due to job 124250405991. | Downstream consequence of job 124250405991 failure. Resolves with root fix above. |
| required (aggregate) | All required checks | Job 124250405991 and receipts failure blocked aggregate check. | Downstream consequence. |

### Verification & Artifact Integrity
- `node scripts/dev/derive-browser-ownership.mjs --check` exits 0 (54 browser modes registered).
- `gofmt -l tests/foundation/browser_store_design_test.go` clean (exits 0).
- `node scripts/dev/check-evidence-retention.mjs --base origin/r3/integration` PASS (ADR-20261031 limits respected).
