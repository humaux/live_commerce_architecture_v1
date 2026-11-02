# PR475 field-v1 mutation harness — static implementation handoff

Purpose: retain the bounded implementation, static checks and remaining GitHub acceptance contract. Depends on `contracts/migration-import-v1.md` §6.1 and the frozen root task; used by the root integrator and independent reviewer. Invariant: SOURCE/E1 is not a mutation RED/GREEN witness.

Date: 2026-11-02. Task ID: `d181b879-3a09-49a5-baea-ed37ba29a4c5`; child `/root/import_mutation_harness_0834`, no separate claim or delegation. Role: CI mutation harness author. Actual model/reasoning telemetry: UNKNOWN; root spawn record is authoritative. Base commit: `70d7a41b2a25950ce33bbb27abe09925b40764c6`. Branch: `codex/pr475-inventory-repair`; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr475-inventory-repair`. Candidate commit is the local commit containing this handoff; root must record its exact immutable SHA before review/dispatch.

Authority: root-frozen task and full `475-mutation-scope-0826.md`, SHA256 `3a4541fa96bc3f4488a9302a7aad92d4880f5b549facb0b49e702624d940fc08`; contract §6.1 at the base commit. AGENTS, architecture §§0/1/2/26/27, task DAG, invariants, existing OA injector, actual source anchors and unchanged test oracles were read. Humaux development/file rules and ponytail were read; rejected enumeration was a representative 4/9 subset (coverage 0.4444), not exhaustive. Code graph map plus repository dependency map and targeted symbol reads established the narrow impact. One discovery read of absent `docs/SYSTEM_MAP.md` exited 1; corrected to existing `docs/engineering/dependency-map.md`, exit 0.

## Changed paths and behavior

Only these persistent paths are authored:

* `tests/foundation/import_field_mutation.py`: stdlib-only, closed `unmarked-metadata`, `drop-commit-marker`, `capability-identity` recipes. Both `GITHUB_ACTIONS=true` and `GITHUB_EVENT_NAME=workflow_dispatch` are mandatory. Unknown/absent names, simultaneous OA mutation, staged/unstaged tracked dirt, nonregular/symlinked source, missing/duplicate anchors, preexisting replacements and overlapping edits fail closed. All anchors and resulting replacements are checked in memory before any source/evidence write. Exact HEAD, before/after source SHA256, closed name and actual git patch are retained in `output/ci-gates/import-field-mutation.{json,patch}`. The script does not invoke tests or reinterpret their exits.
* `.github/workflows/gates.yml`: nine lines at the existing dispatch injection hook. Reject both nonempty mutation variables before OA runs; invoke the new injector only for explicit nonempty import mutation and `workflow_dispatch`. Existing OA stanza, ordinary gate bodies, failure handling, test fixtures and `EXTRA_ENV == ''` prebuilt restore restriction are preserved.
* This evidence handoff.

Temporary CI source allowlist is exactly `internal/httpapi/imports.go`, `apps/admin/lib/import-client.ts`, `internal/migrationimport/customers.go`. There are no persistent product-source edits, oracle/fixture edits, migration/dependency/schema changes or general arbitrary patch/script facilities. Five old untracked `output/parity-data-ops/inventory-diagnosis-4556/` evidence files were preserved. The injector was NOT_RUN locally and no mutation output directory or temporary source snapshot was created.

## Actual local checks (SOURCE / E1 only)

All commands below were run in the assigned worktree; none starts Go/PG/browser/build/provider/runtime or executes injector main.

| Actual command | Exit | Observation |
|---|---:|---|
| `git rev-parse HEAD && git status --short` | 0 | Exact frozen base; tracked clean initially; five existing untracked KEEP evidence files. |
| `shasum -a 256 /Volumes/data/live_commerce_architecture_v1/output/integrator/tools/logs/ci-evidence-1238/475-mutation-scope-0826.md` | 0 | Exact root-frozen report hash above. |
| Python stdin static check reproduced below | 0 | AST/compile, three closed recipes, five unique anchors/replacements, in-memory SHA plans, Bash syntax and pre-OA dual-variable ordering passed. |
| `git diff --check` | 0 | No whitespace errors. |
| `git diff --stat` and `git diff -- .github/workflows/gates.yml` | 0 | Existing tracked diff is only the nine workflow additions before staging the two new files. |
| `git diff HEAD -- internal apps tests/admin tests/foundation/customer_import_test.go tests/foundation/order_attempt_mutation.py` | 0 | Empty: product sources and existing designated oracles unchanged. |
| `git diff --check && git diff --exit-code --no-ext-diff --no-textconv HEAD -- internal apps tests && git rev-parse HEAD && git status --short && git config --get core.hooksPath` | 1 | Both diff checks passed and HEAD remained frozen; final config lookup exited 1 because no core.hooksPath is set. Separate read-only hook inventory exited 0 and found no active hooks. |
| `git add -- tests/foundation/import_field_mutation.py .github/workflows/gates.yml output/parity-data-ops/FIELD-MUTATION-HARNESS.md && git diff --cached --check && git diff --cached --stat && git diff --cached --name-only` | 0 | Exactly the three authorized persistent paths staged; whitespace check passed. |

Actual static check command (it reads source and plans bytes in memory; it never imports or calls the injector):

```sh
python3 - <<'PY'
import ast, hashlib, json, subprocess
from pathlib import Path
p=Path('tests/foundation/import_field_mutation.py')
tree=ast.parse(p.read_text(),filename=str(p)); compile(tree,str(p),'exec')
edits=ast.literal_eval(next(n.value for n in tree.body if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='EDITS' for t in n.targets)))
assert set(edits)=={'unmarked-metadata','drop-commit-marker','capability-identity'}
allow={'internal/httpapi/imports.go','apps/admin/lib/import-client.ts','internal/migrationimport/customers.go'}
for name,recipe in edits.items():
 original={}; planned={}
 for path,before,after in recipe:
  assert path in allow
  if path not in original: original[path]=Path(path).read_bytes();planned[path]=original[path]
  before,after=before.encode(),after.encode()
  assert original[path].count(before)==1,(name,path,'anchor')
  assert after not in original[path],(name,path,'preexisting replacement')
  assert planned[path].count(before)==1,(name,path,'overlap')
  planned[path]=planned[path].replace(before,after,1)
  assert planned[path].count(after)==1,(name,path,'replacement')
 print(json.dumps({'mutation':name,'anchors':len(recipe),'planned_only':True,'changes':[{'path':s,'before_sha256':hashlib.sha256(original[s]).hexdigest(),'after_sha256':hashlib.sha256(v).hexdigest()} for s,v in planned.items()]},sort_keys=True))
