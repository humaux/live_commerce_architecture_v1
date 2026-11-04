# C4 HARNESS DIAG1 — static handoff

task_id: c1e5d208-24f4-42a3-9b1e-86a2b2fc4f1a; own canvas c1e5d208-sub-1. Parent claimed and owns the worktree lease; no separate claim or recursive delegation.
base_commit: b483935f6bd31fb273f9d35aa742043223a40a8a (initial worktree clean).
branch: codex/pr365-mock-balance.
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr365-mock-balance.
role: tests-only harness author. Actual model/reasoning telemetry: UNKNOWN.
write_paths: tests/admin/live-console.spec.ts (C4 suffix only), output/parity-live-sessions/C4-HARNESS-DIAG1.md.

## Authority and original RED

Read AGENTS.md, architecture sections 0/1/2/26/27, contracts/tasks.json T09/T21 and invariants I02/I06/I14/I18/I21. Read ponytail and Humaux development/orchestration details; rejected enumeration returned 9/9, coverage 1.0. Focused code_repo_map plus local rg/sed located the existing journal and native control paths. No new framework or diagnostic file format introduced.

Read parent evidence `output/integrator/tools/logs/ci-evidence-1238/365-c4-source-1319.md`; independently verified SHA256 `21e6f6370ed0a2689fcac27153e24d1222cc3852a22e16a4be38b0c91124865a`. Raw `ci-current-1313/job-125293432195.raw.log` lines 654–657 bind execution to 135ff398571669e2d729dd2aff2dffba7b39eadc (merge b483 into c6be8dd13981b13df4c9c994fb66523c9c893b9e). Relative evidence paths in this section are under the parent worktree's `output/integrator/tools/logs/ci-evidence-1238/`, not copied into this commit.

Independently read companion 135ff original API source receipts and recomputed their Git blob hashes: Studio a48f3ce8e991f5e4a8689015199e83e19c33a65c; SessionTasks 66a67b5a7c3cde17315c18a9040e7d4c1ef75528; WorkspaceFrame 76220333188dcaf1ea6121b77c5593f35c99fbfc. Read the five relevant original journals and all six error contexts. These establish original MOCK RED evidence only; no new browser execution occurred.

## Changes and preserved gates

1. Copy harness repair 1/2: the old asynchronous response predicate read any same-store UUID detail body before excluding the source. Original en1440/zhTW390 receipts show real copy POST200 and distinct returned UUID/CLOSED window, then Network.getResponseBody failure. The actual response responsible remains UNKNOWN. The predicate now synchronously excludes the original detail pathname and checks origin, exact store prefix and canonical UUID suffix before any body read. It acquires the candidate browser GET JSON once at arrival and retains that Promise's outcome. After the real POST yields newID, the observed GET pathname must equal that ID's detail path and its original body must match newID/title. Body errors are rethrown; no swallowed failure, synthetic DTO or replacement request GET. Original detail version projection, CLOSED/generation0/null window, success/notice, copied-ID Edit/OpenPrep/Back, one-write/effect and unchanged-source assertions remain.
2. UNKNOWN harness repair 1/2: original zhCN390 error says open workspace-navigation/nav-claims intercepts native Retry. All departure URL/input/Retry/help/noDiscard/one-command/one-receipt assertions were reached. WorkspaceFrame returns before close() on a denied navigation; existing AppShell handles native Escape. Only after all original departure assertions, zhCN now presses Escape and asserts the menu toggle aria-expanded=false before the original native Retry. Original key/body equality, two commands, one effect and one created scene assertions remain. No force click or guard change.
3. Preparation diagnostics only: original two widths fail exact #edit with compound #preparation#edit; Studio's original composer appears clean, so origin of the compound fragment remains UNKNOWN. Existing journal gets a closed projection: pathname STUDIO/CLAIMS/OTHER; hash NONE/PREPARATION/EDIT/PREFLIGHT/OTHER; sameStore/sameScene/multipleFragments booleans. Unknown hash bytes are never recorded. Before/after settings, history Back and Edit plus at most eight passive mainframe-navigation observations and a failure snapshot are recorded. Existing 32-entry journal budget remains. No DOM/history/router/handler write, raw URL/query/title/body/cookie/storage recording, extra request, delay or tolerance change. Finally removes the observer and retains original failure rethrow and zero-write oracle.

