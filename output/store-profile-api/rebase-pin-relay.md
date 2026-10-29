# PR #287 rebase-pin relay — 2026-10-29

Authority: current implementation-card relay, repository AGENTS.md, architecture
sections 0/1/2/26/27, owning merchant-settings contracts, and GitHub raw receipts.
Assigned lane: unit/store-profile-api in .worktrees/store-profile-api; no delegation.
Runtime: Codex; precise model variant/reasoning setting is not exposed.

## Current failure and stop boundary

Head 2a53235921c47b16eaeadae4cbbf4d8c4c994fac, Foundation run 40811803504,
job 123243599576: the sole failed-step finding is unclassified
internal/httpapi/profile.go. Filtered raw receipt is
rebase-pin-relay-foundation.log. This is an implementation-head gate failure,
not causal feature RED. The workflow stops before feature tests.

Fresh origin/r3/integration is 61951609f1dd0937d7d6a8c2f8b0d2faf3d4d05d.
Its pr-modes.mjs and test-local.sh still lack profile.go classification.
The existing checker requires an exact-file entry in those shared CI sources;
the prior relay's generator already refused to infer an exemption. These are
outside the assigned app/module and generated-file allowances. STOP per card:
no registry/exemption/test/threshold/fixture change and no further repair attempt.
Integrator must own or explicitly authorize the evidence-backed classification.
No workaround by relocating the implementation into an already classified file.

## Requested merge and pin

Normally merged the fresh integration ref above, whose only additional commit
since 0b831887a is #301's WebKit order-gate repair. Incoming storefront/test/output
files are base-owned merge content; this lane did not edit order-gate code/tests.
Preserved migration 0209 byte-for-byte. Static git-tree inventory: base R2 set
138 plus this card's 0209 equals existing exact pin 139. No pin change needed.
Regenerated browser-ownership.json with the native generator (53 modes), and
dependency-map.md with depmap.sh (no map diff). Both commands exit 0; these are
static generation, not tests. No new source assertions or new causal RED needed.
Rollback reference before merge: 2a53235921c47b16eaeadae4cbbf4d8c4c994fac.
Four pre-existing untracked lane evidence files were preserved, not staged.

## Acceptance and handoff

Historical causal RED: Foundation 40762550914/job 123078018073, tests-only head
24cac517c734c0d09cd017f523c98d31738c8a24: four NEW BFF/contract assertions fail
on absent profile endpoints/contract (404 versus 200/401/409 and missing amendment).
Raw receipt remains red.log. No downstream PG RED is inferred.
GREEN: PENDING; author assessment E0. Static generation/inventory is E1 only.
New exact-head Foundation/Gates workflow run IDs will be left in the PR comment
and final response; integrator owns the next-session completion relay. No local
test/server/process or background poller started. No merge/auto-merge/deployment.

Outside-card authored path remains generated browser-ownership.json (pre-approved
generator; integrator review). Existing same-app helper changes remain listed in
the PR. Shared CI classification still needs integrator ownership, not an exemption.
NOT_RUN: local tests/servers/PG/browser, tsc, provider SANDBOX/LIVE, production,
independent acceptance. Humaux tools unavailable: no store/coord/canvas write is
claimed; integrator must mirror this handoff with its shared-state lock protocol.
