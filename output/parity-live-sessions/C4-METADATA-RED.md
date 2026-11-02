# C4 metadata tests-first handoff — NOT_RUN

task_id: coordc27f19d3-6e07-4728-9a1b-7e08e5a9e32c / c27f19d3-sub-1.
base_commit: 5c4f3a504c546678b737a8d241b5d950791e1d4d.
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr365-mock-balance.
role: sole tests-only author; requested gpt-6.1-sol / high; actual model ID and reasoning telemetry UNKNOWN.
Evidence level: E1 source/static observations only; product RED/GREEN, browser, Go/PG and independent acceptance NOT_RUN.

## Authority and scope

Parent task card and original repository AGENTS.md; 架构.md §§0/1/2/9/17/26/27; contracts/tasks.json T09 dependencies T05/T07/T08, G06/G07; invariants I01/I02/I06/I10/I11/I14/I18. Humaux originals read: 293ec836-92ee-4c6b-8a3b-1252049383e6 (tests-only freeze), d1e0b80a-d877-42be-a96d-81487608f044 (SOURCE), d272d041-21c0-41f5-ac34-fa44f8d17e73 (design freeze). These are root decisions/SOURCE pointers, not newly inferred owner instructions.

Exact new seven-group locale copy and its display conditions: output/integrator/tools/logs/ci-evidence-1238/review-c4-backfix2-delta.md, SHA256 fde0412dcbadc9e044ae19ef4d0335fd004aec376ea44322a7cc978a3b2f3ad8, read in the integration root. Prior review-c4-design-0946.md and c4-edit-authority-0854.md were read for the original preflight, copied-ID and departure contracts. No Figma reads/writes were made here. Existing field/action labels were checked in the frozen source, then asserted as independent test literals; no production dictionary import was added as an expected-value oracle.

Only authored paths:
- tests/admin/live-console.spec.ts
- tests/admin/studio-ui.spec.ts
- output/parity-live-sessions/C4-METADATA-RED.md

Both old spec files are immutable byte prefixes. No old line, import, helper, hook, fixture, source-ownership assertion, claim helper, production API/DTO/journal, registration, threshold or gate changed. New Studio tests reuse the existing private-evidence test extension locally so failure artifacts omit raw DOM/header/credential snapshots; existing tests retain their original fixture/settings. No runtime, dependency installation, dispatch, ownership generation, push, merge or production action was performed.

## Added cases and intended first failure

All cases are independent default-mode tests; no serial block/skip/expected-failure annotation. EXPECTED_RED names describe hypotheses, not executed outcomes. Setup failures must remain distinct from product failures.

| Cases | Scope / native controls | Required precondition | Intended first product failure at frozen source | Current result |
|---|---|---|---|---|
| preparation, zh-TW 1440/390 (2) | List card → existing View settings → real claims#preflight → history Back → new Edit | Real original GET detail; same store/scene, no business request/receipt | Missing visible Edit; never bridge it with goto/hash mutation | NOT_RUN |
| successful copy, en1440/zh-TW390/zh-CN390 (3) | Copy form/submit → copied result → Edit → history Back → Open preparation → history Back → Back list | Actual POST200, actual returned new-ID detail, CLOSED/generation0 window and one effect receipt | Frozen success text absent (soft), then missing Edit | NOT_RUN |
| UNKNOWN copy, en1440 Back/zh-TW390 Back/zh-CN390 shell Orders (3) | Existing copy form, existing Back/shell, existing original Retry | route.fetch actual POST200 and effect receipt, then abort only client ACK; original Retry visible BEFORE exit | Existing Back/shell unmounts unresolved copy; missing unconfirmed label/help also recorded independently | NOT_RUN |
| REAL PG metadata, en1440 (1) | UI create → real list card → native Edit → invalid title → three field controls → Save/reload → clear schedule/Save/reload | Actual create201/version1, independent signed authority, owned DRAFT | Missing Edit after successful setup; downstream save assertions remain unexecuted until entry exists | NOT_RUN |
| REAL PG dirty, en1440 Back/zh-TW390 Cancel/zh-CN390 Back (3) | Owned draft create result → dirty input → departure → Keep → departure/Escape → departure/Discard → native Edit/reload → clean Cancel | Actual create201/version1; existing editor from create result (guard setup, not entry proof) | Missing Back/Cancel control or confirmation dialog; expected UI failure is distinguished by named step | NOT_RUN |
| REAL PG readonly, all three locales (3) | Existing signed read-only token, actual GET can_manage=false, three disabled fields/Save, reload | No OIDC issuance or write; no forged permission response | No intended failure; retain real negative authorization coverage | NOT_RUN |

