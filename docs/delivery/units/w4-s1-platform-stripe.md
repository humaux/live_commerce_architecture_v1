# Unit W4-S1: platform Stripe for every store (self-serve enablement and attribution, backend, money path)

**Status.** DRAFT, written 2026-10-06 by a Claude Opus subagent. It waits for integrator review and freeze.
- Base: `r3/integration` `2fa471a1`.
- Migration placeholders: **0137** and post-River **0022**. The integrator assigns the final numbers; 0137 is free
  because W4-02B is cancelled.
- Covers [stripe-platform-account-v1](../../../contracts/stripe-platform-account-v1.md) §1–§5, §7 and §8. The
  settlement ledger is a separate unit, W4-S2.
- Worktree `.worktrees/w4-s1-platform-stripe`, branch `unit/w4-s1-platform-stripe`.

**Read first:** PREAMBLE → AGENTS.md → PROCESS.md → this file. Then read
`contracts/stripe-platform-account-v1.md` in full (it is short). Read only these sections elsewhere:
- `stripe-psp-v1.md` §0.2 ("Account identity", "Per-account webhook routing") and §5.4, §9.1;
- `stripe-live-enable-v1.md` §3.3–§3.4 and §7;
- `stripe-refund-v1.md` RD3 and RD13.

## Integrator 裁决 (overrides the body)

**Owner decision (2026-10-06, verbatim):** 「不使用PAYUNi，直接允许商家使用平台的stripe进行收款」

**Rulings.**
- Every money fact stays in the store's own scope. The platform account is shared **only** through derived connections
  that point at the one designated platform connection. Start, worker and refund admission are **not** rewritten. The
  only runtime deltas are:
  - the `aad_*` loader columns;
  - three `create_params` keys;
  - the prepare mapping (contract §3.4, §4.1, §4.2).
- The derived credential is a **byte copy** of the platform envelope. Never re-encrypt it, never decrypt it in SQL or
  `cmd/api`, and never put a plaintext key anywhere new.
- Metadata (`lc_store`, `lc_attempt`) is never authority. A forged `lc_store` → `reference_mismatch` (PF05).
- Evidence ceiling: MOCK + REAL_PG + HTTP_PG. SANDBOX (PF-SBX) needs the owner's test keys, otherwise NOT_RUN. **Never
  LIVE, never real money, never a live key in the test environment** (harness guard SL06).
- Architecture §2.2 deviation AD-PF1 is recorded in the contract §0.2. Do not cite §2.2 to refuse the work.
- Superseded rulings (do not implement): "Stripe only for the owner's HK store, operator-registered, per-store approval
  (D3)" and "Taiwan cards via PAYUNi".

**Open owner questions.** None blocks this unit. Each has a default in contract §11.

| # | Question | Default |
| --- | --- | --- |
| OQ-1 **blocking LIVE opening** | Stripe's permission to collect for third-party TW stores without Connect | not established |
| OQ-2 | Settlement cycle | weekly |
| OQ-3 | Platform fee % and the fee/FX/dispute bearer | 0%; Stripe fees passed through; FX borne by the platform |
| OQ-4 **blocking** | Merchant 代收款 terms text | — |
| OQ-5 **blocking** | Taiwan e-invoice / tax responsibility | — |
| OQ-6 | Stripe payout setup (`payouts_enabled=false` today) and the TWD bank route | — |
| OQ-7 | Dispute handling | — |
| OQ-8 | Buyer-visible names | — |

## Goal / owner flow

1. **Owner (once).** Operator flow on the platform store in TWD: register, webhook, live-approve, qualify, method,
   canary. Then `stripe-admin platform-designate` and `platform-open`.
2. **Merchant.** Admin 「收款設定 → 信用卡」 → reads the terms and the descriptor preview → 「啟用」. The storefront
   offers card payment immediately. The buyer sees 「由 {平台} 代 {店名} 收取」.
3. **Operator.** Blocks one store with `platform-block`, or closes enrollment with `platform-close`.

## Key facts (base `2fa471a1`; run `grep -n` again before implementing)