The first 109092 bytes before `// C4 metadata freeze:` are byte-identical to base, SHA256 `07e3bd132fa3ee1152a0d21e91a8d1dd7aeba253b3c844b48dd5062502da7dc1`. All 331 original expect assertion lines are present unchanged and in order. Test file SHA256: `790ec653fb63469f087dc5483ac9b10372244208135df29155fe5188ca111d43`.

## Actual validation and commands

- `pwd && git status --short && git rev-parse HEAD`: exit 0; clean b483935f baseline.
- Targeted `rg`, `sed`, `cat`, Python original source/journal/hash reads: successful corrected reads exit 0. Read errors are listed below; no absence claim relies on them.
- `git diff --check`: exit 0.
- Babel static TypeScript syntax only, exit 0 (`BABEL_TYPESCRIPT_SYNTAX_OK`):

```sh
node - <<'JS'
const fs=require('node:fs');
const parser=require('/Volumes/data/live_commerce_architecture_v1/node_modules/.pnpm/next@16.3.5_@types+node@24.13.6_react-dom@19.3.0_react@19.3.0__react@19.3.0/node_modules/next/dist/compiled/babel/parser.js');
parser.parse(fs.readFileSync('tests/admin/live-console.spec.ts','utf8'),{sourceType:'module',plugins:['typescript']});
console.log('BABEL_TYPESCRIPT_SYNTAX_OK');
JS
```

- Original prefix and assertion preservation, exit 0 (`PRE_C4_UNCHANGED 109092`, `ORIGINAL_ASSERTIONS_ORDERED 331`):

```sh
python3 - <<'PY'
import subprocess,pathlib,hashlib,re
p='tests/admin/live-console.spec.ts'
base=subprocess.check_output(['git','show','b483935f:'+p]);now=pathlib.Path(p).read_bytes();marker=b'// C4 metadata freeze:'
assert base[:base.index(marker)]==now[:now.index(marker)]
old=[s.strip() for s in base.decode().splitlines() if re.match(r'\s*(?:await )?expect(?:\.soft)?\(',s)]
new=[s.strip() for s in now.decode().splitlines() if re.match(r'\s*(?:await )?expect(?:\.soft)?\(',s)]
pos=0
for s in old:pos=new.index(s,pos)+1
print('PRE_C4_UNCHANGED',base.index(marker),hashlib.sha256(base[:base.index(marker)]).hexdigest())
print('ORIGINAL_ASSERTIONS_ORDERED',len(old));print('TEST_SHA256',hashlib.sha256(now).hexdigest())
PY
```

Local commit is authorized with hooks disabled to avoid local runtime: `git add -- tests/admin/live-console.spec.ts output/parity-live-sessions/C4-HARNESS-DIAG1.md`, then `git -c core.hooksPath=/dev/null commit -m 'test(live): scope C4 copy reads and observe preparation routes'`. Final parent handoff supplies its actual exit/head. No push or auto relay.

Observed read/tool errors retained: initial evidence cat from child tree exit1 and rg exit2 (evidence lives in parent tree); initial original API JSON decoding exit1 (MCP wrapper, corrected structuredContent.content read); child node_modules rg exit2; guessed root node_modules/next bundle require exit1 (actual pnpm parser located, syntax check exit0). These caused no product/test workaround or runtime retry.

## Evidence limits and unresolved issues

E1 static structure only for this new change; author assessment cannot serve as independent acceptance. Runtime RED→GREEN, browser/typecheck/build, architecture/spec model execution, Go/PG, fresh CI, provider SANDBOX/LIVE and E4 independent review are NOT_RUN under this card. Original 135ff automated failures remain the only E3 browser evidence, and remain failures.

Two preparation exact-hash cases and the zhCN copied-list case are unresolved. Compound hash cause remains UNKNOWN; Next cache is a candidate only, with no node_modules or product edit and no second product repair consumed. >20-row MOCK pagination belongs to a separate fixture card; no fixture/parser/scene truncation/skip was introduced. Existing helper/source-ownership exhausted 2/2 repairs are untouched. If the newID browser GET body still fails, its acquisition/abort timing needs independent runtime diagnosis before any further harness repair. No claim of new browser GREEN, whole-PR approval or release readiness.

No server, browser, fixture, container, port or background runtime was started; WAIT_RUN: none. No temporary file or other task directory was deleted. Parent must independently inspect this diff and arrange subsequent execution at the exact committed SHA.
