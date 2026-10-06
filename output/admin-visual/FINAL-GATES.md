<!-- Purpose: Preserve each required browser mode's actual latest receipt and tested source. -->
<!-- Depends on: immutable gates-*/results.tsv records; original logs are retained. -->
<!-- Used by: integrator acceptance; mixed tested sources are explicit, never relabelled as one full rerun. -->
# Final browser receipts

Code source: `6126273bae5e1addbe4fc6a8afa2656074523b08`. Per the integrator's instruction, the complete f1 batch was followed by failed/affected-mode reruns only. **32 exit0; 1 exit1**. Visual exit1 is not waived: admin162 blocking0/R4=0/R10=0, but storefront-home mobile has3 R9 instances. All33 modes have a real receipt.

| Mode | Tested source | Exit | Seconds | Evidence |
| --- | --- | ---: | ---: | --- |
| --browser-product-editor | f1c5199a | 0 | 3953 | [log](gates-20261005T164802Z/browser-product-editor.log) |
| --browser-cvs | f1c5199a | 0 | 87 | [log](gates-20261005T164802Z/browser-cvs.log) |
| --browser-admin-shell | f1c5199a | 0 | 84 | [log](gates-20261005T164802Z/browser-admin-shell.log) |
| --browser-identity | 23e08490 | 0 | 80 | [log](gates-20261005T192028Z/browser-identity.log) |
| --browser-password-auth | 6126273b | 0 | 121 | [log](gates-20261005T194523Z/browser-password-auth.log) |
| --browser-admin-legacy | f1c5199a | 0 | 58 | [log](gates-20261005T164802Z/browser-admin-legacy.log) |
| --browser-merchant-buyer | f1c5199a | 0 | 747 | [log](gates-20261005T164802Z/browser-merchant-buyer.log) |
| --browser-manual-order | f1c5199a | 0 | 64 | [log](gates-20261005T164802Z/browser-manual-order.log) |
| --browser-merchant-orders-bff | f1c5199a | 0 | 22 | [log](gates-20261005T164802Z/browser-merchant-orders-bff.log) |
| --browser-merchant-orders-ui | f1c5199a | 0 | 137 | [log](gates-20261005T164802Z/browser-merchant-orders-ui.log) |
| --browser-input-delivery | f1c5199a | 0 | 18 | [log](gates-20261005T164802Z/browser-input-delivery.log) |
| --browser-studio-bff | f1c5199a | 0 | 17 | [log](gates-20261005T164802Z/browser-studio-bff.log) |
| --browser-studio-ui | f1c5199a | 0 | 53 | [log](gates-20261005T164802Z/browser-studio-ui.log) |
| --browser-live-claims | f1c5199a | 0 | 35 | [log](gates-20261005T164802Z/browser-live-claims.log) |
| --browser-refund-fulfilment | f1c5199a | 0 | 61 | [log](gates-20261005T164802Z/browser-refund-fulfilment.log) |
| --browser-customers-billing | f1c5199a | 0 | 56 | [log](gates-20261005T164802Z/browser-customers-billing.log) |
| --browser-storefront-publish | f1c5199a | 0 | 28 | [log](gates-20261005T164802Z/browser-storefront-publish.log) |
| --browser-design | f1c5199a | 0 | 28 | [log](gates-20261005T164802Z/browser-design.log) |
| --browser-store-domains | f1c5199a | 0 | 28 | [log](gates-20261005T164802Z/browser-store-domains.log) |
| --browser-meta-ads | f1c5199a | 0 | 41 | [log](gates-20261005T164802Z/browser-meta-ads.log) |
| --browser-ads-attribution | f1c5199a | 0 | 104 | [log](gates-20261005T164802Z/browser-ads-attribution.log) |
| --browser-meta-connect | 23e08490 | 0 | 38 | [log](gates-20261005T192028Z/browser-meta-connect.log) |
| --browser-catalog-media | f1c5199a | 0 | 74 | [log](gates-20261005T164802Z/browser-catalog-media.log) |
| --browser-ops-polish | f1c5199a | 0 | 54 | [log](gates-20261005T164802Z/browser-ops-polish.log) |
| --browser-promotions | f1c5199a | 0 | 32 | [log](gates-20261005T164802Z/browser-promotions.log) |
| --browser-catalog-core | f1c5199a | 0 | 26 | [log](gates-20261005T164802Z/browser-catalog-core.log) |
| --browser-checkout-offline | f1c5199a | 0 | 41 | [log](gates-20261005T164802Z/browser-checkout-offline.log) |
| --browser-home-cod | f1c5199a | 0 | 36 | [log](gates-20261005T164802Z/browser-home-cod.log) |
| --browser-e2e | f1c5199a | 0 | 68 | [log](gates-20261005T164802Z/browser-e2e.log) |
| --browser-webkit | f1c5199a | 0 | 267 | [log](gates-20261005T164802Z/browser-webkit.log) |
| --browser-platform-site | f1c5199a | 0 | 77 | [log](gates-20261005T164802Z/browser-platform-site.log) |
| --browser-click-sweep | 6126273b | 0 | 2506 | [log](gates-20261005T194523Z/browser-click-sweep.log) |
| --browser-visual-lint | 6126273b | 1 | 614 | [log](gates-20261005T194523Z/browser-visual-lint.log) |

## Additional fixed-source checks

- `bash scripts/dev/test-node.sh` →0; [log](logs/test-node-6126273b.log).
- `pnpm --filter admin exec tsc --noEmit` →0; [log](logs/tsc-6126273b.log).
- `LC_HEADERS_STRICT=1 LC_HEADER_BASE=5251df89 bash scripts/dev/check-gates.sh` →0; [log](logs/check-gates-6126273b.log).
- The same strict check-gates after evidence staging →0; [packaging log](logs/check-gates-packaged-6126273b.log). `bash scripts/dev/release-gate.sh --strict --only G04` →0; [log](logs/g04-packaged-6126273b.log). G04 is a STATIC subset, **not** a release verdict.
- Full G-UI8:123 page/viewport/locale units,0 load failures;978 clicks =954 pass/0 fail/24 intentional skips;18/18 journey steps;0 known/new failures and0 stale known-defect entries. The runner's generic footer still says5 journeys; the actual structured results and runner summary above are authoritative.
- The24 control skips are3 sign-outs deferred to a dedicated journey,12 single-option selects and9 save controls with tested cancellation paths. A separate storefront `/live` NOT_IMPLEMENTED row is not a passed page/control.
- Final package verification:174 current visual-package files +29 source-qualified f1 CVS files =203 hashes matched;162 before/162 after file hashes and142 source-path hashes matched. Six after files are explicitly labelled CVS audit placeholders, not real print acceptance.
- Author-code/script and authored handoff Markdown whitespace checks →0. A blanket `git diff --cached --check` including raw captured logs returns1 for original terminal CR/trailing spaces; raw test evidence is deliberately retained byte-for-byte, not cleaned or silently rewritten. This does not change the strict project-gate result above.

## Red and interrupted receipts

- f1: identity exact-grant drift; MetaConnect/visual/sweep direct-Go pick503. All retained in gates-20261005T164802Z.
- e5c32282: Team role hit-test expectedtrue/receivedfalse; targeted RED exit1, [details](TEAM-OCCLUSION.md). Extra min-four-chain assertions are expected in that one-chain diagnostic and were not weakened. Full unfiltered612 password-auth then exited0.
- 23e08490 full click-sweep: **INTERRUPTED** after independent Team P2 discovery (exit1 preserved); not a product-failure verdict or a pass. Complete612 sweep subsequently exited0.
