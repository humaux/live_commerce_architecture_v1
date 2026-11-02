# F03 product candidate — first fix 1/2

- task_id: `359e36e2-e7b3-4cff-b866-4b5cb663665e`; delegated element: `53ea-sub-f03-fix-1706`. Root retains coord and exclusive author lane; no child claim/delegation.
- base_commit: `0412b48a6d6c2a0e9babd6973c066bbf086f92b7`; branch: `codex/pr475-inventory-repair`; worktree: `.worktrees/codex-pr475-inventory-repair`.
- Role: UI implementation author, not independent acceptance. Assigned model: `gpt-6.1-sol`; reasoning: `high` (`fork_turns="none"`). Actual runtime model/reasoning telemetry: **UNKNOWN**.
- Allowed/final paths: `apps/admin/components/ImportWizardSteps.tsx`, `apps/admin/components/imports/data-ops.css`, this report only.
- Evidence: **E1 structure candidate; runtime GREEN and independent review NOT_RUN**. This report does not close F03 or approve PR475, mobile visual parity, or production release.

## Root cause and frozen authority

The signed summary occupied the first of three cells and inherited the shared order-table first-cell width of 21%. At mobile widths, `orders.css:85` applies a grid row with 13px inline padding. Separately, the small signed pager used `hidden`, but `.import-actions` author CSS sets `display:flex`.

Read original CI evidence at `output/integrator/tools/logs/ci-evidence-1238/475-f03-red-1704/{REPORT.md,playwright.log}` in the main evidence tree. REPORT SHA256: `bd1c959873f0ade9bddc4e506ce6c1acfea8f63624259956fe2e0e54647e5754`. Run `41482738569`, job `125407601779`, execution `250c2f7454593db916bea13fdf0b3a77d4ee2bd0`, parents `c6be8dd13981b13df4c9c994fb66523c9c893b9e` + base above. Six independent locale/width cases reached both checks after actual POST200 and the original 2-new/0-update/1-failed preconditions: desktop right delta221.96875>2, mobile left/right13>2; both pager buttons visible true versus false. Four existing preview/stale cases show the same defects. No claim that their later privacy/state checks independently passed.

The SOURCE card was read in full: `output/integrator/tools/logs/ci-evidence-1238/475-f03-source-1555.md`, SHA256 `f16cc591f888b26188ecdc665e0646c9ba02e5aae5168304d44d12c6a3cb54b3`. Authority is the underlying fixed source, contract and direct official PNGs, not the report itself. Viewed official desktop exports `475-visual-review-1140/figma-1028-6685.png` and `figma-1198-10518.png` from Figma file `iJ9lDdFIXgTlKkt5Tvrc0X`; their hashes are respectively `295b87cccce672ebb9f9d50adb87ae44692250ce3c8a32b17c072190d5dff0d9` and `78664bac3c3fd182bb3efac89cb7494ad794f1c19db68cb503ac5e8c2925ff7a`. Reused the parent's frozen context/screenshot read; no new design or provider call. Original mobile exports crop above these rows: below-fold visual geometry remains **UNKNOWN**.

Read repository AGENTS, architecture §§0/1/2/26/27, task DAG slices T14/T18/T21, import contract §§1–6.1, relevant invariants I01/I02/I05/I06/I09/I11/I18, both direct component call sites and the unchanged independent F03 assertions. Applied `figma-implement-design`, ponytail, and Humaux visual/development rules. Rejected lookup was a representative5/9, coverage0.5556, **not a complete history**; parent's separate lookup also does not authorize weakened checks.

## Implementation

1. Signed rows now have one `td colSpan={3}` containing the exact existing localized composite summary and both existing consent/city warning conditions. The visually hidden signed header has one matching three-column header with `scope="col"` and the existing row/outcome/code labels. Stable row keys/test IDs, failures-first ordering and the original 50-row slice are unchanged. Unsigned rows retain their three cells, contents and warning semantics.
2. Signed results of50 or fewer rows omit the pager DOM. Unsigned results and signed results above50 retain the previous/next elements, disabled bounds, state handlers and live range output verbatim.
3. Only the new signed table class receives CSS: reset inherited first-cell width; at the shared order-table mobile breakpoint680px, remove inline row inset/grid gap and use block row/cell layout. Selectors outrank the generic order rules without changing global CSS or F02/stock/card rules. Existing tokens and all file header comments remain.

