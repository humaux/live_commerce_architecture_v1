# FEAT-MANUAL-DISCOUNT — REBASE report

PR: #394 (`unit/feat-manual-discount` -> `r3/integration`).
Authority: integrator's exact REBASE card, 2026-11-01; AGENTS.md; 架构.md §§0,1,2,26,27;
docs/engineering/contractdrift.md. Role: dispatched implementation lane; Codex GPT-6 runtime,
exact model identifier/reasoning setting unavailable. No delegation. Assigned worktree only.

## Change and conflict resolution

- Original HEAD: `65d02773ac43f7663e66bee255eeac70d0e6036b`.
- Merged base: `87c67e33c329a43bb6fc0d51d9160ac4c9826b98` (#396); main checkout untouched.
- Exactly four conflicts, all card-listed: storefront contract, dispatcher pin, R2 upgrade oracle,
  browser ownership. No unexpected conflict.
- Storefront contract keeps base FEAT-LIVE-REPLAY amendment first, then FEAT-MANUAL-DISCOUNT.
- Shared HTTP error map merged automatically; both base replay and feature discount messages remain.
- Renamed migrations/0216_manual_order_discount.sql -> 0220_manual_order_discount.sql;
  updated its Purpose prefix and all active references outside output. Historical `renumbered from 0216`
  remains only as required provenance. No SQL behavior change.
- Reference-only updates: contracts/cart-quote-v1.md, contracts/core-openapi.json description,
  contracts/storefront-v2.md, internal/merchanttools/discount.go header and internal/storefront/manual_discount.go header.
- Took base R2 file and appended the assigned 0220 provenance line; count/oracle 143 -> 144.
- Took base dispatcher file; only buyerhttp.Routes hash/reviewed changes against base. Functions 83 -> 83,
  declarations 154 -> 154, exact lists unchanged; no additions/removals.
- Base hash `31499ffb3ce0f159bcacb0f380bcbfe7fb94c220f5b91231d91e5edd840fbbd7` -> merged candidate
  `a816f2699027fc26ca5fa3b7c87258e8b809ebb3c97e24baff797f88b9e9ba30`.
- Required pending-independent-review wording set verbatim. Base reviewed text retained verbatim in
  output/buyer-route-table/manual-discount-pin-candidate.md and the appended PIN.md REBASE note.
- New base closure delta only: type orderQuoteResponse adds Adjustments; projectOrder copies stored quote.Adjustments.
  All other buyerhttp files equal base, including #396 replay behavior. No route, auth, scope or client-money change.
- Browser ownership regenerated then checked before commit. Dependency map regenerated, no diff.
- Preexisting unstaged FIX1.md, FIX4.md, PIN.md and untracked evidence retained. Only this card's appended
  PIN note is staged, using the old committed PIN content plus that append; prior dirty PIN evidence stays unstaged.
- Packet command's generated experiments/results/packet-check.json was restored to its original tracked content.

## Verification and evidence

No local tests or servers. Static evidence E1 only; author PASS E0.

| Command | Exit | Evidence |
| --- | --- | --- |
| go run ./scripts/dev/contractdrift with base pin | 1, expected changed-closure diagnostic | rebase-scanner-before.log |
| same scanner with merged candidate | 0, errors=0 | rebase-scanner-after.log |
| node scripts/dev/derive-browser-ownership.mjs | 0 | rebase-ownership.log |
| node scripts/dev/derive-browser-ownership.mjs --check | 0 | rebase-ownership-check.log |
| python3 scripts/check_packet.py | 0 | rebase-packet.log |
| bash scripts/dev/depmap.sh | 0, no diff | rebase-depmap.log |
| go build ./... | 0 | rebase-build.log |
| go vet ./... | 0 | rebase-vet.log |
| git diff --cached --check | 0 | pre-commit static whitespace check |

Logs are in output/feat-manual-discount/. Committed scanner logs retain the exact diagnostic/summary
and raw-log SHA-256; full raw logs stay local and untracked, excluding trailing-whitespace-heavy warnings. This is a rebase of an already RED-tested card, with no new
behavior/tests; no redundant tests-only push. Historical RED: Foundation run 41093578253/job124193423117,
head fc28de173547886f74900c5caf4ab1f7a675b451 (GitHub failure/head reverified): three executed Node assertions
for thumbnail preservation, discount request grammar and discount result parsing. Raw assertions retained in red.log.
Historical Gates run 41093578464; its job IDs remain in red.log. Historical evidence is not current-head GREEN.

Current GREEN: pending the single rebased push's own PR CI. Workflow run/job IDs and final head are posted
in a PR comment after push, avoiding any evidence-only follow-up commit. Wakeup: integrator receives GitHub
completion and relays a fresh session; this session will not poll longer than ~10 minutes.

## Scope, risks and NOT_RUN

Outside REBASE card write scope: NONE. The OpenAPI description and two source headers are explicitly
covered by step 2's every-reference requirement; generated ownership/depmap covered by card. Files imported
unchanged from #396 are merge inputs, not extra feature work. Existing PR-wide outside-original-feature paths
remain listed in its PR body; no additional authorization is inferred.

Risk: candidate closure requires independent whole-closure review; merged 0219/0220 must pass actual PostgreSQL
upgrade tests in CI. No test, fixture, threshold, exemption, baseline or scanner change. Gate-adapter assertions untouched.
NOT_RUN: all local runtime/unit/PG/browser/model tests and servers; new-head CI until completion;
independent whole-closure review/E4; SANDBOX/LIVE, deployment. No merge, auto-merge, force-push, status posting,
other worktree/branch changes, provider call, running fixture/container/port or task-created background watcher.

STATUS: WAIT_CI
