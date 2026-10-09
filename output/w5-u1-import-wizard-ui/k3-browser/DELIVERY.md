<!-- Purpose: K3 current source/delivery evidence.
Depends on: original main raw SHA256=978dd282830ff317d27faad44553067f6c75abf25a5969c2bfd91dc3201f903f.
Used by: integrator normal/calibration CI. -->
<!-- Purpose: narrow W5 K3 acceptance followup for safe failure CSV and confirmed coded refusal guidance.
Depends on: parentb20e1bcf, spec-only8850a2c2, unchanged genuine Go browser harness and planned root error/CSV normalization.
Used by: root GitHub normal and named calibration browser gates; static checks do not claim runtime acceptance. -->
# W5 K3 browser followup

- task_id: `a83bad3d-2b4d-4b4a-b598-85f9e4a99cea`; own subcanvas `codex-w5-u1-ui-sub-browser`; no new claim/delegation.
- Role/config: existing independent test_worker; configured gpt-6.1-sol/high, runtime UNKNOWN.
- Original browser checkpoint: `97264f53c7f88fa5862334374b80ad844c5a6096`. Authorized parent target: `b20e1bcf998b4e66450b9e18a957fe5c6fb8183c`; own merge base: `9a8cccef7cf5b1586aa7f4afa22d9604211a3894`.
- Spec-only commit: `8850a2c26e5ac24654f4e8cc55b219f675a17ee2`, `unit/w5-u1-browser-worker`, clean.
- Changed path ONLY: `tests/admin/import-wizard.spec.ts` (28 inserts/7 removals). Go harness untouched; no apps/shared/lock/dependency changes.
- Summary: local failed CSV expectation now exactly row/outcome/code, with unchanged failure codes and explicit three cells/failed outcome. Also corrected older server raw4-column expectations to the existing real BFF's safe3-column projection: no external_id column at all, exact failed row numbers/outcomes/codes; original BOM/filename/raw-cell privacy assertions retained. This is the same intent, with stricter privacy than an empty ID column.
- Big5 and oversize uploads are explicitly LOCAL header/size guidance: assert appropriate copy, no Go request and no UNKNOWN retry. Existing usable-mapping5001-line valid CSV under2MiB now proves actual Go POST preview422/SHA, visible `c.errors.too_many_rows`, no mutation/UNKNOWN. Existing changed-map409 now proves actual Go409/visible `c.idempotencyConflict`/no mutation/UNKNOWN. No fake successful/error response or new harness. Fifteen existing real-click cases remain (12 declarations, one expands to4 locales/viewports); no new terminal mismatch fault requested or implemented.

## Actual commands / exit codes

| Command | Exit | Evidence |
| --- | --- | --- |
| `git merge --no-edit unit/w5-u1-import-wizard-ui` | 0 | Authorized merge9a8cccef; no conflicts/reverts. This is own validation branch, no release merge/push. |
| `node --check --experimental-strip-types tests/admin/import-wizard.spec.ts` | 0 | `syntax.log`; syntax only. |
| `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module esnext --moduleResolution bundler --esModuleInterop --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/import-wizard.spec.ts` | 0 | `spec-tsc.log` / `spec-tsc-status.json`, runw5-k3-browser-spec-tsc PID64095 FINISHED; outer120s process-group deadline; actual integrated repo libs, no overlay. |
| `git diff --cached --check` | 0 | Actual spec-only staged diff. |
| `git commit -m 'test: align import failure CSV and confirm coded refusal guidance'` | 0 | Commit above. |
| `git rev-parse HEAD`, `git status --short`, `shasum -a 256 <spec/harness>` | 0 | Clean branch, hashes below. |

Spec SHA-256: `7ad40ef290a5e15710929de77f716f094b2f60ae633bd44d0cc5f1b4fe978f80`.
Unchanged Go harness SHA-256: `1fca0a6876b9d42a7829a9be66c4fa40277119d973482b8ec38d48f0502da4aa`.

## Required GitHub gates (NOT_RUN here)

1. Normal `bash scripts/dev/test-local.sh --browser-migration-import`, actual signed HTTPS/Go/PG stack, all15 cases.
2. Same mode with `LC_MIUI_CALIBRATION=drop-customer-commit`: expected named RED `MIUI-RED-COMMIT-RECEIPT`, after ready genuine stack, then normal green.
3. Same mode with `LC_MIUI_CALIBRATION=truncate-preview`: expected named RED `MIUI-RED-PREVIEW-DATA`, after ready genuine stack, then normal green.

Calibration source/thresholds were untouched. Unrelated import/setup/login failures do not count as red. Root retains CLI/GATES/config and production normalization ownership. In mergedb20 the local helper still emits row/code/warning; this spec intentionally targets root's requested unification to row/outcome/code and must run with that production fix. Actual BFF server projection already uses row/outcome/code. Auth.ts was not changed.

Overall E1 static source followup. Browser/PG/Next/build/heavy/CI/runtime calibration NOT_RUN, independent runtime review pending. No new terminal mismatch case, no inherited helper red/green presented as this delta's browser proof. All local processes finished, own canvas records PID/log/deadline/matches/handoff; parent auto completion notification is wake mechanism. No unattended process, push/PR/recursive delegation or peer-file revert.

Evidence: `/Volumes/data/live_commerce_architecture_v1/output/w5-u1-import-wizard-ui/k3-browser/`.
