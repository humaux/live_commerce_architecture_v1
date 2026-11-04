# Meta UNKNOWN Back repair — author handoff

- task_id: `fe55d3aa-87c7-4a7b-ae49-43b3137d4177`; author `meta_unknown_fix_0843` (ui_worker responsibility, no delegation).
- base_commit: `55c49af6006ed6632950053f5c36775fb08e1031`; branch `codex/pr365-mock-balance`.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr365-mock-balance`.
- Requested model/reasoning: `gpt-6.1-sol` / `high`; actual model/reasoning: `UNKNOWN`.
- Allowed and changed paths: `apps/admin/components/MetaConnect.tsx`, this handoff.

## Frozen implementation

Only the intro/bridge Back button at `MetaConnect.tsx:229` changes from `disabled={busy}` to `disabled={busy || uncertain}`. An UNKNOWN connect start therefore retains the existing bridge recovery control in the current render. The official same-key retry expression, connect key, uncertainty state, Back handler, readback branch, journal, API, permission controls, CSS, fixtures and tests are byte-preserved. No new backend state or capability is introduced.

Root task assignment freezes this repair after independent P1-B in `output/integrator/tools/logs/ci-evidence-1238/review365-a327-0758.md` (read at repository root). Source authority is `MetaConnect.tsx@55c49af6`, particularly lines 140–161, 213, 218 and 226–241. Read `AGENTS.md`, architecture sections 0, 1, 2, 26, 27, relevant task DAG and invariants I01/I02/I06/I21. I06 requires preserving the semantic key under UNKNOWN.

The root-supplied genuine RED is Gates `41412447980`, bundle-08 job `125169676856`, artifact `13126412974`, execution `d1f3ef4e` with parents `c6be` / `55c`, raw log SHA256 `9d6dc152a8e0646fc7aa5bf59dbecd99793dfadb1c8d016304c8c166c9283fb9`, at root `output/integrator/tools/logs/ci-evidence-1238/365-meta-red-0840/`. Root reports 18 PASS / 2 FAIL at spec line 759 for desktop/mobile Back enabled. This author did not refetch or independently execute that CI result; it is a locator, not this patch's green evidence.

## Actual checks and exit codes

| Command / operation | Exit | Result |
|---|---:|---|
| `pwd; git status --short; git rev-parse HEAD; git branch --show-current` | 0 | Clean assigned tree, expected base and branch |
| `python3` exact-byte replacement with clean/base/count assertions | 0 | Exactly one frozen expression replaced |
| `node -e` resolve probes for root `node_modules/typescript-api` and `node_modules/typescript` | 0 | Both unavailable; no dependencies installed |
| `command -v tsc` | 1 | Compiler unavailable |
| `ls /opt/homebrew/lib/node_modules/typescript/lib/typescript.js /usr/local/lib/node_modules/typescript/lib/typescript.js` | 1 | Neither compiler module exists |
| `python3` compare patched bytes to `git show HEAD:apps/admin/components/MetaConnect.tsx` with only the exact frozen replacement; subprocess `git diff --exit-code HEAD -- tests` | 0 | One-expression comparison PASS; tracked tests unchanged |
| `git diff --check` | 0 | No whitespace defects |
| `git diff --numstat` | 0 | Product diff is 1 insertion / 1 deletion |
| `shasum -a 256` source and three test paths | 0 | Source and test pins below |

Base source SHA256: `fd94c71db462843c26c9f3e8d1d9dc91fd07de59a41009f0a781f183b5f85b42`.
Patched source SHA256: `7d102deb56e9481cc984667c57bc6db0dc9865cc12ea749eb4540350e13797a3`.
Unchanged `tests/admin/meta-connect.spec.ts`: `b0cac22a546e2af72129bd7489d38d6ace21bc4af1f74d318d1fde22d437e7af`.
Unchanged `tests/admin/meta-connect-gate.spec.ts`: `0fc43a710716f937daf7ad8b7432013464a638d66c80b77fb817e9e81704d698`.
Unchanged `tests/admin/live-console.spec.ts`: `6fa246d8f9eec44ac84fd046fb068534ab29acbd0ba539bd79ddf44577868752`.

## Evidence limits and next acceptance

E1 author structural evidence only; product completion and current-SHA browser green are UNKNOWN. Syntax parser/typecheck: NOT_RUN because no installed compiler was found; no installation attempted. Local Go, PostgreSQL, browser, build, runtime, provider, SANDBOX and LIVE: NOT_RUN under this task's static-only scope. No push, merge or CI trigger by this author. No background processes or temporary fixtures were created.

Root must independently review the two-path diff, push through its existing flow, execute unchanged affected CI real-click cases at the new head, then obtain independent high review. Required behavioral evidence remains: genuine committed start with lost acknowledgement, UNKNOWN alert, Back disabled at 1440 and 390 widths, original official retry enabled, same key and body on replay, and one external effect. This author does not claim those assertions passed after this change. Other PR findings and whole-PR approval remain outside this card.
