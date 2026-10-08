# ci-pr-gates delivery
- Branch: unit/ci-pr-gates   Base: 35abffca (origin/r3/integration)   Model: Claude Sonnet 5.5
- Summary: `scripts/dev/pr-modes.mjs` (pure `planPr(paths)` + CLI `<base> <head>` / `--stdin`; browser set parsed from the test-local.sh usage line with release-gate.sh's pattern, minus `EXCLUDED_MODES`).
  `.github/workflows/gates.yml`: `pull_request` (r3/integration; opened/synchronize/reopened), plan job runs pr-modes on base.sha...head.sha (fetch-depth 0), per-PR concurrency `gates-pr-<n>` cancel-in-progress,
  `extra_env` forced empty off-dispatch, `deploy` job (reusable deploy-smoke) and a final `required` job. `deploy-smoke.yml`: `workflow_call` added, its own `pull_request` trigger now ignores r3/integration.
  Docs: GATES.md "Pull-request gates", PROCESS.md section 2 step 6. `scripts/dev/test-node.sh` runs the new test.
- Required check name: `Gates (GitHub runners) / required`.
- Deploy-smoke choice: reusable workflow (`uses: ./.github/workflows/deploy-smoke.yml`), because it needs no duplicated steps and `required` can wait on it; its old unconditional PR trigger is limited to other base branches (no double run, a UI-only PR no longer pays ~50 min).
- Excluded mode: `--stripe-browser` (SP18 needs STRIPE_SECRET_KEY test key from secrets.env + STRIPE_BROWSER/STRIPE_SANDBOX; gates.yml has no secrets, it would end NOT_RUN). `--browser-webkit` stays (WebKit installed by the workflow).
- Jobs (ci-plan output): backend/docs PR = 11 gate jobs (+ plan + required); UI PR = 70 gate jobs (foundation 11, click-sweep 10, visual-lint 4, 45 other browser modes) + plan + aggregate + required; deploy PR adds the deploy-smoke job (~50 min).
- Tests: `node --test tests/ci/*.test.mjs` -> exit 0 (18 pass; red before implementation: output/ci-pr-gates/red.log, green: green.log)
- Gates run: `bash scripts/dev/check-gates.sh` -> exit 0 (run with a read-only symlink to another worktree's node_modules because this worktree has none; the symlink was removed; without it the run stops at tests/admin/shell-architecture.test.mjs: Cannot find module 'typescript-api'). check-headers OK. YAML parse (python yaml) of both workflows OK.
- Evidence class: MOCK (workflow logic not executed on GitHub yet)
- NOT_RUN: `actionlint` (not installed); a real PR run on GitHub (needs a push; integrator).
- Risks: (1) 70 parallel jobs on a UI PR vs a 20-concurrent free tier: they queue, wall time several times the ~16 min per-job estimate; each PR push cancels the previous run. (2) pr-modes.mjs runs from the PR's own merge ref, so a PR that edits it could shrink its own set; `scripts/` and `.github/` are UI paths and the reviewer must read changes to pr-modes.mjs/gates.yml. (3) The ruleset (owner) must require only `Gates (GitHub runners) / required` and `review/independent`. (4) deploy-smoke runs as a called workflow: its check name becomes `Gates (GitHub runners) / deploy / smoke`.
- Integrator to-do: push, open a PR to confirm the plan job output and the `required` name on GitHub; owner approves the ruleset.
