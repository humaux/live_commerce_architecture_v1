CVS-SNAP changes buyer-entered CVS selection to a server snapshot lookup: the buyer chooses a chain and store number, and the server returns the exact directory name and address while retaining source `buyer_entered`.

This first push contains regression tests only. Implementation is deliberately pending assertion-level CI RED. Added coverage runs in existing Go unit, TCV14 real-PG/HTTP, storefront unit and CVS browser gates. ECPay-map assertions are unchanged. Browser coverage uses real fill/click operations, asserts directory misses and exact 131386 identity, and checks the persisted pickup ID after reload/re-entry; it writes a click ledger in CI evidence.

Authority: owner 2026-10-28 “门市的方案改一下，直接使用快照，直接返回商超的准确地址”; owner 2026-10-26 chain + store number. Snapshot 20261028 SHA-256: 25390346cf849ebd565ecea990efbc663a1a3043c6d73f283c06d8c3debc167c (verified by static file hash; not yet committed in this tests-only stage).

Migration reservation: 0206 / R2 pin 136. OC-05 owns 0205 / pin 135. **0205 has not merged into r3/integration at PR creation** (remote base 5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248). This card will not take 0205.

Planned data choice: deterministic generated INSERTs in 0206, from a committed generator and the immutable snapshot. Both fresh and R2 installs will consume the same migration bytes without runtime file lookup or changes to migrate.go. No generator/implementation has been committed yet.

Validation: E0 / CI PENDING. Local static checks only: gofmt, node --check, git diff --check (exit 0). All local tests, PG, browser runs and servers NOT_RUN by owner instruction. RED and GREEN run/job receipts will be recorded after CI supplies them; no green claim yet. See output/cvs-snapshot/red.log and handoff.md.

Files outside card write paths: none (apps/storefront/tests/cvs-contract.test.mjs is the explicitly requested storefront CVS unit test). No buyerhttp handler, drift baseline, dispatcher list or CI gate adapter edits. No merge or auto-merge.

🤖 Generated with Codex (dispatched by Claude Code integrator).