15 new cases total: 8 Console MOCK, 7 REAL PG fixture cases. Console is explicitly MOCK; its metadata PATCH/read-only support is not fabricated. Requests used for fault setup remain the existing loopback fixture API; product operations use native controls.

The metadata case requires exactly one atomic PATCH for the three-field Save (version1→2, Taipei2030-01-01T08:00 → UTC2030-01-01T00:00:00Z), then one separate explicit Save clearing the optional schedule (version2→3). Both reload persisted results, retain ratio/title, and reject any other business method/path. Initial unchanged Save and invalid title produce zero writes. Creation is separate setup, not counted as metadata Save.

Copy identity comes only from the actual successful POST response plus the returned scene GET, never a source ID or synthetic hash success. Success exits preserve the source snapshot and forbid extra business writes even when an Edit assertion fails. UNKNOWN cases do not traverse Edit: their original existing Back/shell counterexamples can fail independently. The original key/body stay in memory only; artifacts contain boolean equality/counts, not raw commands/headers. Retry must reuse both bytes, return the original committed new ID and leave one created object/effect. No UNKNOWN discard action is offered by the test contract.

## Existing coverage and explicit gaps

The unchanged Studio main case retains real persistence, invalid schedule, stale-version CAS, history/locale/shell dirty behavior, POST-create lost ACK, session-boundary refusal and revalidation. Its read-only/expired and native conceal cases remain byte-identical. The old live-console line613 goto remains byte-identical and is not claimed as native Edit entry evidence.

NOT_COVERED/NOT_RUN: metadata PATCH-specific lost ACK and recovery, separately isolated writable-nondraft permission state, dirty Discard while another pending/recovery fence is active, full focus-trap traversal, native local links, Figma/code tokenDiff and all runtime/visual acceptance. No fixture was extended to invent these states. New cases test Keep/Escape input and focus retention; they do not claim an executed keyboard or persistence result.

## Exact fixture cohort effects — parent-owned Go adaptation

Four writer cases each invoke c4OwnedDraft exactly once: metadata(en)1 + dirty(en/zh-TW/zh-CN)3. This is N=4 signedLogin calls and N=4 actual UI create POST requests, with nonempty original keys and actual201 required. No new arm-fault call. Three readonly cases use setSession and issue zero new OIDC sessions/creates. Console logins belong to its separate fixture.

playwright.config.ts remains fullyParallel=false/workers=1; new Studio cases are appended after every original case, not inside a parallel/serial block. Parent explicitly owns the separate Go full-mode exact mapping wantIssued5→9 and wantKeys5→9 plus distinct/nonempty/not-old new-tail-key assertions. Focused mode2/4 and all original fault counts, key indices and SQL readbacks must remain unchanged. This author did not edit Go. A failing setup must be reported as setup failure, never hidden by weakening these counts.

## Actual static commands and receipts

