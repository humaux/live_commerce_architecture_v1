# R11 authoritative UNKNOWN UI correction

- task_id: `7dfa5128-7828-4ff3-9d3c-901677926039`
- base_commit: `72f36edbdde8d2726f15f12fbd065f825ad9b94f`
- commit: `62a77c98313ea11ee305bdc41e9a05ebbb14840d` (footer-only amend of `156354fa`; source unchanged)
- worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution-r11-ui`
- branch: `unit/ads-attribution-r11-ui`
- role: ui_worker; actual model/reasoning: UNKNOWN (runtime does not expose the current identity)
- skills: frontend-architect (receipt/control separation), playwright (preserve real-click acceptance boundaries; no browser run)

## Changed paths

1. `apps/admin/components/AttributionAudienceRead.tsx`
2. `apps/admin/lib/attribution-copy.ts`
3. `tests/admin/attribution-audience.test.ts`

Root review 34d69c85 identified that authoritative operation UNKNOWN is cached in the idempotency response, so another same-key POST cannot poll later status. The UI removes that misleading check button and its unused three-language copy. New requests remain disabled. Three-language status tells the user the existing result is unknown, to read the report later, and not create another read. The durable receipt survives reload unchanged with the same operation ID and key, without another POST. Only a missing/unconfirmed transport acknowledgement (phase `unknown`) retains the same-key retry. Exact receipt state parsing, HTTP, auth, journal model, and real-click browser assertions are unchanged.

## Commands and evidence

Run from the worktree above. Log files are adjacent to this document.

| Command | Exit | Evidence |
| --- | --- | --- |
| `node --test --experimental-strip-types tests/admin/attribution-audience.test.ts` (tests changed first, source unchanged) | 1 | `node-red.log`: 8 tests, 4 PASS / 4 FAIL; three-locale rendering and authoritative-UNKNOWN hook fail |
| same targeted command after source correction | 0 | `node-green.log`: 8 PASS / 0 FAIL |
| `bash scripts/dev/test-node.sh` | 0 | `node-suite.log`: 381 PASS / 0 FAIL across suites |
| `pnpm typecheck:admin` | 0 | `typecheck-admin.log` |
| `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module ESNext --moduleResolution Bundler --allowImportingTsExtensions --esModuleInterop --typeRoots apps/admin/node_modules/@types --types node tests/admin/attribution-audience.test.ts` | 0 | `typecheck-tests.log` |
| `pnpm exec prettier --write apps/admin/components/AttributionAudienceRead.tsx apps/admin/lib/attribution-copy.ts tests/admin/attribution-audience.test.ts` | 0 | Formatting completed before GREEN checks |
| `git diff --check` | 0 | No whitespace errors |
| `git commit --amend -m 'fix(admin): distinguish cached audience UNKNOWN from transport loss' -m 'Co-Authored-By: Codex <noreply@openai.com>'` | 0 | Footer corrected to frozen address; no source changes |

## Scope and remaining gates

MODEL_ONLY / SSR-hook and static verification, not product acceptance. PG, browser, deployment and external writes: NOT_RUN. Root owns independent review and integrated final gates. The suite explicitly leaves `tests/media/r04-input-runner.test.mjs` NOT_RUN because `COMMERCE_R04_LIVEKIT_BINARY` is unset; unrelated to this UI unit. No backend fix or polling endpoint is introduced. An authoritative UNKNOWN can remain unresolved until backend reconciliation; the UI does not promise a new POST will resolve it. No known unresolved issue within the requested three-file correction.
