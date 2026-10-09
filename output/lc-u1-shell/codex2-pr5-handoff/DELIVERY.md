<!-- Purpose: Codex-2 acceptance of the inherited PR5 repair batch after the latest trunk union.
Depends on: pr5-2ad65bcd packet, inherited de968f18 repairs, ea212c5e merge and fresh local gates.
Used by: integrator K3 pre-review and push; does not authorize deployment. -->
# PR5 Codex-2 handoff — 2026-10-08

- Branch/worktree: `unit/lc-u1-shell`, `.worktrees/lc-u1-shell`.
- Entry was clean **01fd3ddaff565a4a9e467d1b39469fabdca098cf**, an explicit Codex-1 handoff. It already contained the stock/ops repair **de968f18**, the earlier inbox UNION **0456860b**, and evidence **4832ba81**. These commits and their source were preserved, not reimplemented.
- Required sync: fetch + merge own remote (already up to date), then merge current `origin/r3/integration` **97e34a4c7646de9d0753fb83ed88d56c1b43f01c**. Fresh tested merge is **ea212c5ec76930bf9ac31055a3290cc46c31b73a**, parents01fd3dda and97e34a4c. No rebase, force-push or push.
- Author of the inherited repair: Codex-1. Codex-2 owns merge verification and fresh acceptance. One high-effort read-only explorer independently reviewed stock reconciliation, ops navigation and inherited red evidence; no delegated writer. Exact deployed model identifier unavailable. K3 pre-review remains the integrator's next step.

## Scope and conflict reconciliation

The latest merge introduced only two `test-local.sh` mode-list conflicts: local had `--browser-live-console`, incoming had `--browser-reports`; inbox was present on both sides. Validation, usage and admin-build conditions now retain all three. Independent inventory proof: **80 + 80 → 81 unique modes, missing=[] for both parents** (`union.json`). Both runtime dispatch branches and GATES rows remain.

The earlier next.config/sweep conflicts were already resolved in0456860b and are byte-identical in the newly tested merge. `configuration-union.json` compares both original parents: all header source routes remain, including M7 no-referrer and unchanged CSP; neither parent declares rewrites. The sweep options still mount CommentStream, Inbox and MsgTemplates, and retain trunk's authorized empty-inbox response/authentication check. No new header or fixture behavior was authored here.

A setup command initially failed before editing because the main evidence parent directory was absent. A following syntax check caught conflict markers temporarily staged by that command sequence; no bad commit was made. After creating the directory, the actual union, shell syntax, marker absence and index state were checked successfully. This setup failure is not counted as a behavior-red test.

The requested `docs/delivery/units/lc-u1*.md` glob was absent in both worktree and main checkout. Scope was recovered from the explicit packet, inherited delivery and `contracts/live-console-v1.md` §§1,2.6,7 and its LC-U1 unit table. No missing contract decision was invented.

## Packet closure

- **REAL ops-polish:** inherited fix updates the stale navigation oracle to include the approved current Live group. It asserts explicit child IDs, exact collapse/reopen counts and current-page marking, and retains denied destinations. Fresh `--browser-ops-polish` passes both buyer and admin halves; admin **8/8**. Original CI job112944751566 evidence and root cause are in the preserved inherited delivery.
- **4212525433:** stable `offer_id` key; idle rows follow current stock, edited rows preserve their quantity and CAS snapshot. A changed stock snapshot disables Save until explicit confirmation. Existing test red was expected23/received20; its log was inspected and preserved. Fresh unfiltered live-console passes **11/11**, including six locale/width flows proving input23 + focus survive an external20/v4 poll, confirmation sends no write, Save sends delta3/version4, and reload retains23. Cancel, UNKNOWN and permission checks also pass.
- **4212525437:** IG-only notice condition/copy is unchanged and remains in `../FOLLOWUPS.md`.
- Old serial `foundation` is ignored as instructed. Required new PR CI is still pending; no foundation or provider acceptance is inferred.

## Actual checks on ea212c5e source

| Command | Exit / result | Evidence |
|---|---|---|
| `node --test --experimental-strip-types tests/admin/shell-registry.test.ts` | 0; **11 PASS** (G-UI1) | registry.log |
| `bash scripts/dev/check-gates.sh` | 0; **81 modes**, header ratchet | gates.log |
| `bash scripts/dev/test-node.sh` | 0; **933 PASS / 0 FAIL / 0 SKIP**, summed runners | node.log |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | types.log |
| `bash scripts/dev/test-local.sh --browser-ops-polish` | 0; buyer + admin Go gates, admin **8 PASS** | ops-polish.log |
| `bash scripts/dev/test-local.sh --browser-live-console`, `LC_BROWSER_CONSOLE_GREP` unset | 0; **11 PASS**, no filter/skips | live-console.log |
| `bash -n scripts/dev/test-local.sh`; staged diff check; union assertions | 0 | union.json / configuration-union.json |

Per-command JSON receipts retain actual child exits and elapsed times. Node/static gates ran on the exact tree subsequently committed as ea212c5e; both browser modes ran pinned to that commit, strictly serial with heartbeat PG lock and a3600-second wrapper deadline. No product/spec source changed during or after them. A fresh English390px stock-warning screenshot was inspected:23 remains focused, current20 is shown, Save is disabled and confirmation is readable; this is limited visual inspection, not full visual acceptance.

Evidence root in the **main checkout**: `/Volumes/data/live_commerce_architecture_v1/output/lc-u1-shell/codex2-pr5-handoff/`. `fresh-browser/` holds current traces/screenshots, `inherited-evidence/` preserves the prior red/green and union records formerly present only in the worktree. `tested-source.json` binds seven relevant files. `receipt.json` is written after the final documentation commit and binds it to the unchanged tested source.

**Evidence class:** E3 local automated acceptance. Ops-polish uses signed MOCK IdP and actual Go/PG. Live-console uses actual Next/BFF/session/CSRF and PG identity, but console/inventory upstream and receipts are MOCK; not LC-B1/LC-B7 inventory SQL or Meta/provider acceptance.

**NOT_RUN:** K3 pre-review, fresh required GitHub round, other browser modes/global click-sweep/visual-lint, backend inventory SQL/provider SANDBOX/LIVE, optional R04 (binary unset). No extra focused Go suite was needed for this merge-only authored change; both browser-tag harnesses compiled and executed. All owned runtime processes finish before final commit. Integrator reviews and pushes; Codex-2 stops after commit.
