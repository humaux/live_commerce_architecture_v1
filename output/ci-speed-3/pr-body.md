## Policy change (integrator-approved)

1. Amend only `new selection never drops legacy modes or deploy for any tracked path`: for app and tests/admin|e2e|ui|storefront paths, require every derived owner (nonempty, otherwise full universe). Preserve backend-only expectations, conservative ALL paths and deploy flags. Authority: integrator ruling 2026-10-28 and owner “太慢了，需要提升ci的速度…好好想想怎么提高速度”; CI-SELECT “narrow per domain + nightly”.

The planner now derives app/spec/helper selection from committed execution ownership and preserves narrowing through stdin, Git diffs and GITHUB_OUTPUT. The generator records registry regex/harness/suite, server/build and import evidence; the existing backend-coverage CI suite checks drift and a corrupted-artifact rejection witness. Unknown/unresolvable inputs, shared infrastructure and registry changes remain conservative. Foundation's registered Node suite accounts for prep-only computed app loaders; those unresolved inputs themselves still select ALL.

Schedule and `workflow_dispatch(full_browser_universe=true)` retain the FULL browser universe. Push remains foundation-only per the integrator correction; no workflow/ci-plan/test-node edits. The base-present acceptance spec is `tests/admin/live-settings.spec.ts`; the unowned reauth path retains ALL.

## Evidence and measured limits

- RED SHA `04036e51ba4e7ed1236a825e1f96c635f563ec44`: Foundation run **40631602073**, job **122629013295**, failed the new spec/CLI, missing derivation, helper/app/monotonicity and conservative-universe assertions. Raw filtered failures: `output/ci-speed-3/red-assertions.log`; IDs/context: `red.log`. Gates run 40631602098 succeeded on RED but is not card GREEN.
- E1 implementation syntax checks and packet structure check exited 0. No local tests or servers. CI GREEN and independent review pending; author PASS is E0.
- Static ownership: known live-settings spec 53 -> 1 owner; shared native-device helper has 6 owners. Last ten merged PR diffs: **473 -> 473 (0%)** due to conservative shared/foundation paths. Full receipt table and method: `output/ci-speed-3/impact.md`.
- **The <=15 single-app target is not met**: admin 47 owners, storefront 29. A narrower component/route policy would require a separate approved contract; no actual owner was omitted to satisfy a count.
- Artifact prepared from integration `997480141a7959ad6256a29e43bb1af31b97a404` using read-only `--source-ref`, since #248/#237 advanced ownership inputs after this branch forked. CI checks the real merge checkout; newer base drift is not a PASS.

## Scope and handoff

Base: `5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-speed-3`. Branch: `unit/ci-speed-3-ui-select`. Dispatched implementer: Codex; no recursive delegation. Exact backend model identifier/effort not exposed.

No final file changes outside card write paths; generated JSON is next to the existing covers derivation as the card explicitly requires. No workflow, ci-plan, test-node, next-startup assertions, baselines, dispatcher pins, migrations or R2 pin changed. Independent review remains with the integrator; no merge/auto-merge. An uncommitted upstream merge was immediately aborted after rechecking the no-merge rule; no upstream changes were retained. Humaux unavailable, so shared memory/canvas/coord writes were not performed. Details: `output/ci-speed-3/handoff.md`.

🤖 Generated with Codex (dispatched by Claude Code integrator).