workflow=Path('.github/workflows/gates.yml').read_text()
start=workflow.index('          # Reject a combined fault before either injector can modify source.')
end=workflow.index('          # One planned mode:',start)
stanza=workflow[start:end]
subprocess.run(['bash','-n'],input=stanza,text=True,check=True)
assert stanza.index('if [[ -n "${LC_OA_MUTATION:-}" &&')<stanza.index('python3 tests/foundation/order_attempt_mutation.py')
print('STATIC_ONLY Python AST/compile, all source anchors/replacements, Bash stanza syntax and pre-OA conflict order: PASS; injector main NOT_RUN')
PY
```

Resulting static plans, not execution receipts:

| Closed name / source | Baseline SHA256 | In-memory mutated SHA256 |
|---|---|---|
| unmarked-metadata / imports.go | `da16de3cde48ac43a993c64069a8679d1c1cc8e261748ad04d0c360f3258ffae` | `12eba12923a21efdb140d57dbcda5bc1cc6746aec4a5f6a6454c5be2a0bee884` |
| drop-commit-marker / import-client.ts | `55bc4491612f2fe156e5eb43e5c9b8bd9cea5d8d616e71542fd6b82d9735ab13` | `6a938091cef858938f0c68a4d46d08f4541bde6852613a44e41e851658d924a6` |
| capability-identity / customers.go | `8a7dffd749e21d9ac10b8572ff7516823a7528b652c19fdaac9f32693bb3207b` | `17797f50f2e8e5864bb6483faab70c265bf645f8962aea96218f1221f4ca9ae1` |
| capability-identity / imports.go | `da16de3cde48ac43a993c64069a8679d1c1cc8e261748ad04d0c360f3258ffae` | `d781bad9d93b6234a4e7545a7f9a4333c97d562bb9dccb983e9717cdbfc23ebe` |

## Remaining acceptance and limits — NOT_RUN

Root must independently review this diff, obtain non-author CI/security review, then push/dispatch under its separate authorization. This author performs no push, dispatch, merge or production action. Local guard execution, source mutation, tests, Go compilation/race/vet, PG, browser, builds, provider requests and mutation RED/GREEN are NOT_RUN. No current runtime or contract-§6.1 completion is claimed. UNKNOWN: actual injected runtime/guard behavior and downstream named failures until GitHub execution.

Run three separate expected-RED dispatches from clean checkouts of exactly the same immutable harness SHA, followed by clean corrected GREEN at that same SHA with no mutation variable. Retain actual patch, mutation name, HEAD/source hashes, raw GitHub run/job/attempt URL, unchanged test counts/exit and named assertion. Corrected source hashes must match the baseline above. A compile/auth/setup/zero-test error is not a mutation kill. Preserve ordinary RED status; never swallow it in an expected-failure wrapper. The clean GREEN must cover all three unchanged oracles:

* `unmarked-metadata`: actual HTTP+real PG `TestCustomerImport/CI16_row-error_capability_contract` unmarked 200 and fresh stale 409 legacy raw-key controls, `stale=false/marked=false` and `stale=true/marked=false`.
* `drop-commit-marker`: actual `sendImport` → native Request → real POST/auth/BFF → MOCK upstream 409, existing `actual sendImport opts in only explicitly: customer preview, stale commit and receipt keep one marker and original command`. Its commit marker assertion must fail. The existing `--browser-migration-import` mode runs this Node preflight; if it stops there browser is NOT_RUN, not failed or passed. Do not call this real PG transport evidence.
* `capability-identity`: actual HTTP+real PG same bytes/mapping/N replay both capability directions, `marked-first=false` and `marked-first=true`, retaining batch/owner/profile/ID/audit exactly-once and CSV/privacy controls. Temporary variadic bool carries actual admitted HTTP capability into the existing command hash payload; it is not a permanent product API.

Suggested existing modes are `focused:^TestCustomerImport$/^CI16_row-error_capability_contract$` for the backend faults and `--browser-migration-import` for the client fault; the corrected GREEN includes both. Nonempty extra_env already prevents restored prebuilt app reuse; preserve this condition. Workflow remains bounded by its existing job timeout; root owns immediate run-ID registration, logs, deadline and completion wakeup. Mutation snapshots are disposable CI checkouts; do not reset/clean this shared author worktree. No local process, container, port, fixture or background job was started by this card.
