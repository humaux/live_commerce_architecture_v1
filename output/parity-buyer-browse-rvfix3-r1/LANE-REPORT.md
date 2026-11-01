# RVFIX3-440 lane report (relay 1)

| finding | class | change |
|---|---|---|
| gate (bundle:02) --browser-storefront red on 067f3b447 (new on-accent/hover assertions) | a (expected RED) | apps/storefront/app/globals.css:777 color `var(--on-action)`; :778 hover `color-mix(in srgb, var(--action) 88%, #000)` |
| receipts, required | follow from bundle:02 | none |

Unrun locally; CI decides.

STATUS: DONE
