# DEFECTS — unit meta-connect (independent test author; product code NOT fixed by the tester)

## D1 — one-store-one-Page is not enforced under concurrency (race in `meta_connect_finish`)

- **Failing test**: `TestMetaConnectGateOwnership/one_store_cannot_hold_two_Pages:_concurrent_picks_of_two_Pages_->_one_201,_the_other_refused`
  (tests/foundation/meta_connect_gate_test.go, MCG03). Observed: two concurrent picks of two different Pages by ONE store both
  answered 201 and both bindings were left enabled (`[201 201]`).
- **Root cause**: the one-connection-per-store guard reads `integration.meta_connections` for the store
  (`migrations/0095_meta_connect.sql`, `meta_connect_prepare` line ~269 and `meta_connect_finish` line ~320,
  `SELECT c.page_id INTO v_other ... FOR UPDATE`). When the store has no connection row yet, `FOR UPDATE` locks nothing, so two
  concurrent transactions both see "no other Page" and both proceed. `meta_connections` has `UNIQUE(page_id)` but no uniqueness on
  `(tenant_id, store_id)`, so both inserts commit.
- **Why it matters**: every other surface assumes one connection per store — the sequential path refuses with
  `already_connected` (MC409), the status card models a single Page/IG, Studio's claim-source picker offers one Page. Two
  concurrent winners leave two enabled bindings + routes + credentials on one store, and the card/picker can only show one.
- **Suggested fix** (for the implementer, not done here): a `UNIQUE(tenant_id, store_id)` on `integration.meta_connections`
  (or `pg_advisory_xact_lock(hashtext(store_id::text))` in `meta_connect_prepare` before the check) so the second concurrent
  pick serializes and raises `already_connected`.
- **Repro**: `bash scripts/dev/test-focused.sh '^TestMetaConnectGateOwnership'` — the subtest races two picks on purpose.
  Deterministic enough: both picks fired from a `sync.WaitGroup` gate; observed [201 201] on the first run.

No other product defects found: MCG01/MCG02/MCG03(other subtests)/MCG04/MCG05/MCG06/MCG07 and the MCG08/MCG09 node gates all
pass against the unmutated product (see the green logs in this directory).

## D2 (P2, found by integrator running MCG10 browser gate 2026-10-01)
Disconnect leaves the Page subscribed at Meta (fake graph `subscribed` stays true for the disconnected Page; spec line 265).
Root cause: after the custody ruling (API seals with HPKE v2 and can never open a Page token) the API-side disconnect
no longer calls DELETE /{page}/subscribed_apps. Fix direction: disconnect enqueues an unsubscribe job executed by
claims-worker (the only holder of the private ring), best effort, audited; contract "Merchant connect (R4)" says
disconnect DELETEs subscribed_apps best effort. Test kept failing: meta-connect-gate.spec.ts "...disconnect...".
