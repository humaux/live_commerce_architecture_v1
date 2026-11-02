# PR365 canonical route tests-only candidate

Task `c8794bdb-cfc2-4560-a621-156d01bf4600`, child `c879-sub-1`; base_commit `dafd21b72d9d500db9fc893602b291c8abdaba16`. Author task specified Sol high; actual model/reasoning UNKNOWN without independent receipt. Worktree `.worktrees/codex-pr365-mock-balance`, branch `codex/pr365-mock-balance`, initial HEAD=base and status clean. Root owns task/author lease; child did not claim or delegate. Deadline 2026-11-02T17:06Z; one bounded task candidate; the first local E1 commit was corrected after root caught the reporter-retention defect, without running it or changing product/acceptance budgets.

**B is E1 structure-ready only. A awaits a separate root freeze and is not implemented (BLOCKED within this card, not an owner-permission problem). Product fix, runtime RED/GREEN and independent acceptance are NOT_RUN.** Root explicitly confirmed this narrowed delivery after the fixture-authority gap was reported.

## Authority and scope

Read AGENTS.md, architecture sections 0/1/2/26/27, T09 DAG (depends T05/T07/T08), `contracts/studio-v1.md`, I01/I02/I06/I11/I14, original Studio/Console tests and helpers at base. Humaux current development detail `7bdb220b-0798-4b1d-aff5-16ee94cae75f` and rejected enumeration 9/9, coverage=1, not truncated; ponytail read and applied. No code-index tools used: rg/symbol references were the equivalent read-only navigation.

Fixed original SOURCE input under root `output/integrator/tools/logs/ci-evidence-1238/365-ci-route-bytes-1630/`:

- REPORT.md SHA256 `eb7421de37df17468c5c6b9e6a346c93a7538246de1e9412156b63c603b33471`, Humaux `20a8cef1-19bd-46ef-9fcb-0b4d29401d5d` full original read.
- DECISION-APPENDIX.md SHA256 `07992a86e5a9926d7baeb713e820e0f1adb9f4725fea6ea9ca510a95d0f209ab`, Humaux `9e59a8c8-eef9-4e67-9694-d688e41e682c` full original read.
- Both actual file hashes rechecked MATCH. These establish static CI SOURCE, not the dirty final URL or actual cache branch.

Only changed paths:

1. `tests/admin/studio-ui2-c4-metadata.spec.ts`: +55/-0 lines.
2. `output/parity-live-sessions/CANONICAL-ROUTE-RED.md`: this report.

No product, fixture, Go counter, shared gate, ownership generator, dependency, legacy Studio104 or Console case change. No push/dispatch/provider/network/browser/Go/PG/test runtime. No subprocess, fixture, container, port or temporary file was left running/created for this candidate.

## B implementation and evidence contract

The original dirty clean Cancel click (base185) remains untouched. Only the following original preparation-heading assertion (base186; candidate231) is surrounded by a failure-only catch. All original actions/assertions/timeouts/order remain byte-preserved. The original exception object is rethrown after diagnosis; diagnostic failures cannot replace it. Earlier Keep/Escape/Discard, reentry and reload assertions are unchanged. A failing final assertion remains failing.

One read-only page.evaluate samples fixed route/DOM projection only after that assertion throws. One 100ms asynchronous read budget; no retry or extra click/read. Promise.race consumes both late success and rejection and clears its timer. Exactly one synchronous private JSON evidence write follows the read, using the existing live-console writeFileSync/mode0600/flush pattern. The write is explicitly outside the 100ms read budget and may add filesystem latency; the original assertion still keeps its original10s budget. No extra timeout or sleep; JS timers are not hard real-time guarantees. Late evaluation completion cannot write evidence.

