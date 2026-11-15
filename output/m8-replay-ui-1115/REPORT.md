# M8 Replay Public UI — Delivery Report

- **Task ID**: `task_bfbc8f74d025`
- **Dispatch ID**: `ctx_5732d72c65a7`
- **Coordinator**: `orca_session_id:codex_759cd980_5a90_4d23_88cc_66f1621da8d8` (coord `b7a5fb7c-50c4-46b2-a635-8175eec8c15d`)
- **Worker**: `ui_worker` (`orca_session_id:25ba190e-b88d-44e0-8cd7-d06cd3fa924c`)
- **Model**: Antigravity Gemini 3.8 Flash High
- **Base Commit**: `f31999ddc098743a82adc6d308043e5c801af513`
- **Base Tree**: `9b2a2fca247fba0403841a607661439542c033bd`
- **Branch**: `luogangan7-lgtm/fb-m8-replay-ui-1115`
- **Manifest SHA**: `5a16ae9ba63efe40eaf110741c05f3b1b15676187cdb32785187579357443eef`

---

## 1. Summary of Changes

### New Files Created
- `apps/storefront/app/[locale]/replays/page.tsx`: SSR replay list route. Validates locale, guards with `gate()`, loads replays via `must(await listReplays())`. Renders formal app bar with store brand and back navigation, replay list with ended timestamps and "觀看回放" links, or empty state with "目前沒有公開回放" and "返回商店".
- `apps/storefront/app/[locale]/replays/[sessionID]/page.tsx`: SSR replay detail route. Validates canonical UUID and locale, guards with `gate()`, loads replay detail via `must(await getReplay(sessionID))` and store currency via `must(await storeCurrency())`. Embeds official Facebook player iframe, maps offer SKU to current variant price and stock hint (`目前有貨` or `目前已售完`), renders "查看商品" link to product detail. Sold-out CTA remains "查看商品", no cart write.
- `apps/storefront/app/[locale]/replays/replay.module.css`: Colocated scoped CSS module. Reuses `:global(body:has(.replay-root))` routepresence pattern to hide `.sf-header`, `.sf-announce`, and `.sf-footer` while `.replay-root` is present; preserves previewbanner, #main/skiplink, and CartProvider. Enforces exact formal layout tokens (navy `#142942`, body `#1f2937`, muted `#596579`, canvas `#f6f7f8`, line `#dde5ee`, surface `#ffffff`), desktop 1440x900 single 720px column, mobile 390x844 with 16px margins (358px content), movie 688x340 on desktop and 326x200 on mobile.

### Modified Files
- `apps/storefront/lib/shop-contract.ts`:
  - Added types: `ReplayCard`, `ReplayList`, `ReplayOffer`, `ReplayDetail`.
  - Added pure validator `isValidFacebookPermalink`: strictly validates HTTPS `facebook.com` / `www.facebook.com`, single optional page path segment, numeric video ID, no query/fragment/credentials/non-default port/encoded path/controls.
  - Added pure validator `isValidFacebookEmbedUrl`: strictly validates HTTPS Facebook embed endpoint `/plugins/video.php` with exactly one `href` query parameter matching the item's permalink.
  - Added pure parsers `parseReplayList` (caps at 50 items) and `parseReplayDetail` (filters offers to only those with valid product detail and matching visible `sku_id` in current variants; omits invalid/missing SKUs).
- `apps/storefront/lib/shop-upstream.ts`:
  - Replaced unparsed return types of `listReplays` and `getReplay` with strict `parsed(..., parseReplayList)` and `parsed(..., parseReplayDetail)` using `cache()`.
- `apps/storefront/lib/shop-copy.ts`:
  - Added typed copy for `zh-TW`, `zh-CN`, and `en` for all replay chrome, titles, stock states, and controls.
  - Exported `formatReplayEnded` for localized date/time formatting (`10/30 14:00 已結束`).
- `apps/storefront/lib/routes.ts`:
  - Exported `replaysPath(locale)` and `replayPath(locale, sessionID)`.
- `apps/storefront/tests/shop.test.mjs`:
  - Added negative parser cases for hostile embed/permalink/duplicate href/mismatch/encoded/credentials/host suffix, list capping at 50, and SKU/current price resolution.
- `tests/storefront/shop-fake-api.mjs`:
  - Added `replays` state and endpoints for `GET /v1/buyer/replays` and `GET /v1/buyer/replays/{sessionID}`.
- `tests/storefront/buyer-after-parity.mjs`:
  - Appended browser test scenarios covering all 8 formal roles (`1468:107106`, `1468:107151`, `1468:107197`, `1468:107244`, `1468:107292`, `1468:107339`, `1468:107387`, `1468:107430`), list-to-detail clicks, refresh persistence, back to list and store, offer to product navigation with standard chrome restoration, sold out CTA, empty list, negative URL refusals, and cart write prevention.
- `output/ci-select-backend-browser/browser-ownership.json`:
  - Derived via static metadata script `node scripts/dev/derive-browser-ownership.mjs` (56 modes).

---

## 2. 8 Formal Roles Coverage

| Node ID | Viewport | Page / State | Frame Reference | Test Status |
| --- | --- | --- | --- | --- |
| `1468:107106` | Desktop 1440x900 | 直播回放 · 列表 | Desktop List | Covered (`buyer-after-parity.mjs`) |
| `1468:107151` | Mobile 390x844 | 直播回放 · 列表 | Mobile List | Covered (`buyer-after-parity.mjs`) |
| `1468:107197` | Desktop 1440x900 | 直播回放 · 詳情 (有貨) | Desktop Detail In-stock | Covered (`buyer-after-parity.mjs`) |
| `1468:107244` | Mobile 390x844 | 直播回放 · 詳情 (有貨) | Mobile Detail In-stock | Covered (`buyer-after-parity.mjs`) |
| `1468:107292` | Desktop 1440x900 | 直播回放 · 商品售完 | Desktop Detail Sold Out | Covered (`buyer-after-parity.mjs`) |
| `1468:107339` | Mobile 390x844 | 直播回放 · 商品售完 | Mobile Detail Sold Out | Covered (`buyer-after-parity.mjs`) |
| `1468:107387` | Desktop 1440x900 | 直播回放 · 無回放 | Desktop Empty List | Covered (`buyer-after-parity.mjs`) |
| `1468:107430` | Mobile 390x844 | 直播回放 · 無回放 | Mobile Empty List | Covered (`buyer-after-parity.mjs`) |

---

## 3. Local Execution Prohibition & NOT_RUN Gates

In accordance with strict card instructions:
- **NO LOCAL tests/builds/lint/typecheck/go list/go vet/PG/Docker/install/devservers**: All CI test commands remain strictly `NOT_RUN` on this host.
- Static metadata command executed: `node scripts/dev/derive-browser-ownership.mjs` (exit 0).
- Remaining gates: Normal CI execution on PR (`r3/integration`), independent Codex review.

---

## 4. Humaux Unit Proxy Text

```json
{
  "unit": "M8-REPLAY-PUBLIC-UI",
  "task_id": "task_bfbc8f74d025",
  "dispatch_id": "ctx_5732d72c65a7",
  "base": "f31999ddc098743a82adc6d308043e5c801af513",
  "roles_covered": [
    "1468:107106", "1468:107151", "1468:107197", "1468:107244",
    "1468:107292", "1468:107339", "1468:107387", "1468:107430"
  ],
  "status": "IMPLEMENTATION_COMPLETE_CI_READY"
}
```
