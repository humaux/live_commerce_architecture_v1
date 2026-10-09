<!-- Purpose: identify the exact test-fixture boundary preventing full LC-U2b2 browser acceptance.
Depends on: frozen LC-U2b2 no-Go scope, current signed browser_inbox_ui_test.go and LC-B3b real projections.
Used by: integrator scope ruling or dedicated test-fixture author; this document does not authorize a Go edit. -->
# LC-U2b2 remaining browser fixture

Status: APPROVED by integrator on 2026-10-08: the no-Go rule covers PRODUCT code only. Test fixture implementation is present and compiles; real PG/browser acceptance is currently environment-blocked. The request below is retained as historical context; no further scope approval is pending.

The frozen brief says **“Scope (UI and BFF only; no Go, migration or OpenAPI changes)”**. The existing normal inbox runner passes, but it cannot prove the new positive link cases:

- `tests/foundation/browser_inbox_ui_test.go:108,180`: the real signed OIDC identity is `live_operator`; the real role catalogue lacks `customers:read`. It proves the denied state only. Reader/viewer fixture tokens do not supply the required combined permissions either.
- The current `open` conversation has no full A13 peer-linked claims/orders/ordinal/auto-reply dataset. The bundle fixture supplies BR1+1, not a completed purchase history for that conversation.
- The existing `__test` handlers are read-only observations, and browserEnvironment filters database credentials. TypeScript cannot safely or legitimately create the missing role/data via these interfaces.

Minimum fixture addition, using existing real APIs/seed helpers rather than fabricated response DTOs:

1. A separate signed merchant actor with `inbox:read`, `inbox:reply`, `customers:read`, `orders:read`. Preserve the existing live_operator for the no-customers:read matrix. Use the real OIDC/session chain; no invented browser permission payload.
2. Two visible, active customer targets. Merely inserting `buyer.owners` is insufficient: the real merchant customer projection also needs the existing order/claim/import-profile relation.
3. An owned conversation with a real `inbox.bundle_peers` relationship, accepted claim lines and recorded live-price amounts, real order-origin rows, a CONFIRMED/AWAITING_COLLECTION order for nonzero ordinal, and an automatic private-reply operation. Keep any `bundles.owner_id` different from the manually picked target to prove no identity inference.
4. Expose only synthetic fixture identifiers/expected scalar counts to the browser through the existing allowlisted LC_BROWSER_* environment. Do not print credentials, names, message bodies or copied claim-link credentials.

Then extend the existing inbox spec at zh-TW/zh-CN/en × 1440/390:

- Open the conversation: claim keyword/quantity/session, recorded TWD total, order number/state and ordinal/auto-reply are visible from real Go/PG facts.
- Picker starts blank. Pick a customer and click Link; reload/reopen; the manually linked label and correct customer name persist.
- Two real pages share the conversation. Page B changes the link while page A retains an older A9 version. A's click returns 409; selected choice remains, winning link/name and fresh version refresh; A explicitly retries successfully. No automatic POST.
- Click Unlink, reload and verify the link is absent.
- The existing live_operator sees disabled controls with the localized reason (already added and passed in all six locale/width combinations).
- Keep absent-orders coverage and the existing privacy/UNKNOWN tests. The retained-thread calibration must still fail exactly INU05, not fixture setup.

Implemented in `tests/foundation/browser_inbox_customer_link_fixture_test.go` using real signed intake/private-reply Finish and A16/confirm/cancel handlers, with scoped projection assertions. Direct replica-based order helpers were deliberately not reused. See DELIVERY.md for current evidence and runtime blockers.
