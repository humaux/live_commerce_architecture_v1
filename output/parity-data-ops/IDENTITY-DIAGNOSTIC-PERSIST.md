# PR475 identity diagnostic persistence

- task_id: `9151d54a-379c-4d12-b83c-a93149437868` (root claimed; no child claim).
- base_commit: `5d9701ab4a95f50d0c4e41b5e441525ea8a8658b`.
- Role: delegated tests-only author; requested `gpt-6.1-sol` / medium; actual model ID UNKNOWN.
- Worktree: `.worktrees/codex-pr475-inventory-repair`; allowed/changed paths: `tests/admin/settings-v2.spec.ts`, this report only.
- Evidence level: E1 / SOURCE structure ready. Dynamic persistence, browser, Go/PG, SANDBOX, LIVE, CI and independent acceptance NOT_RUN here. No push.

The existing failure capture serialized a bounded whitelist of observations into an in-memory JSON attachment. The installed Playwright 1.63.0 path handling and list reporter were independently inspected in the source report below; an in-memory non-text attachment has no file for CI upload. The catch now writes the exact existing `JSON.stringify(observations)` to `testInfo.outputPath("market-selection-diagnostic.json")` using Node's `writeFile`, then attaches that path as `application/json`. Its original diagnostic try/catch and `throw error` remain, so an I/O or attachment failure cannot replace the original click failure. No new diagnostic fields or sensitive values were added.

All bytes outside the one new import and the attachment catch replacement are identical to base. That includes observations, capture fields, GET aliases and cap, cases, fixture, clicks, waits, timeouts, blocked/zero-write checks and command-journal settlement. All 276 `expect` / `expect.poll` occurrences are retained. This instruments failure evidence; the historical disabled Save cause and failed loop case remain UNKNOWN. It does not repair or accept the product.

Sources read in full:

- `output/integrator/tools/logs/ci-evidence-1238/475-identity-source-0922.md`, SHA256 `7708eb99955b3f52703035ccfb892abf1a7d411d77496a44c966c6b2aa3ceecc` (main repository; SOURCE reviewer report, rooted in installed Playwright worker/list source and archived original failure).
- `output/integrator/tools/logs/ci-evidence-1238/475-identity-failure-0917/report.md`, original run `41415715766`, job `125180405176`, checkout `6da29fbd82c57634e2188e725100cbcfcaaf3115`; original click timeout and missing JSON preserved.
- Repository `AGENTS.md`, `架构.md` §§0/1/2/26/27, identity task/DAG in `contracts/tasks.json`, and `contracts/invariants.json`; relevant I11 privacy, I18 evidence, I21 author/acceptance separation remain unchanged.

Actual commands and receipts (all exit 0):

1. `git rev-parse HEAD`, `git status --short`: expected base; tracked clean before edit, exactly five old untracked inventory-diagnosis evidence files, all KEEP.
2. `shasum -a 256 /Volumes/data/live_commerce_architecture_v1/output/integrator/tools/logs/ci-evidence-1238/475-identity-source-0922.md`: exact hash above. `cat`, `rg`, and `sed` targeted source/contract/report reads succeeded.
3. `git diff --check`: no whitespace errors.
4. `python3 - <<'PY'` inline source check: load base via `subprocess.check_output(['git','show','5d9701ab4a95f50d0c4e41b5e441525ea8a8658b:tests/admin/settings-v2.spec.ts'])`; construct expected bytes with exactly the import insertion and catch replacement; assert current bytes equal expected and `re.findall(rb'\bexpect(?:\.poll)?\(', current) == re.findall(rb'\bexpect(?:\.poll)?\(', base)`. Printed `SOURCE exact import/catch delta PASS; all other bytes and assertion occurrences unchanged`, assertion occurrences `276`.
5. `git log -1 --format='%B'`, `git diff --stat`, `git diff -- tests/admin/settings-v2.spec.ts`: inspected exact 7 additions/1 deletion in the test.

Test SHA256 before: `92bf38ba53a78d6a0826f3d2b0cb55b8ac36051a4b6e291250721ce46f2a6cbf`; after: `5e7370fe8f8c30540f4516e888c6e92a010ca60f717b1ff297adc6440a571ac3`. This static check is not a new acceptance gate and does not prove runtime artifact persistence. Parent owns independent diff review, prepush, standard CI and archived JSON evidence. No background process, fixture, container, port or temporary file was created by this task; original five untracked evidence files remain untouched.

Unresolved: dynamic artifact presence/parse/whitelist verification NOT_RUN; disabled-button root cause UNKNOWN; independent acceptance NOT_RUN. Do not infer PR475 acceptance from this instrumentation.
