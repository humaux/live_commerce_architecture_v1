<!-- Purpose: Deliver the complete PR5 2ad65bcd repair packet without expanding deferred findings.
Depends on: integrator triage pr5-2ad65bcd, LC-U1/W0 navigation contract, source de968f18 and current local gate evidence.
Used by: integrator K3 pre-review and one subsequent PR push; no deployment authorization. -->
# LC-U1 PR5 — ops-polish and stock edit preservation

- PR: https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/pull/5
- Worktree/branch: `.worktrees/lc-u1-shell`, `unit/lc-u1-shell`.
- Synced exactly as requested: fetch, merge `origin/unit/lc-u1-shell` (fast-forward to **2ad65bcd**), then merge `origin/r3/integration` including #10/#11/#12 → **6d75db16a796821cc329d9a3727b919359a4ed2f**.
- Repair source: **de968f18477f648aeb99a22a59f65f423385e27b**. Final evidence commit does not change product/spec source. Author: Codex; exact model/effort identifiers not exposed; no subagent writers.
- Scope: four frontend/spec files, owned evidence/FOLLOWUPS only. No Go/SQL/API/permission/CI changes authored in this batch. No push, review-thread resolution, or deployment.

## Packet results

### REAL CI: ops-polish admin navigation

CI job **112944751566**, run **37654995775**, failed only the three locale variants of OP4. `ci-ops/playwright.log` records the exact mismatch: the test expected only top-level groups, while the current Live group correctly expanded `nav-studio`, `nav-live-console`, `nav-claims`.

`WorkspaceFrame.groupView` expands the current group; LC-U1's approved registry now contains three authorized live destinations. Shell and original spec were unchanged between 2ad65bcd and merge 6d75db16. This is a stale collapsed-group test expectation, not a broken route or a reason to hide current-page navigation.

`assertRegistryNavigation` now has an explicit Studio-page expectation (independent IDs, not derived from the registry). It asserts the three destinations and current-page marker, really clicks collapse (exact **8** entries; children absent) and reopen, then opens Catalog (original exact **11** count). The original denied Customers/Messages/placeholders assertions remain intact. Orders keeps the collapsed initial expectation. No fixture grant, assertion threshold, or shell behavior was weakened.

### 4212525433: stock polling discards unsaved input — FIXED, pending independent review

The composite React key included offer and stock versions, so every changed poll remounted OfferRow and reinitialized its input. The new real-click regression reproduced **expected 23 / received 20** after an external stock update and automatic A1 poll (`red/playwright.log`, screenshot `red/poll-loses-input.png`).

- Key only by stable `offer_id`.
- Idle rows derive their value from current A1 stock; editing rows retain a separate input + stock/CAS snapshot. No effect unconditionally rewrites the draft.
- A changed stock version/value/warehouse/tracking flag shows an inline three-language warning and current sellable quantity. Save is disabled until the merchant explicitly confirms the updated snapshot. Confirm itself sends no command.
- Save computes delta against that confirmed snapshot and passes its version, not a silently rebased poll version. Success clears the local edit; Cancel discards it with no write. Existing server CAS, permission checks, idempotency and UNKNOWN fences remain authoritative.
- Browser proof: idle 17→18 sync; edit **23** → external stock **20/v4** → automatic poll → **23 and focus retained**, warning visible, Save disabled → explicit confirm → save **delta 3, expected_version 4** → final **23** after reload. Cancel is also a real click with unchanged receipt count.

The external update is a test stimulus through the actual authenticated BFF into the existing MOCK console upstream, representing another operator. No DOM/storage patch or fabricated A1 response is used. Authentication setup reuses the suite's `authorityCookie` helper because Node APIRequestContext does not send the signed Secure cookie over the HTTP loopback fixture automatically.

Two author test-setup errors were preserved but **not counted as behavior-red**: missing Playwright context fixture (`test-setup-missing-context.log`) and omission of that existing cookie helper (`test-setup-api-cookie.log`). The later focused red independently reached the intended input-loss assertion. No timeout/retry changes were made.