Allowlist: pathname STUDIO/CLAIMS/OTHER; hash EDIT/PREPARATION/OTHER; fragmentCount capped8; fragmentKinds is [] when hash absent, otherwise up to8 EDIT/PREPARATION/OTHER enums in fragment order; sameStore/sameScene booleans; preparation/editor heading, dialog and dirty-marker counts capped16; cancel/edit/save enabled boolean or null when missing/ambiguous; activeElement INPUT/SELECT/BUTTON/A/BODY/OTHER/NONE; rounded finite scrollX/Y clamped to +/-1000000, else null. Counts describe matched DOM nodes, not a new visibility oracle. Editor heading uses the existing .studio-editor h2; no new product selector. Button enabled reads disabled only, not successful navigation/guard execution.

No raw URL, title/text, field value, DOM snapshot, credential, request header, history/Next/React private state leaves the page. URL parsing and fixed-label comparisons stay inside page.evaluate. No app/history/cache/listener/DOM mutation. File c4-dirty186-<locale>.json holds class REAL_PG, readBudgetMs100, and read.status OK/TIMEOUT/UNAVAILABLE with only allowlisted primitives. Existing private-evidence guards remain unchanged.

Future evidence pointer: `${LC_BROWSER_EVIDENCE}/c4-dirty186-en.json`, `.../c4-dirty186-zh-TW.json`, `.../c4-dirty186-zh-CN.json`. At fixed dafd `tests/foundation/browser_studio_ui_test.go:191`, evidence is `output/playwright/studio-ui-<UTC>/`;242 forwards it to the browser. Fixed `.github/workflows/gates.yml:386-394` uploads output/playwright for nonbundle jobs with always(); bundle members are moved to output/members/<art>/playwright at361 and uploaded at417-427 etc. Thus the file belongs to existing downloadable evidence trees; actual file creation/upload/contents remain NOT_RUN. File errors are swallowed to preserve the original error, so absent file means UNKNOWN. Scroll/focus equivalence remains UNKNOWN; no invented threshold or scroll PASS.

Root caught that the Go command227 forces --reporter=list, overriding configured JSON reporters. First local commit5cfccdcfb used an annotation and incorrectly predicted config JSON retention; it was not pushed or executed. That delivery claim is withdrawn here. Read-only installed Playwright1.63.0 SOURCE: lib/common/index.js522 uses takeFirst(configCLIOverrides.reporter,...); lib/runner/index.js4665 begins ListReporter,4797 onTestEnd only emits status/failure; formatFailure1328-1418 reads errors+attachments, and formatResultFailure1419+ reads errors. Arbitrary annotations have no proven list output path. Root authorized replacing that unretained annotation with the existing JSON-file evidence pattern, explicitly separating100ms read budget from filesystem write. This is an evidence-path correction, not a dirty/product third fix. The old local commit remains in history.


## A exact blocker and minimal separate-scope suggestion

At `tests/foundation/browser_studio_ui_test.go@dafd:249`, full suite freezes `wantFaults=3, wantKeys=9, wantIssued=9`. Line291 queries real identity.sessions joined to session.issued and the exact mock-IdP issuer/browser-subject, excluding the fixture bearer hash; issued must equal9. Signed browser identity binding is line67 to h.lp.actor; line149 rejects the original h.lp.token bearer as browser authority.

The nine signed logins are **five legacy plus four C4**:

- `tests/admin/studio-ui.spec.ts@dafd:162`: main STU04 login.
- same file390: separate replacement login inside that main case.
- same file433: STU05 route-settling login.
- same file503: native dirty/conceal login.
- same file522: native uncertain/conceal login.
- `tests/admin/studio-ui2-c4-metadata.spec.ts@dafd:67`: c4OwnedDraft calls signedLogin once for atomic-save case and once per three-locale dirty case, total4; exactly four UI creates are coupled to these writer cases.

Existing `signedLogin` in the allowed spec34-45 goes `/en/` -> real identity-service sign-in button -> attached nav-group-live -> opens responsive menu if present -> real Studio navigation -> list headings. It does not accept a token, return reusable authority, or expose session storage state. Each fresh independent test page needs its own legitimate authority. Adding this helper to two independent 1440/390 route cases would change issued9 to11 even without creating drafts. This is a real frozen oracle conflict, not a missing dependency/timeout.

