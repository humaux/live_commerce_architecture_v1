# Native mode registry independent source review

**FINAL VERDICT: PASS for the scoped source/metadata repair. The known P2 is closed by the final seven-case recheck below. Both intermediate findings and the premature closure/correction are retained as history. No other confirmed scoped P0/P1/P2 remains.** This is not independent backend/browser acceptance.

- task_id: `6c3b7b4a-sub-independent-review`; parent: `6c3b7b4a`.
- base_commit / current HEAD: `88d3ba1369de0b13c4a9d4ba8051c1540ca9cfd6`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-pr-modes-coverage`.
- Reviewed current working files, including the unstaged guard-order repair; index alone is not the reviewed artifact.
- Role: independent read-only reviewer; dispatch history gpt-6.1-sol/high, exact runtime model identifier UNKNOWN. No delegation, source edits, commits or heavy tests.
- Change paths: `scripts/dev/{test-local.sh,test-local-runtime.sh,pr-modes.mjs,check-gates.sh,release-gate.sh,test-node.sh}`, `tests/ci/{mode-registry.test.mjs,pr-modes.test.mjs}`, and three admin model consumer tests (`meta-health`, `picklist`, `product-media-ui`). Reviewer wrote only this primary output file.

## First-round finding and RED (indentation resolved; residual below)

**[P2] `scripts/dev/pr-modes.mjs:33–43` and `scripts/dev/test-local.sh:1480–1483`: valid Bash case arms can be silently omitted rather than rejected.** Both parsers only recognize case labels with exactly two leading spaces. Changing the inbox arm from two spaces to three preserves Bash syntax and runtime dispatch, but `registryModes()` succeeds with 82 entries instead of 83 and drops `--browser-inbox`. A future or reformatted mode can therefore fall outside discovery/planning without the promised malformed-registry rejection. Existing docs may catch a removed documented mode, but that is not fail-closed source parsing and does not cover an unrecognized newly added arm.

Minimal repair: ensure every registry case arm is recognized (reject unmatched/unsupported arms), or consistently support Bash whitespace in both discovery paths. Test altered indentation and unsupported alias/arm syntax explicitly; each must either be correctly discovered/dispatched or rejected, never silently ignored. Preserve the historical usage fallback. No test thresholds, timeout/retry or acceptance universe should be relaxed.

Actual bounded negative command, executed in the author worktree without any write:

```sh
node --input-type=module <<'NODE'
import fs from 'node:fs';import assert from 'node:assert/strict';import {spawnSync} from 'node:child_process';import {modeEntries} from './scripts/dev/pr-modes.mjs';
const s=fs.readFileSync('scripts/dev/test-local.sh','utf8');
const m=s.replace(/^  --browser-inbox\)/m,'   --browser-inbox)');
const syntax=spawnSync('/bin/bash',['-n'],{input:m,encoding:'utf8'});
assert.equal(syntax.status,0);
console.log(JSON.stringify({bashSyntaxExit:syntax.status,baselineEntries:modeEntries(s).length,mutantEntries:modeEntries(m).length,hasInbox:modeEntries(m).some(e=>e.name==='--browser-inbox')}));
assert.throws(()=>modeEntries(m),/invalid|unparsed|registry/, 'malformed registry layout must reject rather than omit a valid Bash arm');
NODE
```

Exit **1** (RED): `{"bashSyntaxExit":0,"baselineEntries":83,"mutantEntries":82,"hasInbox":false}`, then `Missing expected exception`. No mutated source file was created. GREEN for this negative is **NOT_RUN**, pending the repair.

## Other reviewed behavior

- Current `/bin/bash` is **3.2.57**. `/bin/bash -n scripts/dev/test-local.sh` and `/bin/bash -n scripts/dev/test-local-runtime.sh` both exited **0**. No associative-array/mapfile/newer-Bash dependence was found in the new dispatcher.
- The moved runtime body, starting at `test_container=`, is **99 lines byte-identical** to the corresponding original block at the base. Independent Python `git show`/string comparison exited **0** and reported `runtime_body_byte_identical=True`. Fixture ownership labels, loopback binding, fresh-cluster helper, trap/lock cleanup and JSON count predicates were preserved.
- `lc_select_mode` is invoked in a conditional, but the current selected arms only assign metadata/define preparation/run functions; its storefront opt-in chooses definitions without running the mode. Actual `lc_prepare`, build helpers, sourced PG runtime and `lc_run` are then invoked at top level, so no new conditional-call suppression of their `errexit` was found. Existing positional-argument uses are inside their own nested helper functions.
- The latest runtime live-key guard executes before `cd`; Docker/Go/Node presence checks and synthetic exports execute before argument validation, matching the baseline ordering. `--list`/`--dry-run` avoid those runtime actions and do not print evaluated credentials. Dry-run function definitions are displayed, not executed.
- `pr-modes` reads historical Git contents as data. The historical usage fallback and original quoted/trimmed CLI classification union remain present; no new execution of fetched revision scripts was introduced. Current mode count is 83; browser discovery returns 51. No actual current-arm omission was found.
- Existing planner cases and consumer assertions were inspected: source-shape predicates were translated to selected-plan checks, including both build order and the four storefront MOCK combinations. The picklist native Linux launcher check still examines the selected actual run body. No confirmed semantic assertion weakening was found; the missing parser negative above is a coverage gap.

## Retained independent-worker evidence read, not rerun

Primary `output/ci-mode-registry/oracle/commit-evidence/candidate-v2/SUMMARY.json` records **120 cases, 0 comparison differences, 0 unexpected outcomes**; its runner hash matches the current unstaged runner below. `candidate-listings-v2/SUMMARY.json` records **85 metadata cases, 0 failures**, with the same runner hash. The retained v1 summary has 240 comparison differences, and the current v2 result confirms the guard-order repair was exercised by that worker's MOCK command/environment oracle. These are independent-worker artifacts; this reviewer did not rerun that oracle or claim actual PG/browser execution.

Earlier `node-step.json` and `check-gates-step.json` record exits 0 on tree `4a579125d5fab2471ffd11cc975b3237732c2064`; they predate the latest small ordering change and must not alone be presented as final-current runtime evidence. Source review does not substitute for the author's required real backend/browser/click-sweep shard 1/10 steps.

## First-round reviewed SHA256 and limits (historical)

| Path | SHA256 |
| --- | --- |
| `scripts/dev/test-local.sh` | `272729d1c3d7924ea380f15e3d860eca239ade495a017bb6411c51420a9c3b89` |
| `scripts/dev/test-local-runtime.sh` | `95cdf57a14cae8a213b4224df9f456fe00fdd9dc59067494c4c7f731cb546687` |
| `scripts/dev/pr-modes.mjs` | `302a3a8739e7d60f2e5f3aacb94882d3be93074b4dfb9c01d8c70bdceb5e286d` |
| `scripts/dev/check-gates.sh` | `1d5f947ca8ce09cad8bdb9d47e5f5b98377e75daa43ef9220b014c5e8ead748c` |
| `scripts/dev/release-gate.sh` | `9b7b731e0d120a5f076dff080783d48308c5b4be23d61de7726000af4a9ee2af` |
| `tests/ci/mode-registry.test.mjs` | `e07d8803de847f7cb8712d69634f2200b15d9018a9bb2002d024a1487d2af695` |
| `tests/ci/pr-modes.test.mjs` | `b67116a2406346a7149ad2d72afc9b06a77b4a4d93634b846036ce9c6b15420b` |

Actual source/hash/summary reads exited 0. The first Humaux search call initially failed because the tool was not loaded; tool discovery and retry succeeded. Browser/Go/PG/build/full Node/full gates by this reviewer: **NOT_RUN**. The indentation variant is fixed below; the complete P2 and separately required runtime acceptance remain open. Evidence level for the overall source review remains E1; the tested metadata variants have actual RED→GREEN evidence.

## Targeted five-case receipt — 2026-10-08 (partial resolution only)

Both JS and native Bash now accept whitespace-indented mode labels consistently. They exclude prepare/run function contents before checking case arms, reject the tested alias/unknown arms, and reject duplicate names. Historical usage fallback remains present. The new negative tests add coverage rather than remove original acceptance checks. The trailing-comment residual was confirmed afterward and is documented below.

Independent bounded recheck: `node --input-type=module` with the script below, working directory the author worktree, **exit 0**. It only reads production source and invokes `/bin/bash -n`/`--list` on an isolated temporary copy with `PATH=/usr/bin:/bin`; it does not run a mode. The copy was removed in `finally`.

```js
import fs from 'node:fs';import os from 'node:os';import path from 'node:path';import assert from 'node:assert/strict';import {spawnSync} from 'node:child_process';import {registryModes} from './scripts/dev/pr-modes.mjs';
const source=fs.readFileSync('scripts/dev/test-local.sh','utf8');const expected=registryModes(source);
const dir=fs.mkdtempSync(path.join(os.tmpdir(),'lc-registry-review-'));fs.mkdirSync(path.join(dir,'scripts/dev'),{recursive:true});
try {
 for (const [label,text,valid] of [
  ['current',source,true],['three-space',source.replace(/^  --browser-inbox\)/m,'   --browser-inbox)'),true],
  ['alias',source.replace(/^  --browser-inbox\)/m,'  --browser-inbox|--alias)'),false],
  ['unknown',source.replace(/^  --browser-inbox\)/m,'  unrecognised)'),false],
  ['duplicate',source.replace(/^  --browser-meta-health-ui\)/m,'  --browser-inbox)'),false]]) {
  fs.writeFileSync(path.join(dir,'scripts/dev/test-local.sh'),text);
  const syntax=spawnSync('/bin/bash',['-n','scripts/dev/test-local.sh'],{cwd:dir,encoding:'utf8'});assert.equal(syntax.status,0);
  const result=spawnSync('/bin/bash',['scripts/dev/test-local.sh','--list'],{cwd:dir,env:{PATH:'/usr/bin:/bin'},encoding:'utf8'});
  let names,error;try{names=registryModes(text);}catch(e){error=e.message;}
  if(valid){assert.deepEqual(names,expected);assert.equal(result.status,0,result.stderr);assert.deepEqual(result.stdout.trim().split('\n'),expected);}
  else {assert.ok(error);assert.equal(result.status,2,result.stderr);}
  console.log(JSON.stringify({label,syntaxExit:syntax.status,bashListExit:result.status,jsCount:names?.length??null,jsError:error??null}));
 }
} finally {fs.rmSync(dir,{recursive:true,force:true});}
```

Actual results:

```json
{"label":"current","syntaxExit":0,"bashListExit":0,"jsCount":83,"jsError":null}
{"label":"three-space","syntaxExit":0,"bashListExit":0,"jsCount":83,"jsError":null}
{"label":"alias","syntaxExit":0,"bashListExit":2,"jsCount":null,"jsError":"unparsed mode registry arm"}
{"label":"unknown","syntaxExit":0,"bashListExit":2,"jsCount":null,"jsError":"unparsed mode registry arm"}
{"label":"duplicate","syntaxExit":0,"bashListExit":2,"jsCount":null,"jsError":"empty or duplicate mode registry"}
```

Final reviewed SHA256:

- `scripts/dev/test-local.sh`: `c225d4a22369c13a8724949263c14e9e276db593414fe57cb25d325c8a3f6ea0`.
- `scripts/dev/pr-modes.mjs`: `852650c9f2eec3b35e3c73d11c2637e2c6fa8d6b7570532b25c9402f0d8b66bf`.
- `tests/ci/mode-registry.test.mjs`: `d8735219b96ced065f7add1b2ffcf84830599fa58c56514183c8b028b733f45e`.

No source edits or heavy tests by reviewer. Previous v2 oracle/full-gate results bind earlier hashes and are not silently promoted to these final bytes. Real backend/browser/click shard acceptance remains the parent's required next step. The five cases above pass, but they do not close the complete fail-closed finding: see the correction below.

## Intermediate correction: commented-arm P2 remained (historical; final closure below)

The provisional closure was too broad. Both new all-arm scanners still require `)` at the end of the line, so a valid Bash trailing comment bypasses both the entry and the all-arm scans. Current source hashes are the three final hashes above.

Actual isolated-copy metadata command (`node --input-type=module`, exit **0**, author worktree as cwd): read the current runner, replace the first `  --browser-inbox)` with `  --browser-inbox) # valid Bash arm comment`, copy it to a fresh `os.tmpdir()` directory at `scripts/dev/test-local.sh`, run `/bin/bash -n` and `/bin/bash scripts/dev/test-local.sh --list` with `PATH=/usr/bin:/bin`, and compare `registryModes(mutatedSource)`. The temporary copy was removed in `finally`. Exact output:

