<!-- Purpose: deploy-prep-r3 code-side implementation and gate receipt. Depends on: unit brief, CHECKLIST, RELEASE-GATE-PLAN and hash-bound evidence. Used by: Claude Opus review/CI/merge. -->
# deploy-prep-r3 delivery

## CI correction r1 — supersedes the original local receipt below

Current correction is based on integrator merge `9bd4ad24`. CI found the normalized-JSON false-field assumption and
ShellCheck SC1007. Both are fixed with regression controls; see [CI-r1 delivery](ci-r1/DELIVERY.md) and
`ci-r1/EVIDENCE.sha256` for current commands/hashes. The original receipt and `EVIDENCE.sha256` below are historical
evidence for `14a64391`, not acceptance of the merged/corrected release. Owner will rerun GitHub CI; no push/deploy here.

- Branch/commit: `unit/deploy-prep-r3`, this delivery commit (`git log -1 --format=%H -- output/deploy-prep-r3/DELIVERY.md`). Base: `86e404a4f0411bb4349b65234c4afb2f85688e19`; starting brief commit: `a3122263e5e18d8f37ffcf6377cf6fa8fbd08f93`.
- Model/role: Codex-4 implementer, runtime model variant/effort not exposed. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/deploy-prep-r3`. Owned paths: deploy wiring/scripts/env/runbook, dependency maps, R3 Node controls and existing SL06 foundation acceptance bridge, Node gate registration and `output/deploy-prep-r3/`. No Go production source, migrations, module/lockfiles or shared API schema edits.
- Summary: twelve source deliveries collected with line anchors/hashes/dispositions in CHECKLIST. `ops-admin.sh:65` admits eleven platform operator commands and refuses unknowns; `ops-admin.sh:72` confines export paths; existing Stripe platform/sandbox/LIVE-pair gates tested with a stub only. Existing foundation `TestStripeSL06LiveProcess/shell_ops_admin_r3` invokes the same controls in CI.
- `compose.yml:578` + logins/manifest/build list isolate platform operator authority; provisioning checks nine definers and unexpected membership. Export bind survives one-shot exit, with UID65532/0700 initialization in `host-setup.sh:81` and `smoke.sh:243`; preflight P02 checks it.
- `preflight.sh:368` refuses cancelled PAYUNi; API/claims-worker examples include safe health flags, worker Page app id and matched metadata. Expiry-worker admin-origin links are compose-wired. `smoke-r3.sh` plus smoke S48/S49 enforce exact unauthenticated 401 or explicitly-disabled 404; POST probes have valid MOCK bodies and no credentials.
- `deploy/README.md:28` documents migrate/provision → API/workers → admin/storefront barrier, weekly settlement, support, ledger and ads procedures; both dependency maps regenerated. RELEASE-GATE-PLAN distinguishes required release-SHA CI from provider NOT_RUN.
- Contract/interface changes: no API/SQL/signature change. Deployment config admits PAYUNi-disabled and Meta health knobs; adds ops-only platform service/login and persistent export path; wrapper rejects unsafe/missing/duplicate `--out`. New smoke cases S48/S49 are included in expected-case receipts. No new dependencies or image pins.

## Tests and local gates

All commands run in the unit worktree with test/MOCK values. Final source/evidence hashes: `EVIDENCE.sha256`.

| Exact command | Exit | Evidence |
| --- | ---: | --- |
| `node --test tests/deploy/deploy-prep-r3.test.mjs` on unchanged base | 1 | `red.log`: original behavior failures; `red-smoke.log`: missing probe gate; `red-export-dir.log`: independent mount-source defect reproduced before fix |
| `git archive a3122263 deploy` into isolated baseline; copy final test; `node --test output/deploy-prep-r3/baseline/tests/deploy/deploy-prep-r3.test.mjs` | 1 (expected) | `red-baseline.log`: exact final test, four failed groups; baseline fixture removed afterward |
| `node --test tests/deploy/deploy-prep-r3.test.mjs tests/deploy/meta-connect-preflight.test.mjs tests/deploy/ops-alert-preflight.test.mjs` | 0 | `green.log`: 40/40 tests, no skips; includes actual Linux fixture ownership, allow/refuse controls, config and probe negatives |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates.log`: 73 modes registered, test registered, header ratchet passed; existing UI file-size warnings only |
| `LC_HEADERS_STRICT=1 bash scripts/dev/check-headers.sh 86e404a4` | 0 | `headers-strict.log`: changed source header labels verified |
| `bash deploy/scripts/check-pins.sh` | 0 | `check-pins.log` |
| `bash deploy/scripts/smoke.sh static` | 0 | `smoke-static.log`, `smoke-static-result.json`, `smoke-static-evidence/`: S01–S06/S48 PASS; S01 shellcheck NOT_RUN locally; lcentry coverage 96.7% |
| `go vet ./...` | 0 | `go-vet.log` (empty successful output) |
| `bash scripts/dev/gen-deps.sh` | 0 | `gen-deps.log`: architecture map, 93 packages |
| `bash scripts/dev/depmap.sh --check` before regeneration | 1 | `depmap-stale-red.log` preserves base-to-regenerated diff. Pre-existing stale map detected; regenerated with `bash scripts/dev/depmap.sh` →0 (`depmap-generate.log`) |
| `bash scripts/dev/depmap.sh --check` after regeneration | 0 | `depmap-check.log` |
| `bash scripts/dev/release-gate.sh --list` | 0 | `release-gate-list.log`; list only, no release suite run |
| `command -v shellcheck` | 1 | `shellcheck-availability.log`: not installed. Intended `shellcheck -S warning -x` on changed scripts is NOT_RUN, no execution exit code |
| `git diff --check` | 0 | `diff-check.log` |

