<!-- Purpose: P2 follow-up implementation and evidence receipt. Depends on: owner review list, trunk0d6b4b5f and current source/gate hashes. Used by: integrator review, foundation shards and deploy-smoke CI. -->
# deploy-prep-r3-p2 delivery

Shared-loader follow-up supersedes the original source receipt below for lib.sh/Node controls; see
[loader/DELIVERY.md](loader/DELIVERY.md) and its current hash receipt. The original P2 gates remain historical evidence
for `121789dc`; integrator will push/run the combined foundation-shards + deploy-smoke once.

- Branch/commit: `unit/deploy-prep-r3-p2`, this delivery commit (`git log -1 --format=%H -- output/deploy-prep-r3-p2/DELIVERY.md`). Base: `0d6b4b5f214b99589e459ec6a02cd75a54bf5933`. First Git mutation was the requested `git switch -c unit/deploy-prep-r3-p2 origin/r3/integration`; clean start.
- Role/model/worktree: Codex-4 implementer (runtime model variant/effort not exposed), `/Volumes/data/live_commerce_architecture_v1/.worktrees/deploy-prep-r3`. Owned source: expiry-worker config/loops/tests, deploy README/compose/env/ops/preflight/smoke, existing R3 Node controls and generated dependency maps. No migration, shared schema or module/lockfile edits. Read-only explorer configured gpt-6-luna/medium; independent reviewer gpt-6.1-sol/high; no parallel source writers or recursive delegation.

## Review items

1. README prerequisites now require restricted `/etc/live-commerce` tarball, host-setup or UID65532/0700 directory install, add-missing-only secrets-init (no --rederive), and preflight green before upgrade. Names automatic `pg-ops.sh backup --tag pre-upgrade-<tag>` plus config tarball as rollback point. `deploy.sh` is the executor; it stops all old processes before migrate/up, with no separate API-before-admin startup enforcement.
2. `LC_MERCHANT_ALERT_MAIL=0` maps to independent `COMMERCE_MERCHANT_ALERT_MAIL`; runtime accepts two separate opt-ins, loads shared SMTP when either is on, constructs the buyer worker only on buyer opt-in and reads/uses admin origin only on merchant opt-in. Matrix covers off/off, buyer-only, merchant-only, both, invalid flag/origin. Preflight strictly validates 0|1 and app/SMTP requirements; Node tests prove actual Compose default-off and explicit-on wiring.
3. LIVE Stripe gate reads both LC_* values exclusively through `lc_env_file_get` from compose.env. Caller LC_* and COMMERCE_* spoof cannot satisfy it; valid file overrides exported false/spoof values. Existing kill-switch behavior stays intact; no global lib.sh interpolation change.
4. Platform allowlist asserts exact Docker service/binary/subcommand. Audit requires a nonempty file, recorded exit and no value leak; a missing-file negative rejects. P02 covers valid/missing/symlink/owner/group/mode (actual path/mode with scoped MOCK uid/gid); real preflight covers bad Advanced Access grammar, permission/DM mismatch and merchant-only SMTP rules. Added full smoke S10s–S10y and expected-case registry, including isolated-copy directory mutation, merchant-mail red/green, grammar and mismatch.
5. Static smoke header explicitly requires Docker and forbids production-host execution. PAYUNi flag is documented as P06 tripwire only; code is removed. Generated dependency maps updated for worker flag/doc changes.

Optional export-only service: deferred for this unit; separate service, wrapper selection and custody/smoke test rewiring exceed the small follow-up. Existing export path/security guards remain. No new dependency or production action.

## Actual commands / exits

| Command | Exit | Evidence |
| --- | ---: | --- |
| `go test ./cmd/expiry-worker -run TestBuyerMailConfig -count=1` before implementation | 1 expected | `mail-red.log`: buyer-only origin previously enabled merchant alerts |
| `node --test --test-name-pattern='sanctioned operator' tests/deploy/deploy-prep-r3.test.mjs` before implementation | 1 expected | `live-red.log`: exported LC pair was admitted |
| `go test ./cmd/expiry-worker -count=1` | 0 | `mail-green.log` |
| `GOTOOLCHAIN=go1.27.1 go test -race ./cmd/expiry-worker -count=1` | 0 | `mail-race.log` |
| `node --test tests/deploy/deploy-prep-r3.test.mjs tests/deploy/meta-connect-preflight.test.mjs tests/deploy/ops-alert-preflight.test.mjs` | 0 | `node-green.log`: 45/45, no skips |
| temporary official ShellCheck0.10.0: `shellcheck -S warning -x deploy/scripts/*.sh deploy/postgres/*.sh deploy/postgres/ops/*.sh` | 0 | `shellcheck.log`; no new disable directives |
| `PATH=<temporary-shellcheck-bin>:$PATH bash deploy/scripts/smoke.sh static` | 0 | `smoke-static.log`, `smoke-static-result.json`: S01–S06/S48 PASS, shellcheck included |
| `python3 .github/scripts/smoke-verdict.py <static-result.json> 0 static` | 0 | `smoke-verdict.log`: no missing/NOT_RUN case |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates.log`: 77 modes, 1217 top-level foundation tests assigned exactly once |
| `LC_HEADERS_STRICT=1 bash scripts/dev/check-headers.sh 0d6b4b5f` | 0 | `headers.log` |
| `go vet ./...` | 0 | `go-vet.log` |
| `bash scripts/dev/gen-deps.sh` / `bash scripts/dev/depmap.sh` / `bash scripts/dev/depmap.sh --check` | 0 / 0 / 0 | `gen-deps.log`, `depmap-gen.log`, `depmap-check.log` |
| `git diff --check` | 0 | `diff-check.log` |

E3 DB-free/MOCK/static; current hashes in EVIDENCE.sha256. Independent reviewer reran Node9/9, pinned cached Go1.27.1
configuration tests, syntax and lint; no remaining confirmed P0/P1/P2 in scope (`independent-review-round2.log`). The
initial app-profile omission in a new config assertion was corrected by explicitly rendering app as well as ops;
assertions were preserved. Failure evidence is retained; temporary tool/config fixtures and own containers removed.

## CI gates / NOT_RUN

- **gates: unit + all foundation-shards**, including SL06 shell_ops_admin_r3/file-only gate and expiry-worker regressions.
- **deploy-smoke: static + full**, including actual preflight S10s–S10y, operator/service custody and mail default-off wiring.
- Full foundation/PG, browser, full smoke and real SMTP/provider tests remain NOT_RUN locally; integrator runs CI on this commit. No push, SSH, production host/key, message, money or deploy action occurred. Owner release approval stays separate.
