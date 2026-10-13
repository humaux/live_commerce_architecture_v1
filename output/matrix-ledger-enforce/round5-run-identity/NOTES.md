# PR28 R5 run identity scoped delivery

task_id: 27efd779-sub-run-identity (parent 27efd779-1a3c-4559-839c-89fddd8aa01c)
base_commit: 760e354e8d466e2e5876d1fb5d433d74e3d10d2e
owner: codex4-pr28-r5-run-identity
role: scoped Go test implementation
model: inherited parent; exact model/effort runtime not separately exposed (UNKNOWN)
worktree: .worktrees/pr28-r5-run-identity (detached)
write_paths: tests/foundation/catalog_matrix_evidence_test.go and output/matrix-ledger-enforce/round5-run-identity/

E3 focused automated fixture scope; E1 source provenance. Existing validator accepted arbitrary successful runs with equal counts. Added exact successful consumer file/title multisets: PE 1, review 9, feedback 2; frozen CC12 desktop en and mobile zh-TW. Original final receipts/counts/hash validation remain. Expected-int signature unchanged. No extra Go source filename or required producer path added; owned Go source already hard pinned. SOURCE-PINS.json records read-only consumer/title provenance; parent decides any additional hard producer pins in its owned validator.

red.receipt.json records actual go test command and exit 1 on unchanged summary validator: all 12 rehashed identity mutations leaked. red-source.go.txt preserves the exact red file hash. green.receipt.json records actual focused command and exit 0: 6 hash controls + 10 shared controls + 17 run outcome controls + 12 identities + variable location/duration positive test. Uncompressed logs retained. Each identity corruption updates native receipt hash, canonical manifest hash and ARTIFACTS index together and explicitly refuses hash-only failure.

NOT_RUN: canonical evidence gate on newly changed source, actual browser/PG/full Node/CI/independent acceptance; source changes intentionally stale old canonical proof until parent rerun. No commit, push, GitHub or canonical DELIVERY/index/source edits. No started service or persistent fixture remains (Go t.TempDir cleanup only).

Parent apply implementation.patch, independently rerun focused tests after integrating current native ledger changes, and regenerate canonical run/evidence receipts. Full gate readiness is not claimed.
