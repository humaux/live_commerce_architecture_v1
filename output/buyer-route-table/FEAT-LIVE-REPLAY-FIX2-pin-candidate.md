# FEAT-LIVE-REPLAY-FIX2 — buyer dispatcher pin candidate

Authority: integrator grant in the 2026-10-31 FIX2 card; only `buyerhttp.Routes` may change. Independent PR reviewer must inspect the entire dispatcher closure before acceptance. This candidate is not an independent-review receipt.

Integration base: `629f42152f864621a99ac2247c3baf5d751a422f`; merged branch before this repair: `bc2a62f12be50adfa354ebdeb57406bc0359437e`.

- Old SHA: `bfbb4441b77afb0b48f0c3bd0c326b75e276a528fa54e588a645cbe0f814c43a`.
- Candidate SHA: `31499ffb3ce0f159bcacb0f380bcbfe7fb94c220f5b91231d91e5edd840fbbd7`.
- Candidate SHA authority: GitHub Gates run `41106628502`, plan job `124238049731`, raw dispatcher-freeze diagnostic (`output/FEAT-LIVE-REPLAY/fix2-grant-plan.log`). Buyer implementation is unchanged since that diagnostic; intervening integration merge changes inbox-editor only. This is original CI output, not a locally executed gate or an invented pin.
- Exact route-row delta: 46 + 2 = 48, adding only `GET /v1/buyer/replays` and `GET /v1/buyer/replays/{id}`. The independently AST-enumerated row oracle stays exact.

## Every added or changed function/declaration

| Symbol | Change and why it belongs to the closure |
| --- | --- |
| `handler.ServeHTTP` | Changed: before shared dispatch, replay query rejection returns the contract's 422. It still admits no query authority; authentication, BFF key, scope and normal refusal paths remain enforced. HTTP dispatcher root, already frozen. |
| `handler.dispatch` | Changed: selects replayGet for exactly the two replay kinds. Already frozen direct dispatcher. |
| `isPublicRoute` | Changed: the two replay kinds use the existing public BFF-key + verified-origin boundary rather than a buyer capability, as the replay contract requires. Already reachable/frozen. |
| `handler.replayGet` | Added: directly called by dispatch; readonly transaction, noBody/canonical UUID/query checks, scoped `live.buyer_replays`, closed DTO decoding, current catalog product/variant filtering and image projection. No provider call or client price. It is code-reachable rather than only a row target, so it must be frozen. |
| route-kind `const` block | Changed: adds `replayListRoute`, `replayDetailRoute` to the existing iota block. The whole block is frozen, including consequent catalog enum positions. Existing symbolic switches remain intact. |
| `type replayItem` | Added: list and detail decoder/output used directly by replayGet; contains session, title, end time and stored permalink only. |
| `type replayOffer` | Added: detail output used directly by replayGet; current catalog product projection, offer and SKU IDs. |

The function list gains only `handler.replayGet`; declarations gain `func handler.replayGet`, `type replayItem`, `type replayOffer`, and the const-block identifier list gains exactly the two route kinds. Existing helpers/types (`noBody`, `mapPublicCatalogError`, `strictDecode`, `projectV2`, `attachV2ImageSizes`, `writeOK`, `v2Detail`, `v2Query`, `responseError`) already belong to the frozen closure. Cross-package calls remain the scanner's existing boundaries. Every other pin entry and the drift baseline/allowlists/extractor are unchanged, except the separately and explicitly granted exact row-count oracle.

Validation: relevant Go vet/gofmt and packet structure checks are E1 only. No local test/gate execution; PR CI must recompute the fingerprint, function list and declarations. Independent review/E4 is PENDING. If another buyer-route card merges first, merge integration and recompute the candidate and exact row pin under the same grant.
