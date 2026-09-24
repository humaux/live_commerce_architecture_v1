# Buyer destination and carrier-neutral pickup source v1

Status: IMPLEMENTED_INTERNAL_ONLY, 2026-09-24; contract `f974391`, code/tests `5cefa83`.
Prerequisite only: no actual checkout/stock write, provider, map callback or public route.
Actual bounded gates: [164-test regression and browser compatibility](../docs/implementation/2026-09-24-buyer-destination-acceptance.md).

## Merchant-attested source (fulfillment)

`PickupInput`: Kind (`cvs_711`/`cvs_familymart`), Namespace
(`[a-z][a-z0-9_.:-]{0,63}`), Code (`[A-Za-z0-9_-]{1,32}`, exact string),
ExpectedVersion (0..MaxInt64-1), Name (1..120 printable), Address (1..400 printable),
EvidenceRef (1..240 printable), TTLSeconds (1..604800). Never coerce store codes to
numbers, guess a namespace, or convert one provider's code to another's.

`AttestPickup(ctx,tx,merchantScope,token,key,in) (Pickup,error)` uses
integration:manage + exact authenticated transaction scope BEFORE replay; principal
included in command digest. Lock scoped kind/namespace/code advisory -> head FOR
UPDATE, CAS expected version, append immutable source row and update head enabled.
Use DB clock for AttestedAt, ValidUntil=AttestedAt+TTL. Seven days is a local maximum
attestation age, not a provider guarantee. Each version has a fresh opaque UUID ID.
SQL verification_kind is fixed MANUAL_ATTESTED; merchant cannot claim DIRECTORY_VERIFIED.

`RevokePickup(ctx,tx,scope,token,key,pickupID) error` authenticates before replay,
locks the same source head, rejects superseded ID, sets enabled=false, audits in
same transaction; repeats against the same current source are harmless. Attesting a
new revision re-enables it, while old command replay does not. Source row never mutates.
This reusable store-scoped catalogue record can serve many buyers: not per-order
merchant approval. The eventual official directory adapter remains a separate gate.

`Pickup`: ID, Kind, Namespace, Code, Version, Country (`TW`), Name, Address,
VerificationKind, AttestedAt, ValidUntil. Public projection omits EvidenceRef/actor.
`ReadPickup(ctx,tx,buyerScope,id)` authenticates scope and reads immutable history;
`LockPickup(ctx,tx,buyerScope,id)` also locks current head FOR SHARE and rejects
superseded/disabled/expired/future-attested source after waits using DB clock.
Reuse pricing's SELECT + column UPDATE privilege with UPDATE RLS WITH CHECK(false)
for buyer row locking: this MUST NOT permit even a no-op source update.

## Buyer snapshot (storefront)

`DestinationInput`: ExpectedVersion (0..MaxInt64-1), CartVersion (>0), Kind
(`home`/two CVS kinds), Country uppercase2, RecipientName, Phone, HomeAddress,
PickupID. HomeAddress fields: Region(optional,max100), City(required,max100),
PostalCode(optional,max20), Line1(required,max200), Line2(optional,max200).
RecipientName 1..120 printable; Phone 6..32 chars of `[+0-9() -]` with 6..20 digits,
preserved as supplied; syntax is not carrier acceptance or telephone ownership.
All text must be valid UTF8 printable, nonblank when required, and have no leading/
trailing whitespace; optional empty fields allowed. CVS requires TW, exact valid
PickupID and empty HomeAddress; home requires no PickupID. No buyer code/name/address
or verified flag may substitute for the trusted pickup relation.

`SetDestination(ctx,tx,buyerScope,key,in) (Destination,error)`: buyer.WithScope is
the authentication boundary; CheckScope precedes any read/replay. buyer.RunCommand
operation destination.set stores ONLY result `{id,version}` (request is hashed),
never recipient/address in receipt. Replay reads owned immutable snapshot and does
not imply current eligibility. Session-scoped dedup is acceptable for this intent,
not for future permanent checkout receipts.

Lock existing owned cart FOR SHARE; nonempty cart and current CartVersion required.
CVS locks source head; then owner/cart destination advisory and head FOR UPDATE.
CAS ExpectedVersion (0 means absent). Snapshot IDs are generated server-side, append
version and advance head atomically; operation event contains IDs/action only.
SelectedAt is final DB clock AFTER lock waits. ExpiresAt=min(SelectedAt+30min,
source.ValidUntil when CVS); recheck source attestation/freshness then insert.
No catalogue/market lock is taken after destination/source locks.

`Destination`: ID, Version, CartID, CartVersion, Kind, Country, RecipientName, Phone,
HomeAddress, optional Pickup (*fulfillment.Pickup), SelectedAt, ExpiresAt.
`GetDestination(ctx,tx,s,id)` authenticates owner, permits immutable historical read.
CVS stores ONLY the scoped pickup ID FK, not buyer-supplied copied fields. Get joins
the immutable trusted revision. Ordinary merchant/worker/issuer has no destination
PII grant. SQL FK binds owner/cart/session and pickup ID/kind/country.

`RevalidateDestination(ctx,tx,s,id,cartVersion,country,kind) (Destination,error)`:
scope; owned snapshot; cart share lock/current nonempty cart; source head share lock;
destination head share lock; require exact current ID/version, owner/cartID/cartVersion,
country/kind, valid text/field combinations. Final DB clock: SelectedAt<=now,
ExpiresAt>now, ExpiresAt<=SelectedAt+30m; for CVS source.AttestedAt<=SelectedAt and
ExpiresAt<=source.ValidUntil, current source still enabled/fresh. Re-check after waits.
Rows inserted directly by ordinary buyer SQL and buyer receipts are NOT attestation.
This is read/lock validation, NOT authorization to reserve stock or send a provider call.

Checkout lock order is cart/quote -> market/policy -> catalogue -> pickup source
head (if CVS) -> destination head -> service head -> allocation head -> warehouses/
balances -> final DB time -> aggregate facts. RevalidateDestination may reenter the
already held cart share lock. Later extra waits REQUIRE another clock check before
hold. Source mutation never takes a destination/cart lock, preventing reverse order.

## Database and privacy

Migration0012: immutable fulfillment.pickup_versions, current pickup_heads;
immutable storefront.destination_snapshots and destination_heads; owner-only
destination_events. Forced RLS, composite FKs, actor/session INSERT checks. No
history update/delete grants, no buyer source INSERT, no PII in command response or
event. PostgreSQL CHECKs provide structural/length/ASCII-control bounds; Go performs
full Unicode validation on writes AND revalidation. Retention/deletion workflow,
production encryption-at-rest and actual merchant order PII access remain separate
release gates; no universal legal retention period is invented.

## Gate

Real PG roles/RLS/FKs and source no-op mutation denial; before-replay authentication;
actor/source CAS, buyer destination CAS and concurrent replay; leading zero/namespace;
home+CVS snapshots; cross-owner/store/session provenance; no duplicated PII in receipts/
events; late old selection fails; cart/source revoke/reattest/future time/expiry
rejected, including after actual lock waits; complete rollback on event/receipt faults;
historical reads remain stable. Full race/vet regression and browser compatibility.
NOT_RUN: official directory search/map callbacks, provider route/COD acceptance,
public UI, actual checkout/stock/expiry/payment, complete SaaS gates.

Decision: retain destination head to enforce latest selection and delayed-callback
CAS. Reject client verified flags, direct source writes, copying trusted addresses
into buyer-controlled columns, per-order merchant approval, and fixed carrier gate.
