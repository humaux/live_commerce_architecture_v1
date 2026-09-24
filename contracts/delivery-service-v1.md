# Merchant delivery service revisions v1

Status: FROZEN_FOR_IMPLEMENTATION, 2026-09-20, base `058912c`.
Executable prerequisite of checkout, not complete shipping, public UI or carrier
acceptance. Implements the configuration portion of merchant-service-settings-v1.
Internal configuration accepted at `2a5e43c` on 2026-09-24;
see [real-PG and independent review evidence](../docs/implementation/2026-09-24-delivery-service-acceptance.md).

## Surface

Package `internal/fulfillment`, caller-owned transaction; no pool, HTTP or provider
I/O. `SetService(ctx, tx, platform.Scope, token, key, ServiceInput) (Service,error)`;
`GetService(ctx, tx, scope, token, marketID,country,code) (Service,error)`.
Writes require `integration:manage`; reads require `integration:read`. Both recheck
opaque credential and exact transaction GUC scope before replay or reading.
No new permissions/role and no automatic grant to existing users.

Service key: tenant/store/market/country/code. Code is `[a-z][a-z0-9_-]{0,39}`.
Input: MarketID, Country (ISO-shaped uppercase two letters), Code, ExpectedVersion
(0=create), PolicyVersion (>0), NameHans/NameHant/NameEN (nonblank printable UTF-8,
1..120 codepoints), DeliveryKind (`home|cvs_711|cvs_familymart`), Mode (`MANUAL|API`),
Enabled, Visible, SortOrder (0..1000), optional BindingID and BindingVersion.
Currency is derived from locked market/policy, never a free input. CVS requires TW.
Output includes those saved configuration fields, Version, Currency, PolicyMethod.
No `available=true`, credentials, external asset ID or carrier-verification claim.

Label validation uses Go's UTF-8/Unicode printable rules at the supported SetService
boundary. SQL additionally guards length, ASCII-space-only and control labels; its
locale-based character classes are not Go Unicode parity. A trusted runtime SQL
caller can bypass some cosmetic Unicode rules, but cannot bypass the API-disabled,
binding, scope or composite-FK constraints. No public SQL write interface is exposed.
Do not replace the SQL regex with `[:print:]` and claim equivalent validation;
PG18.6 accepts format/private-use characters that Go rejects. If another label-write
entrypoint is added, it must reuse the Go validation or receive a separate reviewed
database validation gate. This bounded defense-in-depth limitation remains P2.

## Pricing without a second engine

Extend allowed pricing method keys to existing legacy values OR
`delivery:<code>`. `pricing.DeliveryMethod(code) (string,error)` returns this exact
reserved key after validation. A merchant first creates the matching policy through
existing `SetPolicy`, then configures service referencing its current enabled version.
Two home services can therefore have independent fees. Do not copy amounts/tax into
service tables; keep `Calculate` unchanged. No other arbitrary method is accepted.
Old cart/quote behavior remains compatible; new method alone is not checkout authority.
Quote can become stale when policy changes; service must be revised to reference the
new policy before being usable again. Create, enable and configuration changes must
reference the current enabled policy. The only stale/disabled-policy exception is
an existing Enabled=true service being switched to false with all fields unchanged
except Enabled and optional Visible true->false. It preserves the exact historical
policy/binding references (never any other old version) and does not require current
binding readiness or an active market; an operator must always be able to stop new use. Other changes
must use the current enabled policy. The historical FK must still exist.

## Persistence and transitions

Forward migration `0010_delivery_services.sql`; new `fulfillment.service_versions`
and `fulfillment.service_heads`. Composite keys include tenant/store/market/country/code.
Immutable versions include actor and current config. `policy_method` is generated
from code; composite FK binds exact pricing policy version/currency. Optional binding
FK includes tenant/store, and non-null binding implies positive expected version.
Actor FK points to merchant membership. Head references a saved version.
Only merchant runtime SELECT/INSERT versions and SELECT/INSERT/UPDATE(current_version)
head, with forced tenant/store RLS and principal-bound inserts. No DELETE, version
UPDATE, buyer/worker/issuer grant. Future checkout has its own read authority.

Lock order: credential/scope -> command key -> market -> current policy head ->
service advisory key -> service head -> optional integration binding. Future checkout
must use the same policy -> service -> binding order. All row locks
held to caller commit/rollback. Create missing head serialized by natural key.
ExpectedVersion must match; append exactly version+1 and advance head with CAS.
Request hash includes actor and every input; successful command receipt and audit
`fulfillment.service.set` commit atomically. Identical same-key replay returns original
saved config even if later revised; changed input conflicts. Replay does not claim
current availability. Failed validation/version/audit leaves no facts.

MANUAL forbids a binding; may be enabled without a carrier. API may store a draft
with or without binding. If present, verify same-store binding/version; it can be
disabled while saving draft. Since no carrier adapter is implemented, API Enabled
must be false in BOTH Go and SQL. Do not equate existing binding.enabled with provider
authorization/readiness. Future enabling requires a reviewed migration and real
adapter capability gates, not removing the check merely to turn the switch green.
Visibility is independent of Enabled, including disabled-but-visible saved settings;
public lists must later filter effective availability and not expose drafts.

## Acceptance / explicit limits

Real PG/race: create/CAS/read/replay across multiple same-kind services with different
fees; immutable history; all localized labels; disabled policy/service changes;
cross-tenant/store/actor and permission denial; foreign binding, forged GUC, invalid
country/code/labels/version; direct buyer/worker SQL denial; direct invalid merchant
SQL constraints; parallel same-key and different-key same-version winner; audit/receipt
failure rolls back version/head. Full existing cart/quote/dispatcher regression.
Do not claim inventory hold, destination directory, API readiness, buyer listing,
payment, UI or provider sandbox/live passes from these tests. They remain required
later scope, not deleted requirements. No new dependency.

Independent preflight identified two P1 ambiguities, resolved above before coding:
stale-policy stop transition is exact and narrow; service precedes binding in lock
order. No open P0; implementation/real-PG review remains required before acceptance.
