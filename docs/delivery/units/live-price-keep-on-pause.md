# Unit: live price is kept for existing claims when a live offer is paused (money path)

Status: IN REVIEW (Claude Sonnet implementer; an Opus money review follows). Branch `unit/live-price-keep-on-pause`, base = W3-06B keyword tools
(61b70d74) + trunk f8f01b74. Migration **0158** (0159 is W6-05B, 0160 W6-06B, in parallel). Backend only; no `apps/` change.

## Owner decision 2026-10-07 (binding, overrides the frozen LTG03 behaviour)
「已经认领的买家保留直播价，只拒绝新的认领。」
- When a merchant deactivates (pauses) a live offer: NEW comment claims on it are refused (`offer_inactive`, unchanged). Buyers who ALREADY hold a
  claim/bundle line or a claim link for that offer keep the live price that was granted to them, at preview/quote, at checkout begin and at redeem,
  exactly as if the offer were still active, until their claim/link expires per the existing rules.
- Reactivation changes nothing for them (they already keep it). The catalog price applies only to buyers without a prior claim.
- Unopened links: their lines must NOT be skipped as `offer_inactive` any more; they redeem at the granted live price.
- Must still hold: a buyer can't use the paused offer's live price for MORE quantity than they claimed (cap per claim line unchanged); a paused
  offer can't be newly added to a cart via any other path at live price; expired claims/links follow existing expiry (no extension); stock is still
  decided at checkout (claims never reserve stock).

## What changes
- Interface first: amendment "Live price kept on pause" in `contracts/live-keyword-claims-v1.md` (rule 5/7 of "Live tools (R4)", the §12 typo
  recovery sentence and the W3-06B batch warning text are superseded by it).
- Migration 0158 replaces, from their CURRENT bodies (0129 / 0092+0101 / 0129): `claims.live_prices`, `claims.preview_live_prices`,
  `claims.for_buyer_lines`, dropping the `o.active` clause (the claim line + bound owner + unexpired link + remaining quantity stay the authority).
  `claims.consume_live_prices` never read `o.active` (it re-proves the claim line only), `claims.preview_link` / `claims.redeem_link` keep returning
  `offer_active` honestly: only Go stopped using it as a gate.
- Go `internal/claims/buyer.go`: preview `Available` and redeem no longer depend on the offer being active. One SKU can be carried into the cart by
  one claim line only (cart origin is per SKU), so when a bundle holds several pending lines on one SKU (typo recovery: paused `A11` + active `A1`)
  the active offer's line wins and the others stay pending, skipped as `offer_inactive` (unchanged KC04 behaviour; never an `ErrConflict`).
- Gates updated to the new behaviour (each changed assertion cites this decision): LTG03 `TestLiveToolsGateOfferLifecycleAndExpiry`,
  `TestKeywordToolsBatchDeactivateIsAPause`, `TestLiveToolsLivePriceOnlyThroughClaim`-family pause step in `live_tools_test.go`,
  `TestRedeemPlanning` (unit). New `TestLivePriceKeepOnPause*` (a)-(g), plus a SQL-level same-SKU gate.
- R2 count 83 -> 84 (new migration); ACL pins unchanged (no grant change: CREATE OR REPLACE keeps owner and EXECUTE).

## Gates
`test-focused.sh '^(TestLivePriceKeepOnPause|TestLiveToolsGate|TestKeywordTools|TestRedeemPlanning|TestLiveClaimsKC04|TestLiveClaimsKC09)'`; `check-gates.sh`.
Heavy (CI only): full foundation suite.
