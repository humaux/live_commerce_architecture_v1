<!-- Purpose: preserve current W6 P2 local evidence and narrow review.
Depends on: commands in DELIVERY and original main raw SHA256=0e29ce15d36ec3759f86f79a5077ebaf818510d1d82a6f24e25b97a71f3b734c.
Used by: integrator normal/calibration retry. -->
<!-- Purpose: independent narrow W6 P2 actual-route test source handoff and SHA-bound red/green evidence.
Depends on: test-only commitfe6794a8, actual auth/backend/routes, root-source.patch and installed Next runtime modules.
Used by: root integration and source reviewer; no production/Next/PG/browser acceptance claimed. -->
# W6 P2 independent tests

- task_id: `e052872b-32cf-4a78-bbb4-b8d1e106023f`, child `codex-w6-u1-ui-sub-p2-tests`; no separate claim/delegation.
- Role/config: existing test_worker, configured `gpt6.1/high` per task card; runtime UNKNOWN.
- Base: `00979c2779bcdca384cc8cf31fd8e0678d5a748e`.
- Test-only commit: `fe6794a808a65b5c6106acda9ce135e9a120a546`, branch `unit/w6-u1-p2-tests`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w6-u1-p2-tests`, clean after commit.
- Paths: `tests/admin/w6-route-seam.test.mjs` (52 lines), `tests/admin/w6-real-route-loader.mjs` (71), `tests/admin/w6-integration.test.ts` (only prior fake-dispatch block plus header/import).
- Summary: actual native Request → actual leaf/catch-all → real auth.ts cookie/Origin/CSRF/authenticatedStores/backend/response projection. Only upstream fetch is fake. Reuses registerCustomerTagsLoader; maps aliases and actual installed Next compiled server-only/empty.js and headers.js; standard AsyncLocalStorage is runtime bootstrap only. No handwritten auth/CSRF/localError functions. All five old query cases remain; valid tag/legacy empty/q requests prove forwarding, bare/encoded keys422 before any backend, positive JSON/CSV prove VaryCookie/private/no-store/nosniff and no cookie/CSRF forwarding, malformed queries and real CSRF failure prove no upstream work.

## Actual red and green

Exact command in own worktree:

```sh
node --test --experimental-strip-types tests/admin/w6-route-seam.test.mjs tests/admin/w6-integration.test.ts
```

- Baseline RED: exit1, PASS6/FAIL3/SKIP0; `red.log` / `red-status.json`, runw6-p2-node-red PID89202 finished. Intended assertions: `P2-BARE-TAG-422 ?tag` actual405/expected422, `P2-REPORT-VARY-COOKIE products` and `.csv` actualnull/expectedCookie. No dependency/loader/setup failure counted.
- Root then supplied `root-source.patch`; own temporary validation source hashes matched root exactly: customer `9cc4be843615235d214db9fa2f332a6889022cdb01684041eb6a454d34348474`, report `083268654682222a7ff0c30dab5f3d5c8adb03d13f37400e05d6cf7ad8046134`.
- Candidate GREEN: exit0, PASS9/FAIL0/SKIP0; `green.log` / `green-status.json`, runw6-p2-node-green PID89879 finished. Both Node runs have outer120s process-group timeout and recorded command/PID/start/end/log/exit code. Child monitored completion, then sent root named red before root source edits and named green afterward. No unattended run remains.
- Baseline route hashes: customer `7d4b282eb1610366b7b79d326ba4507369f2ec79773f9502fe027edc82aacc88`; report `ce6477857a2d81e4d5d4958c51019e2d34ecc29036679b2aada54aa6238dc124`.
- `checks.json` records exact base/candidate route and test hashes/commands/counts.

Other actual commands: `git worktree add -b unit/w6-u1-p2-tests <own-path> <base>` exit0; `pnpm install --offline --frozen-lockfile` exit0 (install session90727 monitored, main `p2-install.log`, no lock/dependency version change); both new MJS `node --check` exit0; `git apply --check <root-source.patch>` exit0; `git apply <root-source.patch>` exit0; staged diff check exit0; commit exit0; final git status exit0/clean. Git branch-existence preflight returned nonexisting ref as expected; no reset/overwrite of another branch.

After candidate green, restored ONLY the two owned temporary route copies to base using exact path-scoped git restore. These source paths were never staged/committed; no root/peer edits were reverted. Consequently this isolated test branch intentionally remains RED without root's two source fixes. Root must apply the test commit together with its source fix, then independently rerun the exact command and register test-node/GATES as appropriate.

## Evidence level and limits

E3/MOCK native Node route boundary red→green binds exact candidate file hashes. Test-only source commit is E1 until integrated with root source fix and independently rerun. PG, browser, production Next, Go, builds, heavy CI gates NOT_RUN. Root's focused PG seed pass is separate parent evidence, not this test worker's run. Source-only reviewer is `/root/w1_ci_diagnosis`; review pending at this handoff.

No source/lockfile/dependency additions, push, PR, release merge, recursive agents, PG/Next/browser server, or production operation. No failed samples deleted or thresholds weakened. The remaining source-fragment renderer tests were preserved untouched; the only authority stand-ins removed were from the targeted old fake dispatch test.

Humaux research persisted/read back as `6fc7b47c-6323-4161-af3f-f1af94c6545a`; own subcanvas updated and locks released. Incremental code_index accepted all3 paths, but later shared index status showed1 file/10 entities and both MJS helper code_memory_link attempts returned linked=false/entity not found. MJS helper indexing/links are UNKNOWN/NOT_LINKED, not claimed complete. This does not change the actual Node test evidence.

Evidence directory: `/Volumes/data/live_commerce_architecture_v1/output/w6-u1-customers-reports-ui/p2/`.
