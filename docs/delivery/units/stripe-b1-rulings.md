# Stripe B1 integrator rulings (2026-09-29) — binding for stripe-b1-start-http,
# stripe-b1-ingress-assembly, stripe-b1-tests-b

These settle the open items raised while freezing the three briefs. Where a brief's default
differs, this file wins.

1. **Attempt id for refresh/cancel** — ACCEPTED as drafted: read it from the buyer's own
   `checkout.command_results` row; the SQL `request_stripe_signal` check is the authority and
   rejects a wrong id (fail closed). Comment the call site with that reasoning.
2. **Provider of a handoff** — REJECTED the SAVEPOINT + error-message match (message text is not
   an API). Instead append to `migrations/0061_stripe_psp.sql` one scoped, read-only
   `SECURITY DEFINER` function (pinned `search_path`, PUBLIC revoked, `COMMENT ON`) that returns
   the provider of the order's current payment attempt (`'stripe'|'payuni'|NULL`) for the
   caller's tenant/store scope, EXECUTE granted only to the hosted runtime role that already calls
   `take_stripe_handoff`. Follow the scoping pattern of the existing hosted definer functions.
   Owned by stripe-b1-start-http. (0061 has never run outside disposable test DBs, so it is
   amended in place; the migration checksum changes on purpose.)
3. **Prepare replay digest** — ACCEPTED: PAYUNi keeps its existing digest bytes; Stripe uses the
   Stripe config digest.
4. **`cancel_requested`** — AMENDED: the field is emitted only for orders whose attempt provider
   is Stripe (nil → omitted for PAYUNi). PAYUNi responses stay byte-identical whether or not Stripe
   is enabled; SP14 golden files assert both configurations.
5. **Environment** — ACCEPTED: API and worker read no `STRIPE_*` variables; webhook signing
   keyring from `COMMERCE_STRIPE_WEBHOOK_*` via the existing `accounts.LoadKeyring` with remapped
   names. `deploy/compose.yml` + `deploy/env/*.example` must list the new variable names (values
   never).
6. **Webhook edge cases** — ACCEPTED: endpoint profile ≠ API profile → 404; signing key cannot be
   decrypted → 503 `signing_unavailable` (Stripe retries).
7. **Registrar probe** — ACCEPTED: add `stripe.(*Client).ProbeCheckout` to the stage-A adapter
   as a narrowly scoped addition (its own golden-body unit test, same key/profile guards, same
   LIVE refusal). SANDBOX qualify runs with `STRIPE_SANDBOX=1`.
8. **Registrar cannot read the account row** — MITIGATED: `Register` and `Rotate` must call the
   adapter's `VerifyAccount` with the supplied secret key and require the returned account id to
   equal the operator-supplied `accountID` (and `livemode=false` in SANDBOX) **before** any SQL
   write; mismatch → `ErrRejected`, nothing written. `SetWebhookEndpoint` keeps the runtime
   fail-closed check only (documented limit).
9. **MOCK tier** — ACCEPTED: SP08–SP12 run the same assembly functions the binaries call,
   in-process, with real PG + real River + `stripetest`; binaries are exercised by SP15
   (startup/flags/SIGTERM) only. The evidence file says so.
10. **Ownership** — ACCEPTED: registrar logic in `internal/payments/stripeadmin`, thin
    `cmd/stripe-admin`. The integrator delegates `cmd/api` mount / worker assembly to
    stripe-b1-ingress-assembly and `buyerhttp` routes to stripe-b1-start-http, and reviews them.
11. **Order** — stripe-b1-pool-fix merges before any PG gate of these units can be green. Units may
    start coding now; their PG runs happen after the integrator merges pool-fix into the release
    branch and they rebase.