```json
{"syntax":0,"bashList":0,"bashCount":82,"jsCount":82,"hasInbox":false,"error":null}
```

This is a confirmed residual of the original P2, not a new product runtime finding. Detect every top-level case arm even when text follows `)`, then either support the comment/inline tail consistently or reject it. Include this counterexample alongside indentation/alias/unknown/duplicate cases. No source/test edits or heavy tests by this reviewer. P2 remains open; real runtime gates remain separate and NOT_RUN by reviewer.

## Final seven-case recheck: known P2 closed

The second repair validates every remaining declaration against the supported registry grammar after excluding prepare/run functions. Unsupported trailing comments and inline commands now fail rather than disappear. The current 83 entries retain their names/order. No additional confirmed finding in this targeted repair.

Actual command: `node --input-type=module` using the same isolated-copy harness above, adding these two variants to its matrix and asserting `expected.length === 83` before the loop:

```js
['trailing-comment',source.replace(/^  --browser-inbox\)/m,'  --browser-inbox) # valid Bash arm comment'),false],
['inline-tail',source.replace(/^  --browser-inbox\)/m,'  --browser-inbox) :'),false]
```

Working directory: the author worktree. The complete seven-case command exited **0**. Each variant is copied only into a fresh temporary directory, syntax-checked with `/bin/bash -n`, then passed to native `/bin/bash ... --list` with `PATH=/usr/bin:/bin` and the actual JS `registryModes`; the temporary directory is removed in `finally`. Actual output:

