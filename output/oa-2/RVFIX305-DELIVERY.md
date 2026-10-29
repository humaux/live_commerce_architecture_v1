# RVFIX305 — assertion RED batch

Authority: owner RVFIX305 card; final findings of cxf-rv-305.log; OA-2 unit
brief and contracts/order-attempt-v1.md §4. No migration assignment was added.

Role: integration_worker responsibility, Codex GPT-6; reasoning setting is not
exposed by this runtime. Worktree: .worktrees/oa-2; branch: unit/oa-2; PR #305.
Base before integration merge: a57cdb1a76a5ccfc72defc9a8b751bc29ea1f786.
Integration: f8fd8900577815a328643ed878d88a6a247d2ab2, merged as aace3b5a5.

## P1: assertion-level RED

This push intentionally mutates only the existing OA-2 model surface, without
changing any existing test assertion. All nine existing model tests must fail
on behavior (permit receipt, UNKNOWN classification, generation error, retry
refusal, terminal disposition, sweep receipt, reminder identity, query-only
reconciliation). It must compile. These are MODEL_ONLY witnesses, not database
or scheduler acceptance. Restore order_attempt.go from aace3b5a5 after observing
the intended CI failures; never merge this mutation head.

## P1: SQL handoff reconciliation order

The new document check requires a locked-state branch before fresh-send
eligibility, a READY/unconsumed-permit-only refusal and explicit preservation
of consumed-permit evidence. It also requires separate cancel-after-UNKNOWN,
rotation-after-UNKNOWN and policy-loss-after-UNKNOWN SQL acceptance witnesses.
The current handoff intentionally remains unchanged for this RED push. The
check is document structure only; it cannot establish SQL runtime correctness.

## P2 for issue

The reminder identity CHECK admits SQL NULL when either key is omitted. Per
RVFIX305 instructions this is deferred to a PR comment for integrator issue
tracking; do not adopt this schema before adding explicit NOT NULL predicates
and negative real-DB cases for each omitted key.

## Evidence and continuation

RED and current-head GREEN: PENDING until native CI receipts are observed.
Run/job identifiers are appended to rvfix305/red.log and a PR comment, not followed by
an evidence-only push. Integrator relays a fresh session after the native run.
Local: gofmt and go vet only; exact exit codes are in rvfix305/red.log.
NOT_RUN: local tests/servers, REAL_PG/ACL/lock barriers/DB mutations, scheduler
and cancel/rotation/reminder composition, providers, SANDBOX/LIVE, production.
No activation, migration, contract, R2 pin, dispatcher composition or gate
exemption changes. Outside original write paths: generated browser ownership
only, as explicitly required by the card. Own assessment E0; static checks E1.