- pwd; git status --short; git rev-parse HEAD: exit0; correct worktree, clean initial tree, exact base above.
- Scoped rg/sed/cat reads and Humaux originals/rejected enumeration: successful reads exit0; rejected coverage11/11. Early guessed source paths/globs/JSON-list parsing returned exit1 and were corrected by scoped source reads; they are not gate results.
- `git diff --check`: exit0, after final spec edits.
- Python `git show <base>:<spec>` + current.read_bytes().startswith(base_bytes): exit0 for both; no old-prefix mutation.
- First static TypeScript API import: exit1, ERR_MODULE_NOT_FOUND typescript-api; no package installed. Native preinstalled TypeScript7.0.2 was then located read-only.
- Native strict/noEmit static compiler via `node /Volumes/data/live_commerce_architecture_v1/node_modules/.pnpm/typescript@7.0.2/node_modules/typescript/bin/tsc --project /dev/stdin --pretty false`: exit1. First config missed existing Node typeRoots; corrected config included the preinstalled @types/node24.13.6 path. Final diagnostics are exactly two TS2339 errors in unchanged live-console.spec.ts312:66/312:110 (`Facts.comments.public_unknown`). No appended-region diagnostics. This is not a green full typecheck; old error was preserved as required.
- Browser/test/Go/PG commands: NOT_RUN by task authorization. No RED execution has been claimed.

Reproduce the final static compiler configuration without writing another file:

```python
import json, subprocess
from pathlib import Path
r = str(Path.cwd())
m = "/Volumes/data/live_commerce_architecture_v1"
opts = dict(target="ES2023", module="ESNext", moduleResolution="Bundler", jsx="react-jsx", strict=True,
            noEmit=True, skipLibCheck=True, esModuleInterop=True, resolveJsonModule=True, allowImportingTsExtensions=True,
            types=["node"], typeRoots=[m+"/apps/admin/node_modules/@types",m+"/node_modules/.pnpm/@types+node@24.13.6/node_modules/@types"])
opts["paths"] = {"@/*":[r+"/apps/admin/*"], "@live-commerce/i18n":[r+"/packages/i18n/src/index.ts"],
 "@live-commerce/format":[r+"/packages/format/src/index.ts"], "@live-commerce/ui":[r+"/packages/ui/src/AppShell.tsx"],
 "@live-commerce/ui/*":[r+"/packages/ui/src/*"], "@live-commerce/markdown-lite":[r+"/packages/markdown-lite/src/index.ts"],
 "react":[m+"/apps/admin/node_modules/@types/react/index.d.ts"], "react/*":[m+"/apps/admin/node_modules/@types/react/*"]}
files = [r+"/tests/admin/live-console.spec.ts",r+"/tests/admin/studio-ui.spec.ts"]
result = subprocess.run(["node",m+"/node_modules/.pnpm/typescript@7.0.2/node_modules/typescript/bin/tsc",
 "--project","/dev/stdin","--pretty","false"],input=json.dumps(dict(compilerOptions=opts,files=files)),text=True)
raise SystemExit(result.returncode)
```

## Integrity / evidence / remaining work

| Spec | Base prefix bytes / lines | Base SHA256 | Current SHA256 |
|---|---|---|---|
| live-console.spec.ts |109090 /1416|6fa246d8f9eec44ac84fd046fb068534ab29acbd0ba539bd79ddf44577868752|acfd218f18ceebe6d0abef74ded4a3f4ed658d1066d473251ac9a76fc9ec44d4|
| studio-ui.spec.ts |33311 /535|c53bd85544b1541f3bb259aab084ab0a39a12fa13acd94d1099de48bddee5217|18d17e5d86dcb6041cca8ab2a81cf9c2986b64b67508e87a1efd05222c402658|

Existing UI architecture800-line ratchet scans only apps/admin, apps/storefront, packages/ui and packages/format (scripts/dev/ui-architecture-gate.mjs146ff); tests/admin is outside that actual scan. No gate waiver was added.

Evidence path: this report; future runtime attachments C4-owned-draft / C4-copy-real-response / C4-UNKNOWN-precondition / C4-UNKNOWN-retry / C4-metadata-zero-write / C4-dirty-zero-write are specifications only until executed. Test-run evidence does not yet exist. Local commit SHA is supplied in the handoff and must bind these exact spec hashes.

