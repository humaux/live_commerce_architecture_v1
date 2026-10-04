# Failed acceptance evidence

These files are intentionally retained failure evidence, not passing screenshots.

- `product-editor-final.log` / `product-editor-PE14.png`: final required edit assertion is disabled by the safe-readback guard.
- `catalog-core-final.log`: unchanged legacy create/redirect assertion times out.
- `catalog-media-final.log` / `catalog-media.png`: existing-product photo editing surface is absent.
- `merchant-buyer-final.log` / `merchant-buyer.png`: legacy create helper awaits old `/products` response.

All are from final source `c91f8cad`. Full original trace and local runtime logs remain in the `output/playwright/` paths listed in SUMMARY.md.
