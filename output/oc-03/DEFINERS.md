# OC-03 guarded definers

Guard: `PERFORM identity.require_recent_auth(<session hash arg>, interval '10 minutes')`, inserted in place (pg_get_functiondef + exactly-one-anchor replace) right after the definer's own authorization succeeded and before any lock or write. An unknown, revoked or expired session therefore still answers 401/404/403 as before; only an authorized member with a stale `authenticated_at` gets PT403 `reauth_required`. EXECUTE on `identity.require_recent_auth(bytea,interval)` is granted to the definer's owner role (read from `pg_proc.proowner` at apply time).

| Family | Definer (full signature) | Latest definition | Owner role | Hash arg | Patched in |
|---|---|---|---|---|---|
| staff | `identity.staff_invite(bytea,uuid,uuid,text,text,text,bytea)` | 0089 | commerce_staff_writer | p_hash | 0203 |
| staff | `identity.staff_set_role(bytea,uuid,uuid,text)` | 0089 | commerce_staff_writer | p_hash | 0203 |
| staff | `identity.staff_remove(bytea,uuid,uuid)` | 0089 | commerce_staff_writer | p_hash | 0203 |
| platform Stripe | `payments.set_platform_stripe(bytea,uuid,text,boolean,text,text,bigint)` | 0137 | commerce_payment_registry_writer | p_token | 0203 |
| refund | `payments.request_stripe_refund(bytea,uuid,uuid,text,bytea,bigint,text,bigint,uuid,bigint)` | post_river/0016 | commerce_checkout_writer | p_hash | post_river/0028 |
| offline refund / bank transfer | `payments.decide_bank_transfer(bytea,uuid,uuid,text,bytea,text,text)` (confirm, reject, refund_offline, refund_offline_restock) | 0099 | commerce_checkout_writer | p_hash | 0203 |
| export (orders CSV, orders:export) | `identity.export_unshipped_orders(bytea,uuid,integer)` | 0073 | commerce_auth | p_hash | 0203 |
| export (finance CSV) | `identity.export_finance_summary(bytea,uuid,date,date)` | 0078 | commerce_auth | p_hash | 0203 |
| export (products export.csv, channels, funnel, manual orders) | `identity.export_report(bytea,uuid,text,date,date,uuid,text)` | 0147 | commerce_auth | p_hash | 0203 |
| export (customer privacy), merchant branch only | `customers.record_export(bytea,uuid,uuid,text,uuid,jsonb)` | 0078 | commerce_privacy_writer | p_hash | 0203 |
| export (customer privacy: imported profile) | `customers.export_import_profile(bytea,uuid,uuid)` | 0156 | commerce_privacy_writer | p_hash | 0203 |
| erasure, merchant branch only | `customers.erase_owner(bytea,uuid,uuid,text,uuid)` | 0078, patched in place by 0192 | commerce_privacy_writer | p_hash | 0203 |
| billing:manage | `billing.pin_customer(bytea,uuid,text,text,text)` | 0079 | commerce_billing_writer | p_hash | 0203 |
| billing:manage | `billing.record_checkout_session(bytea,uuid,text,text,timestamptz)` | 0079 | commerce_billing_writer | p_hash | 0203 |
| ads | `ads.approve_draft(bytea,uuid,uuid,integer)` | 0074 | commerce_ads_writer | p_hash | 0203 |
| ads | `ads.publish_draft(bytea,uuid,uuid,integer,uuid,bigint)` | 0074 | commerce_ads_writer | p_hash | 0203 |

Owner role disabled (no reauth guard): `identity.staff_invite` and `identity.staff_set_role` refuse role `owner`; `identity.staff_accept(bytea,bytea)` (0089, commerce_staff_writer) refuses an owner invitation. All PT403 `owner_role_disabled`.

0201 functions changed in 0203 (issue #215 items 1 and 2): `identity.complete_oidc_reauth` (GREATEST of the old value and LEAST(auth_time, now)); `identity.require_recent_auth` (authenticated_at more than 1 minute ahead, or infinity, is not recent).

## Not guarded, and why (integrator decision needed where marked)

- `payments.request_stripe_refund_refresh` (post_river/0013): records a throttled status-refresh signal, moves no money. Not a refund command.
- `billing.refresh_subscription` (0079): refresh-on-read mirror of Stripe state, called on a plain billing page view; guarding it would break reads for any member with an older session. `identity.read_billing`, `payments.read_store_settlements` (billing:manage) are reads.
- Settlement statement CSV (`cmd/stripe-admin settlement-export`): an operator CLI over `internal/payments/stripeadmin`, no merchant session hash. DECISION: out of the merchant re-auth scope.
- Carrier / pick-list CSV (`internal/merchantorders/carrier_export.go`): formatted in Go from `fulfillment.read_pick_list` after a Go-side `resolve_access` (orders:export). There is no export-only SQL entry; the SQL guard would have to sit on the shared read. DECISION: Go-side call to `identity.require_recent_auth` in the same transaction (OC-15) or a dedicated export definer.
- `claims.erase_actor`, `claims.replay_actor_erasures`, `customers.replay_erasures`, `ops.order_attempt_erase_*`: retention/operator entries (commerce_retention_operator, tenant/store arguments), no session hash. Not merchant-session commands.
- Buyer branches of `customers.record_export` / `customers.erase_owner` and `customers.buyer_export_orders`: authenticated by a buyer capability session, which has no `authenticated_at`.
- `identity.staff_revoke_invite`: not in the §16 list (revoking an invitation grants nothing).
