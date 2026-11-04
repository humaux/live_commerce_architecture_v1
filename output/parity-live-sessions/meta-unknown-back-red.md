# PR365 P1-B independent tests-only handoff

- task_id: `coord5c54bf15-sub-meta-unknown-back`; parent: `coord5c54bf15-2d04-42aa-8a08-195d85e19d52`.
- base_commit/source head: `a3278e5b967792b7378cfa0c5a4bab7b0d033d25`.
- Role: independent test author for P1-B; requested `gpt-6.1-sol` high, actual runtime model/reasoning identity `UNKNOWN` (not introspected). No recursive delegation.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr365-mock-balance`; actual branch `codex/pr365-mock-balance`. Parent confirmed the exact tree/head; no branch change. Initial tree clean. Exact authored path locks acquired before writing.
- Allowed/changed paths: `tests/admin/meta-connect-gate.spec.ts` and this report only. Production, harness, fixture, copy, DTO, ownership and gate changes: none. No push/merge.

## Authority and implementation

Original Humaux independent-review evidence `1178bd85-0f68-4f54-bb91-0f4286fa780f` (agent review evidence, not owner authority) and independent report `output/integrator/tools/logs/ci-evidence-1238/review365-a327-0758.md` were read. Repository authority: AGENTS, architecture sections 0/1/2/9/16/26/27, task DAG (T09 depends on T05/T07/T08), invariants including I06, source MetaConnect.tsx:148-161,213-229, POST BFF `/api/meta/connect`, keyed `metaconnect.Service.Start`, existing browser spec and Go harness. Development detail `7bdb220b-0798-4b1d-aff5-16ee94cae75f` and ponytail skill read; targeted code map used with direct source reads. Rejected enumeration was representative 3/9, coverage 0.3333, not the complete set.

Append-only independent tests outside the existing serial describe use the existing 1440x900/390x844 viewport matrix, signed login, Settings controls, actual Next/Go/PG routes and fake Graph harness. First actual UI start POST uses `route.fetch()`, checks real status 200 and a closed `{dialog_url}` BFF receipt with the official HTTPS host, then aborts only that ACK. No success response or UI state is fabricated. The actual `uncertainStart` alert must be visible, official retry enabled, and Back disabled. The latter is the expected pre-fix RED against the enabled Back in source a3278e5; **runtime RED is NOT_RUN**.

After that assertion, a real official-retry click must produce exactly two total start requests with one key and byte-identical body. Genuine replay must return the same receipt (SHA256 equality asserted as a boolean; no URL/state/token is printed in assertions). Existing fake dialog drives the real callback and real pick page. Cancel and reload leave the connection list empty; existing server facts must show unchanged Page connections, bindings, routes and heads. No pick/disconnect write is allowed. This checks receipt replay and absence of Page effects, not a direct OAuth-state/audit-row count oracle; the existing `/facts` contract has no such count. No fresh fixture/control API was added.

All original 711 lines are byte-preserved, including malformed-JSON start behavior and pick/disconnect readback tests. Final spec is 776 lines, below the 800-line limit. Test file SHA256: `0fc43a710716f937daf7ad8b7432013464a638d66c80b77fb817e9e81704d698`.

## Actual commands and exits

| Command | Exit | Evidence/result |
|---|---:|---|
| `git status --short && git rev-parse HEAD && git branch --show-current` | 0 | Clean source tree, a3278e5, actual branch above |
| `node --input-type=module -e 'import {readFileSync} from "node:fs"; import {stripTypeScriptTypes} from "node:module"; stripTypeScriptTypes(readFileSync("tests/admin/meta-connect-gate.spec.ts", "utf8")); console.log("TypeScript strip syntax OK");'` | 0 | TypeScript strip syntax OK; syntax-only, no imports/test runtime executed |
| `git diff --check -- tests/admin/meta-connect-gate.spec.ts` | 0 | No whitespace errors |
| `python3` inline static preservation/hash check | 0 | `git show a3278e5:tests/admin/meta-connect-gate.spec.ts` is exact byte prefix; line count 776; SHA256 above |
| `git diff --stat -- tests/admin/meta-connect-gate.spec.ts` | 0 | 65 additions, zero deletions |

The inline Python check read the file, read its source bytes with `git show`, asserted `new.startswith(old)` and `len(new.splitlines()) <= 800`, and printed the SHA256. One exploratory rg command used an unmatched zsh glob (`tests/admin/playwright*`) and did not run; subsequent targeted source reads succeeded. This was not a validation command or a runtime test.

## Click ledger and remaining work

| Control/path | Intended operation and visible oracle | Actual |
|---|---|---|
| Signed identity login; Settings advanced entry | Reach real merchant Meta card | NOT_RUN |
| Connect; official intro; official bridge | Commit genuine start then lose ACK; show actual UNKNOWN alert | NOT_RUN |
| Back to connections | Disabled while connect UNKNOWN; expected current-product RED | NOT_RUN |
| Official same-action retry | Enabled, real click, same key/body/receipt, real callback pick | NOT_RUN |
| Pick Cancel; reload | Empty persistent list, no Page effects/pick/disconnect writes | NOT_RUN |

Evidence level: **E1 syntax/static preservation only**; runtime correctness and actual RED/GREEN `NOT_RUN`. Local Go, PG, browser, build, runtime tests and provider calls were forbidden by this card and were not run. SANDBOX/LIVE/whole-PR acceptance, direct start-ledger count, independent review of these new tests and current-head CI are NOT_RUN. Parent/integrator must audit this diff, regenerate ownership if required, run pinned pre-fix CI to record causal RED, implement the minimal product guard separately, then run unchanged tests/current-head gates and independent acceptance. No product fix or P1 closure is claimed. No server, fixture, container or background process was started; no task cleanup deletion was needed. WAIT_RUN: none owned by this author; parent/native completion/saas monitor receive handoff.