- **Store-scoped composite FKs force derived per-store rows.**
  - attempts → `account_credentials(tenant,store,connection,version)` (0016:48);
  - qualifications → credentials (0016:15);
  - method_versions → qualifications (0016:26).
- **Unique indexes** `stripe_one_account_per_store_environment` and `stripe_account_identity_unique` (0061:30-33).
- **Loader** `integration.load_stripe_credential` (0061:947) returns the scope used for AAD. Go AAD is built in
  `internal/integrations/accounts/stripe_crypto.go:144` (`stripeAPIAAD`) and opened in `OpenStripeAPI` (:164).
- **Latest bodies.** Take the most recent of `start_stripe_payment`, `stripe_webhook_prepare`, `load_stripe_refund`,
  `qualify_stripe_method`, `set_stripe_method` and `revoke_stripe_live` from 0077, 0096,
  `post_river/0016_stripe_live.sql` and `post_river/0019_worker_authorities.sql`. Find them with
  `grep -ln 'FUNCTION <name>' migrations -r | sort | tail -1`.
- **Create key set** is built in SQL in `start_stripe_payment` (post_river/0016:90-97) and validated in Go
  `internal/integrations/psp/stripe/params.go`.
- **CLI:** `cmd/stripe-admin/{main.go,live.go}` with `internal/payments/stripeadmin/{registrar.go,live.go}`. Allowlist
  `deploy/scripts/ops-admin.sh:56-57`.
- **Hosted view:** `internal/checkout/hosted.go` (Stripe branch) and `checkout.hosted_payment_view_v2`.

## Scope

1. **SQL 0137** (contract §3.1–§3.3):
   - new tables `payments.stripe_platform` and `payments.platform_stripe_enrollments`;
   - widened `merchant_accounts`, `account_credentials`, `account_qualifications`, plus the guard triggers;
   - functions `set_platform_stripe`, `read_platform_stripe`, `designate_stripe_platform`,
     `set_stripe_platform_open`, `block_platform_stripe`, and the internal `platform_stripe_fanout`;
   - re-created registrar definers with the fan-out and derived-refusal deltas;
   - loaders gain the four `aad_*` return columns (DROP + CREATE + re-grant in the same migration).
2. **SQL post-River 0022:**
   - `start_stripe_payment`: +`lc_store` (two keys), plus `statement_descriptor_suffix` for derived connections with a
     suffix;
   - `stripe_webhook_prepare` and the refund-event branch: mapping delta per contract §4.2;
   - `hosted_payment_view_v2`: the `collector` object.
3. **Go:**
   - `stripe_crypto.go`: open with the `aad_*` scope. Callers in `payments/stripe_runtime.go`,
     `payments/stripe_refund.go` and `stripeadmin/registrar.go` pass it through. A derived row whose `aad_*` scope
     differs from its own scope is admitted only when the account id matches.
   - `params.go`: allowed keys + SP03 vectors.
   - `checkout/hosted.go`: `collector`.
4. **Merchant HTTP:**
   - `GET /v1/admin/stores/{store_id}/payments/card` → `read_platform_stripe`;
   - `PUT …/payments/card` with body `{enabled, terms_version, descriptor_suffix?, expected_version}` →
     `set_platform_stripe`;
   - errors map to 403/409/422 codes named in the contract;
   - the profile comes from `COMMERCE_PAYMENT_PROFILE`, never from the request.
5. **CLI:** `stripe-admin platform-designate | platform-open | platform-close | platform-block | platform-unblock`, with
   flags per contract §3.3 and `--operator`/`--ticket`. `ops-admin.sh` allowlist gains these five. None takes a
   secret. `platform-block` and `platform-close` do not require the flag+ref pair (kill switch). `platform-open` on
   LIVE does require it.
6. **Contracts.** The integrator freezes `stripe-platform-account-v1.md`. The pointers in stripe-psp, stripe-live-enable
   and stripe-refund are already written.

## Non-goals

