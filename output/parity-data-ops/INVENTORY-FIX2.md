# PR475 inventory repair — E1 handoff

task_id: `53ea9f21-3f3b-4626-87ab-42dc853507b9-sub-inventory-repair`
parent task_id: `53ea9f21-3f3b-4626-87ab-42dc853507b9`
base_commit: `1c2bee34d9e053c9102681edcb8cf084eec69345`
branch: `codex/pr475-inventory-repair`
worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr475-inventory-repair`
role: inventory UI implementation worker; actual runtime model and reasoning effort: **UNKNOWN** (not exposed to this worker). No recursive delegation. Parent owns claim and integration.

The initial worktree was clean at the assigned base. This is one implementation round, committed locally only. Resulting commit SHA is recorded in the parent handoff and Humaux; the source hashes below bind this report without a self-referential commit hash. Current-source runtime GREEN and independent acceptance are **NOT_RUN**. This report does not claim completion.

## Authority and scope

Read AGENTS.md, 架构.md §§0/1/2/11/26/27, contracts/tasks.json T04/T05, contracts/invariants.json, contracts/admin-ledger-v1.md, Humaux `d39917c4-db68-4415-abcf-ab2a4f88df6e` and current development detail `7bdb220b-0798-4b1d-aff5-16ee94cae75f`. Rejected coverage was supplied by the parent (9/9); no new architecture or backend state was introduced. Applied ponytail and impeccable layout guidance to the existing implementation. The audit used incumbent CSS and original CI geometry before editing; it did not redesign the signed C6 surface.

Changed paths:

- `apps/admin/components/imports/data-ops.css`: only existing inventory inspector selectors.
- `tests/admin/parity-data-ops.spec.ts`: only the inventory reason-heading locator.
- `tests/ui/click-sweep.mjs`: only the J1 inventory row locator.
- `output/ci-select-backend-browser/browser-ownership.json`: generated discovery hashes; parent regenerates after integration.
- This report. `Ledger.tsx` required no edit.

I01/I02/I03/I04/I06/I11/I16/I18/I19/I21 remain applicable. No inventory quantities, fixture values, amount/timezone rules, submission/retry semantics, thresholds, timeout/retry settings, observer protocols, or other journeys changed. The native filechooser and timeOrigin assertions were untouched.

## Original RED → root cause → fix

All RED evidence is native GitHub Gates run `41292495090`, PR head `809b3c9c044da7c0c2f73e251cacd6e67fa68cff`, execution merge `fb9002883f5b8f533a5c38b27901144100452eb8`. Original raw files remain read-only in the parent's `parity-data-ops` worktree under `output/parity-data-ops/fix1-current-ci/artifacts/`. Their hashes were recomputed and matched `FIX1-current-evidence-manifest.json`.

| Failure | Original evidence | Root cause and minimal repair |
|---|---|---|
| Real tray containment failure, 1366×768 | job `124807346472`, artifact `13078212264`, `gate-browser-admin-legacy/playwright/run.5WABsLW0/admin-ledger-20261101T092606.111819885/playwright.log:27–42`; `ledger.spec.ts:479/532`: close button `1294..1361`, SVG `1319..1336`, tray `293..1313`. Raw log SHA256 `e757aa2babfe44b6a7546d8d4d02e34a190c1843bddd0901edfe37b5dc6d7e7e`. | `.data-ops .inspector { position: static }` overrode the legacy inspector's relative containing block, while `.close-details` remained absolute. Also the scoped generic button padding replaced the icon's 7px padding. Restore the inspector's relative containing block and the icon's existing 7px padding, retaining the existing 44px minimum touch target. The original four-viewport geometry gate and tolerances are unchanged. New geometry is NOT_RUN. |
| Required reason text resolves to two nodes | job `124807346464`, artifacts `13077987561` / `13078276821`, `parity-data-ops.spec.ts:130`; directly read catalog-core log at `playwright/run.kpKqQUlj/catalog-core/20261101T092919.460868536/playwright.log`, SHA256 `1927fba45069fbd62aa2dc5c24ef9c878150a6b6399c7dd18219c9534192ef8c`. Strict locator returns h2 plus disclosure label. | Preserve the exact required-reason wording and visibility assertion; target the level-2 heading by accessible role. Product copy and required-reason behavior are unchanged. |
| J1 expected available 9, element missing | job `124807346731`, artifact `13078925634`, `gate-browser-click-sweep_10_10/playwright/run.gcHjDOZN/ui-click-sweep/ledger.json`, `r00087`; SHA256 `b296b1c2376bff5dc6be4ff00aee5ac408999b6c8da21c35f4ef0a62cff1484f`. | `InventoryView.tsx:37–40` renders code + separator + product-name button in one cell; exact bare-code `getByText` cannot match it. Match the exact accessible cell name `${code} · ${title}` within the row, then retain `.available-value` and the unchanged 9/7 assertions. Product save, reload, search clicks, and subsequent journey steps remain. This differs from FIX1's earlier missing-search-input root cause. |

Directly viewed original `ui-click-sweep/screenshots/r00087.png` (SHA256 `da58171ebba47dd397f54984aeb6c6212c48d86eebb4bac3f58b6bbf4974ce6e`): both intended SKU rows and available values **9 / 7** are visible. This establishes the old locator failure against visible data; it is not new-source runtime or direct PostgreSQL acceptance.

## Actual static commands and receipts

Commands ran in this isolated worktree. No local test runner, Go, PG, browser, application server, or build was started.

| Actual command | Exit | Evidence |
|---|---:|---|
| `git status --short && git rev-parse HEAD && cat AGENTS.md` | 0 | Empty status; assigned base `1c2bee34d9e053c9102681edcb8cf084eec69345`. |
| `node /Users/luolimo/.codex/skills/impeccable/scripts/context.mjs --target apps/admin/components/Ledger.tsx; cat /Users/luolimo/.codex/skills/impeccable/reference/layout.md; cat /Users/luolimo/.codex/skills/impeccable/reference/craft-floor.md` | 0 | Compound read receipt; static incumbent context and layout references loaded; no context files changed. |
| `node --check tests/ui/click-sweep.mjs` | 0 | JavaScript syntax only. |
| `node /Users/luolimo/.codex/skills/impeccable/scripts/detect.mjs --json --scope layout apps/admin/components/Ledger.tsx apps/admin/components/imports/data-ops.css` | 0 | `[]` both before and after; source-level scan only. |
| `node scripts/dev/derive-browser-ownership.mjs` | 0 | Generated 55 modes at the authorized output path. |
| `node scripts/dev/derive-browser-ownership.mjs --check` | 0 | No drift. |
| `git diff --check` | 0 | No whitespace errors. |
| Inline read-only Python SHA256 check against `FIX1-current-evidence-manifest.json` for the three raw files above | 0 | All three hashes matched; exact raw messages and r00087 were read. |

The first read of `/Volumes/data/live_commerce_architecture_v1/output/parity-data-ops/FIX1.md` reported missing file (sed error); parent supplied the correct `.worktrees/parity-data-ops/` location, then the report and raw evidence were read successfully. A compound discovery command ended exit1 because local compiler/dependency paths are absent. Full strict TypeScript checking was **NOT_RUN**, not a pass; this worktree has neither root nor admin `node_modules`, and no dependency installation or symlink was created.

Current source SHA256:

```text
dd445dc3864bf637d1f880211ada411109f63e7de9bf40dababddff8c0ac010c  apps/admin/components/imports/data-ops.css
0d656f7fdc86fbb5a985de4e44679ea5f14493090c9dd6da50c532dd7f9af0f5  tests/admin/parity-data-ops.spec.ts
c8bf1904f7f9c45a2420d928e4b497f87bcfadf37474780381d94f8c10c92934  tests/ui/click-sweep.mjs
9e9319ee65bd894bc3240095b516c48374fc35f7b116e4ceed6f7c632cf569d2  output/ci-select-backend-browser/browser-ownership.json
```

## Remaining gate and handoff

Parent integrates, regenerates ownership with all lanes, and runs GitHub checks tied to the resulting SHA: unchanged ledger geometry at 1366/1586/2000/375; inventory parity review/result/conflict/unknown/discard at both existing viewports; J1 real clicks with 9/7 after save/reload and subsequent persistence; affected browser modes and required checks. Independent review must be by someone other than this author. This worker must not start a second implementation round for the same failure; report the exact new failure to the parent for adjudication.

Current-source strict typecheck, runtime tests, browser/PG acceptance, new screenshots/click ledger, all-green CI, independent review, SANDBOX and LIVE: **NOT_RUN**. New-source success is **UNKNOWN**. Other PR failures (including editor/search, customer import, offline checkout and startup guard) are outside this card and remain subject to their owners' evidence. This card is E1 structural readiness plus E3 historical failure evidence, never full-product completion.

No background process or WAIT_RUN was created. No fixture, container, port, temporary file, artifact download, or other worker's directory was created or removed. Only the authorized source, generated ownership and this report were written.