### Deferred / ignored by instruction

- **4212525437**: Instagram-specific notice condition/copy and its old test expectation are untouched; see [FOLLOWUPS.md](FOLLOWUPS.md).
- Old serial `foundation` job is ignored as instructed; #10 removed it from PR requirements. This does **not** claim that required PG shards/new full CI have passed.

## Current-source gates

All evidence below is under `pr5-round-2ad65bcd/` unless a path is specified. Browser modes are strictly serial and use the heartbeat lock; no source changes during a run.

| Command / scope | Exit / result | Evidence |
| --- | --- | --- |
| `LC_BROWSER_CONSOLE_GREP='MOCK contract real clicks en-1586' bash scripts/dev/test-local.sh --browser-live-console`, original product + new test | **1**, expected23/received20 | `live-console-red.log`, `red/playwright.log` |
| Same focused command, repaired product | **0**, focused only, not full acceptance | `live-console-focused-green.log` |
| `bash scripts/dev/test-local.sh --browser-ops-polish` | **0**, buyer and admin Go gates PASS; admin **8/8** specs | `ops-polish-green.log` |
| `bash scripts/dev/test-local.sh --browser-live-console` (no grep) | **0**, **11/11** specs, all 6 locale/viewport main flows | `live-console-green.log`, `live-console-playwright.log` |
| `bash scripts/dev/test-node.sh` | **0**, **672 PASS / 0 FAIL**, summed runners | `node.log` |
| `pnpm --filter admin exec tsc --noEmit` | **0** | `tsc.log` |
| `LC_HEADER_BASE=6d75db16 bash scripts/dev/check-gates.sh` | **0**, **79 modes** | `check-gates.log` |
| `git diff --check` | 0 | author command receipt |

### Source SHA-256

- LiveConsole.tsx: `feaf0d1691068b47bdb0b5693f5e979a9e4e6295d917ed4c1d5e59a97902a272`
- workspace-copy.ts: `60238ac7411089a9be2bdce876a5148fc96ccdf2cb7e6d7b34bacc81407ca696`
- live-console.spec.ts: `7cbdd8b465d600954b633ba1a8ad97d2707f8e5935d6a6fdd61859afa5f4e736`
- ops-polish.spec.ts: `1b4bdc4d61d6308f67f8e3fdd412a98e521d440c280db7202265228bc2e11115`

## Click evidence / acceptance boundary

| Surface | Real operation | Required result |
| --- | --- | --- |
| Studio navigation, 3 locales | Collapse/reopen Live, then open Catalog | Exact permitted entries/counts; denied entries absent |
| Console, 1586 and 390 × 3 locales | Type stock; allow automatic poll | Idle sync, edited quantity and focus preserved |
| Changed-stock warning | Confirm, then Save | Confirm writes nothing; delta3/version4; reload23 |
| Stock editor | Type then Cancel | Restore current server value, zero additional receipt |

All click-ledger rows passed on the final source. Six warning screenshots are retained in `pr5-round-2ad65bcd/shots/*-stock-edit-poll.png`, including the existing no-horizontal-overflow assertion. Other previous lifecycle, polling, unknown recovery, cross-store and permission tests are retained and passed. English desktop/mobile shots were inspected: input remains focused at23, current20 warning and disabled Save are readable.

Evidence class: ops-polish uses actual Go/PG + signed MOCK IdP; live-console uses real browser/Next/BFF/PG identity with **MOCK console/inventory upstream and receipts**, not backend inventory SQL or provider acceptance. E3 is limited to completed automated local checks. Independent K3 review, fresh required PR CI, global click sweep/visual lint, other browser modes and production/live providers remain NOT_RUN in this batch. Optional R04 input runner is NOT_RUN (missing configured binary), excluded from 672 passes.

Only this task's downloaded unfiltered CI artifact was removed after retaining the relevant failure log/context/screenshot; the original is re-downloadable from GitHub. No other worktree/process was changed. Integrator runs K3 pre-review and pushes once after this batch is accepted; author stops after final commit.
