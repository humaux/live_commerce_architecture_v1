<!-- Purpose: Hand off the D3 Claims mode/picker/prompt repair with backend parity and exact local evidence.
Depends on: integration 8c3851e4, merge 8e070703, Go claims enum, migration 0115 and live-a7-contains-match §4.
Used by: Integrator review, GitHub browser rerun and LC-U1 delivery. -->
# D3 Claims enum and mode/prompt picker

- Branch/worktree: `unit/lc-u1-shell`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u1-shell`.
- Base: `8e070703fcfbee5747ec1bdf992823ae316fb51d`, merging integration `8c3851e4`. The workflow now uses trunk's `--browser*|--stripe-browser*` Xvfb branch and retains shared-store sweep workers=1.
- Author: Codex (system identity GPT-6; runtime effort telemetry not exposed). Independent read-only explorer reviewed the D3 diff and found no concrete P0/P1 code issue; no runtime tests were run by that reviewer.
- The final commit contains parser, picker, host-copy, parity tests and this evidence together. The hashes below bind the tested source before that final commit.

## Behavior and interface

`claims-model.ts` now reads all three actual Go/SQL match modes. `matchModes` is shared with `console-model.ts`, so the two admin readers cannot drift independently. Existing unknown/alias rejection stays closed. M1 board reads and M2 keyed window replies no longer reject `KEYWORD_QTY_CONTAINS`.

Studio Claims has the third mode and its conservative rejection hint. The mode is still locked while OPEN; CLOSED writes keep the same expected version and command payload. The host prompt removes the exact-only “comment only the code” instruction for this mode. Existing EXACT and QTY_ONLY copy in zh-TW/zh-CN/en remains byte-for-byte tested.

Japanese is **host-prompt copy only** (`HostPromptLanguage`). Source `reply_locale`, claim-link language and admin/storefront route locales remain zh-TW/zh-CN/en. A negative source-parser test proves `ja` is still rejected, with `en` as a valid positive control. Copy feedback tracks the actual copied prompt text, so changing language/mode does not show a stale “Copied” claim.

**Documented copy deviation:** the A7 brief's English example `I'll take A1+2` is incompatible with `contains_v2.go`: it contains multiple ASCII fragments, which Go rejects. The new English example uses the valid `A1+2`; the existing frozen EXACT/QTY_ONLY text is unchanged. No Go grammar, SQL or contract file was edited. Integrator should reconcile the brief's example with the actual grammar when updating that document.

## Red → green and local commands

| Command | Exit / result | Evidence |
| --- | --- | --- |
| `node --experimental-strip-types --test tests/admin/claims-backend-parity.test.ts` on old implementation | 1: actual Go-supported mode rejected; wrong Contains prompt | `d3-red.log` |
| Same parity test plus `claims-model.test.ts` and `claims-request.test.ts` | 0, 12/12 | `d3-green.log` |
| `bash scripts/dev/test-node.sh` | 0, 477/477 | `d3-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `d3-tsc.log` |
| `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module ESNext --moduleResolution bundler --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/e2e/live-tools.spec.ts tests/admin/claims-backend-parity.test.ts` | 0 | `d3-test-tsc.log` |
| `go test -run '^TestContainsV2' ./internal/claims/grammar` | 0 | `d3-go-grammar.log` |
| `bash scripts/dev/check-gates.sh` | 0, 72 documented modes | `d3-check-gates.log` |
| `git diff --check`; Ruby YAML parser on workflow | 0 | Command receipts |

The parity test reads the Go constants and all three migration-0115 `match_mode` CHECKs, exercises the real Claims parser with each mode, and checks the picker options against the backend values. It also tests Contains copy and Japanese's presentation-only scope. It is registered in `test-node.sh`.

## CI gates

- `--browser-e2e`: existing live-tools four-cell matrix (zh-TW/en × desktop/390) now uses real clicks to select CONTAINS, opens the window, verifies the locked mode after reload, copies zh-TW/en/ja prompts and checks clipboard text. All original stock/price/order assertions remain. The source reply selector is also asserted to exclude ja.
- `--browser-live-claims`: existing Claims deep-link/read/write and EXACT regression.
- `--browser-live-console`: shared mode enum and Console/A5/A7 parser regression.

All new-source browser executions, screenshots and full foundation checks are **NOT_RUN** here; the integrator runs them on GitHub. Existing run `37457303107` was observed `in_progress` on head `81b9209f` and cannot accept this later D3 source. No local browser/PG run, push, workflow dispatch or deployment occurred. Optional R04 pinned media-binary suite remains NOT_RUN because the binary is unset.

D3 has E3 local parity evidence and a source review. Whole LC-U1 runtime acceptance remains pending; the earlier TCV09/Facebook-CSP ruling is unchanged by this repair.

## Tested source SHA256

| File | SHA256 |
| --- | --- |
| `apps/admin/components/StudioClaims.tsx` | `66e2194bea7746ae59ecdf49a9ef2cd93e7c4460420971854c88c81abc245c07` |
| `apps/admin/lib/claims-copy.ts` | `1bbd0775c2050d2f791399775e65951aaaf3b766057a3e91d706a696939a2812` |
| `apps/admin/lib/claims-model.ts` | `8404b6aa40db43e75d08f55b63e5ec8b703a90c07f320108b1c026f7cf65b341` |
| `apps/admin/src/features/live/console-model.ts` | `b6412ece7e00d9154addc7f55b20fc85809388d440bf004cdfc6f8ff3658c5b7` |
| `scripts/dev/test-node.sh` | `cbe174dcaac1726e6df683a09202aa2235c9e82b5b40635980fcf551e18f111c` |
| `tests/e2e/live-tools.spec.ts` | `c6f4eafdf8eeaa45e735462b53748457bda084175fdb4b59003ddc9e7aeac5f6` |
| `tests/admin/claims-backend-parity.test.ts` | `a47145ba77d87458ab6654b7fca1ffe303599383b7196215d6bdf089f832d32f` |
