# Merchant orders C — visual review record

## Scope and verdict

This directory records the approved C inline-expansion composition and its
bounded visual finish review. The independent reviewer compared the approved
1586×992 comp with the initial captures, then reviewed all nine final captures
in `fix-round-1/` after the source correction at `f70dc42`.

Initial verdict: **FIX**, limited to the fifth payment column inheriting a 9%
width and clipping the important `Authorized · not captured` state. The source
now assigns all five columns 21/20/18/21/20 percent and lets badges wrap within
their cells. Final verdict: **RESOLVED** for that original P1. This verdict
covers that correction only; it is not a new whole-surface review, MOU01–06
acceptance, production approval, or release approval.

## Final required captures

All images are actual local browser captures using synthetic fixtures. `inline`
is the order list and selected row; `inline-detail` is the expanded detail
viewport. Each capture is 1586×992 on desktop or 390×844 on mobile.

| Locale | Desktop | Mobile list | Mobile detail |
|---|---|---|---|
| English | [inline-en-1586x992.png](fix-round-1/inline-en-1586x992.png) | [inline-en-390x844.png](fix-round-1/inline-en-390x844.png) | [inline-detail-en-390x844.png](fix-round-1/inline-detail-en-390x844.png) |
| 简体中文 | [inline-zh-CN-1586x992.png](fix-round-1/inline-zh-CN-1586x992.png) | [inline-zh-CN-390x844.png](fix-round-1/inline-zh-CN-390x844.png) | [inline-detail-zh-CN-390x844.png](fix-round-1/inline-detail-zh-CN-390x844.png) |
| 繁體中文 | [inline-zh-TW-1586x992.png](fix-round-1/inline-zh-TW-1586x992.png) | [inline-zh-TW-390x844.png](fix-round-1/inline-zh-TW-390x844.png) | [inline-detail-zh-TW-390x844.png](fix-round-1/inline-detail-zh-TW-390x844.png) |

The three desktop captures show the full payment label (`Authorized · not
captured` / `已授权·未扣款`) within the fifth column. The six mobile captures
retain the order/detail hierarchy without clipping the payment badge.

## Provenance and remaining gate

- Approved composition: [merchant-orders-inline.png](../../mocks/decision/merchant-orders-inline.png)
  with [seed sidecar](../../mocks/decision/merchant-orders-inline.png.json),
  seed `549adef8`; C 表格原位展开 was selected for this exact comp.
- Initial captures are retained at this directory root; final required
  same-viewport captures are in `fix-round-1/`.
- The root acceptance record is
  [2026-09-27-merchant-orders-ui-acceptance.md](../../../docs/implementation/2026-09-27-merchant-orders-ui-acceptance.md).
  Its aggregate local command exited 1: six functional browser cases passed;
  native visibility remained NOT_RUN. The observed native history return had
  `pageshow.persisted=false`, so it does not prove bfcache restoration.
- No production/provider calls were made. Buyer B is unchanged. The comp and
  captures use synthetic data; no raster image is shipped by the UI.
