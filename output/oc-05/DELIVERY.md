# OC-05 GET 422 fix — attempt 1 for this blocker (2026-10-29)

This receipt supersedes pending status statements below, which remain historical.
Authority: integrator's PR #254 GET-422 fix card; merchant-settings-wizard-v1 OC-05; repository transport code.
Role: dispatched OC-05 implementer; runtime model GPT-6, effort not exposed; worktree .worktrees/oc05; branch unit/oc05-quickstart-marker.
Entry head: 27c0d51d034f401603ee106052627a5b1908e8c6. Merged origin/r3/integration at 11f281f8c6439489e402ba70f90737cd31dc1a0a, no conflicts; retained base and pre-existing uncommitted work.

Read all three failed jobs in Gates 40650048011: shard:unit 122733463849, receipts 122843044695, required 122909447868. The latter two propagate the unit failure. Original filtered logs: fix-get-body-red.log. E3 failure: five TestOC05 HTTP tests returned 422 instead of 200; migration and RLS tests passed on that old run. Synthetic merge SHA e5bc7ce102d45fc30179be9677d28a4839b4d7ab, PR head 27c0d51d034f401603ee106052627a5b1908e8c6.

Root cause: httptest.NewRequest wraps strings.NewReader("") into a present request Body, which studioRoute correctly rejects for GET before authorization. Cross-scope tests also sent a JSON body on GET, preventing their 404 assertions from reaching authorization. Construct bodyless GET with a nil reader and use {} only on POST. All existing status and domain assertions remain unchanged. Add TestOC05GETRejectsBodies for both an empty-but-present body and {} to retain the strict transport boundary. No production code, gate, threshold, exemption or baseline changed.

