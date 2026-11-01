# PARITY-DOMAINS FIX2 (PR #407, run 41210850297 @ a333244ba)

| Failing job | Failing test | Evidence | Class |
|---|---|---|---|
| gate (bundle:07) | none. All 6 members `success`, receipts PASS (cvs 37, customers-billing 188, identity 6, migration-import 57, returns-ui 19, manual-order 27). Job red at step "Bundle member 3 evidence" | `##[error]Failed to FinalizeArtifact: Unable to make request: ECONNRESET` (job 124566968606, 14:57:55Z) while uploading artifact `gate-browser-identity` | (c) GitHub Actions artifact-upload infra flake; no product or test code involved |
| required | n/a | `gate=failure` only because of bundle:07 (job 124572923533) | derived |
| aggregate / receipts / all other gates | pass | `gh pr checks 407` | n/a |

Fix: no code change; the new push reruns the gates. Merged origin/r3/integration cleanly; `derive-browser-ownership.mjs` and `--check` produce no diff (54 modes).
