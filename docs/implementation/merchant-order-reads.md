# Merchant order reads: authority and rollout

Implemented against the frozen
[merchant order contract](../../contracts/merchant-orders-v1.md), with bounded
local [MOR01–06 acceptance](2026-09-25-merchant-orders-acceptance.md) at `25d4303`.
This runbook does not authorize production rollout. Merchant UI and shipment
operations are separate deliveries.

## Call chain and stored data

`httpapi.NewHandler` registers GET list/detail under
`/v1/admin/stores/{store_id}/orders`. Existing bearer authentication and
`platform.WithScope` require `orders:read`; that scope already requires an active
merchant session, tenant, store, membership and `store:read`.

`internal/merchantorders` reuses `pagination.Page`, binding its two-key cursor to
the authenticated tenant, store and commercial-state filter. The SQL function
`identity.read_merchant_orders` independently checks the token and transaction
scope. Its one bounded data statement reads order, original attempt, financial
facts, sticky review and work item from the same snapshot. A separate fresh auth
statement follows any data wait, before either data or a not-found response.
Go rechecks the original scope revision before decoding the result.

The privileged function returns only the contracted projection. Lists have no
recipient/address data. Detail includes whitelisted fields from the original
checkout snapshot, not current catalog/destination records. Both routes return
private/no-store responses. A malformed projection is unavailable, never a
partial success. Payment state is factual, not inferred from a redirect or order
status. `READY` is a durable payment work item, **not shipped**. Pickup history
is the original merchant attestation, **not a carrier eligibility guarantee**.

## Browser ingress

The admin's existing `/api/stores/{store}/orders[/{order}]` BFF reuses its
HttpOnly session cookie, authorized store listing and server-only bearer. It
does not enable the dev fixture bearer for order/recipient reads. Go remains
the `orders:read` authority; a selected store or catalog access is insufficient.

Next 16.3.5 reconstructs URLs before app-route handling, which can discard a bare
`?` or normalize encoded/empty query segments. Therefore `next.config.ts` retains
the official `skipProxyUrlNormalize` flag and `proxy.ts` checks the raw order
query first. `lib/orders-request.ts` is shared with the app route: never replace
this with two drifting validators or a browser-provided "original URL" header.
The early check rejects syntax only; it does not grant store or session access.

Reproduce with `bash scripts/dev/test-local.sh --browser-merchant-orders-bff`.
This includes production Next, signed-mock OIDC, real Go/PostgreSQL, a local dev
fixture-only negative control and raw Node HTTP requests that preserve malformed
query syntax. The command uses only task-owned disposable infrastructure. Rerun
identity/settings and merchant-to-buyer gates after Next or Proxy changes.
Current evidence and retained failures are in the
[BFF evidence register](2026-09-25-merchant-orders-bff-acceptance.md). This does not
add an order page or authorize production access.

## Permissions and migration 0027

No new SQL role, SDK, service, queue or transaction engine is introduced.
`commerce_auth` is the existing NOLOGIN definer. It receives schema USAGE and
column-level SELECT only for the five tables used by the projection. Runtime
receives function EXECUTE but no direct checkout schema/table privileges.
Historic 0018 scoped runtime access to financial facts/review/work is unchanged.

Fresh initial-store creators receive `orders:read` atomically with their other
grants. Existing memberships are **not backfilled**. Replaying onboarding after
revocation does not restore permission. Deployers must arrange an explicitly
approved provisioning change for existing stores; neither catalog permission
nor a remembered creator identity authorizes the endpoint.

Migration 0027 extends the permission CHECK, replaces the onboarding function,
adds the private projection/ACL and adds a tenant/store history index. Like the
existing migration runner, this is an atomic forward migration. Its ordinary
CREATE INDEX and constraint changes may block concurrent writes. Do not run it
against a customer's live store without a backup, measured table size/lock
budget, a deployment window and owner approval. Disposable-fixture migration
success is not a zero-downtime production rollout proof.

Rollback is an application rollout decision, not dropping checkout data or
removing grants from users. Do not alter checksums of already deployed migrations.
Keep existing buyer, payment and expiry services running unless an approved
rollout explicitly requires otherwise.

## Verification and limits

`bash scripts/dev/test-local.sh --merchant-orders` runs the independent focused
real-PG/race tests on a labelled disposable fixture. The root must also repeat
the complete `bash scripts/dev/test-local.sh` regression and `go vet` gate after
integration. Source review is independent of authorship; preserve failed runs.

Required checks include direct SQL calls (not just the Go fence), cross-store
and cross-owner fixtures, post-wait revocation/expiry including missing results,
immutable detail history, real signed-mock financial transitions, exact safe
fields and no mutation of orders, inventory, facts, events or queue state.

This release does not add merchant pages, exports, shipment labels, fulfillment
commands, manual paid flags, refunds or provider calls. No additional browser
gate is claimed by backend tests. UI delivery must add its own desktop/mobile,
locale and session-switch acceptance before being presented as operational.
