# Order attempt v1

Status: **DRAFT — DESIGN, not implementation or runtime acceptance**. Owner: integrator; freeze requires independent review. Date: 2026-10-13.

Responsibility: atomic merchant-created orders, durable intent/replay, and payment-notification validity. Dependencies: PostgreSQL, existing checkout/inventory/claims domain rules, River, merchant authentication and privacy contracts. Consumers: ManualOrder, LC-U3, A15/A16, recovery, reminders, cancellation and the dispatcher. Invariants: I01–I09, I11, I14–I16, I18–I24 in `invariants.json`.

This draft is based on trunk `0bfe36e53acb694f4697521202e24f0790638941`, the integrator's `output/integrator/research/pr32-attempt-research.md`, and paused PR #32 `77e08616`. It proposes replacing the separate-commit placement/recovery rules in live-console-v1 §5.1 and manual-order portions of storefront-v2. It also absorbs **DM-CANCEL-GUARD**. Existing contracts remain effective until this draft is frozen and implemented; #32 UI work must then consume the backend contract, not implement a second authority.

## 1. Transaction feasibility and choice

**All local order effects can share one physical PG connection and transaction. Different pool logins are an authority/API obstacle, not a reason to commit between domains.** No provider call is needed to create the local pay entry, entered CVS destination, hold or order.

| Option | Feasibility and decision |
|---|---|
| A. SECURITY DEFINER composition | **Recommend.** One Go-owned execution `pgx.Tx` invokes narrow domain-owned definers; a composite final-write function may orchestrate SQL. Retain pure Go pricing/template/encryption in the owning admission/execution Tx (§2). A literal all-SQL function would duplicate/port that logic and is not the recommendation. All order effects have one commit. |
| B. One connection with controlled role switching | PG supports `SET LOCAL ROLE`, but current startup checks reject mixed runtime memberships and require `session_user = current_user`. Granting buyer/checkout roles to merchant runtime would broaden authority and break those checks. Reject for this unit; do not relax them. |
| C. Preserve separate cart/quote/destination/Begin/receipt commits | Technically possible only with a durable step ledger, replay and compensation for each step. No unavoidable local boundary justifies this complexity. Reject for new attempts; use bounded reconciliation only for legacy data (§5). |

