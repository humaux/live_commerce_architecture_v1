# CVS-SNAP handoff

claim_type: problem · project: live_commerce_architecture_v1 · module: CVS-SNAP · date: 2026-10-28

Dispatched implementation card, working only in the assigned worktree/branch. Role: dispatched card implementer (Codex); runtime does not disclose an exact model/effort identifier. No child agents. Humaux/coord/code-index tools are unavailable in this session; no shared-state writes attempted. Repository source and owner card are authority.

Current phase: tests-only RED candidate; E0 until GitHub supplies failing assertion receipts. Local format/syntax checks only. Implementation has not started. Tests were added to existing registered gates so no shard inventory or CI routing edit is needed. Existing ECPay-map assertions remain intact.

Rollback point: base 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248; tracked working tree was clean before edits. The owner-provided untracked snapshot must be retained, never removed by a reset. Its SHA-256 was read and matches 25390346cf849ebd565ecea990efbc663a1a3043c6d73f283c06d8c3debc167c. Snapshot remains untracked during the tests-only stage; commit it with implementation.

Acceptance remaining:
- Read CI assertion failures and record run/job/head/merge identities in red.log BEFORE implementation.
- Migration 0206 and assigned pin 136 (OC-05 owns 0205/pin 135; 0205 absent remotely at PR creation). Never select another number/count to hide a missing prerequisite.
- Deterministic committed generator and INSERT migration from snapshot 20261028; fresh install/R2 upgrade; directory owner/ACL/comment tests. No new dependency needed.
- Go/SQL format twins, ignore legacy buyer name/address, directory lookup and exact response; buyerhttp handler frozen.
- Update both existing code tables: internal/fulfillment/testdata/cvs_store_codes.golden and tests/integrations/ecpay/testdata/store_code.json (both are currently consumed).
- Adapt existing buyer-entered synthetic store fixtures to existing snapshot codes; update superseded name/address expectations and canonical version reuse assertions. Preserve stale-cart, replay, limits, namespace and ECPay tests.
- Buyer component code-only input, canonical read-only result, all existing locales; update cvs-pap-only browser fixture too.
- Amend §16.1/F21 and conflicting §16.7 old format/existence text within the same contract.
- CI contractdrift 0 NEW/TOUCHED (stop on any), all CVS gates, R2 fresh/upgrade, G07/full foundation, independent integrator review. No merge/auto-merge.
- On GREEN add final run/job receipts as a PR comment, never an evidence-only push.

Wait policy: at most about 10 minutes of CI polling; record exact needed job IDs, then hand back to the integrator for a resumed session. No background local processes.
