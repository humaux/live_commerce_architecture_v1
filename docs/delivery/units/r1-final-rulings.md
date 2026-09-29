# R1 final-wave integrator rulings (2026-09-29)

Binding for the final integration (t12-e2e, maintainability, deploy-release) and unit sec-fixes.

## Product
F1. MDEF-1 (P1): `apps/admin/lib/claims-model.ts` `parseBundle` and the `Bundle` type accept platform
    `manual | facebook | instagram` (Go emits `facebook`/`instagram` for Meta bundles per
    meta-claims-intake-v1). Fix + node test; `--browser-e2e` must go green.
F2. Meta route activation for operators: add `meta-admin route` (calls `meta_inbox.activate_route`,
    and deactivate) plus the `commerce_meta_curator` login in deploy provisioning, if the SQL function
    and role exist; otherwise the smallest SQL addition that lets an operator bind a Page/IG webhook
    route to a store, with a real-PG gate. Without it a deployed store cannot receive Meta comments.
F3. Taiwan CVS pickup selection in the storefront is R2 (T13). R1 buyers use home delivery; the
    merchant may still record a CVS carrier at shipment. Reported to the owner as a known limit.

## Test infrastructure
F4. Test PG `max_connections` 30 → 60 in `scripts/dev/test-local.sh`, `scripts/dev/test-focused.sh`
    and the Playwright isolated fixture (memcg stays 1g; shared_buffers unchanged). Reason: T12 peaks
    at 25–27/30 and MCI09 exhausted slots in the full foundation run.
F5. `release-gate.sh`: a browser mode or package that runs zero tests is FAIL, never PASS; a test that
    skips with a `NOT_RUN:` message is NOT_RUN (not FAIL). Parse `go test -json` where available.
F6. Browser harnesses that start `next dev` kill the whole process group on exit (studio-bff,
    orders-bff and any other) — leaked next-server children held ports.
F7. Browser gates must not rewrite tracked `.impeccable/review/*.png`; write run output under output/.

## Accepted as-is
F8. apps/admin/proxy.ts matcher `/api/stores/:path*` (maintainability root-cause fix): accepted.
F9. Node unit suites without a test-local mode run from `release-gate.sh` (GATES.md lists them).
F10. Remaining hand-written "Depends on / Used by" header lists (~45): sweep them out (P2), the
     generated dependency map is the only source.
F11. media-worker stays out of the R1 compose stack (live media is not in R1); smoke S29m/I8 BLOCKED is
     an accepted known limit owned by the media lane. Operator CLIs ship in the lc-go image; no
     long-running service mounts registrar DSNs (S44).
F12. T12 SANDBOX tier stays NOT_RUN: the real hosted Stripe page with card 4242 is already proven by
     SP18 (stripe-browser, both viewports); T12 proves the loop with Stripe MOCK.

## Security review (6 verified P2, 0 P0/P1) — unit sec-fixes
S1. Checkout create path gets the refund path's RD4 protection: after a recorded first-send rejection
    (400/401/403) `mark_stripe_create_sent` returns CLOSED, never RESEND.
S2. Stripe webhook: validate the endpoint UUID against a registered endpoint (cheap indexed lookup) or
    read/limit the body before taking an admission slot, so unauthenticated requests cannot hold all
    32 slots; add a per-request read deadline.
S3. Meta webhook POST: bounded in-flight admission (same pattern as Stripe) + read deadline.
S4. Registrar `Qualify` (SANDBOX probe) uses the stored credential at `expected_version` and checks the
    connection's registered account id; env key only as the operator input it verifies against.
S5. Registrar `Rotate` binds the AAD to the registered account id read via the registrar-only
    `payments.stripe_endpoint_account`-style scoped read (or a new one), not the env value.
S6. Tracking URL: Go canonicalizer rejects hosts containing characters outside RFC 3986 reg-name
    (`<`, `>`, `"`, etc.) and the SQL CHECK is tightened to match; buyer/admin renderers unchanged.
Each fix: root cause, a test that fails before, contract note where the contract text changes.
