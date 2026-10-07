<!-- Purpose: shared-loader LIVE pair follow-up receipt. Depends on: lib.sh, actual Compose rendering and current hash-bound tests. Used by: integrator's combined CI/review. -->
# P2 shared-loader follow-up

- Branch: `unit/deploy-prep-r3-p2`; base `121789dcb72df434c05e9c0726749df6aa2c2aa4`; commit = this delivery commit. Codex-4 implementer (runtime variant/effort not exposed); read-only reviewer configured gpt-6.1-sol/high. No parallel source writers.
- Root cause: Docker Compose prefers exported LC_STRIPE_LIVE_* over env-file values. The shared loader previously retained caller values, so ops CLI hardening alone did not protect API/payment-worker interpolation.
- Fix: `lc_load_env` unsets both names on every load before file parsing. Readonly state that cannot be cleared fails with a named error. Other knobs keep caller-over-file precedence. Only source changes are lib.sh and the existing R3 Node controls; no API/SQL/migration/dependency change.
- Actual Compose red: caller approval + file zero rendered API LIVE=1, expected0 (`red.log`, exit1). Green includes zero/missing file pair→API/worker0+blank ref, actual file pair overrides caller0/ref, ordinary IMAGE_TAG override retained, readonly caller failure. Independent reviewer also checked repeated loads and both readonly fields; no confirmed P0/P1 (`independent-review.log`).

| Command | Exit |
| --- | ---: |
| `node --test --test-name-pattern='shared loader' tests/deploy/deploy-prep-r3.test.mjs` before fix | 1 expected |
| `node --test tests/deploy/deploy-prep-r3.test.mjs tests/deploy/meta-connect-preflight.test.mjs tests/deploy/ops-alert-preflight.test.mjs` | 0 (46/46) |
| official temporary ShellCheck0.10.0: `shellcheck -S warning -x deploy/scripts/*.sh deploy/postgres/*.sh deploy/postgres/ops/*.sh` | 0 |
| `PATH=<temporary-shellcheck-bin>:$PATH bash deploy/scripts/smoke.sh static` | 0 (S01–S06/S48 PASS) |
| `python3 .github/scripts/smoke-verdict.py <current-static-result.json> 0 static` | 0 |
| `bash scripts/dev/check-gates.sh` | 0 |
| `LC_HEADERS_STRICT=1 bash scripts/dev/check-headers.sh 121789dc` | 0 |
| `git diff --check` | 0 |

E3 DB-free/MOCK/static, current files in EVIDENCE.sha256. Temporary official lint tool/fixtures removed; failure evidence retained.
CI needed: **gates unit + all foundation-shards, deploy-smoke static + full** on the combined follow-up SHA. Full PG/browser/production/provider execution remains NOT_RUN. Commit only; no push, SSH, live keys, production host, mail or deploy action.
