# W5-U1 historical orders child delivery

- task_id: `a83bad3d-2b4d-4b4a-b598-85f9e4a99cea`; own subcanvas `codex-w5-u1-ui-sub-history`.
- base_commit: `dc11b5ce8ceaf3c755d7192545f34f3c579108cf` (owner-approved W6 checkpoint; stale DRAFT/merge-wait brief overridden by parent task).
- Branch/commit: `unit/w5-u1-history-worker` / `ec45d6499db2d9ac2ac1693fccb32fc5e7db3692`; clean WT, no push/merge.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/w5-u1-history-worker`.
- Role: ui_worker; configured model `gpt-6.1-sol`, reasoning `high`; runtime identifier UNKNOWN.
- Changed paths: 11 owned new files in `paths.txt`; current file hashes `sha256.txt`, SHA `commit.txt`.

## Implementation / frozen interface

`CustomerHistoricalOrders({ locale: Locale, store: string, customer: string, boundary: string })` remains the frozen public interface. Root owns CustomerDetail active/imported gating and mounts this component; child does not edit root components/registry/scripts or another worker's import model/client/copy.

Exact GET BFF leaf uses the actual Go W5-03B limit/after grammar (not the stale brief's cursor name), authenticated server session/store scope and a strict safe projection. Rows decode only order_id/ordered_at/status/total_minor/currency/items_summary/city; canonical Taiwan22 city or null, TWD, 2000-row customer cap and 50-row UI page cap. Unknown keys/address/phone/email fields are refused, malformed or thrown backend reads become unavailable, error bodies are sanitized and responses private no-store/Vary Cookie.

The read-only section shows original external SHOPLINE order numbers as plain text, shared exact minor-unit money and Asia/Taipei time formatting, server visible-row/total count, original status, item summary and city. All3locales state history is excluded from revenue/reports. No sum/revenue calculation, live-order link, refund/shipment action, write request or history storage/logging. Existing TableFrame plus constrained overflow CSS supports narrow screens; real390layout NOT_RUN.

Prev/Next keep only scoped opaque cursor trail; double click cannot skip to an unseen cursor. Refresh resets page1, store/customer/boundary changes remount the private state, and inherited guarded-read plus explicit parent session fences reject stale reads. Empty archive differs from an empty page with a nonzero server total. Forbidden/signed-out/not-found/unavailable/loading remain honest distinct states.

## Actual commands and exit codes

Commands ran in the child worktree; logs in this main-checkout evidence directory, bound to commit/hash files.

| Command | Exit | Evidence |
| --- | --- | --- |
| `node --test --experimental-strip-types --test-reporter=tap tests/admin/import-history-model.test.ts` after deliberately relaxing city validation | 1 | `red.log`: 4 PASS / 1 FAIL; `model-before-mutation.ts` restored snapshot |
| `node --test --experimental-strip-types --test-reporter=tap tests/admin/import-history-client.test.mjs` after deliberately removing parent preflight fence | 1 | `red-client.log`: 2 PASS / 1 FAIL; `client-before-mutation.ts` restored snapshot |
| `node --test --experimental-strip-types --test-reporter=tap tests/admin/import-history-ui.test.mjs` after deliberately allowing double-click page increment | 1 | `red-component.log`: 2 PASS / 1 FAIL; `component-before-mutation.tsx` restored snapshot |
| `node --test --experimental-strip-types --test-reporter=tap tests/admin/import-history-bff.test.mjs` before thrown-upstream guard | 1 | `red-bff.log`: 4 PASS / 1 FAIL |
| `node --test --experimental-strip-types --test-reporter=tap tests/admin/import-history-model.test.ts tests/admin/import-history-client.test.mjs tests/admin/import-history-bff.test.mjs tests/admin/import-history-ui.test.mjs` | 0 | `green.log`: 16 PASS / 0 FAIL |
| `node_modules/.bin/tsc --noEmit --incremental false -p apps/admin/tsconfig.json` | 0 | `typecheck.log` |
| `bash scripts/dev/check-headers.sh dc11b5ce8ceaf3c755d7192545f34f3c579108cf` after staging all files | 0 | `headers.log` |
| `git diff --cached --check` | 0 | no findings |
| `git commit -m 'feat(admin): show guarded read-only customer historical orders'` | 0 | `commit.log` |

No missing import/module failure was counted as acceptance red. Controlled mutations break actual privacy/fence/navigation properties, are restored byte-for-byte before green, and tests/thresholds were not weakened. UI tests render actual component/shared TableFrame and formatter code in React SSR with a mock guarded read, and invoke actual component navigation callbacks with controlled hooks. These are MOCK local tests, not browser click acceptance.

## NOT_RUN / unresolved / parent integration

- Real browser click ledger/persistence, 390px actual responsive table, --browser-migration-import, --browser-customers-billing and click sweep: NOT_RUN. Expected controls recorded in `CLICK-LEDGER.md`, actual all NOT_RUN.
- PostgreSQL/foundation, Next build/full router construction, full check-gates/test-local and final independent review: NOT_RUN by explicit parent local restriction.
- Exact BFF leaf DB-free handler/auth-adapter tests run; real Next middleware/session integration remains parent acceptance.
- Root must independently re-run/review, wire CustomerDetail active/imported gating and register four focused tests in shared test-node script. Source interfaces/schema/lockfiles unchanged.
- Runtime model identifier UNKNOWN; configured assignment recorded above.
- Humaux fix/subcanvas/code linkage recorded in `HUMAUX.json` after persisted tool readback.

No processes, ports, containers or new fixture directories started. Synthetic data only. Retain red failure snapshots/evidence; no other-owner edits or cleanup.
