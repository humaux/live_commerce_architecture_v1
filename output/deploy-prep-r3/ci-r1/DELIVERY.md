<!-- Purpose: CI-r1 correction receipt for deploy-prep-r3. Depends on: actual GitHub failure logs, current source, ShellCheck and regression controls. Used by: owner/integrator CI rerun and review. -->
# deploy-prep-r3 CI-r1 delivery

- Branch/commit: `unit/deploy-prep-r3`, this correction commit (`git log -1 --format=%H -- output/deploy-prep-r3/ci-r1/DELIVERY.md`). Base: `9bd4ad24440d2e4def4ed45022d988c5c5a077c5` (integrator merge). `git pull --ff-only` returned already up to date; worktree was clean before correction.
- Role/model: Codex-4 implementer (runtime variant/effort not exposed). Read-only independent security reviewer: configured `gpt-6.1-sol / high`, same unit worktree, no source writes. Owned source paths: `deploy/scripts/ops-admin.sh` and `tests/deploy/deploy-prep-r3.test.mjs`; delivery/evidence only under `output/deploy-prep-r3/`.
- Root cause 1: [foundation run 37584610325](https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/actions/runs/37584610325), SL06 `shell_ops_admin_r3`, test line 122 expected normalized JSON `create_host_path === false`, but Ubuntu's serializer omitted false (`undefined`). This is not a missing service, binary or permission failure. Older [compose-go types](https://raw.githubusercontent.com/compose-spec/compose-go/v2.4.9/types/types.go) used `omitempty`; source YAML omission can default to true ([Docker volumes reference](https://docs.docker.com/reference/compose-file/services/#volumes)), so a bare `?? false` would be insufficient.
- Fix 1: normalize false/undefined only with an explicit source-YAML false assertion. Reject true, missing source flag and missing bind options. Copy the actual export bind into an isolated Compose create probe: safe config must refuse absent source with the precise bind-source error; true mutation succeeds and therefore fails the safety predicate. No service starts, no PG process/build/pull/network or secret is involved; exact projects/volumes are removed.
- Root cause 2: [deploy-smoke run 37584606194](https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/actions/runs/37584606194) reported S01/S48 FAIL. Artifact API returned `total_count: 0`, so the S01 artifact is unavailable. The complete local S01 script set reproduced SC1007 at `ops-admin.sh:75`, ambiguous empty assignment `export_out= export_count=0`.
- Fix 2: `export_out='' export_count=0` makes intent explicit, preserving runtime behavior. No disable directives or threshold changes. Full deploy-script ShellCheck now passes; compatibility controls also make S48 pass.
- Contract/interface changes: none. Compose and operator behavior stay unchanged; regression coverage grows from four to six R3 groups. Tests remain wired into Node, smoke S48 and existing SL06 foundation gate.

## Actual commands and exits

| Command | Exit | Evidence |
| --- | ---: | --- |
| `gh run view 37584610325 --log-failed` | 0 | `foundation-failure-excerpt.log`; relevant assertion and original-log digest in `PROVENANCE.md` |
| `gh run view 37584606194 --log-failed` | 0 | `smoke-failed.log`: S01/S48 failures |
| `PATH=<serializer-control-bin>:$PATH node --test --test-name-pattern='platform operator custody' tests/deploy/deploy-prep-r3.test.mjs` before fix | 1 expected | `custody-red.log`: same undefined/false failure as CI |
| `PATH=<serializer-control-bin>:$PATH node --test tests/deploy/deploy-prep-r3.test.mjs` after fix | 0 | `serializer-green.log`: 6/6, no skips |
| `node --test tests/deploy/deploy-prep-r3.test.mjs tests/deploy/meta-connect-preflight.test.mjs tests/deploy/ops-alert-preflight.test.mjs` | 0 | `node-green.log`: 42/42, no skips |
| `<temporary-official-shellcheck-0.10.0>/shellcheck -S warning -x deploy/scripts/*.sh deploy/postgres/*.sh deploy/postgres/ops/*.sh` before/after fix | 1 → 0 | `shellcheck-red.log` (SC1007), `shellcheck-green.log` (empty successful output); source/version/digest in PROVENANCE |
| `PATH=<shellcheck-bin>:<serializer-control-bin>:$PATH bash deploy/scripts/smoke.sh static` | 0 | `smoke-static-green.log`, `smoke-static-result.json`: S01–S06/S48 all PASS, shellcheck included |
| `python3 .github/scripts/smoke-verdict.py <current-static-result.json> 0 static` | 0 | `smoke-verdict.log`: 8 PASS, 0 FAIL, 0 NOT_RUN, no missing case |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates.log`: 76 documented modes, header/test registration passes |
| `bash deploy/scripts/check-pins.sh` | 0 | `check-pins.log` |
| `go vet ./...` | 0 | `go-vet.log` |
| `git diff --check` | 0 | `diff-check.log` |

Evidence class: **E3 DB-free/MOCK/static**, current source/evidence hashes in `EVIDENCE.sha256`. Independent reviewer
reran 6/6, serializer custody, syntax and ShellCheck; no confirmed P0/P1 remains in this correction's scope
(`independent-review-round2.log`). The omission control changes only config JSON serialization, not Docker behavior;
it does not claim an actual Ubuntu runner execution. Existing source/behavior/mutation predicates remain enforced.

## CI gates / limits

- Owner reruns `.github/workflows/gates.yml` foundation (including SL06 `shell_ops_admin_r3`) and `.github/workflows/deploy-smoke.yml` static + full on this commit. These GitHub reruns are NOT_RUN here.
- Full PG/foundation, browser, image builds, full smoke and provider SANDBOX/LIVE are NOT_RUN locally. No SSH, production host, live keys, git push, merge or deploy.
- Only the official temporary ShellCheck binary was used; no host installation or project dependency was added. Temporary binary/archive and fixture helpers are removed after verification, with provenance/control source preserved. Keep original unit CHECKLIST/release-plan follow-ups for integrator/owner.
