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
