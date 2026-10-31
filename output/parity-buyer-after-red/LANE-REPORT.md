# PARITY-BUYER-AFTER round 1 (tests-only RED) lane report

No spec was written. The card requires assertions of each signed frame's exact visible copy and controls (role + name). In this lane (no Figma MCP) that copy exists only in the Figma frames. Every signed file I could read gives frame NAMES only:

- `output/ui-parity/FIGMA-INVENTORY.md` (§3 C8, Appendix A `U:b-order` / `U:b-privacy` / `U:b-claim`): ids and titles.
- `.worktrees/ux-states/output/ux-states/` (buyer-states.json, shots/buyer-{expired,unknown,...}-*.png): covers only the generic expired / unknown / offline / forbidden / empty state panels, not these screens.
- Searches for 「訂單連結已失效」「認領連結失效」「隱私與同意設定」「資料副本暫時無法下載」「認領結果待確認」 in `output/`, `docs/`, `apps/`, `packages/` and every `ux-*` worktree found only inventory rows and integrator tool logs, with no frame body text.

Asserting copy I would have to guess breaks the card ("exact visible copy") and the integrator's no-invention rule, so nothing was committed.

| Finding | Class | What I changed |
|---|---|---|
| Frame body copy / controls missing for all 92 frames | c (needs integrator export) | nothing (this report only) |

## Frames the integrator must export (get_design_context text + control names; desktop and mobile share copy, so one of each pair is enough)

- Order status: `4:9963`, `25:943`, `63:1475`, `63:1477`
- Payment / COD / cancel state families: `1198:14549`, `1198:14615`, `1198:14682`, `1198:14748`, `1198:14815`, `1207:11941`, `1207:12062`, `1222:19371`, `1222:19438`, `1222:19657`, `1222:19724`, and COD states `1222:21567`…`1222:22080`
- Pay entry: `29:1064`, `1198:10796`
- Payment return: `1207:16447`
- Lookup / link / after-sales: `26:708`, `26:746`, `26:773`, `26:815`, `1253:17225`
- Privacy / deletion: `5:15460`, `5:15707`, `26:888`, `29:950`, `29:1011`, `37:1083`, `37:1140`, `37:1201`, `37:1266`, `1222:14056`
- Claim: `4:12347`, `66:1746`, `66:1754`

## Implementation notes for the next round (from the repo, not from frames)

- The real routes are `apps/storefront/app/[locale]/{orders,order-link,privacy,data-deletion,claim}` and `apps/storefront/app/payment/return`. Their components are `OrderLink`, `PrivacyCenter`, `ClaimLink`, `CodOrderStatus`, `OrderFlow`, `LookupForm`.
- MOCK harness precedent: `tests/storefront/shop-gate.mjs` + `shop-fake-api.mjs` (the fake has no orders / privacy / claim routes yet). The real-stack precedent is `tests/storefront/privacy-buyer.mjs`, which already drives `/privacy` and `/data-deletion`.
- Parity-sheet precedent: `tests/storefront/checkout-v2.mjs:427` writes `parity/<frame-id>.png`.

STATUS: BLOCKED frame body copy and control names for U:b-order, U:b-privacy and U:b-claim are not in any signed delivery file readable by this lane; the integrator must export frames 4:9963, 25:943, 26:708, 26:746, 26:773, 26:815, 29:1064, 1207:16447, 1253:17225, 5:15460, 5:15707, 26:888, 29:950, 29:1011, 37:1083, 37:1140, 37:1201, 37:1266, 4:12347, 66:1746, 66:1754 (plus the state families listed above)