Parent must independently review/run foundation, browser-live-console, browser-live-console-parity and browser-studio-ui at a fixed execution SHA with its exact cohort adaptation. Preserve raw failures, distinguish setup from EXPECTED_RED stages, and only then authorize a separate production UI fix. No gate was weakened. Old type errors and all runtime gaps remain unresolved. No process, server, browser, port, container, fixture or temporary file was started/created by this author; cleanup NOT_APPLICABLE. No other task paths were removed.


## Follow-up ec81e27e — explicit waiter budget, first source repair 1/2

task_id: ec81e27e-f342-444c-996c-eeb795c511bf / ec81e27e-sub-1.
base_commit: 533ab5b8b6b64f19e5287df95bbaae8ce0ca0aa9.
Only this report and tests/admin/live-console.spec.ts changed; no other collaborator edits were reverted.

Original failure evidence: Foundation run41432000696/job125232584505, execution8ae34aff3da07ab6562639818a8e88bde4293056, 27/28 Node cases passed and the waiter-budget case failed. Raw source: output/integrator/tools/logs/ci-evidence-1238/365-c4-foundation-1105/foundation-job-125232584505.raw.log, SHA256371da4c7f1ce37ce1192ffdd4f4425c4affd39a607ac965203a62ff38dc84bea (parent-provided raw receipt; no rerun here).

Root cause: the three new C4 waitForResponse calls omitted the explicit second argument required by unchanged tests/admin/live-console-render.test.ts108-126. That guard requires a literal object with numeric timeout >0 and <=15000. Repair adds exactly { timeout: 15000 } at the preparation GET waiter1443, copied-detail waiter1491 (options on closing line1496), and copy POST waiter1497. No callback, predicate, genuine POST/new-ID assertion, key/body, privacy boundary, old-prefix byte or gate was changed.

Actual static commands: Python exact three replacements plus reverse-comparison against533ab5b8 and byte-prefix comparison against5c4f3a5: exit0. git diff --check: exit0. No test/runtime, browser, Go/PG, ownership generation, push, rerun or background work. All post-fix runtime/gate outcomes remain NOT_RUN; this is E1 source repair only, not a claimed green Foundation run. The historical initial handoff and hashes above remain unchanged as prior-candidate evidence.


## d51b32ea — isolated Studio evidence policy and Console first-error diagnostics

task_id: d51b32ea-f374-44fb-b5ae-611382eb9411 / d51b32ea-sub-1.
base_commit: b152df234d5a8658edcd7083b2cf15342fc78c9e.
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr365-mock-balance.
role: single tests-only author; requested Sol/high, actual model/reasoning telemetry UNKNOWN.
Authority: root's explicit six-path card and accepted SOURCE report365-c4-source-1125.md SHA256a9bcfa3b304ce1b0c9b6b7a1200030dbee2ccb7c6c567aaa2855cda4905abab6. Root subsequently authorized explicit15000ms click/response options throughout the NEW Studio spec, including its local setup helpers. Collection repair attempt1/2; Console first diagnostic. This candidate does not claim a product RED or a completed product repair.

Only six changed paths: tests/admin/studio-ui.spec.ts; new tests/admin/studio-ui2-c4-metadata.spec.ts; tests/admin/live-console.spec.ts; playwright.config.ts; tests/foundation/browser_studio_ui_test.go; this report. No UI/product/permission/Go fixture behavior/API/journal/gate/package change. Root/integrator exclusively owns generated ownership and any later push.

### Studio migration and exact preserved scope

Legacy studio-ui.spec.ts is restored to its exact535-line original bytes, SHA256c53bd85544b1541f3bb259aab084ab0a39a12fa13acd94d1099de48bddee5217. The original trace/action/DOM/screenshot policy and every old test/helper/hook remain byte-identical. All seven C4 cases moved to studio-ui2-c4-metadata.spec.ts; no failing case or assertion was removed. That file uses the existing private-evidence wrapper and FILE-top-level baseURL/origin, headless=false, trace/screenshot/video=off, same240000ms case budget. No nested worker-option use remains. Only necessary local login/navigation/token setup was copied from the original helper bodies; importing the old spec would register its tests twice and was not used.

