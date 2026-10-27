# OC-15 re-authentication Go/HTTP: delivery

- Role: commerce_worker (Claude Opus 5.5, subagent, no recursive delegation). Branch `unit/oc15-reauth-http`, worktree `.claude/worktrees/oc15`, base `origin/r3/integration` 1e7e46f1a (OC-03 #217 merged). PR #229 (base r3/integration), no auto-merge, not merged.
- Authority: contracts/merchant-password-auth-v1.md §16 (route rows added here as §16.1); contracts/tasks.json OC-15; output/oc-03/DEFINERS.md; issue #219 item 2 (integrator: `bank_transfer_closed` into the httperror table).
- Evidence class: CI only (no local go test, no local PG; locally only go build / go vet / gofmt / depmap.sh). Self-reported PASS = E0; independent review NOT_RUN.
- RED: Gates run 40464693233, job 122072954385 (gate shard:unit), head 45b7f5c28 (tests only). Only `livecommerce/tests/identityhttp` failed; every RH test failed on assertions, none on compile (`red.log`):
  - RH01/RH02/RH03 start routes: HTTP 404 not_found (routes absent); RH03 login/complete used the login-only `Complete` (401 unauthorized for a re-auth flow).
  - RH04: `reauth_required`, `owner_role_disabled` (403) and `bank_transfer_closed` (422) rewritten to `internal`.
  - RH10/RH11: re-auth routes 404.
  - RH12: staff stale invite / set-role / remove and owner invite / set-role answered 403 `forbidden`.
  - RH13: orders unshipped.csv, finance summary.csv, customer erasure, bank-transfer confirm and refund-offline answered 403 `forbidden`; the fresh-session export already returned 200.
- First implementation run: Gates 40477067546, job 122134936138 (shard:unit), head 47710283f. Every other shard and browser gate passed; shard:unit failed only RH11: the test derived the throttle row key as the bucket HMAC, but `identity.auth_throttle_hit` stores sha256(HMAC || int4(window) || int4(offset)) (0070 F1). Fix 1 of 2: the test now derives the stored key per window (900 s email-pw for start, 600 s binding for complete); the assertion is unchanged.
- Pre-review GREEN: Gates run 40508411671, head 9c3abb3b9, 82 checks success (shard:unit job 122212308796; TestBtcRefusesNewWrites in job 122212307486).
- Review round 1 RED: Gates run 40536880127, shard:unit job 122309094476, head fde4971c7 (tests only). Only RH13 `billing checkout` failed: 403 `forbidden`, want `reauth_required` (billing/errors.go collapsed PT403). In the same run the new RH13 cases for products report CSV and platform Stripe settings, the RH12 owner-invitation accept case and `TestClassifyRawPT403ReauthCodes` already passed (`red.log`).
- Review fix push 2078866de: Foundation job 122356517088 and Gates `plan` job 122367278321 failed in contractdrift (3 x `ERROR (TOUCHED) UNRESOLVED`: the opaque billing WebhookHandler / mountPlatformBilling dispatch lists `internal/billing/errors.go:35`, the whole `mapError`). Integrator ruling (option 3): billing is cut from this PR; `internal/billing/errors.go` reverted to r3/integration, the RH13 billing-checkout case removed, the drift tool and baseline untouched.
- Final GREEN and head SHA: a commit cannot carry its own SHA or the run that tests it; both are in the PR body and the worker's report.

## What changed
- `internal/platform/reauth_errors.go` (new): `ErrReauthRequired`, `ErrOwnerRoleDisabled` (both wrap `ErrForbidden`) and `PT403(message, fallback)`; unit test `reauth_errors_test.go`.
- PT403 mappers now keep the two §16 codes: `internal/identity/staff.go`, `internal/merchantorders/orders.go` (also refunds and bank-transfer decisions, which fall through to it), `internal/reporting/finance.go` (all report exports), `internal/customers/read.go`, `internal/notify/settings.go`, `internal/payments/settlement/service.go`.
- `internal/httpapi/handler.go` `classify`: the two typed errors before `ErrForbidden`; a raw PgError PT403 with one of the two messages is mapped the same way (covers `internal/ads`, which passes raw driver errors).
- `internal/httperror/error.go`: `reauth_required`, `owner_role_disabled`, `bank_transfer_closed`.
- `internal/identityhttp`: `reauth_routes.go` (new) - password `reauth/start` / `reauth/complete`, OIDC `login/reauth/start`, the re-auth branch of `login/complete` (`CompleteLogin`: `{reauth:true, authenticated_at}`, no token); `password.go` (`reauth_failed` in `passwordMessages`, routes mounted); `handler.go` (OIDC route, callback through `CompleteLogin`); `staff.go` (two codes in the staff table, tested before the owner check). Routes are mounted through optional interfaces, so the existing transport fakes keep compiling.
- `contracts/merchant-password-auth-v1.md` §16.1: the four route rows.
- `tests/identityhttp/reauth_http_test.go` (RH01-RH04 transport, RH10-RH13 real PG).
- Review round 1 (Opus REQUEST_CHANGES, DeepSeek APPROVE+P2), billing item cut by the integrator (see NOT_RUN/BLOCKED); kept: `internal/httpapi/handler_test.go` `TestClassifyRawPT403ReauthCodes` (granted: reauth_required, owner_role_disabled, other message -> 500 internal); compile-time assertions `var _ passwordReauthService = (*identity.Passwords)(nil)`, `var _ oidcReauthService = (*identity.Service)(nil)` in reauth_routes.go; RH12 accept of a pre-0203 owner invitation -> 403 owner_role_disabled and no staff row; RH13 products report CSV and platform Stripe settings (PUT .../payments/card, PaymentProfile SANDBOX); `tests/identityhttp/doc.go` lists the asserted domains; dependency map regenerated.
- Coordinator follow-ups after #211/#233 landed (base merged, no conflict): `tests/foundation/bank_transfer_closed_test.go` route assertion restored to the exact code (`cofCode(out) == "bank_transfer_closed"` plus 422 and retryable=false; file granted for that one assertion); §16 "Method now" now points the mail-unavailable limit at §11 instead of "pending owner confirmation".

## Outside write_paths (integrator approval needed)
- `tests/identityhttp/doc.go`: package comment; a test-only package has no `go list .Doc`, so `check-pkgdocs.sh` fails without it.
- `docs/engineering/dependency-map.md`: regenerated by `scripts/dev/depmap.sh` (new package; identity and identityhttp now import platform). `depmap.sh --check` runs in Foundation checks.
- Not changed, requested: the admin bank-transfer error table `apps/admin/lib/transfer-copy.ts` has no `bank_transfer_closed` copy (#219 item 2); the BFF allowlists for `reauth_required` / `owner_role_disabled` are OC-04.

## NOT_RUN / not done
- BLOCKED (integrator ruling, card OC-15b): billing:manage (`billing.pin_customer` / `record_checkout_session`, e.g. POST .../billing/checkout) still answers 403 `forbidden`, not `reauth_required` (fails closed). RED proof: Gates run 40536880127, shard:unit job 122309094476 @fde4971c7. The fix (`platform.PT403` in `internal/billing/errors.go` mapError) trips contractdrift (Foundation job 122356517088): editing `mapError` touches the unresolved billing webhook dispatch. Contract §16 carries the pending note.
- Local go test and PG (by rule). Independent review.
- Stripe refund (POST .../orders/{id}/refunds): NOT_RUN. The route needs a River client, and `requireRefundEnvironment` answers 422 not_refundable for an order without a paid Stripe attempt before the guarded definer runs, so the case needs a paid Stripe order fixture. Same mapper as the asserted orders CSV (mapRefundError -> mapError); the SQL guard is OC-03 OG01.
- Customer privacy export (POST .../customers/{id}/exports): NOT_RUN. `customers.Export` loads the customer before `record_export`, so it needs a customer fixture; erasure (asserted) goes through the same `mapMerchantError`; the SQL guard is OC-03 OG01.
- Meta ads approve/publish: NOT_RUN over HTTP (needs `Options.Ads` with OAuth/River); the raw-PT403 branch it relies on is unit-tested (`TestClassifyRawPT403ReauthCodes`).
- notify settings and settlement statements have no guarded definer; their mappers pass the codes through but no stale-session path exists to assert.
- OIDC re-auth end to end against a real IdP: transport only (RH03); the SQL side is OC-02 RA04/RA07.
- BFF routes `apps/admin/app/api/auth/password/reauth/*`, the callback `reauth:true` branch and the UI dialog (OC-04).

## Risks
- Until OC-04 ships the BFF allowlists and dialog, an unknown upstream code may still surface as a generic error in the admin UI; the Go side now answers the exact codes.
- `merchanttools` products CSV (`GET .../products/export.csv`) does not go through a guarded definer (OC-03 guarded `identity.export_report`, the reports export). Not changed here.
- The password start throttles before checking the credential, so an OIDC-only principal calling the password route spends a principal/session bucket and gets 403 `reauth_failed`; the UI must pick the OIDC route for such principals.
