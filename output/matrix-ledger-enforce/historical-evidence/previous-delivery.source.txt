# matrix-ledger-enforce — PR28 final execution-filter binding

- Branch: `unit/matrix-ledger-enforce`; remote base `f1f404a15d15f08d01269e1ec06a07df9de8d492`.
- `git fetch origin && git merge origin/unit/matrix-ledger-enforce` → exit 0, already up to date. No new trunk merge in this filter-only round; merged trunk ancestor remains `fef7920378c0b8cde843d22b27d33c62b07451b7`.
- Tested source: **`e0fb76ce4034721a5429f701469a3e9fd4b756e7`**, [SOURCE.json](filter-bindings/SOURCE.json). Fourteen unit executable hashes are checkout-bound, including both changed execution files; twelve shared context hashes are informational. The evidence-only handoff changes no frozen executable bytes.
- Author: Codex-4 `/root`; parent model/effort not exposed. Scoped reviewer `/root/filter_binding_audit`, `explorer`, `gpt-6.1-sol`, medium, read-only at the tested SHA; E1 PASS, no recursive delegation or writes. Humaux task `d5aa1878-b297-4184-b95d-420fece44aa4`.
- Scope: only threads **4251056896** and **4251056902**. No product, route, contract, migration, dependency, browser wait/assertion, suite-filter or calibration change. Execution/content binding and all prior outcome/identity/expected-claim checks remain.

## Red → green

Required Node suite commands now refuse either `--test-name-pattern` or `--test-skip-pattern`, including space, equals and quoted forms. Go command argv is inspected after verifying/removing its three genuine planner forwarding tokens; additional `-run` or `-skip` is refused in both space and equals forms. Literal argv is parsed without shell evaluation. An unrelated filtered Node command remains permitted; the original actual registry commands remain positive.

The real command mutations retain all required suite filenames and Go forwarding. [Unchanged old module RED](filter-bindings/red.json): `node --test tests/ci/product-editor-matrix-execution.test.mjs` exits **1**, 12 failed / 9 passed. [Final GREEN](filter-bindings/green.json): same command exits **0**, 21 passed / zero failed. Six original tests are retained. [Uncompressed proof and preparation history](filter-bindings/NOTES.md) explains the initial replacement-string fixture error, corrected callback mutation, exact-append assertion and genuine rerun; initial logs do not substitute for RED evidence.

## Current actual gates

| Exact command | Exit | Current evidence |
|---|---:|---|
| `node --test tests/ci/product-editor-matrix-execution.test.mjs` | 0 | [21 controls](filter-bindings/green.json) |
| `bash scripts/dev/test-node.sh` | 0 | [full Node receipt](filter-bindings/test-node.json) |
| `bash scripts/dev/check-gates.sh` | 0 | [first complete receipt](filter-bindings/check-gates.json) and [final source-identical retry](filter-bindings/check-gates-retry.json); execution binding, contract drift and documented/run mode registry |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | [original full mode](filter-bindings/browser-product-editor.json) |
| `go test ./tests/foundation -run '^TestProductEditorMatrixLedger' -count=1 -v` | 0 | [canonical and corruption controls](filter-bindings/go-ledger.json), [final evidence-index validation](filter-bindings/go-ledger-final.json) |

The fresh original browser mode ran on the frozen source: **12** PE/review/feedback + **2** CC12, zero skips; both native receipts are passed with empty failedTests. Go confirms archived/replacement SKU persistence in isolated REAL_PG. Ledger: 322 raw rows, **267** unique required PASS identities, 89 per `zh-TW`/`zh-CN`/`en` at 390px. Its projected rows exactly match the unchanged golden fixture. [Canonical manifest](filter-bindings/browser/browser-evidence.json) records five byte-exact native copies, source hashes and counts; [ARTIFACTS.json](filter-bindings/ARTIFACTS.json) binds their bytes externally.

The first fresh attempt exited **1 before Playwright** because the unchanged 20-second foundation first-migration context timed out; browser cases were **NOT_RUN**. [Original failure](filter-bindings/browser-fixture-not-run.json), [log](filter-bindings/browser-fixture-not-run.log) and [scope note](filter-bindings/browser-fixture-not-run-note.json) are retained. The source-identical second original mode passes in full; no timeout or assertion was changed. Resource snapshots are observations, not a proven timeout cause.

Supplementary exact-staged `bash scripts/dev/check-gates.sh` exited **1** after 486.049s: its execution binding passed, but the unchanged contractdrift `TestReviewR1bSinkValuesRealCLI/spread` exceeded the existing CLI deadline. [Actual receipt](filter-bindings/check-gates-final.json) and [uncompressed log](filter-bindings/check-gates-final.log) are retained separately from the earlier complete exit-0 gate. No fixture, timeout, assertion or executable source was changed. The resource snapshot is observational, not proof of cause. The single bounded source-identical retry of the exact original command exited **0** after 411.037s, with all fourteen frozen executable hashes unchanged; [retry receipt](filter-bindings/check-gates-retry.json) and [log](filter-bindings/check-gates-retry.log) preserve that distinct result. No further retry was needed.

## Prior rounds and limits

[Previous round-5 DELIVERY](filter-bindings/previous-round5-delivery.source.txt) retains all round-4/5 fixes, exact commands, red injections, initial archive failure and scoped audit. [Historical round-5 source](round5/SOURCE.json) remains bound to `d93700a4ab0fb541aa8d3ee2d540c0d94b29f50f`; its native bytes were not rehashed to claim this new execution source. **filter-bindings is the designated current canonical directory.** Accepted FOLLOWUPS remain; the owner explicitly defers further flag spellings (`-test.run`/`--run`/`--skip`), per-option bulk coverage and more control types to FOLLOWUPS. This handoff implements none of them.

- Required CI: foundation-shards and original `--browser-product-editor`; new CI/final independent review **NOT_RUN locally**, retained for the integrator. Scoped E1 review is not K3/CI approval.
- MOCK identity + isolated REAL_PG/browser only; no provider, production, deployment, PR merge or thread resolution.
- No TypeScript changed; prior round's typecheck remains historical, not a new current-source typecheck claim. Optional media r04 remains NOT_RUN because its pinned binary is unset; required Node suites pass.
- Native logs retain tool-emitted bytes; source/docs diff checks exclude native receipt whitespace. Inert source snapshots do not use runnable `.spec`/`.test` filenames. Foreign `output/ext-agents/` is excluded from this handoff.

| Thread | One-line integrator reply |
|---|---|
| 4251056896 | Required matrix Node suite commands reject both test-name and test-skip filters in space/equals/quoted forms; retained-filename negatives are RED on the old gate and GREEN on the final gate, and the original command remains positive. |
| 4251056902 | Go selector checks parse literal argv and reject extra run/skip filters in space and equals forms while preserving real planner forwarding; both equals-form bypasses have RED→GREEN controls. |