| Original C4 definition at b152 | New definition | Expanded cases | Preserved contract |
|---|---|---|---|
|studio-ui.spec.ts572|studio-ui2-c4-metadata.spec.ts88|1|Native Edit, invalid/unchanged denial, three-field single Save PATCH, Taipei UTC, explicit optional-time clear Save and reload/version persistence|
|studio-ui.spec.ts636|studio-ui2-c4-metadata.spec.ts152|3|en1440 Back, zh-TW390 Cancel, zh-CN390 Back; original Keep/Escape/focus/Discard/clean Cancel and zero-write checks|
|studio-ui.spec.ts676|studio-ui2-c4-metadata.spec.ts192|3|Original actual read-only GET, disabled three fields/Save and reload, no mutation|

The new tail and necessary helper bodies compare exactly to b152 AFTER normalizing only the approved15000ms click/response options and removing the forbidden nested use (now file-top-level). All literals, locators, test names, operation order and business oracles otherwise match. Canonical seven-case tail SHA2567acf4493897b29f2604ea46b0341b9613c5434c657bf1188a9dff865ff4722e5. This is semantic preservation with explicitly authorized timeout options, not a claim that the moved file is byte-identical. Eighteen lexical native click sites and five response waiter sites in the new file have explicit15000ms bounds; no extra visibility waits were added there. Old535 lines were not touched.

Registration is changed at BOTH actual boundaries: playwright.config.ts32 studio-ui array now includes studio-ui2-c4-metadata.spec.ts; browser_studio_ui_test.go227's same Playwright invocation selects both literal spec paths. The new filename follows studio-ui.spec.ts alphabetically; a static Node localeCompare check printed old then new. workers1/fullyParallelfalse remain unchanged. Actual runtime ordering still NOT_RUN.

Go's ONLY byte delta is the extra spec string in that command, verified by reversing that string and comparing the entire file to b152 after gofmt. The330s harness deadline, full9/9 counts, focused2/4, fault3, starts/stops, original key indices and tail-key uniqueness checks, SQL/privacy assertions and permissions are untouched. The new four writer cases still generate exactly four legitimate signed logins and four UI create201 requests. No new fault arm or key-generation behavior. Focused old-main grep still excludes C4 names.

### New Console diagnostic boundaries

All bytes before the C4 metadata marker are unchanged from b152, including old source-ownership/claim-helper cases. B152's exact three waitForResponse calls, callbacks, predicates and literal15000ms budgets remain present. Every original new-C4 business assertion line remains present; the preparation final facts assertion is only named through an equivalent local after variable. No receipt count, effect budget, monetary/identity/UNKNOWN requirement or test-wide deadline changed.

The new native helper first checks the same exact control visible with10000ms and clicks with15000ms. Only new C4 calls use it. Real history Back is bounded15000ms; copy-name fill is bounded15000ms. No API substitute, force click, DOM/hash manipulation or skip. Original copy POST/returned-ID GET and retry flows remain actual requests. Prepared response promises are observed immediately/through Promise.all, so an earlier bounded click failure does not leave an unhandled response waiter rejection that could replace the original action error; the original waiter calls are unchanged.

Each case has a unique, strictly validated file such as c4-metadata-prep-zh-TW-1440.json, c4-metadata-copy-en-1440.json or c4-metadata-unknown-zh-CN-390.json in its existing test-owned evidence directory. The journal uses synchronous writeFileSync with flush=true/mode0600, at most32 phase entries, <=8 projected receipt rows per entry, bounded enum/number/UUID/hex/instant fields and no arbitrary property spread from an upstream response. Full counts are retained when the projected receipt list is bounded. Only constant phase/case identifiers, elapsed times, actual HTTP status, selected/source/copied/detail synthetic IDs, detail version, window state/generation/opened_at, receipt action/status/effect and key_hash/body_hash, and finite write/oracle counters can reach disk. Raw titles, commands/bodies, keys, headers, cookies, exception text, DOM, buyer text and real PII are never added to these files. Existing business checks still inspect the full original data in memory.