Evidence class: **E3 MOCK/static**, final file hashes. Independent review reproduced/fixed P1 missing export bind source;
container-local fixture and six P02 controls passed (`independent-review-round1.log`, `independent-review-round2.log`, Humaux
`cd06d60f-678c-4bcf-b06d-9d61701d5445`). The review's source hashes precede final header comments and stronger MOCK tests;
the final E3 receipt pins all final files. P0/P1 in reviewed unit scope: none confirmed remaining.

## CI gates (integrator runs; NOT_RUN in this unit)

- **`.github/workflows/deploy-smoke.yml` — static + full**, including shellcheck, actual provision/operator startup and S49 route probes. CI verdict rejects NOT_RUN or missing cases. No production host.
- `.github/workflows/gates.yml` — unit/full foundation and impacted modes listed in RELEASE-GATE-PLAN, including SL06's registered R3 allowlist bridge. Full `release-gate.sh --strict --only G07` is mandatory after merge (login/provision privilege shape changed); full release groups/B-* receipts must bind the final release SHA.
- No full foundation/PG/browser/image build/full smoke/SANDBOX provider/LIVE gate run locally. No gate is silently replaced by focused MOCK passing.

## Risks / integrator to-do

- Run/review CI, then merge; no push/merge/deploy occurred in this unit. Owner 「等全部做完後再上線」 remains binding.
- Existing hosts must prepare export directory before any new stripe-admin run; preflight now fails closed for missing/wrong-owner/mode paths. Sandbox/LIVE credentials and owner approvals remain external.
- Weekly `unmapped_source` HOLD needs an approved append-only resolution workflow before LIVE; payout CLI only records an independently completed transfer. Support order-page fences and SG-OPEN-1/2 PII/consent remain integrator/owner decisions.
- Reconcile source delivery follow-ups: migration/upgrade counts, shared OpenAPI/schema, pending UI/BFF and review resolutions. `--returns` and `--migration-import` are absent on base: register them or prove complete coverage in G07, never dispatch unknown modes.
- No deliberate new `ponytail:` shortcut. Existing media/reference-only limits and accepted NOT_RUN catalogue are preserved. All task-created baseline/config fixtures and test containers are removed; static/review failure evidence retained.
