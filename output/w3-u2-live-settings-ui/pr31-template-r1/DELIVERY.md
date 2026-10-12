# PR31 template compatibility — comment4231878513

task_id: e0cab99c-sub-templates (parent e0cab99c-5ed3-4d22-a3cd-88700fb57c71)
base_commit: 539f1cb8138975e27ef75cdaf0563f154117c698
branch: codex/pr31-template-compat
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/pr31-template-compat
actual model / reasoning: UNKNOWN; inherited parent configuration is not exposed to this subagent.

## Implementation

Only product path `apps/admin/lib/live-settings-model.ts`: align POST validation and GET projection with frozen Go `internal/msgtemplates/publish.go:59` 120-Unicode-rune name contract. Use existing Array.from count, including astral emoji. Remove unsupported template row-count rejection entirely; the existing proxy 1 MiB upstream response byte limit remains unchanged, as do all DTO fields, item validations, authentication, permissions, CSRF and command receipts. Other resources keep their original list caps.

Only test path `tests/admin/live-settings-bff.test.ts`: eight regressions run through actual Request objects, the real exact Next message-templates handler, signed-session/store authority and MOCK Go HTTP responses. This is a BFF regression receipt, not UI click acceptance.

## Evidence and actual commands

- RED: `node --test --experimental-strip-types --test-name-pattern=template compatibility tests/admin/live-settings-bff.test.ts`, exit 1; six intended positive compatibility failures and two protection passes. `red.log`, `red-sha256.txt`.
- Initial RED retained in `red-initial-envelope-expectation.log`: a new test assumed an error envelope with only code; corrected to existing localError envelope before final RED. No existing test/assertion changed or weakened.
- GREEN: `node --test --experimental-strip-types tests/admin/live-settings-bff.test.ts tests/admin/live-settings-model.test.ts`, exit 0; 18/18, no skips. `green.log`, `green-sha256.txt`.
- Initial admin typecheck: `node node_modules/typescript/bin/tsc --noEmit --incremental false -p apps/admin/tsconfig.json`, exit 1; root cached admin node_modules lacks @live-commerce/ui and @live-commerce/format. `typecheck-initial-cache.log` preserves errors.
- Correct parent-cache admin typecheck: same command, exit 0 after own cache symlinks pointed to `/Volumes/data/live_commerce_architecture_v1/.worktrees/w3-u2-live-settings-ui/{node_modules,apps/admin/node_modules}`. `typecheck.log`.
- Scoped model typecheck: `node node_modules/typescript/bin/tsc --noEmit --strict --target ES2023 --module NodeNext --moduleResolution NodeNext --skipLibCheck apps/admin/lib/live-settings-model.ts`, exit 0. `model-typecheck.log`.
- `git diff --check`, exit 0 for product/tests before staging. `diff-check.log`.
- Initial staged evidence check reported trailing spaces in Node assertion output (exit 2); `diff-check-initial-evidence.log` retains that finding. Plaintext logs normalize trailing whitespace only; no test, assertion or result changed. Final staged check exit 0 after normalization.

GREEN asserts 120 BMP/astral names for GET and POST; 501 and 5001 legal items all remain readable below byte bound; 121 names fail GET503 and POST422 (no forwarding); >1 MiB response and wrong item shape fail503. Metadata/source hashes bind evidence to tested bytes. Synthetic fixtures only; no provider/real PII/keys.

## Boundary and cleanup

Evidence level E3 in the tested MOCK BFF environment only. Independent review, parent full Node/gates/browser, Go/REAL_PG/SANDBOX/LIVE/production and push: NOT_RUN by this card's parent-only scope. No P0/P1 conclusions outside this comment. Admin-wide typecheck passed after authorized parent-cache reuse; initial cache failure is retained. Node HTTP server closes in test.after; no background processes, DB fixture, container or port left. Cache reuse symlinks only, no installation/dependency/lock changes. Humaux fix stored as dc32c120-ab96-4b2e-9177-d80de4d5d9b0; parent will independently inspect and cherry-pick.
