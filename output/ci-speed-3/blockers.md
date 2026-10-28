# CI-SPEED-3 preflight blockers

Status: BLOCKED before RED; static inspection only (E0; no runtime acceptance).
Date: 2026-10-28.
Assignment: implementation worker dispatched by Claude integrator; no delegation.
Branch: `unit/ci-speed-3-ui-select`.
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-speed-3`.
Base/head: `5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248`.
Runtime model: GPT-6 Codex; exact backend model identifier and effort are not exposed.

## Authority and scope

The current dispatch card requires stopping for blockers outside this card. The
2026-10-28 ruling permits amending only the tracked-path monotonicity test as
specified; it does not authorize workflow or registry edits. Main checkout was
not modified. No tests, servers, generators or builds were run locally.

## 1. Push full-universe requirement needs an excluded workflow edit

Card step 4 and test (e) require push-to-r3/integration to run the full browser
universe. At this base, `.github/workflows/gates.yml:87-92` invokes the planner
for schedule/full-universe dispatch only. The `MODES` expression at line 112
falls through to `["foundation-shards"]` for push. Push never invokes
`pr-modes.mjs`, so changing the selector cannot implement this requirement.

The workflow is explicitly excluded from this card and owned by PR #248.
Recovery: its owner supplies the push full-universe wiring, or the integrator
revises the push requirement explicitly. Preserve the requirement until then.

## 2. Named reauth acceptance path has no derivable execution owner

`git ls-files 'tests/admin/oc/*'` returned no files. A literal search for
`reauth` in `scripts/dev/test-local.sh` and `playwright.config.ts` returned no
matches (exit 1). Read-only GitHub checks confirmed the remote integration
branch head is also `5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248`, and:

```text
gh api 'repos/humaux/live_commerce_architecture_v1/contents/tests/admin/oc/reauth.spec.ts?ref=r3/integration' --jq .sha
exit 1; HTTP 404 Not Found
```

Test (a) requires this specific path to select only executing owners. The same
card and ruling require unknown/unowned paths to select ALL. This path cannot
currently demonstrate narrowing without inventing ownership or changing the
registry/specs outside scope.

Recovery: land the actual spec and runner registration, or explicitly approve
an existing registered spec as the acceptance example.

## Handoff

- No PR, commits or push; no RED/GREEN run/job IDs and no WAIT_RUN.
- Implementation, RED assertions, GREEN, impact measurement and independent
  review: NOT_RUN.
- Only `output/ci-speed-3/blockers.md` was written; no out-of-scope files changed.
- Humaux tools were unavailable in the exposed tool inventory: no memory store,
  canvas update, coordination write or lock acquisition was performed.
- No background process or pending CI run was started by this worker.

## Superseded by integrator correction, 2026-10-28

Both blockers above are resolved by the latest dispatch: push remains foundation-only;
use base-present live-settings.spec.ts. Current state: PR #266 tests-only RED queued.
See handoff.md and red.log for exact-head run IDs. Historical text above is not current status.