```json
{"label":"current","syntaxExit":0,"bashListExit":0,"jsCount":83,"jsError":null}
{"label":"three-space","syntaxExit":0,"bashListExit":0,"jsCount":83,"jsError":null}
{"label":"alias","syntaxExit":0,"bashListExit":2,"jsCount":null,"jsError":"unsupported mode registry declaration"}
{"label":"unknown","syntaxExit":0,"bashListExit":2,"jsCount":null,"jsError":"unsupported mode registry declaration"}
{"label":"duplicate","syntaxExit":0,"bashListExit":2,"jsCount":null,"jsError":"empty or duplicate mode registry"}
{"label":"trailing-comment","syntaxExit":0,"bashListExit":2,"jsCount":null,"jsError":"unsupported mode registry declaration"}
{"label":"inline-tail","syntaxExit":0,"bashListExit":2,"jsCount":null,"jsError":"unsupported mode registry declaration"}
```

Final reviewed SHA256 (supersedes all earlier source hashes for the affected files):

- `scripts/dev/test-local.sh`: `04d788579912289ddbd3be444a988448da8119ca2ac67c37be4428454a31eacf`.
- `scripts/dev/pr-modes.mjs`: `05064ca72d93f6769ab1b1621bc9561afbfe45d2d591fb6b8b4d470435242b97`.
- `tests/ci/mode-registry.test.mjs`: `d175eb3fab7a57799c6f36145d44a1ce9b22462aca2656587afb53b84700ea84`.
- `scripts/dev/test-local-runtime.sh`: unchanged `95cdf57a14cae8a213b4224df9f456fe00fdd9dc59067494c4c7f731cb546687`.

Task/base unchanged: `6c3b7b4a-sub-independent-review` / `88d3ba1369de0b13c4a9d4ba8051c1540ca9cfd6`. No author source edits or heavy tests by reviewer. Metadata regression has actual RED→GREEN evidence; broader source review is E1. Prior 120-case MOCK oracle and full-gate results bind older hashes; the worker's current-hash rerun and required real backend/browser/click-shard acceptance remain separate. Reviewer runtime acceptance: **NOT_RUN**.