Phases are written before/after potentially blocking setup, card, preflight, Back/Edit, submit, actual status/response, returned detail, copy-text, copy/UNKNOWN departure, retry and final-oracle operations. Actual status is persisted before parsing/validation where the response is available; successful copy and lost-ACK setup persist the original returned IDs/window and receipt hash projections before attempting the missing UI. No copied response, source ID or result is fabricated. Existing private evidence guards and legacy evidence settings are unchanged.

Preparation and success-copy catch/rethrow the original hard failure. Their finalizer records secondary API/diagnostic failures as failed/not_run without replacing an already existing hard/soft failure. If the primary path otherwise passes, a failed final facts/receipt/zero-write check STILL throws and fails; no catch-and-pass route exists. Successful paths retain original no-extra-write/source-unchanged/one-effect checks. An earlier failure can leave later oracles NOT_RUN, which is explicit and never reclassified as green. Journal writes themselves fail a clean path; their secondary failure is suppressed only while an existing original test failure is already authoritative. No original failure file was deleted.

### Actual static verification and exits

- Initial git status/rev-parse: exit0; clean baseb152. No other collaborator paths reverted.
- Python migration/reverse-byte comparisons: final exit0. Legacy535 exact; normalized seven-case tail and required setup exact; Go only new spec literal; Console old prefix/exact b152 three waiter calls preserved.
- Python original-oracle line multiset comparison: exit0, no missing/changed original oracle lines; equivalent preparation final facts local-variable substitution checked explicitly.
- One early static probe exit1 because its own expected count incorrectly assumed two braced response callbacks; corrected to compare the exact three original waiter calls. No test or production behavior was altered to accommodate that probe.
- gofmt -w tests/foundation/browser_studio_ui_test.go: exit0; full-file reverse comparison confirmed no formatting/fixture delta beyond the command string.
- Native preinstalled TypeScript7.0.2 strict/noEmit over three specs and config: exit1, exactly unchanged live-console.spec.ts312:66/312:110 TS2339 public_unknown errors. New region diagnostics0; no green full typecheck claim.
- Same static compiler with noCheck/noEmit (syntax-only, same stdin config and files): exit0. This is parsing only and does not replace strict checking or a project gate. No source module/test was executed or collected.
- Node filename localeCompare check: exit0, studio-ui.spec.ts precedes studio-ui2-c4-metadata.spec.ts. Runtime worker/order assertions NOT_RUN.
- git diff --check: exit0 after source changes; actual commit receipt supplied in the parent handoff. Complete candidate diff is available from git diff b152df234d5a8658edcd7083b2cf15342fc78c9e <delivery_commit> -- <six paths>.

Post-change source hashes:
- tests/admin/studio-ui.spec.ts: c53bd85544b1541f3bb259aab084ab0a39a12fa13acd94d1099de48bddee5217
- tests/admin/studio-ui2-c4-metadata.spec.ts: a2bc6644926549e9cc1b555d9f4389baf044d65249add15ba53187033302df38
- tests/admin/live-console.spec.ts: 8e6f88ee72c59931d117107082e852ed03732f4bb8130cf0b6f7d7a575958db2
- playwright.config.ts: 7d22a5cfb63aab68d206d0162a40dfdac3f63256ece1ef222f9f3351e4378c04
- tests/foundation/browser_studio_ui_test.go: 97683286c4f1fd407c7eb9b8969975fd736bc9299238f59fd8d7d3ded0058332

Ownership derivation/check and all registered foundation/browser-studio-ui/browser-live-console/browser-live-console-parity gates are NOT_RUN by this author. Root must generate ownership from the tracked candidate and independently run at a fixed execution SHA, checking collection, legacy evidence preserved, new private evidence absent, all seven migrated case names/order/counters, original failures retained and phase projections actually present. No runtime/Go/PG/browser/provider, test collection, server/container/port, background job, install, ownership generation, push, merge or deployment was started. No temporary script or fixture was created; cleanup NOT_APPLICABLE. Current evidence level E1 static candidate only; product RED/GREEN/E3/E4 all NOT_RUN.
