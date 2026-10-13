<!-- Purpose: record UX-HARDEN-R1 scope, provenance and SHA-bound CI-only red/green acceptance.
     Depends on: product-editor/storefront browser gates and GitHub Actions receipts.
     Used by: owner, integrator and independent PR reviewers. -->
# UX-HARDEN-R1 delivery

Status: behavioral red tests submitted; product fixes and acceptance pending.

- Branch: `unit/ux-harden-r1`; PR target: `r3/integration`.
- Base after `git fetch origin`: `0bfe36e53acb694f4697521202e24f0790638941`.
- Before the red-test commit, `git fetch origin` succeeded; `origin/unit/ux-harden-r1` did not yet exist (ref lookup exit 128), so there was no own remote branch to merge.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-harden-r1`.
- Author: primary agent, UI implementation/integration role; actual model `gpt-6.1-sol`, reasoning `ultra`. Config top-level model/effort and own session turn context verify these values.
- Behavioral-test delegate: `/root/ux_browser_red`, `test_worker`, actual `gpt-6.1-sol`, reasoning `high`; same assigned unit worktree, exclusive ownership of `tests/admin/product-editor-layout.acceptance.ts` while root reads product source. No recursive delegation.
- Read-only delegate: `/root/ux_shop_static`, `explorer`, actual `gpt-6.1-sol`, reasoning `medium`; same worktree, no write paths.

## Scope and acceptance

1. Distinct localized names for section navigation, section group, readiness toggle and heading.
2. Focus remains on the same section button when resizing across 900px in either direction.
3. Manual section-strip scrolling survives unrelated draft renders; changing active section reveals the selected button.
4. Overflow hint visibility changes retain the same reserved height.
5. The real storefront chip check waits for layout/loading to settle, retaining its no-horizontal-scroll and page-overflow assertions.

Behavior is measured in the existing product-editor browser acceptance; no source-regex proof. Ads money formatting is outside this unit. No API, Go, migration or transaction changes.

## CI gates

All executions are GitHub-only under the owner's 2026-10-13 rule. No local tests, typechecks, builds, dev servers or database containers are run.

- Red: push behavioral tests on unchanged product source, dispatch `gates.yml` with `--browser-product-editor`; record run ID and actual UXH failures here before implementing fixes.
- Green: `--browser-product-editor`, default REAL-stack `--browser-storefront`; PR-required browser universe and foundation checks apply from the existing path planner.
- Foundation checks supply `scripts/dev/test-node.sh` and `scripts/dev/check-gates.sh`; typecheck/build results must be read from CI.

## NOT_RUN

- All test/gate executions are pending at this checkpoint; no PASS or release-readiness claim.
- Production, physical-device rotation, vendor SANDBOX/LIVE requests and deployment are outside scope.