Product hashes after the final edit:

| Path | SHA256 |
|---|---|
|`apps/admin/components/ImportWizardSteps.tsx`|`0b3f758497ec16e007ec65757ac416ef983e209cdbaa0bc9124f1d0a250a374e`|
|`apps/admin/components/imports/data-ops.css`|`c78c98a20a5299017c2786891008d38f33ae33e8a74aee8752b95afb95d7d888`|

Unchanged frozen regression hashes: parity-customer-import.spec.ts `2d211e2ec4a6963f0148ea9826c0988e3ec004a1c073c3183cb7a039e2b74805`; orders.css `43e73be1220b7ec5d578cad74a51251cff550fd2cca9f0ea864f01eb3a802b83`; import-wizard.css `218a9b8b45f2dc72c5e3170c138b07e07f2ac90d5a97fff52c8581fbc2c1ac52`.

## Actual commands and limitations

| Command / operation | Exit / result |
|---|---|
|`git status --short`; `git rev-parse HEAD`; bounded `rg`/`sed`/`cat` source and contract reads|0 on corrected worktree reads; base verified, five old KEEP files present|
|Two exploratory read batches against the main evidence tree instead of the product worktree|One batch exit1 for missing REPORT path; another shell batch exit0 despite missing ImportWizard reads. Corrected source reads in the dedicated worktree exit0; no mutation|
|`test -d node_modules && test -d apps/admin/node_modules`|1; local dependency tree absent; no install attempted|
|`git diff --check`|0 after final product edit|
|`python3 scripts/check_packet.py`|0, `PASS_PACKET_STRUCTURE_ONLY`; not TS compilation or product validation|
|`git restore --source=HEAD -- experiments/results/packet-check.json`|0; checker unexpectedly rewrote this generated timestamp artifact; restored only this task's side effect immediately; no residual out-of-scope diff|
|`git diff --stat`; `git diff -- <two product paths>`; `shasum -a 256 <product/frozen/evidence paths>`|0; reviewed small leaf diff and bindings above|
|Humaux initial own-canvas update without mermaid|Rejected, no write; corrected update with mermaid succeeded under acquired dispatch lock, own element only; lock released|
|`git add -- apps/admin/components/ImportWizardSteps.tsx apps/admin/components/imports/data-ops.css output/parity-data-ops/F03-FIX.md && git commit -m 'fix(imports): span signed verdict summaries and omit small pager'`|0; local commit only; final SHA supplied in handoff after report-receipt amendment; no push|
|Typecheck/build/browser/Go/PG/mutation execution/provider/SANDBOX/LIVE/current-SHA CI/independent acceptance|**NOT_RUN**, root owns execution/review; no runtime started|

The packet command wrote an unintended generated artifact before restoration; this is disclosed rather than presented as a write-free check. No test, fixture, DTO, copy dictionary, state handler, journal, global CSS, shared schema, lockfile or ownership artifact has a residual edit. No gate or threshold was changed.

## Remaining acceptance and cleanup

Root must verify exact final diff/ownership, regenerate authorized ownership artifacts, push and obtain current-SHA CI for the unchanged six locale/width geometry≤2 and hidden-pager assertions plus old preview/stale checks. Original privacy/rollback/UNKNOWN/request identity/amount/high-row50→51→1 pagination assertions remain blocking. A full44-frame/token comparison and independent review remain **NOT_RUN**; this bounded fix does not waive them. Signed warning presentation and large signed pagination still require runtime regression verification.

All five original untracked `output/parity-data-ops/inventory-diagnosis-4556/` KEEP artifacts remain untouched. No process, fixture, container, port, temp dependency tree or background runtime was started. No production, remote write, push, merge or deployment occurred.