- the settlement ledger, balance transactions and payouts (W4-S2);
- admin and storefront UI (W4-U1, Codex);
- Connect, `on_behalf_of`, transfers;
- multi-currency platforms (TWD only);
- automated fan-out jobs above 2000 enrollments;
- dispute webhooks;
- removing PAYUNi (`pay-remove-payuni`);
- migrating merchants off existing primary Stripe connections. They stay valid, and a store cannot hold both
  (`stripe_store_has_own_account`).

## Migration path for existing per-store approvals and bindings

- **Production.** No LIVE Stripe row exists in production: no production host yet, and SL-LIVE01 is NOT_RUN. 0137 moves
  no data.
- **The owner's existing primary SANDBOX or LIVE connection** is designated as the platform connection
  (`platform-designate`). Its approval and canary rows are reused unchanged.
- **Any other store with its own primary Stripe connection** keeps working as before. To join the platform it must
  first be disabled and have its connection retired by the operator. That is not in this unit: listed as an OPEN item,
  and none exists today.
- **0137 asserts.** If any `stripe_live_approvals` row exists on a non-platform connection, the migration does NOT fail.
  It only `RAISE NOTICE`s with the count, so the integrator sees it in the migrate log.

## SQL authority (SECURITY DEFINER shape)

- **Merchant functions**:
  - owner `commerce_payment_registry_writer`, `SECURITY DEFINER`, `SET search_path=pg_catalog`;
  - `identity.resolve_access(p_token, p_store, '<perm>')` first, then `set_config('app.tenant_id'|'app.store_id', …, true)`
    from the resolved scope;
  - `REVOKE ALL … FROM PUBLIC; GRANT EXECUTE … TO commerce_runtime`.
- **Operator functions**: same owner, `require_stripe_registrar_scope` on the platform store,
  `GRANT EXECUTE … TO commerce_payment_registrar` only.
- `platform_stripe_fanout`: no EXECUTE grant to any login. Called only from definers owned by the same role.
- Every function: `COMMENT ON` names the owner package, the callers and the non-goals.

**ACL pins to update** (the integrator updates the expected rows; the author proposes them in DELIVERY):
- `tests/foundation/stripe_authority_test.go`: new functions in the matrix;
- `tests/foundation/stripe_live_schema_test.go`: SL02 column-privilege matrix for the new columns and tables;
- `tests/foundation/worker_authority_split_test.go`: WAS02, five worker logins with no EXECUTE on the new functions;
- `tests/foundation/merchant_orders_v2_acl_test.go`: runtime EXECUTE list;
- `tests/foundation/r2_integration_upgrade_test.go`: upgrade replay, if 0137 touches post-River ordering.

## Write paths (single owner)

**Backend (Claude Sonnet):**
- `migrations/0137_platform_stripe.sql`, `migrations/post_river/0022_platform_stripe.sql`;
- `internal/payments/platformstripe/` (new: service + HTTP adapter types);
- `internal/httpapi/payment_card.go` (new route file);
- `internal/integrations/accounts/stripe_crypto.go` (AAD scope);
- `internal/integrations/psp/stripe/params.go`;
- `internal/payments/stripe_runtime.go`, `internal/payments/stripe_refund.go` (pass `aad_*`);
- `internal/checkout/hosted.go` (collector);
- `internal/payments/stripeadmin/platform.go` (new), `cmd/stripe-admin/platform.go` (new);
- `tests/foundation/platform_stripe_test.go` (author smoke, PF02 happy path only).

**Integrator hooks:**
- `internal/httpapi/handler.go` (one registration);
- `cmd/stripe-admin/main.go` (dispatch line);
- `deploy/scripts/ops-admin.sh` (allowlist);
- `scripts/dev/test-local.sh` (mode `--platform-stripe` if needed);
- contract freeze, GATES.md, the ACL pins above.

**Independent tests (Kimi K3):** `tests/foundation/platform_stripe_gate_test.go` (PF01–PF08).

## Tests (red first, then green; save the red run to `output/w4-s1-platform-stripe/red.log`)

