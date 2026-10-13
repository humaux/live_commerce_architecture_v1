VERDICT: FAIL

Reviewed: `contracts/order-attempt-v1.md` @ 2251abe4 (PR #50), whole file (§1–§8, 353 lines), plus the delta against 99674d85. Reviewer: Claude (Opus 5.5), read-only, E1/SOURCE. No tests run (NOT_RUN).

The revision closes all 6 round-1 P1s and all P2s. It introduces one new P1: §4 requires every notification to have an integration operation, and the real `integration.operations` schema cannot store one when the refusal happened before dispatch was possible. The fix is small and local to §4. After that fix (and with the owner's two inputs) the contract is freezable. The P2s below can be resolved during implementation review.

## Round-1 status

| # | Status | Evidence |
|---|---|---|
| P1-1 PENDING liveness / retry exhaustion | CLOSED | §2: `execute_by = admitted_at + 15 min`, never extended. §3.2: `OrderAttemptSweep` re-enqueues missing/discarded/exhausted jobs; past the deadline it does a fenced ABORTED(EXPIRED) that also releases resources; absence proof stated; SQLSTATE/PT table; heartbeat alerts. §6i, M3. |
| P1-2 frozen price vs C2 revalidation | CLOSED | §2 HONORED/REVERIFIED table, "never calls `CreateQuote`", quote TTL replaced by `execute_by`. §6j, M1. |
| P1-3 cart schema cannot hold the plan | CLOSED | §2 attempt-owned materialization: `cart_kind`, `create_attempt_id`, creator-kind XOR across the whole 0013 chain, origin-keyed lines, buyer storefront cart never read or locked. §6j. |
| P1-4 two partly mapped state machines | CLOSED (see new P1 N1) | §4 bidirectional table, both rows in one Tx, generic `retry` refused for all payment-DM states (matches W6-05B's READY re-queue and FAILED_FINAL reopen, `external-operation-v1.md:124–158`), `PaymentDMLeaseSweep`. §6l, M4/M7. Checked: the dispatcher's budget exhaustion leaves the operation UNKNOWN and only cancels the River job (`dispatcher.go:259–260,293–298`), so it is consistent with "keep UNKNOWN, not VOID". |
| P1-5 executor identity / fence | CLOSED | §1 names `lc_api_runtime→commerce_runtime` and `lc_claims_worker→commerce_claims_worker` (`logins.tsv:28,42`). River grants match `migrate.go:151,166–169`, and the river_expiry ban matches `stripe_runtime.go:256–262`. `ValidateSameDatabase` exists at `meta_runtime.go:72`. §3.1 attempt-first FOR UPDATE + recheck. §6k, M6. |
| P1-6 open freeze blockers | CLOSED | Classification (§3.2), versioning/routes/old-client `client_upgrade_required` (§5.1) and lock-site inventory (§3.1) are decided. Retention is now OWNER-DECISION (owner input, not a defect). |
| P2 rotation | CLOSED | §4: only DISPATCHING blocks; `orders:link:revoke`+`orders:read`; audit fields. |
| P2 reminder key | CLOSED | §4 semantic key + cross-version client-key binding; batch child keys. |
| P2 reusable pay entry | CLOSED | §4: 7-day absolute expiry, 15-minute order-only exchange, 0094 limits, PII allowlist, security_reviewer sign-off required. |
| P2 cross-principal key | CLOSED | §2 non-disclosing `idempotency_conflict`; M5. |
| P2 scope granularity | CLOSED (OWNER-DECISION) | §2. |
| P2 plan-key rotation | CLOSED | §2 key-version retention, PLAN_UNAVAILABLE; §6k. |
| P2 River args / logs | CLOSED | §1. |
| P2 operation-first lock inversion | CLOSED | §3.1 last row: payment-DM routed before binding/operation locks. |
| P2 subject-indexed erasure | CLOSED | §2 `order_attempt_subjects`, §3.1 barrier order. |
| P2 A15 selected-only revision | CLOSED | §5.1. |
| P2 missing mutations | CLOSED | §6 M1–M7. |
| P2 §7 full ID inventory | CLOSED | 25 IDs with status. NOT_RUN: #32 `DELIVERY.md` is not in this tree, so the count was not independently verified. |

## New findings

**N1 (P1) §4: an operation for a refused plan is infeasible against the real schema.** §4 says "each admitted payment notification has one unique integration operation, including a terminal refused plan; no-send refusals have no runnable dispatch job". §3.2 PT404 sends "notification conversation/binding gone" to VOID through that path. But `0008_external_operations.sql:34,44,56,63` requires:
- `binding_id NOT NULL` with an FK to `integration.bindings`, so there is no operation when the store has no binding;
- `job_id NOT NULL CHECK (job_id>0)`, so a jobless operation is impossible;
- `generation>0` outside READY.

external-operation-v1 (ACCEPTED) also says the operation and its `InsertTx` job commit together. As written, C2 must either fail (and the order commits without a definitive disposition, against §3) or fabricate a binding/job.
*Fix:* define a refused plan as outbox VOID with **no** operation (`operation_id NULL`; a CHECK that only VOID-before-C3 may be NULL), and add a "VOID ↔ (none)" row to the mapping table. Alternatively, name an explicit amendment of 0008/external-operation-v1. Add a §6l case: notification requested, store has no binding → order COMMITTED, outbox VOID, zero operations/jobs.

**N2 (P2) §1/§3.1 "jobs are inserted last" contradicts the Begin dependency.** `begin_hold` requires the `river_expiry` job to exist before it inserts the order row (`post_river/0021:360–363,370–378`; `checkout.go:348–371`). State that the expiry-queue helper inserts that job inside the attempt Begin helper, before the order row. A fresh insert takes no contended row lock, so this does not change the lock graph; only the "last" wording is wrong.

**N3 (P2) §1/§4 notification job kind is ambiguous.** §1 says notification jobs carry "outbox ID/generation". The existing dispatcher family `external_operation_v1` is guard-pinned to args exactly `{operation_id, version:1}` (`post_river/0014:30–34,55–73`, external-operation-v1 §Data). Name the job kind that runs C3 and the sweeper's reconcile job. If it is new, it needs its own family guard and link trigger. The same applies to the new `order_attempt` execution job: per the existing per-family guard pattern, name its guard (args exactly `{attempt_id, intent_generation}`).

**N4 (P2) §1 the cross-pool probe is described per process.** API and claims-worker are separate processes (`logins.tsv:28,42`), so `ValidateSameDatabase` can only pair pools held by the same process. State the pairs: API runtime↔checkout/buyer/River in the API process; claims-worker↔River in the worker process.

**N5 (P2) worker key custody.** River-path C2 decrypts the frozen plan in Go and prepares pay-entry/DM ciphertext. That means `cmd/claims-worker` needs the plan AEAD key and the pay-entry/send keys. Name this explicitly in the security_reviewer key-custody item; today only the bearer section mentions custody.

**N6 (P2) §2/§3 subject-index lifetime is ambiguous.** Plans are deleted at settlement, but §3 removes subject indexes "as their private plans are shredded", while the retention default cryptoshreds "subject mappings" only 400 days after scope retirement. State that subject rows are deleted at settlement, since erasure needs them only for PENDING. Otherwise a buyer→attempt mapping lives for the staff member's entire tenure.

**N7 (P2) §2 grant expiry.** Existing grants expire 15 minutes after creation (`0129:279–280`). Say that C1 sets the grant/reservation `expires_at := execute_by` exactly, so a valid C2 before the deadline cannot be refused by an earlier grant TTL.

## Freezability and owner inputs

- **RETENTION:** consistent under either owner choice. Safety rests on scope/generation CAS with never-reused random scope IDs (§2, §3); the denial markers only change a stale-key reply from replay to conflict. A shorter, longer or marker-free choice cannot reopen an accepted intent, provided UUID non-reuse and "no PENDING retired" stay.
- **SCOPE:** the recommended principal+store default is fully specified and consistent with §3.1 (no scope lock in C2). The per-target alternative is explicitly deferred, but it is *not* specified: target key for unlinked buyers, cross-entry equivalence, lock level. If the owner picks it, §2/§3.1/§6b need another revision before freeze.

Freezable after N1 is fixed and both owner inputs are recorded. N2–N7 can be resolved in implementation review without reopening the design.
