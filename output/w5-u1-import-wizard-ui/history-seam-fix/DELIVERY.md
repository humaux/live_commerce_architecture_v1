<!-- Purpose: preserve actual history route/client shared-helper correction evidence.
Depends on: sourceafefed5e and original main raw SHA256=641cf8ff1ca970c9d3acc1f354de4333a5f29ab60000d68a9065766daf11b1b5.
Used by: integrator source review; browser rendering acceptance pending. -->
# W5 historical-order real-seam test correction

- task_id: `a83bad3d-2b4d-4b4a-b598-85f9e4a99cea`; child `codex-w5-u1-ui-sub-history`.
- Patch base (incoming root): `ef75877f8f25010ac21058f795ec3cd560d9a899`.
- Worktree source after authorized merge: `a1fa21351f2f7817729978e50aae661160f8a34a`; merged with `git merge --no-edit unit/w5-u1-import-wizard-ui` (initial exit1 add/add CSS conflict, resolved byte-for-byte to MERGE_HEAD's root tokenized CSS; `git commit --no-edit` exit0, `merge.log`). No CSS delta against incoming root.
- OWN correction commit: `afefed5ebcc11af0ba920dce3fd6a7004eb90217`, branch `unit/w5-u1-history-worker`; parent applies this test commit, not the incoming merge.
- Role: ui_worker, configured gpt-6.1-sol/high; actual runtime identifier UNKNOWN.
- Own delta exactly three tests: import-history-bff.test.mjs, import-history-client.test.mjs, import-history-test-loader.mjs. `paths.txt`/`sha256.txt` bind own files. No product source, CSS, shared helper, docs/gates/runner edits.

New PROCESS requirement makes hand-written shared auth/localError/cookie stand-ins a P1. Rewrote old VM-backed BFF test and direct fake-BFF client responses to import actual GET route/auth/backend/settings/model source and use native Request/Response. The client routes its browser-relative fetch in-process through the actual GET; only upstream network responses are synthetic. Real setSessionCookies issues host session/CSRF cookies; real sessionToken/exact-cookie parsing, CSRF/Origin comparison, clearAuthCookies, authenticatedStores, readBody, safeError/localError all execute unchanged.

Frozen dependency `registerImportWireLoader()` is reports-owned commit `059855c3`; SHA256 `145439face274a8271a7f5c4759f345642f5eb073cb2e863c4a864df228cfd94`. It resolves the installed Next react-server empty marker and @ aliases, delegates to the actual TypeScript loader, and substitutes no helper. Own loader adds only extension resolution next/headers→actual installed next/headers.js. The initial module-load failure is setup NOT_RUN, never acceptance RED.

Real error401/403/404/503 Cache-Control **no-store** (not fake private,no-store) retains the correct history ReadError category and real envelope/request_id/retryable/cookie expiration. Successful archive projection remains private,no-store/Vary Cookie. Tests cover canonical GET/query/token ownership, duplicate/missing session cookies, cross-store server denial, unsafe/oversize/MIME rejection, diagnostic redaction, preflight and inflight parent session fencing, and no auto retry. No product defect found in history source; source remains unchanged.

## Actual commands and exit codes

| Command | Exit | Evidence |
| --- | --- | --- |
| `node --test --experimental-strip-types --test-reporter=tap tests/admin/import-history-bff.test.mjs tests/admin/import-history-client.test.mjs` initial Node entry resolution | 1 | `green-initial.log`: module-load/setup failure, NOT acceptance red |
| Same command after actual next/headers.js resolution | 0 | `green-real.log`: 12 PASS / 0 FAIL |
| `HISTORY_SEAM_INJECT_CITY=1 node --test --experimental-strip-types --test-reporter=tap tests/admin/import-history-bff.test.mjs tests/admin/import-history-client.test.mjs` | 1 | `red.log`: 10 PASS / 2 FAIL; mutation only at synthetic upstream network data |
| `HISTORY_SEAM_INJECT_CITY=0 node --test --experimental-strip-types --test-reporter=tap tests/admin/import-history-bff.test.mjs tests/admin/import-history-client.test.mjs` | 0 | `green.log`: 12 PASS / 0 FAIL |
| `node_modules/.bin/tsc --noEmit --incremental false -p apps/admin/tsconfig.json` | 0 | `typecheck.log` |
| `bash scripts/dev/check-headers.sh ef75877f8f25010ac21058f795ec3cd560d9a899` | 0 | `headers.log` |
| `git diff --cached --check` | 0 | no findings |
| `git commit -m 'test(admin): drive history client and BFF through real auth seams'` | 0 | `commit.log` |

Source evidence: own commit/hash files plus `real-source-sha256.txt` for actual auth/backend/client/route and consumed real loader. Mutation taints city/extra address in otherwise successful upstream data: the real route rejects it and unchanged success assertions become red. Green restores valid network data; no production/helper stubbing or threshold weakening.

## Limits / cleanup / parent work

MOCK actual-code Node seams, not browser/PG or live identity. Browser/Next build/server/full gate/full old runners/PG/final independent acceptance NOT_RUN here. Parent has helper059855 applied first, then applies own afefed test delta and independently runs whole affected suites; reviewer performs read-only diff audit. UI VM tests and unrelated W6 modules were outside scope and untouched.

The reports-owned helper was copied byte-for-byte untracked solely for validation, hash compared to059855 then recycled. Retained evidence copy `real-loader.validation-copy.mjs`; no foreign path committed. To rerun this child branch, merge/provide reports helper059855 first; parent's integrated tree already has it. No processes/ports/containers started. Synthetic generated test credentials only; environment/global fetch/document fixtures restore in teardown. Old history evidence preserved.

Humaux fix `75d96d19-20eb-4f82-8756-d2053fcd424a` supersedes old history checkpoint; own subcanvas updated, readHistoricalOrders code-memory link confirmed, changed3MJS files submitted to index (MJS entity coverage UNKNOWN). Locks released. Parent reports afefed applied and independent49/fullNode/spec/gates exit0; child did not repeat or expand testing. See HUMAUX.json. Evidence is already in the main checkout output, not an ephemeral worktree folder.
