Closes humaux/live_commerce_architecture_v1#215, humaux/live_commerce_architecture_v1#228

OIDC re-auth now caps an existing infinite authentication timestamp at the database clock. The identity P2 tests cover guard ordering/window boundaries, both throttle call sites, PD5 limits, and the session INSERT without its default. Contract/deployment notes clarify the fixed SQL window, skew bound, and backfill lock.

Fix 2 merges `r3/integration` at `2bc6d482606a2b0583af15789086fdad727b27ec`, after #262 merged. Assigned migration `0207_recent_auth_followups.sql` is retained; the exact R2 pin is **137 = base 136 + 1**, following #254/0205 → #262/0206 → #265/0207.

The prior g10 log also exposed a fixture dependency error: the pre-0077 Stripe upgrade fixture deferred 0203 but executed dependent 0207. It now defers both, asserts 0207 is absent before upgrade, and requires exactly ten migrations through the real Apply (previously nine). Existing privilege/data/idempotency assertions and production migration anchors remain intact.

Evidence:

- Historical causal RED: `05c641166990fcc04470da95b63497e5acd5e1c8`, run [40655692256](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/40655692256), job [122727348722](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/40655692256/job/122727348722): OG07 assertion failed with `infinity=true capped=false, want false/true`. Filtered raw log: `output/ident-p2/red.log`.
- Prior-head g04: run 40634706241 / job 122681056072 failed exact R2 count 135 vs 137. g10 / job 122681056834 exposed the 0207 fixture error before cancellation. Filtered logs and dispositions of all other failed jobs are in `output/ident-p2/DELIVERY.md` and `fix2/`.
- CVS and g07 logs terminate in cancellation; cancelled meta-health job has no available log (404). Their outcomes need current-head CI.
- Historical unit success run 40634706241 / job 122681056654 and Foundation success run 40634706220 are not current-head acceptance.
- Current-head Gates/Foundation: **PENDING**. Exact head/run/job IDs will be added as a PR comment, with no evidence-only commit after GREEN.
- Local static checks only: `gofmt -d` on both changed foundation test files, `go vet ./tests/foundation`, dependency-map regeneration: exit 0. Generated map unchanged after merging the base. No local tests or servers.

Paths outside the original OC-02/OC-03 write paths, for integrator approval:

- `tests/foundation/stripe_live_schema_test.go`: identity migration dependency in the upgrade fixture (fix 2).
- `contracts/merchant-password-auth-v1.md`, `deploy/README.md`: existing window/skew and backfill notes.
- `output/ident-p2/**`: delivery, filtered evidence, and PR text.

No authored browser spec/page/route changes in fix 2; no UI design change. No edits to the drift baseline or route dispatcher table. Merge-only base files are not additional lane-authored changes. R2 pin changes are explicitly authorized by the fix-2 card.

NOT_RUN: current-head GREEN, independent review, local tests/PG/browser/server, production, real IdP/SMTP and external sandbox probes. #228.7 payment-settings guard decision and #228.8 carrier/pick-list work remain outside implementation. Deployment remains blocked until OC-15 and the owner's deployment approval.

Rollback: revert the fix-2 merge commit through integrator review; no production action or destructive data change was performed. Evidence level E3 applies only to the historical raw CI failures; author assessment E0.

🤖 Generated with Codex (dispatched by Claude Code integrator).