Original tests-only RED: Gates 40619573790 / 122603618623 at head 5da45aea079019ed17d12368a80e4b7183dfb368; recorded in red.log. No redundant RED push; this is the authorized targeted fix after observed assertion RED.
Static evidence E1 only: fix-get-body-static.log (source hash, commands, exit 0). Dependency map regenerated and unchanged.
This fix changes only tests/quickstart/quickstart_test.go and output/oc-05/**. Existing PR out-of-card scope for integrator review: contracts/merchant-settings-wizard-v1.md, internal/httpapi/handler.go, tests/foundation/r2_integration_upgrade_test.go (assigned pin 135), tests/quickstart/doc.go, scripts/dev/pr-modes.mjs (previous classification authorization), docs/engineering/dependency-map.md (generated allowance).
GREEN: PENDING until this push's own Gates/Foundation conclude. Author PASS = E0. Runtime verification, independent review, sandbox/live/provider/production: NOT_RUN. No local tests or servers. No new background process or fixture. Integrator relays a fresh session when CI concludes; wait at most approximately ten minutes to register current-head runs.
Humaux tools unavailable: memory store, code index/link, coord and canvas NOT_RUN. No local/platform memory was read or written. Pre-existing untracked receipts retained.

---

# OC-05 resume receipt (2026-10-28T07:06:34.762008+00:00)

Prepared implementation pushed unchanged to PR #254: `ee8da5325eea7d40e28992c6dfc42045e77e47f0`. Push exit 0; PR head independently read back as this SHA.
RED: Gates 40619573790 / job 122603618623, six TestOC05 failures on their own checks; filtered original excerpts appended to red.log (E3 failure evidence). Synthetic merge receipt SHA 924bfa74545d47744895da2689641e0769638c83.
WAIT_RUN: 40647834448 (Gates, queued; no test job yet).
WAIT_RUN: 40647834419 (Foundation, queued).
WAIT_JOB: 122680684562 (foundation).
GREEN: PENDING. Independent review: NOT_RUN. Author PASS = E0.
Outside card write paths: none. Unchanged pr-modes backend coverage classification requires integrator approval if needed; do not bypass it.
No local tests or servers; no new background processes. Integrator is the designated completion wakeup, as requested. Humaux unavailable: store/canvas/coord updates NOT_RUN.
Evidence updates remain local under output/oc-05; no extra commit was created. Earlier pending entries in DELIVERY.md/red.log are historical and superseded by this receipt.

---

# OC-05 handoff — behavior RED pending

PR: https://github.com/humaux/live_commerce_architecture_v1/pull/254 into r3/integration.
Remote tests-only head: `5da45aea079019ed17d12368a80e4b7183dfb368`.
Base: `79f95784c9dee419346fe3119d4d4f56720d4a3c`; branch `unit/oc05-quickstart-marker`; assigned worktree `.worktrees/oc05`.
Role: dispatched commerce implementation worker (Codex / GPT-6; exact variant/effort not exposed: UNKNOWN). No delegation, merge, auto-merge, force-push or commit-status posting.

## Prepared candidate (local commit, not pushed)

- Assigned `0205_quickstart_marker.sql`: append-only `control.quickstart_markers`, PK tenant/store, database-default completion timestamp. Store creation serialized across old-store backfill; new stores have no marker. Runtime SELECT and INSERT of tenant/store only; no timestamp override, update or delete authority. Scoped FORCE RLS; no new definer or privileged role.
- `internal/quickstart`: authenticated Get and Complete with permission fences before/after work. Derive nonblank store name, enabled current shipping service, and any existing product. Dismissal does not fake derived steps. Primary-key conflict preserves first completion time across retries.
- Admin GET/POST `/v1/admin/stores/{store_id}/quickstart`, store:read / integration:manage, no query, strict empty-object POST and required idempotency key. Routes documented in the owning merchant wizard amendment; assigned R2 pin 135 plus comment only.
- Real-PG tests: new state, live derived product/shipping/current-version data, immutable idempotent completion, cross-tenant/store refusal, strict client-state rejection, full assigned migration replay in a rollback-only upgrade transaction plus normal service read, and direct RLS/timestamp grants.

Authority: dispatched OC-05; tasks.json OC-05; merchant-settings-wizard-v1 AD-03 and owner-choices-20261026-v1. UI OC-06 excluded.

## Evidence and stop point

E1 only for the candidate: allowed local gofmt, `go build ./internal/quickstart ./internal/httpapi`, `go vet ./internal/quickstart ./tests/quickstart`, `git diff --check` all exit 0. Dependency map regenerated only with `scripts/dev/depmap.sh`. Exact file hashes: `static.log`. NO local tests, servers, PG, browser, containers or model experiments.

Initial tests-only `1dbc1c40a`: Foundation `40615454727` / job `122576770772` failed package docs (`doc lacks "never"`). This failed before behavior tests and is NOT valid RED. Original excerpt `red-reachability.log`. One targeted tests-only repair `5da45aea0` adds the truthful package non-goal; no assertion or gate was weakened.

At the 02:30 UTC active queue cutoff:
- Foundation `40619573804` / `122590042610`: SUCCESS (7m2s); real-PG/race selector is skipped for integration PRs in this workflow, so this is not quickstart acceptance.
- Gates `40619573790`: plan `122590613621` SUCCESS; build `122591734081` QUEUED; no quickstart PostgreSQL assertion RED observed. Deploy skipped as unselected.
- Raw/pending receipts: `red.log`, `red-pending.json` (initial head) and `red-current-pending.json` (current tests-only head). TLS handshake errors during an earlier cancel/read attempt were transport errors, not CI conclusions or proof of cancellation.

Implementation push is withheld until the new tests fail on their own assertions. Candidate runtime acceptance, GREEN and independent review: NOT_RUN. Author self-assessment PASS is E0; no completed-card claim.

## Integrator next action and outside-scope dependency

Continue monitoring the existing Gates run (a bounded read-only monitor with terminal notification is registered at handoff; log `output/oc-05/ci-watch.log`). On timeout/transport failure it reports that state rather than a CI verdict. Inspect shard:unit's failed log for TestOC05 assertions. Save valid RED run/job/test receipts before pushing the local candidate; then read the PR's own CI.

Outside write paths touched: NONE. Required classification follow-up (not edited): new `internal/quickstart` is absent from `scripts/dev/pr-modes.mjs` / lc_covers, so `check-backend-coverage.mjs` requires integrator scope extension. A conservative SHARED classification would retain all Go-booting browser modes until OC-06 provides precise coverage. This is statically identified, not an observed candidate CI failure. No scanner, exemption, baseline or route-dispatcher list was changed.

Humaux rationale: `[decision] OC-05 append-only scoped completion and derived steps PR254` (`59d668ec-4c37-41aa-8b88-85e7c33e99ed`). Qualified code-memory link was refused entity-not-found; no link claimed. Only this worker's canvas is updated. No separate coordination claim (dispatched worker).

Rollback: code revert through integrator review; forward-only migration data retained. No production migration or provider action occurred. No local runtime/test processes were started; only the registered bounded CI monitor may remain active.