Token provenance is explicit: `tests/foundation/live_planning_test.go@dafd:31` creates h.token and h.limitedToken;55 grants actor store:read/live:read/live:manage,59 grants limited only store:read;65-66 binds each token to its principal. `browser_studio_ui_test.go@dafd:72` adds limited live:read, but not live:manage. Browser env237-243 exports limitedToken as LC_BROWSER_STUDIO_READONLY_TOKEN and actor expiredToken as LC_BROWSER_STUDIO_EXPIRED_TOKEN; no manageable active browser token is exported. Expired actor token is inserted expired at85-90. The read-only case in the allowed spec independently asserts can_manage=false. No permission conclusion relies merely on variable names.

There is no allowed helper for reusing an existing active manageable signed session across independent tests. Reusing another case's cookie/state would add hidden cross-case dependency; using original fixture bearer bypasses the original signed-chain authority oracle. Neither was implemented. Root reconfirmed no path/counter enlargement; A remains BLOCKED.

Minimal separately frozen scope to consider: append route-only cases to the existing Console fixture/spec, whose facts/receipt helper and native existing-scene pagination/selection/document probe already cover the exact settings->Back->Edit chain at both widths, while retaining all existing assertions. This would require explicit new path ownership and a read of that fixture's login/counter contract before implementation; it is only a suggestion, not authorization or a completed design. Do not add manage tokens or silently change issued/key counts to force A through.

## Actual static commands and exit codes

All commands ran only in the assigned worktree except fixed SOURCE file reads/hashes. No runtime test was run.

- `git status --short`, `git rev-parse HEAD`, `git branch --show-current`: exit0; clean/base/expected branch.
- `git diff --check`: exit0.
- Node24.15 stripTypeScriptTypes + child `node --input-type=module --check`: exit0; syntax only, no import/test evaluation.
- Python inverse normalization + SHA256: exit0; `INVERSE_BYTE_EXACT=true`.
- Python hashlib of both fixed SOURCE files: exit0; both MATCH.
- Read-only exploration had exit1 for a guessed nonexistent console filename and treating tasks.json as an object (actual list); corrected to live-console.spec.ts and list iteration. A later guessed media_planner filename/glob did not exist. None was a product/test execution or replaced a failed acceptance result.
- An initial canvas update without mermaid was rejected without a write; corrected update is tracked separately by Humaux acknowledgement.

Exact syntax command:

```sh
node --input-type=module <<'JS'
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { spawnSync } from 'node:child_process';
const source = readFileSync('tests/admin/studio-ui2-c4-metadata.spec.ts', 'utf8');
const result = spawnSync(process.execPath, ['--input-type=module', '--check'], { input: stripTypeScriptTypes(source), encoding: 'utf8' });
if (result.stdout) process.stdout.write(result.stdout);
if (result.stderr) process.stderr.write(result.stderr);
console.log('stripTypeScriptTypes + node --check exit=' + result.status);
process.exit(result.status ?? 1);
JS
```

Inverse bytes: remove the entire `// C4 dirty186 projection begin ...` through `// C4 dirty186 projection end.` block and its following blank line; remove the added wrapper prefix (begin marker and try line) and suffix (catch/helper-call/throw/closing/end marker), preserving the original assertion line verbatim. Comparison is against `git show dafd21b72d9d500db9fc893602b291c8abdaba16:tests/admin/studio-ui2-c4-metadata.spec.ts`, not a manually rewritten baseline.

SHA256/bytes:

- Base spec: `da2d9489e579cb882b7b1d3c96a4196432cb84e89ea2207f5cc96d9198f20056` /15414.
- Inverse spec: same hash/bytes, exact byte equality.
- Candidate spec: `5accc81b37aaafb60e4d1955251eac7a1facac9d3a3f3547ccdb27f019411bb5` /19225.

Rollback: restore only the assigned spec from the base object; this report was absent at base. No rollback executed. Original failing samples remain.

