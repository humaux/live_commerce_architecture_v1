VERDICT: FAIL

Reviewed: `contracts/order-attempt-v1.md` @ 99674d85 (PR #50, base trunk 0bfe36e5). Reviewer: Claude (Opus 5.5), read-only, E1/SOURCE. No tests run (NOT_RUN).

The core design holds up against the code: one C2 transaction, an immutable receipt, an all-state generation CAS, terminal states never regress, the cancel-before-authorization guarantee and its honest limit. The problems are gaps that would restart the review loop. Six P1s block the freeze.

## P1

**P1-1 §1/§3: no rule for PENDING liveness or River retry exhaustion.** §6 promises "every valid new PENDING settles" only "with fair scheduling". Nothing covers a River job that is discarded after max attempts, or a persistent definer error that is not classified. Nothing bounds how long a PENDING row may hold the scope fence (which blocks every order by that staff principal), the C1 bundle reservations or the live-price grants. Grants "expire with settlement", so they never expire while PENDING. §3 says "missing policy classification is a freeze blocker" but gives no classification.
*Fix:* freeze an `execute_by` deadline into C1. Add a River periodic sweeper that re-enqueues any PENDING row with no live job. Past the deadline, the sweeper takes the fence and aborts with ABORTED(EXPIRED). The absence proof is trivial and should be stated: C2 writes COMMITTED atomically, so a PENDING attempt row locked FOR UPDATE proves zero C2 effects. Add a SQLSTATE table: 40001, 40P01, 55P03, 57014 and connection errors are retryable and keep PENDING; each PTxxx code is either definitive (ABORTED with a stable code) or a notification VOID. A discarded job must never leave PENDING without a sweeper. Add a REAL_PG case for exhausted or discarded jobs and a mutation that removes the sweeper.

**P1-2 §2 vs §3: frozen price contradicts C2 revalidation.** §2 says C1 sets the authoritative price and C2 "must not reprice". §3 says to resume "after old … quote windows", but also that "quote expiry or incompatible required version checks settle ABORTED". The C2 quote row is created at C2 time (`0007_cart_quote.sql:92`, 30-minute CHECK), so "quote expiry" has no defined referent. Today's `begin_hold` path rejects any change to service version, enabled/visible or policy version (`checkout.go:315–340`). It is undefined whether a merchant price edit or a service-fee bump between C1 and a later resume gets honored or aborts.
*Fix:* add a table that lists each frozen fact as either HONORED or REVERIFIED, with the result of each:
- HONORED: unit and live price, policy version, COD surcharge, destination.
- REVERIFIED: stock, SKU active, service enabled, binding, erasure.
Bound validity with `execute_by` (P1-1), not with quote TTL.

**P1-3 §2/§5 A15/A16: the cart schema cannot hold the required plan.** `storefront.carts` is UNIQUE per owner, and `cart_lines` is keyed per SKU (`0007_cart_quote.sql:65,73`). That has three consequences:
- "Duplicate SKU origins remain separate price-eligible quote lines" cannot be stored.
- C2 "cart materialization" overwrites a linked buyer's live storefront cart.
- C2 races that buyer's own session, and other principals ordering for the same owner, on the `cart_id/cart_version` active-order check (`checkout.go:285–294`). Quotes also need a `buyer.capability_sessions` FK, which a worker does not have.
*Fix:* specify an attempt-owned materialization: either an attempt-scoped cart, or quote lines with an origin column and no shared cart. Include its schema change and the session/creator identity C2 uses. State that the buyer's own cart is never touched.

**P1-4 §4: notification state lives in two partly mapped state machines.** The contract maps only SUCCEEDED → ACCEPTED and UNKNOWN/ACKNOWLEDGED → DELIVERY_UNKNOWN. external-operation-v1 also has FAILED_FINAL, BLOCKED_POLICY, STALE_BINDING and CANCELLED, plus an operator `cancel` and a `retry` that re-dispatches after reconcile proof (external-operation-v1 §§ retry/cancel, lines 133–171). Two consequences:
- A VOID outbox row whose operation is retried would send a DM with no live business authority.
- After a crash, an outbox row stays DISPATCHING forever. Its operation only becomes UNKNOWN when something re-claims it, and the contract gives no owner for the outbox transition. That blocks rotation and the UI indefinitely.
*Fix:* add a complete bidirectional mapping table, with every transition written in one Tx. Rules:
- Payment-DM operation kinds are not retryable through the external-operation route; any resend is a new outbox intent through §4.
- Operator `cancel` means outbox VOID in the same Tx.
- An expired lease is reclaimed in reconcile mode by a named sweeper, which moves the operation to UNKNOWN and the outbox to DELIVERY_UNKNOWN together.
Add mutations for "external-operation retry after VOID" and "DISPATCHING without a sweeper".

**P1-5 §1: the executor identity and fence are not named.** The inline path runs on the merchant runtime pool. River runs on a worker pool. `validatePoolAuthority` enforces exactly one membership (`platform.go:401–418`), and River `InsertTx` needs explicit `river_job` grants (`stripe_runtime.go:258–263`). The contract never says which login or logins execute C2, what they are granted, or how inline and River executions of the same attempt are fenced. Both start right after C1.
*Fix:* name each executor login and its exact grants: EXECUTE on the composite definer, River privileges, no table DML. Reuse `platform.ValidateSameDatabase` (`meta_runtime.go:72`) as the cross-pool probe. Require that C2's first step is attempt `FOR UPDATE` plus a PENDING/generation recheck, and that the loser is a no-op replay. Add the mutation "executor skips the attempt fence", targeting ≤1 order with inline and River executing concurrently.

**P1-6 §3/§8: decisions marked as freeze blockers are still open.** These are:
- the legal retention duration for tombstones;
- the revocation and error classification table;
- API versioning, route names and how old-client adapters get `expected_generation`. An adapter that silently reads the current generation violates "clients must not automatically advance".
- the lock-site enumeration, deferred to "implementation review".
§8 itself says these must be frozen before backend assignment.
*Fix:* decide each one in this document, or attach an owner-approved decision record, before the status moves to FROZEN. If adapters cannot supply a generation, they should get `intent_conflict` and a forced UI upgrade.

## P2

- **§4 rotation:** refusing routine rotation when an old-version notification is ACCEPTED or DELIVERY_UNKNOWN means almost every order that got a DM can never be routinely rotated, because DELIVERY_UNKNOWN is effectively permanent. Refuse only on DISPATCHING. Name the permission and audit rules for "security revocation".
- **§4 reminders:** the semantic key is undefined, so it is unclear how a deliberate second reminder differs from a lost-response replay. Batch reminders have the same gap. Define the key as (order_id, link_version, kind, client reminder key or a sequence CAS).
- **§4 reusable pay entry:** this replaces single-use redemption with a reusable bearer URL. Specify credential expiry, rate limiting, and what PII (name, address) the pay page shows to anyone holding a forwarded link. Requires security_reviewer sign-off.
- **§2 key lookup:** the transport-key UNIQUE constraint is tenant-wide. Specify that principal B submitting A's key gets a non-disclosing `idempotency_conflict` (no existence oracle), never a replay.
- **§2 scope granularity:** a per-principal scope serializes all of a staff member's orders across buyers. One slow PENDING (a hot bundle lock, River backlog) stalls live-console throughput, and two tabs ordering for different buyers get `intent_conflict`. Record this as an explicit product decision, or scope by (principal, target conversation/owner).
- **§2 frozen-plan encryption:** key rotation or loss leaves an undecryptable PENDING plan. Retain key versions while any PENDING row references them; otherwise ABORTED(PLAN_UNAVAILABLE) under the fence.
- **§1/§5 invariants:** River job args and logs must carry only attempt_id/generation, never plan data or contact data. State this explicitly.
- **§3 lock order:** the external-operation claim currently locks the operation row first (`dispatcher.go`). Under the new graph it must acquire order → outbox → operation, or the lease must be committed state rather than a held lock. Name this inversion as a must-fix.
- **§3 erasure:** finding PENDING attempts by `buyer_owner_id` misses subject PII held in the frozen plan of a server-created private owner. Index attempts by every subject ID the plan contains.
- **§5 A15 revision:** it must cover only the selected bundles. A conversation-wide revision will be invalidated constantly by live claim churn, blocking A16.
- **§6 mutations missing:**
  - C2 reprices via CreateQuote;
  - cancel without voiding READY or revoking the credential (DM-CANCEL-GUARD itself);
  - no sweeper or exhausted job;
  - external-operation retry after VOID;
  - cross-principal key replay;
  - executor without the attempt fence (P1-5).
- **§7 mapping:** only g02, g07 and g10 of the g-series appear. Enumerate every #32 DELIVERY finding ID with a status (covered, fixed elsewhere, or not covered) so coverage can be checked mechanically.

## Confirmed sound (no change)
- Every remaining separate commit is named and justified: C1, C2, and C3/C4.
- A lost HTTP acknowledgement changes nothing in the database.
- An unknown commit never becomes ABORTED.
- COMMITTED+CANCELLED is accepted as a valid result.
- Recovery reads across all states and never returns `none`.
- The CSRF NOT_ACCEPTED boundary is right.
- The persisted owner survives key rotation.
- Erasure aborts before shredding.
- Cancel before authorization means zero provider calls, and the stated limit is honest: no recall, no remote exactly-once.
- The design adds no second engine or queue.
- Tenant and store scope come only from authentication.
- No client money is authoritative.
