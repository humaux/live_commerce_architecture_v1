# PR #365 — integrator ruling repair

Date: 2026-11-01. Scope: the integrator's P1-A/P1-B ruling, one product repair push;
no test, timeout, fixture, assertion, import, contract or permission change.

## Authority and revision

- Authority: owner-dispatched ruling in this session, matching read-only
  `output/integrator/tools/cards/cxf-parity-live-sessions-ruling.md` in the main checkout;
  `parity-common.md` forbids collapsing allowed actions; `contracts/customers-billing-v1.md`
  U6 / CB11 requires a refusal reason and billing link.
- Branch: `unit/parity-live-sessions` → `r3/integration`, existing PR #365.
- Starting HEAD / rollback: `af99d1c9d4e3eb3b175d0e576ea49da54882e980`.
- Fetched integration: `fde11c1310fce20b576fc98023bd3003c66c205b`.
  `git merge --no-edit origin/r3/integration` exited 0: already up to date
  (existing merge `e42a91e1b` is retained).
- Implementation role: Codex UI lane, this worktree only. Runtime model/effort identifiers
  are not exposed by the execution tools. No delegation or other worktree writes.
- Humaux/coord/canvas tools are unavailable; no shared-memory write or independent
  acceptance is claimed. The pre-existing untracked `DIAG.md` is preserved.

## Root causes and repairs

| Ruling | Root cause | Source change | Acceptance gate |
| --- | --- | --- | --- |
| P1-A | The same `actionError` was rendered by two `alert("window")` call sites. | `apps/admin/components/StudioClaims.tsx:495` is now the sole window-error render. The rail duplicate is removed; the shared renderer at `:278` keeps the reason, billing link and uncertain-result retry semantics. | Existing CB11 real-click RESTRICTED → GOOD → closed chain; strict locator requires a unique alert and billing link. |
| P1-B | The outer Advanced disclosure enclosed every tool, including the prompt and the source form used after reload. | `StudioClaims.tsx:517–531` limits Advanced to secondary round/time/statistics detail. The prompt keyword at `:539–541`, language/copy actions, mode/window controls and source/offer actions are outside every disclosure. | Existing live-tools four-cell zh-TW/en × desktop/mobile chain reloads, selects the keyword, copies all three prompt languages, then binds the source and completes the claim/price flow without reopening Advanced. |

Advanced remains a real secondary-detail disclosure, so unchanged `openClaimTools`
and existing statistics click tests retain their real-click target. No additional UI
strings or locale entries are introduced. Existing zh-TW, zh-CN and en copy is reused.

## RED evidence and current CI

- Original causal tests-only RED: head `395209e39`, Gates run **41052979614**,
  job **124055728603**, four new C4 assertions failed; full identity in `red.log`.
- Current regression RED: head `16b22886b98bf815163c349425e2672f1993e8f7`,
  Gates run **41270522783**, jobs **124742946382** (`bundle:05`, live-tools hidden
  select after reload) and **124742946232** (`bundle:07`, CB11 duplicate refusal).
  GitHub run/job conclusion and head were queried again in this session.
- No new RED push: this ruling explicitly forbids any test change and requires one
  repair push. Existing RED assertions remain untouched.
- Repair-head Gates/Foundation run IDs: PENDING_PUSH. The post-push PR comment will
  record the exact head, run/job IDs and WAIT_RUN handoff; no evidence-only push.
- GREEN: PENDING the PR's own CI. Author self-assessment is E0, static receipts E1;
  previous RED is E3 failure evidence only. No completion/E4 claim.

## Static receipts and boundaries

| Command | Exit | Result |
| --- | --- | --- |
| `node scripts/dev/derive-browser-ownership.mjs` | 0 | Regenerated 55 modes, no file diff. |
| `node scripts/dev/derive-browser-ownership.mjs --check` | 0 | Generated ownership matches. |
| `bash scripts/dev/depmap.sh` | 0 | Regenerated dependency map, no file diff. |
| `python3 scripts/check_packet.py` | 0 | PASS_PACKET_STRUCTURE_ONLY; timestamp-only generated receipt reverted. |
| `git diff --check` | 0 | No whitespace errors. |

Local tsc is unavailable and application dependencies are absent; compilation and
browser verification belong to CI. Commit hooks are disabled for this commit to honor
the prohibition on local tests. No local tests or servers were started.

Outside card write paths in this ruling repair: only this requested evidence file.
Generated ownership/dependency-map files were regenerated without changes. Earlier
PR-wide scope disclosures remain in the PR body; pre-existing local evidence commits
and integration merge are preserved in the single push.

NOT_RUN: local runtime tests/servers (prohibited), local tsc (unavailable), repair-head
CI until its run concludes, independent review, provider SANDBOX/LIVE and production
acceptance; Humaux store/canvas. No production action, merge, auto-merge, force-push,
commit status posting, dependency installation or background server.