## Unresolved / NOT_RUN

A independent route cases: root must separately freeze the complete added-login/create effect budget; BLOCKED in this card, not awaiting owner permissions. B actual snapshot, evidence-file write and CI upload: NOT_RUN. Original dirty186 final route/DOM/root cause: UNKNOWN. Original Console duplicate-hash RED and Studio104 are preserved, not repaired. Product hash transport, dirty guard fix, browser/REAL_PG/CI/static typecheck/full gates, injected RED->GREEN and independent review: NOT_RUN. Root must review this diff and run bound real gates; E1 cannot establish product completion or release readiness.

## Fixed-input equality check

Python pathlib + git show + hashlib comparison: exit0 for all9 unchanged read inputs below. Only the candidate spec is intentionally different; its inverse equality is recorded above. This binds file/line claims to base rather than merely the current working copy.

- `tests/foundation/browser_studio_ui_test.go` SHA256 `97683286c4f1fd407c7eb9b8969975fd736bc9299238f59fd8d7d3ded0058332`; current read bytes exactly equal base object.
- `tests/foundation/live_planning_test.go` SHA256 `6c75378e2034c8c45a4e5594c31f23da99cc87f66baa39b96107706ae0a9ebd1`; current read bytes exactly equal base object.
- `tests/admin/studio-ui.spec.ts` SHA256 `c53bd85544b1541f3bb259aab084ab0a39a12fa13acd94d1099de48bddee5217`; current read bytes exactly equal base object.
- `tests/admin/live-console.spec.ts` SHA256 `68fe08d6322d09b96eb7e142f06656ef0514ef247ca40938dc69537bf0c8973c`; current read bytes exactly equal base object.
- `tests/browser-evidence.mjs` SHA256 `37fca6070130b26a900e723a767c6d3034c6e721a43fdf3e188a7b9e91fd829a`; current read bytes exactly equal base object.
- `playwright.config.ts` SHA256 `7d22a5cfb63aab68d206d0162a40dfdac3f63256ece1ef222f9f3351e4378c04`; current read bytes exactly equal base object.
- `contracts/studio-v1.md` SHA256 `cb924b6fc15cb733caaa23a0d960de0620b37971962ee3f79fac5843603f0d70`; current read bytes exactly equal base object.
- `contracts/tasks.json` SHA256 `87e25036acafb5b708e944ad4f4adcbea53d1ef3867bdf0f1c239d9fadc2cf05`; current read bytes exactly equal base object.
- `contracts/invariants.json` SHA256 `ed4f7fcaad364978c3593a7cbafd45c96d75007cc38de2bd254fa9ca8c652f5f`; current read bytes exactly equal base object.

## Reporter SOURCE provenance

The following installed sources were read without execution; they are not tracked dafd objects or a whole-CI byte-equality claim.

- Installed Playwright1.63.0 `package.json` SHA256 `8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998` /1359B.
- Installed Playwright1.63.0 `lib/common/index.js` SHA256 `f4a6b14f5ba21a118b58a6a25eb0b281a3acc4157023b014e0197145056124a4` /115517B.
- Installed Playwright1.63.0 `lib/runner/index.js` SHA256 `7a2e4e1114d3237d61b87723b4d86a37944c9143f3ce2fb0b2e8867e0c612a5e` /315445B.

## Root-frozen fragmentKinds addendum

After root admitted the 3cac B file-based diagnostic, root explicitly froze one additional safe field: fragmentKinds, at most8 EDIT/PREPARATION/OTHER enums, preserving fragment order; absent hash yields []. This permits distinguishing EDIT then PREPARATION without exposing original fragment strings. Existing hash/fragmentCount remain. Only this field was added to the same page.evaluate, with the same100ms read budget and original throw; no extra browser action/read, product/gate/fixture/counter change. This addendum is not a runtime observation. Delta syntax, git diff --check and original-byte inverse were rerun exit0; no unrelated static/runtime suite was repeated. A and all NOT_RUN/UNKNOWN boundaries above remain.
