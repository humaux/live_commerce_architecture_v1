# Historical accepted browser checkpoint — 492aaa30

This earlier pass is retained as history, not the final-source rerun. Final source `28097ecf` was rerun successfully: `browser-attribution-28097ecf.log`, six cases in each matrix, 36 PNGs plus JSON evidence in `browser-final/{report,checkout,checkout-admin}/`. See `../SUMMARY.md` for the final source, exact raw directories and final gate table.

Command: `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-ads-attribution`

Exit **0**. `browser-attribution-accepted.log`: two Go browser tests PASS; report and checkout-admin Playwright suites each pass six locale/viewport cases. All network mutations target ephemeral local fixtures; Graph is MOCK.

| Evidence directory under `output/playwright/` | Screenshots | Matrix |
|---|---:|---|
| `ads-attribution-report/20261003T225450.766047000/` | 6 | zh-TW, zh-CN, en × 390/1586 |
| `ads-attribution-checkout/20261003T225509.153539000/` | 24 | same matrix × landing/checkout/placed/order |
| `ads-attribution-checkout-admin/20261003T225520.462171000/` | 6 | same matrix |

Independent audit: **36/36 PNG SHA256 hashes match their manifests**. Click ledgers contain 60 + 72 + 36 = **168 records**, including observations as well as clicks; this is not a claim of 168 literal clicks. AT5 creates six real orders through browser clicks, rather than seeding substitute orders.

The storefront mobile context uses the Pixel 7 descriptor (deviceScaleFactor 2.625) and an explicitly overridden CSS viewport of 390×844. Playwright's default device-pixel screenshot scale produces width `round(390×2.625)=1024`, while the report/admin PNGs are 390 at DPR 1. These are correctly labelled CSS viewport widths, not inconsistent layouts. Before every storefront screenshot the driver asserts `document.documentElement.scrollWidth <= innerWidth + 1`. The old artifacts do not separately serialize runtime innerWidth/DPR, so the DPR conclusion uses the committed driver, descriptor and engine log, not invented runtime fields.

Every required report locale has real clicks for draft selection, session selection, explicit audience read, invalid dates and restoration; checkout flows include consent, destination submission, placing the order, persisted buyer status and admin report readback. See the per-directory JSON ledgers and `screenshots.json` manifests.

Earlier red browser logs and screenshots remain untouched. `browser-attribution.log`, `browser-attribution-rerun.log` and `browser-attribution-final.log` are historical FAIL runs, not final acceptance.

AT6 Events Manager SANDBOX and AT9 real live-video read remain **NOT_RUN**; MOCK browser success does not establish either.
