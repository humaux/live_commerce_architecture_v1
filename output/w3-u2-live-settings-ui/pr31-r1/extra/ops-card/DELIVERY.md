# PR31 extraA OP4 navigation contract delivery

- task_id: e0cab99c-5ed3-4d22-a3cd-88700fb57c71-sub-ops (parent card; no separate claim).
- base_commit: 2c4d2f5320d9172e6f0094341bb81f7c9ccb1093.
- Commit: 366bc4a7b41a4a2912439d3d5672226182823178; `git commit -m 'test(admin): update OP4 exact live navigation contract'` exit 0.
- Branch/worktree: codex/pr31-ops-nav; /Volumes/data/live_commerce_architecture_v1/.worktrees/pr31-ops-nav.
- Role: focused test author. Configured model/reasoning: gpt-6.1-sol/medium per parent; actual model UNKNOWN.
- Owned changes: tests/admin/ops-polish.spec.ts; tests/admin/ops-polish-model.test.ts.
- Summary: add nav-live-settings immediately after nav-claims to the exact full navigation assertion and both live collapsed/visible inventory loops. Preserve all three locale callers, closed inventory eight, final catalog count eleven and single-open-group behavior. Add static TypeScript AST expansion of the actual consuming browser assertion against an independent four-route fixture; computed inventories fail closed. No production code edits.
- Evidence class: MODEL_ONLY/static consumer contract. Browser runtime NOT_RUN; parent will independently run --browser-ops-polish twice after integration. No build, Go or full suite run.
- Humaux search/store/subcanvas: UNKNOWN; tool discovery exposed no Humaux tools. Shared state untouched. Intended fix record: root cause stale OP4 expected list after authorized fourth live route; correction maintains exact assertions and inventory toggles. No memory_id available.
- Dependency reuse: own ignored symlinks to parent W3-U2 root/admin node_modules; no dependency install or lockfile change. First dependency diagnostic used the primary cache missing typescript-api; dependency-failure.log exit 1 is not acceptance RED. Intermediate cache w4-s1-platform-stripe supplied installed TypeScript 6.0.3 for RED/initial GREEN; final W3-U2 cache rerun green.

## Actual commands and exit codes

Run from the isolated worktree above. stdout/stderr saved uncompressed.

1. `git worktree add -b codex/pr31-ops-nav .worktrees/pr31-ops-nav 2c4d2f5320d9172e6f0094341bb81f7c9ccb1093` (from main repository): exit 0.
2. `node --test --experimental-strip-types tests/admin/ops-polish-model.test.ts`: initial dependency diagnostic exit 1; dependency-failure.log.
3. `node --test --experimental-strip-types tests/admin/ops-polish-model.test.ts`: valid RED exit 1, 5 pass/1 fail with actual old browser consumer missing nav-live-settings; red.log and red-source-hashes.txt. Original consumer preserved in red-ops-polish.spec.ts.
4. `node --test --experimental-strip-types tests/admin/ops-polish-model.test.ts`: GREEN exit 0, 6 pass/0 fail; green.log.
5. `node_modules/.bin/tsc --noEmit --strict --target ES2023 --module ESNext --moduleResolution Bundler --allowImportingTsExtensions --skipLibCheck --typeRoots apps/admin/node_modules/@types tests/admin/ops-polish-model.test.ts tests/admin/ops-polish.spec.ts`: exit 1, missing explicit Node types; typecheck-missing-types.log retained.
6. `node_modules/.bin/tsc --noEmit --strict --target ES2023 --module ESNext --moduleResolution Bundler --allowImportingTsExtensions --skipLibCheck --types node --typeRoots apps/admin/node_modules/@types tests/admin/ops-polish-model.test.ts tests/admin/ops-polish.spec.ts`: exit 0; typecheck.log.
7. `node --test --experimental-strip-types tests/admin/ops-polish-model.test.ts`: exit 0 using final parent cache, 6 pass/0 fail; green-main-cache.log.
8. `git diff --check`: exit 0.
9. `shasum -a 256 tests/admin/ops-polish.spec.ts tests/admin/ops-polish-model.test.ts`: exit 0; green-source-hashes.txt.

## Unresolved / NOT_RUN

- Independent review and actual BROWSER (MOCK) acceptance: NOT_RUN by this author; parent owns reruns and review.
- Humaux fix store/subcanvas: UNKNOWN, unavailable capability recorded above.
- No assertion of full PR completion or runtime UI correctness from this static seam.