Source evidence, with line numbers at the pinned trunk (PR #32 has different offsets):

| Evidence | Consequence |
|---|---|
| `internal/merchanttools/manual.go:531–573,386`; `order_for_buyer.go:238,416` | Cart, quote, destination, Begin, merchant receipt/source/link, claims and DM currently cross transaction boundaries. PR #32 `manual.go:624–670` still does so. |
| `internal/buyer/buyer.go:125,170`; `internal/platform/platform.go:648` | Both `WithScope` wrappers own their transactions. Passing their pools to each other does not make an atomic transaction. |
| `internal/storefront/{cart.go:90,quote.go:59,destination.go:59}`; `quote.go:119` | Tx-shaped APIs exist, but use buyer DML/RLS; authoritative `pricing.CalculateWith` runs in Go. Extract transaction-aware domain interfaces, not a second pricing algorithm. |
| `migrations/0007_cart_quote.sql:173–191`; `0013_buyer_checkout.sql:241–278`; `internal/buyer/command.go:19–35` | Checkout runtime cannot simply write buyer tables. Buyer commands require an empty principal context; merely changing GUCs is not authentication. |
| `internal/checkout/checkout.go:348–369`; `migrations/post_river/0021_product_core_begin_hold.sql:390–438` | Hold/order/checkout receipt and expiry `InsertTx` already share one transaction. Expose this domain operation within the outer execution Tx, without its own `WithScope`/commit. |
| `internal/checkout/cvs.go:419–428`; `internal/inbox/send_order.go:38–94`; `send.go:79–98`; `internal/integrations/core/jobs.go:22` | Entered CVS data is local SQL; DM planning takes a Tx; template/encryption are Go work. River enqueue need not commit separately. |
| `internal/platform/platform.go:265,400–418,463–468`; `migrations/0128_lc_b4_sends.sql:190–200` | Preserve login/owner-reachability checks. Merchant messaging and buyer scopes must not leak into each other. |

**Required authority boundary.** Admission authenticates tenant/store/principal on the server. The executor loads the attempt's persisted owner and admission grant, never an owner derived from a current signing secret or a request/GUC assertion. Each definer has a NOLOGIN owner, `SECURITY DEFINER SET search_path=pg_catalog`, qualified names, exact EXECUTE/column grants, PUBLIC revocation and COMMENT/ACL pins. Coordinator authority is exact EXECUTE on domain helpers, not membership in all domain owners or general table DML. Helpers establish and restore their internal context, including failure paths. Runtime callers cannot forge a helper's attempt binding or invoke internal stages out of order. Add negative real-role/RLS tests; do not widen the frozen customers-billing-v1 §3.1 privacy-writer list.

The selected design needs new transaction-aware helpers; existing definers' login/context checks cannot just be reused unchanged. Before enabling it, prove all participating pools refer to the **same physical database** through a bounded cross-pool database probe, not matching DSN strings/database names alone. No distributed transaction, second engine or new queue is introduced. PostgreSQL's [role rules](https://www.postgresql.org/docs/18/sql-set-role.html) and [definer safety rules](https://www.postgresql.org/docs/18/sql-createfunction.html#SQL-CREATEFUNCTION-SECURITY) govern this boundary.

Only these commit boundaries remain:

| Commit / boundary | Why and durable record |
|---|---|
| C1 admission | Persist accepted intent, canonical frozen plan, stable owner, generation and River `InsertTx` together. Allows recovery after HTTP/worker death; creates no hold/order/pay entry/DM. An inline request uses the same executor as River. |
| C2 execution | In **one Tx**: quote/cart materialization, local store entry/destination, Begin/hold/order/expiry job, manual source and audit, claim reservation completion and grant accounting, local pay entry, optional business outbox plus job, terminal attempt receipt. No internal commit. |
| C3 dispatch authorization → external HTTP → C4 outcome | PG cannot atomically commit Meta acceptance. Persist DISPATCHING before I/O, then ACCEPTED, DELIVERY_UNKNOWN or proven-not-sent VOID. Business outbox carries the evidence (§4); River job status is not that evidence. Provider payment sessions similarly remain under their existing external-operation contracts. |

The HTTP acknowledgement is outside each commit; losing it changes no database state. Definitive execution refusal rolls back **all** C2 effects and records ABORTED under the still-held attempt fence (outer Tx/savepoint or a subsequent fenced Tx proving zero effects). A database/connection crash rolls back C2 and leaves PENDING. Do not convert an unknown commit outcome into ABORTED.

## 2. Durable data and admission

Names below are logical, not assigned migration numbers. Integrator owns migration numbering and exact grants.

| Record | Required identity and data |
|---|---|
| `order_attempt_scopes` | Opaque `scope_id`; UNIQUE `(tenant_id, store_id, principal_id, operation_family)`. Manual and for-buyer orders share one family. `current_generation` never decreases. Scope is server-resolved, stable across tabs/devices/re-login; cookies, capabilities, buyer labels and browser storage are not its key. |
| `order_attempts` | `attempt_id`, tenant/store/scope, `intent_generation`, transport-key digest, canonical request hash/schema version, encrypted frozen execution plan, persisted `buyer_owner_id`, admission authorization/config provenance, `state`, optional `order_id`, immutable terminal receipt. UNIQUE `(tenant_id, operation_family, transport_key_digest)` and UNIQUE `(scope_id, intent_generation)` **over all states**, plus at most one PENDING per scope. |
| Order binding | UNIQUE `(tenant_id, create_attempt_id)` on the order; exact FK/scope checks back to the attempt. A terminal COMMITTED attempt has exactly one order pointer. |
| Supporting domain records | Claim/bundle reservation binds the exact attempt and order; existing one-live-order-per-bundle invariant remains. Pay entry and payment-notification outbox carry exact tenant/store/order IDs; notification semantic key is unique independently of the River job. |

All tables enforce tenant/store RLS from verified authentication; worker/definer paths re-establish a validated attempt scope. Browser roles have no direct writes. A principal cannot select another principal's attempt merely by supplying a key. Stable internal IDs/digests are protected pseudonymous data, not declared anonymous.

**Admission CAS:** the server returns the current scope generation and its attempt/terminal reference. An explicit new-order action submits `expected_generation` plus the last terminal reference (or initial generation zero), a new transport key and canonical request. Under the scope lock, allocate generation `expected_generation + 1` and commit C1. A different key with a stale generation gets `intent_conflict` even if the winner settled. New generation requires the previous attempt terminal; storage loss, `none`, 404, elapsed time or no pending row does not authorize it. Clients must not automatically advance after mount/conflict/recovery. This serializes one staff principal/store, not the store; other principals still obey bundle locks.

Exact same key/body replays its attempt in **any state** before fresh placement validation; different body is always `idempotency_conflict`. Canonical hashing includes endpoint family, target bundles/conversation, each claim origin, delivery/payment choice and request schema; credential rotation must not change key lookup. Persist the original buyer owner at admission, including server-created private owners where needed; never guess a linked customer from a social label or capability. Old key versions may authenticate credentials but cannot redefine order ownership.

**C1 establishes the authoritative price** and freezes destination/service/config facts and per-origin live-price authorization as attempt preparation data only. Commerce quote/cart/destination rows appear in C2. C2 materializes that quote and may verify arithmetic against frozen inputs; it must not call today's `CreateQuote` to establish another price. No client amount is authoritative. Required resource/safety checks may atomically abort, not reprice. Used live-price grants are consumed with C2; unused grants expire with settlement; admission artifacts are released on ABORTED.

## 3. State, locking and recovery

```text
not admitted -- C1 + generation CAS --> PENDING -- C2 --> COMMITTED
                                         \-- fenced zero-effects decision --> ABORTED
```

`UNKNOWN` is a transport observation, never a fourth authoritative state. Terminal states never regress or change order identity. `COMMITTED` means the complete **local** pipeline is durable, including a definitive notification disposition; it does not mean paid or delivered.

| Transition / operation | Authority and rule |
|---|---|
| Admit | Current merchant authentication, CSRF and placement permissions (including catalog, inventory and orders; claims/message permissions where requested), server generation CAS. No order effects. |
| Execute/resume PENDING | Internal executor's durable admission grant plus frozen plan; current read authentication controls disclosure, not the ability of a queued worker to finish. Do not call ordinary `Place` and repeat its mutable permission/option gates. Integrity, inventory and explicit erasure/cancel fences remain mandatory. |
| Commit | The complete C2 write set and receipt become visible together. Receipt records the initial notification planning disposition/outbox ID; current delivery status is separate. Message authorization/window/privacy loss yields VOID/not_sent, not a permanently PENDING order. |
| Abort/seal | Under the same fence, prove no committed C2 order/resources and prevent the old executor from committing; then persist reason and terminal receipt. A 422/409, missing receipt, credential error or failed lookup alone is not absence proof. |
| Recover/read | Primary, uncached lookup by attempt ID or original key across **all states**. PENDING invokes/schedules the same frozen executor; terminal replies use receipt/pointer, not the encrypted body. Recovery transport keys never create an order or select a different intent. |
| Cancel an order | Attempt remains COMMITTED; current order state becomes CANCELLED. Release/refund follows existing order rules. In the same cancellation Tx revoke collection credential and void READY notifications (§4). |

**Lock order:** authenticate/resolve identifiers without domain locks, then acquire applicable locks in this order: buyer owner → attempt scope → attempt → claim bundles/reservations (sorted IDs) → parcel group → order serialization key/existing order row → cart/quote/pricing/catalog/delivery/inventory in a documented deterministic domain order → pay entry → integration binding → business outbox → integration operation → jobs. New orders use their preallocated attempt-bound order key before insertion. Skip irrelevant levels, never reverse them. Preserve group-before-order (`0155_returns.sql:664–795`) and binding-before-operation precedence. Erasure, execution, reminders, dispatch, expiry and cancellation must share this graph; implementation review enumerates actual SQL lock sites and rejects inversions. Nested pool transactions are prohibited.

| Event | Required result |
|---|---|
| Crash before C1 | No attempt accepted; no business effects. Retry admission with the same key/generation. |
| Crash after C1 / anywhere before C2 commit | One PENDING plus runnable/reconcilable job; no new committed partial hold/order. Resume same attempt, including after old source-marker/quote windows. |
| Lost C2 or recovery response | Replay the same immutable receipt/order. Never query only PENDING; never return `none` for a settled intent. Live `order_state` is a separate read, so COMMITTED+CANCELLED is valid. |
| Cross-tab/device or re-login | Same server scope/generation wins once regardless of different cookies/keys. Client guard is UX only; mount reads authority before admitting a new intent. |
| Credential/key rotation | Read persisted owner; unavailable original binding keeps the fence and reports repair-required. Lookup under a newly derived owner cannot prove absence. |
| CSRF/session change before dispatch | Explicit fresh `NOT_ACCEPTED` means no C1 happened for that call. It releases only a fresh never-sent UX attempt. A prior UNKNOWN/key is still recovered server-side; a later 401/403 does not prove its absence. |
| Permission/service changes after admission | Frozen execution does not depend on the original browser session or re-run placement gates. Explicit safety revocation can abort before effects; missing message permission produces terminal notification refusal. No endless `capability`/removed-option retry loop (4259504799). |
| Erasure wins the lock | New PENDING has no committed order: atomically ABORTED(ERASED), fence executor/jobs, release admission artifacts, then shred private plan. Recovery settles from non-PII receipt; new explicit generation is possible. |
| C2 wins before erasure | Preserve COMMITTED/order pointer. Apply existing legal-retention/CD7 rules and scoped redaction; never erase proof of an existing order or turn it into ABORTED. Terminal replay never needs a body. |

Revocation classification is identical for HTTP and River: session expiry/RBAC changes govern **current disclosure**, not revocation of an accepted placement; messaging consent/window/binding/inbox permission loss makes **notification VOID**; erasure, explicit admitted-intent cancellation/store closure, insufficient stock or a now-unfulfillable service makes a **zero-effects PENDING execution ABORTED**. Quote expiry or incompatible required version checks settle ABORTED rather than reprice. For legacy already-committed Begin, changed placement checks cannot erase the order: reconcile from its snapshot, then apply ordinary cancel/expiry/refund policy if necessary. Missing policy classification is a freeze blocker, not an UNKNOWN retry loop.

Privacy erasure uses a narrow domain-owned definer, not extra SELECT grants for `commerce_privacy_writer`. Validate its exact owner/store scope before deactivation and retain that authenticated binding during cleanup; re-resolving a now-inactive capability is not scope proof. Full frozen PII is retained only while needed for PENDING execution, then deleted on settlement/erasure. Retain the minimal key/hash, generation, stable binding, order pointer and redacted terminal receipt for replay; no bearer token or contact data in receipts/logs/client storage. Proposed default: no automatic deletion of safety tombstones while the scope is usable; an approved retention policy may retire a scope and cryptoshred its mappings, but must then permanently reject its old generations/keys. The exact legal retention duration is a freeze decision, not an inferred GDPR exemption.

## 4. Pay entry and payment notification

Three separate facts: **orders** authorize collection; **pay entry + link_version** authorize a credential; **business outbox** records a notification. Neither a River job nor an integration-operation status substitutes for all three.

- **Copy = read.** Read the existing order-scoped pay entry under the order lock; never call regeneration. A reusable entry may exchange for short-lived order-only buyer authority, but must not grant a customer profile/cart or bypass payable/erasure checks. This replaces old single-use redemption (`0094_merchant_tools.sql:171–196`) and regeneration (`0104_authz_ip_scope_link_replay.sql:141–169`); preserve already-issued links through the adapter in §5.
- **Reminder = new notification intent.** Reuse the valid entry/version by default; a semantic reminder key deduplicates lost responses. Persist exact `order_id`, even for checkout reminders. A READY reminder for buyer A must not block buyer B's unrelated order in the same store.
- **Rotation = explicit CAS revoke.** Require `expected_link_version`, explain that old URLs stop working, serialize on the order, issue one new version and revoke the old credential/VOID its READY notifications in the same Tx. Do not silently requeue them. If an old-version notification is DISPATCHING, DELIVERY_UNKNOWN or ACCEPTED, refuse routine rotation; cancellation/security revocation may invalidate it with an explicit audit, without pretending the remote message was recalled.
- **Disclosure is proved, not guessed.** Return `buyer_link` only after confirming under the order lock its exact committed credential/version, payable state and permitted disclosure. Return null for CANCELLED, erased, `window_closed`, `link_unavailable` or any not-established path; never retain a pre-lock candidate in an error result. Validity is established at the read's serialization point, not promised forever after another actor cancels.
- The UI does not offer a regenerating copy for queued/planned sends or `link_unavailable`; it displays queued/reminder-in-flight. A real read-only copy may be exposed when the server can return the same established entry. No hidden rotation fallback.

Outbox record: tenant/store/order/attempt, immutable notification semantic key, kind, `link_version`, destination/binding, worker generation, state, encrypted payload and dispatch/outcome evidence. Order/version are mandatory for every payment-DM producer. Each dispatchable outbox row binds **one unique integration operation** with the same semantic identity. C2 creates both plus the job; C3 authorizes/claims both in one Tx; C4 records both outcomes in one Tx. No existing dispatcher, retry endpoint or reconciliation path may bypass the business row. Integration SUCCEEDED with provider acceptance evidence maps to ACCEPTED; UNKNOWN/ACKNOWLEDGED without that evidence maps to DELIVERY_UNKNOWN, never success. Preserve external-operation-v1 lease/generation checks and reconcile-only behavior.

```text
READY -- locked authorization/CAS --> DISPATCHING --> ACCEPTED
  |                                      |-------> DELIVERY_UNKNOWN
  \---------------> VOID                 \-------> VOID (proven not sent)
```

Before C3, lock the order and outbox in §3 order; revalidate payable state, privacy, consent/window/binding/message authority, current credential/version and worker generation. If invalid, VOID without HTTP. C3 commits DISPATCHING with a single-use dispatch generation; only the invocation that wins READY→DISPATCHING may perform its one HTTP call. Seeing an existing DISPATCHING row is never a resumable send permit. Provider I/O occurs outside PG. A lost acknowledgement or crash after authorization (including before I/O) is DELIVERY_UNKNOWN unless the operation adapter proves no send occurred. Never automatically resend it or allocate a new semantic key. ACCEPTED means provider acceptance, not buyer delivery. Outcome commits are generation-fenced; late evidence may refine UNKNOWN to ACCEPTED but cannot trigger another send. Duplicate River delivery cannot authorize a second provider call.

Cancel atomically revokes the pay entry and voids **READY** rows. Guarantee: cancel committed before dispatch authorization ⇒ **zero provider calls**. If dispatch authorization won, a remote send may already be in flight; preserve DISPATCHING/UNKNOWN/ACCEPTED evidence and make the link unpayable. Do not claim remote exactly-once or recall. Current `0129_lc_b6_order_for_buyer.sql:521–611`, `0144_checkout_reminders.sql:165–184` and `internal/integrations/core/dispatcher.go:303,365–370,472–491` lack this complete authorization boundary; a lock-free pre-send check is insufficient.

## 5. Migration and API compatibility

**Cutover is fenced.** Inventory both trunk and PR #32 experimental tables/functions first; never assume 0170–0172 are installed everywhere. Integrator assigns new migration numbers, with one exact R2 count increment/comment per migration. Preserve SECURITY DEFINER/ACL/COMMENT pins and frozen privacy grants. Disable old placement writers during backfill, or first make every old entry route through the new admission fence; no period permits two authorities.

| Existing record | Required conversion before enabling its scope |
|---|---|
| Proven complete local pipeline | Verify the endpoint-specific order/source/audit/pay entry, claims and exact notification operation or definitive refusal, then backfill COMMITTED and original owner/key/hash/receipt. An upper receipt alone is insufficient: A16 currently commits it before DM planning (`order_for_buyer.go:219–228,303,430,461`). Retain cancelled state separately; never recreate effects merely because an HTTP result was lost. |
| Pending with no order/hold/Begin after locked reconciliation | ABORTED with durable absence seal; retire old workers and release pending claims before allowing a new generation. |
| Begin-only hold/order | Bind exact original owner/Begin key and immutable checkout snapshot; finish source/audit/pay entry/claim bookkeeping/notification disposition in one fenced reconciliation Tx, regardless of age. If the order expired/cancelled, record COMMITTED with that current state and no collection link/DM; do not revive resources. |
| Erased/missing body or ambiguous owner/order binding | Recover from durable checkout facts where sufficient. Otherwise quarantine **behind the admission fence**, with an explicit operator reconciliation record; never report absent or derive another owner. Migration cannot declare the scope accepted until it is proven COMMITTED or safely ABORTED. |
| Old links and reminder plans without order_id | Resolve only from exact persisted order/operation/credential provenance. Bind each legacy credential to its original expiry, redemption/revocation state and an order link version, so rotation/cancel also invalidates the adapter. Preserve a redeemable link; redeemed, expired or revoked links remain unusable. Ambiguous READY plans become VOID with audit, without a store-wide match; preserve in-flight/UNKNOWN evidence. New entries use §4 semantics. |

Legacy step ledger records original owner, key/hash, order/hold IDs and proofs for Begin, manual receipt/source/pay entry, claims and DM. No timestamp or ten-minute marker is proof. Unavoidable old partial effects are reconciled, not exposed as new-protocol PENDING-with-zero-effects. Erasure of legacy partials must settle/reconcile or invoke the existing cancel/refund policy before dropping necessary private material; never delete a body first and strand a unique pending fence.

Proposed API deltas below are **relative resource names**, not live route registrations. Freeze/implementation must update Go router, contract/OpenAPI and both relevant BFF allowlists together, with contract-drift errors=0. Existing routes remain adapters during a versioned rollout; older clients cannot bypass the server generation fence.

| API | Required shape/behavior |
|---|---|
| A15 `inbox/order-prefill` (read) | Preserve current 50-line/5-bundle bounds, return `complete`, `total_line_count`, `total_bundle_count`, scoped selection revision and every returned line's exact claim origin. Counts and rows come from one consistent snapshot. If incomplete, UI must refuse A16 and show why; admission independently verifies the exact selected bundle set/revision, so bypassing UI cannot place a silent partial order. Duplicate SKU origins remain separate price-eligible quote lines; aggregate only inventory quantities. |
| A16 `orders/for-buyer` and manual placement | Accept original idempotency key, `expected_generation`/previous terminal reference, versioned canonical body and prefill selection revision where applicable. Server returns an attempt envelope; no client-supplied tenant, owner or price authority. A15 revision is checked on admission only, not during frozen replay. |
| Scoped attempt lookup/recovery | Authenticated primary reads identify exact attempt/key or current scope generation; recovery is a CSRF-protected command that resumes it without accepting a replacement body. ManualOrder and drawer share this seam. Authorization failure is not `none`; a truly unadmitted key returns NOT_ACCEPTED plus authoritative scope/generation, never permission to invent a fresh generation. |
| Envelope | `{attempt_id, scope_id, intent_generation, attempt_state, terminal_receipt, order_id, order_state, send, buyer_link}`. Receipt holds immutable disposition/order pointer, not a URL or mutable order state. PENDING=202; terminal replay=200 (initial committed placement may retain 201). Typed conflicts preserve `idempotency_conflict`, `intent_conflict`, `bundle_reserved`/`bundle_order_exists`; never map them to `internal`. |
| Terminal/refusal parsing | COMMITTED+CANCELLED is valid with null link and terminal UI. ABORTED carries a stable code and proves the generation may be advanced explicitly. A coded 409/422 with no terminal envelope does **not** unlock an unresolved attempt. |

All private success/error responses, including prefill, restriction checks and recovery, use `Cache-Control: private, no-store`; no negative cache. Generation conflicts expose only the caller's permitted scope, not another principal's request. Read permissions are rechecked before returning contact/order data; worker continuation is not permission to disclose it.

## 6. Acceptance gate — required before backend freeze implementation is accepted

**CI only. No local runs.** This contract adds no test implementation and claims no green result. Backend delivery must publish exact source SHA, migration/role set, CI run IDs, native logs and mutation red→restored-green receipts. Model proof alone is MODEL_ONLY; automatic real-PG/fake-provider proof is E3 in that tested environment, not LIVE Meta acceptance.

**Go model + `testing.F`:** model admission, two principals/scopes, two tabs, keys/generations, workers, crash/replay, erasure, cancel, rotation and dispatch/outcome loss. Fixed a–h seeds plus generated action sequences assert: ≤1 order per intent; all-state key/body binding; terminal monotonicity; pending uniqueness; no C2 partial effects; inventory/resource conservation; no stale-generation write; no absence under another owner; no invalid-link disclosure; cancel-before-dispatch implies zero calls; UNKNOWN never causes an automatic resend. With fair scheduling, available PG and no outstanding external outcome needed for C2, every valid new PENDING settles. Persist minimized failing seeds; do not substitute tests mirroring only helper implementation.

**REAL_PG matrix:** actual migrated runtime roles, real HTTP/BFF seams where applicable, at least two connections and channel/SQL barriers that prove the intended lock has been reached. Positive fixture/readback sentinels precede negative assertions. Use backend termination/process kill and commit-ack dropping, not sleeps. Assert order/source/audit/receipt/holds/claims/grants/pay-entry/outbox/job counts and identities, not only HTTP status.

| Case | Barrier/fault and exact acceptance |
|---|---|
| a — crash matrix | Kill before/after C1 and after each C2 stage: quote, store entry, destination, Begin/hold/order, source/pay entry, bundle/grants, outbox/job, receipt; then just before/after C2 commit and dropped HTTP response. Before commit: zero C2 effects; after commit: one complete order, same receipt. Explicit recovery/worker completes once. Include aged legacy Begin-only, without source-window exemption. |
| b — cross-client | Two tabs/devices/sessions race different keys at the same scope/generation, then one retries after the other settled; exactly one admitted generation/order. New explicit CAS succeeds once. Different scopes proceed; different principals competing for one bundle cannot steal its reservation. |
| c — lost recovery | Commit recovery, drop its response, replay with same and different transport keys targeting the same attempt; always same terminal receipt/order, including after a newer generation exists. |
| d — key rotation | Rotate credential derivation/signing key between admission/commit and recovery. Original owner/order/hold remains authoritative; unavailable binding cannot yield ABORTED/NOT_ACCEPTED. |
| e — erasure/privacy | Race erase before/after C2 using the shared lock. Pending erase settles ABORTED and explicit new generation succeeds without private body; committed erase retains order proof. Include legacy erased partial, cross-store negative, frozen D-S2 ACL equality and AdsAttributionR3Erasure scope regression. |
| f — cancellation | Cancel committed pickup/COD/unpaid orders before recovery: COMMITTED+CANCELLED parses, same pointer, no renewed hold/link/DM. Existing cancel/refund/inventory invariants stay exact. |
| g — links/notifications | Two-buyer reminder batch with dispatcher paused; independent READY rows. Concurrent copy/reminder/rotation yields no dead candidate. Cancel-before-authorization: zero calls; authorization-first: evidence retained, link invalidated. Barrier between eligibility read and authorization proves revalidation is locked; stale outcome cannot overwrite a newer disposition. Include duplicate jobs, post-C3/pre-I/O crash, stale generations, legacy link states/no-order-id rows and fake provider accepted-then-disconnected: one call, DELIVERY_UNKNOWN, no retry. |
| h — pre-admission | Real client/BFF CSRF/session fence prevents fetch/admission for a fresh intent; zero rows. Repeat with earlier UNKNOWN: retained server attempt, no fresh key permission. |
| 4259504799 — resume | Pause after admission and in a seeded legacy Begin-only state; revoke inbox:reply/remove a placement option, then recover. Finish or safely abort from durable state without ordinary Place's gates; committed order has definitive not_sent/VOID where messaging is no longer allowed. Fence eventually releases only with terminal proof. |

Required mutations (each must make its designated current-SHA CI case red): pending-only lookup (c); omit all-state generation uniqueness/CAS (b); independently commit Begin or outbox (a); derive owner from current key (d); erase body before sealing (e); finalize from Begin receipt alone (a); reject CANCELLED (f); detach eligibility check from locked dispatch authorization, accept stale outcome generation, or resume DISPATCHING/retry UNKNOWN (g); rotate on copy/store-wide reminder match/return pre-lock link/resurrect expired legacy link (g); clear earlier UNKNOWN on fresh 403 (h); resume via ordinary Place (4259504799); omit A15 completeness or merge claim origins (prefill); expose internal definer/direct DML to runtime or privacy writer (ACL/RLS). Restore each mutation and prove the original assertions green; never increase budgets or loosen pins.

UI/backend acceptance additionally covers real manual-order/drawer, live-console, inbox and affected payment/reminder modes selected by the registry/PR planner, with persisted click/PG readback. Go model/fuzz, full relevant foundation shards, Node/typecheck, contract drift, migration upgrade/count/ACL pins and independent money/privacy review are all required; integrator owns the final frozen matrix and CI selection.

## 7. Open #32 findings → closure obligation

This is coverage, **not a claim that the findings are fixed**. Inventory is the paused #32 DELIVERY plus the integrator's subsequent 4259504799 research finding.

| Open finding | Contract clause / boundary |
|---|---|
| 4257479294 — cross-tab/re-login duplicates | §2 server scope + all-state generation CAS; §6b. |
| 4257479276 — aged Begin-only | §1 atomic C2, §5 legacy reconciliation, §6a. |
| 4258064775 — rotated owner capability | §2 persisted original owner; §3 rotation; §6d. |
| 4258686743 — lost settled recovery response | §3 all-state terminal replay; §6c. |
| 4257479307 — pre-dispatch CSRF/session fence | §3 NOT_ACCEPTED boundary; §6h. |
| 4258686722 — recovered CANCELLED parser | §3 independent current order state; §5 envelope; §6f. |
| 4259170302 — erasure strands pending fence | §3 locked abort-before-shred; §5 legacy handling; §6e. |
| 4259504799 — placement gates block frozen resume | §3 execution authority, terminal message refusal; dedicated §6 row. |
| 4257479285 — unestablished/window-closed link | §4 locked disclosure with null on unestablished paths; §6g. |
| 4256912975 — superseded Place token | §4 copy/read, no candidate leakage and version authority; §6g. |
| 4256890998 — reminder blocks unrelated buyer | §4 exact order identity; §5 narrow legacy conversion; §6g two-buyer batch. |
| 4257755379 — copy on link_unavailable | §4 reminder-in-flight UI/read-only copy rule; §6 UI acceptance. |
| DM-CANCEL-GUARD — queued payment DM after cancel | **Absorbed:** §4 cancel/dispatch linearization and VOID; §6g. |
| 4258064767 — duplicate SKU loses claim origin | §2 origin-aware canonical plan; §5 A15/A16 separate quote lines; §6 prefill mutation. |
| 4259170307 — silent A15 LIMIT 50 truncation | §5 completeness + admission verification; §6 prefill mutation. |
| 4259170294 — restriction success lacks no-store | §5 private-response invariant; actual BFF header/UI regression repair remains a separate UI task. |
| 4258064753 — picker search stays busy | **Not covered:** isolated picker state/UI test, no attempt semantics. |
| Review summary — linked-customer latest-order email | **Not covered:** authorized prefill read implementation/test; preserve existing customer authorization. |
| g02 privacy-writer excess SELECT; AdsAttributionR3Erasure PT404 | §1/§3 narrow definer and retained authenticated erasure scope; §6e. Frozen grant list must not expand. |
| g07 unused request grant; g10 typed conflict/reservation ownership | §2 grant settlement/bundle invariant; §5 typed conflicts; §6a/b. Existing exact regression tests remain required. |
| Foundation strip-only import/fixture regressions | **Not covered:** Node/fixture repairs need their own current CI result. |
| R2 migration count for experimental 0171/0172 | §5 exact additive migration pin; historical PR #32 CI still needs reconciliation, not a relaxed check. |
| Shipments SQL call-site diagnostic; intermittent card primary read | **Not covered:** separate/trunk diagnosis and native CI evidence; this document cannot classify either as fixed or a flake. |

## 8. Draft handoff

Only this contract is changed. Runtime behavior, migrations, tests, routes and dependency files are unchanged. Source feasibility/mapping review is **E1/SOURCE**; model, mutation, REAL_PG, browser and all local gates are **NOT_RUN (contract-only; owner CI-only)**. Independent review must freeze the transaction helper authority/lock graph, retention/legacy cutover decisions and API versioning before assigning backend implementation. Migration numbers and release approval are not assigned by this draft.
