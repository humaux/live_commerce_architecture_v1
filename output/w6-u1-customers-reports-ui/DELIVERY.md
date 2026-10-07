<!-- Purpose: W6-U1 source checkpoint and acceptance handoff.
Depends on: frozen W6-01B0139/W6-02B0147 APIs, the source/test manifest and integration branch.
Used by: integrator CI and independent acceptance; no provider or production claim. -->
# W6-U1 customers tags/notes and reports

Status: source checkpoint, final acceptance pending. Local focused checks passed; independent UNKNOWN fix is being integrated. Final SHA and evidence are recorded in the main checkout receipt after the final commit.

Role: Codex-2/root integrator; UI workers and independent test/review children used isolated worktrees. Configured children gpt-6.1-sol/high; exact deployed runtime model UNKNOWN. Task e052872b-32cf-4a78-bbb4-b8d1e106023f, branch unit/w6-u1-customers-reports-ui, initial base88fe248d. Existing Qwen WIP1694c075 retained and audited. Main evidence directory: /Volumes/data/live_commerce_architecture_v1/output/w6-u1-customers-reports-ui/.

## CI gates

GitHub only, to be run by integrator:
- bash scripts/dev/test-local.sh --browser-customers-billing (preserves CB11 and adds TestBrowserW6Customers)
- bash scripts/dev/test-local.sh --browser-reports
- bash scripts/dev/test-local.sh --browser-admin-shell
- bash scripts/dev/test-local.sh --browser-click-sweep (current10shards + aggregate)
- bash scripts/dev/test-local.sh --browser-visual-lint (current4shards + aggregate)
- pnpm --filter @live-commerce/admin build

Calibration pending on GitHub: LC_W6UI_CALIBRATION=reports-truncate-response with --browser-reports must be RED on RPUI-RED-REPORT-DATA; LC_W6UI_CALIBRATION=customers-drop-response with --browser-customers-billing must be RED on CTUI-RED-NOTE-CONFIRMED. Normal modes must be GREEN. Setup/type/build failures do not count as calibration.

## NOT_RUN

Real browser/PG/Next build and GitHub CI are NOT_RUN locally under owner's RAM rule. Provider SANDBOX/LIVE and production are NOT_RUN. Browser runtime calibration is pending. Current backend omits caller/author display identity, so old notes for write-only staff are conservatively read-only; current-session created-note authorship is proven. Older live-session titles beyond the existing20-item lookup show a localized unavailable fallback. No profit metric or authorization by UI assumption.