| # | Case | Expected |
| --- | --- | --- |
| PF01 | schema: derived row on a non-designated connection; mismatched account or env; a second primary row on the same account; credential bytes ≠ platform envelope; REAL_LIVE qualification with neither link | each rejected; FORCE RLS on; ACL matrix as §3.5 |
| PF02 | merchant enable: no `billing:manage`; platform NONE/DESIGNATED/CLOSED/REVOKED; blocked; store suspended; store has its own connection; currency ≠ TWD (LIVE); stale terms; suffix too long (L=10 → 11 chars); wrong version | each refused with its code. A valid enable creates binding, derived account, credential v1, qualification, method head and enrollment. A replay creates nothing. Disable succeeds in every one of those states. |
| PF03 | fan-out: platform rotate / qualify / live-revoke with 3 stores, 1 of them blocked | 2 stores updated in the same transaction (new credential version, new qualification, head CAS), the blocked one untouched; revoke leaves no valid derived qualification; 2001 enrollments → `platform_fanout_too_large`; rotate/qualify/approve/webhook on a derived connection → `stripe_platform_derived` |
| PF04 | MOCK start on stores A and B | attempt scope = own store and derived connection; `create_params` has `metadata[lc_store]` and `payment_intent_data[metadata][lc_store]`, plus a suffix only for B (suffix set); worker opens the envelope via `aad_*`; a primary-store attempt's body is byte-identical to before, except the `lc_store` keys |
| PF05 | one platform endpoint, events for A, B, forged `lc_store`, an unrelated account's session, redelivery, a refund event | receipts in A/B scope; `reference_mismatch`; `unknown_session`; `DUPLICATE`; refund receipt in the refund's store |
| PF06 | refunds: A's merchant targets B's order; over capacity; after disable, block, close and revoke | not found; refused by RD3; accepted |
| PF07 | kill switches (contract §7) | new start PT409 while an in-flight MOCK attempt still reaches CAPTURED or CLOSED_UNPAID; blocked then re-enable refused; unblock then re-enable re-derives |
| PF08 | routes | DB-free router build (route conflicts); GET/PUT happy path + 403/409/422; hosted view `collector` for derived, `null` for primary; no `acct_`, key or approval id in any JSON |

## Gates

**Local, focused regexes only (PREAMBLE §2):**
- `bash scripts/dev/test-focused.sh '^TestPlatformStripe'`
- `bash scripts/dev/test-focused.sh '^TestStripe(SP03|SP19|SL02|SL03)'`
- `go test ./internal/integrations/psp/stripe/... ./internal/integrations/accounts/... ./cmd/stripe-admin/...`
- `bash scripts/dev/check-gates.sh` (includes the header ratchet)

**CI gates** (list them in DELIVERY; the integrator runs `.github/workflows/gates.yml`):
- the full foundation suite (SP01–SP21, RF01–RF12, SL01–SL09 regression = PF14);
- `test-local.sh --stripe-browser`, `--browser-payment`, `--browser-refund-fulfilment`;
- `release-gate.sh --strict`.

## Evidence / role / dependencies

- **Evidence:** MOCK + REAL_PG + HTTP_PG. PF-SBX is SANDBOX and NOT_RUN without the owner's test keys. **LIVE is never
  run by an agent.** The platform canary and the first real-merchant order are owner-run (PF-LIVE, I17).
- **Role:** backend = **Claude Sonnet**, because DeepSeek is unavailable. Independent gate: Kimi K3 (PF01–PF08).
  Money and secrets review: Claude Opus (security_reviewer ≠ author).
- **Dependencies:** none must merge first. 0136 (W4-01B) is merged. Do not run this in parallel with any unit that
  re-creates `start_stripe_payment` or `stripe_webhook_prepare`. W4-S2 starts after this merges (it reads derived
  connections).

## OPEN (with recommendations)

- **S1-OPEN-1: retiring a store's own primary connection so it can join the platform.**
  **Recommendation:** a later operator CLI `retire` (disable method + endpoint, keep rows). No such store exists today.
- **S1-OPEN-2: `aad_*` columns vs a second loader.**
  **Recommendation:** add the columns, which is one signature change per loader. A parallel loader would duplicate the
  lease fence.
