# RVFIX-440 relay 1 lane report

CI of head 93b7247a7 (run 41258218002): only `gate (bundle:02)` → `--browser-storefront` failed (receipts/required follow). Cause read from the job artifact gate log.

| failing job/test or finding | class | what changed (file:line) |
|---|---|---|
| `--browser-storefront` PARITY-BUYER-BROWSE: 782:3967 / 782:4143 x 3 locales "not descending: 148,128,68,89,46,..." | (b) my spec read the wrong number: `price_desc` sorts by highest variant price, the card shows the lowest ("from") price | tests/storefront/buyer-browse-parity.spec.ts:56-58 (`trendsDown` replaces strict `descending`), :355-360, :386-396, :410; no product or gate file changed |
| browser ownership | regenerated | output/ci-select-backend-browser/browser-ownership.json |
| RVFIX.md | CI run id filled, fix recorded | output/parity-buyer-browse/RVFIX.md |

Nothing run locally (owner rule). WAIT_RUN: the next CI run on the new head decides; expected 24 COVERED / 41 NOT_COVERED / 4 DEFERRED / 4 BLOCKED.

STATUS: DONE
