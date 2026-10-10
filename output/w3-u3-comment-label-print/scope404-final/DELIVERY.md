<!-- Purpose: W3-U3 exact post-PR36 verification request and current local evidence; no browser PASS claim.
Depends on: merged PR36, frozen source22ef3ac2, unchanged specs and the current heavy-gate execution rule.
Used by: integrator CI dispatch and this thread's completion follow-up. -->
# W3-U3 — PR36 merged; browser acceptance pending

- Source: `22ef3ac2a2b5263c9e2add87a34a03b154953614`; branch `unit/w3-u3-comment-label-print`.
- GH #36: MERGED `2026-10-10T05:12:55Z`, merge `287e08aa6c694030333dd00797c0d3bbac15abc6`.
  Fetched origin matches; merge SHA is an ancestor of this source (exit0).
- `e4e48ec5` exact revert is an ancestor. BFF auth/catchall/inbox-bff tests equal trunk;
  no `58510545` re-proof is reintroduced. No conflict, product change or duplicate Go/SQL repair.
- Full accepted label/stream/print CSS and both specs equal `0b153c90`.
  Workspace SHA256 `3df8c6fa47d7e559ff535ed7f146d3dec307ce4d973f66ab65906216b4d1182b`;
  label SHA256 `c5f1b3f7a685c1fd284cc03416f6b96cdaf509f71f114acb19bf3bf3bd12b156`.
- Read-only reviewer `w3_merge_audit` (`gpt-6.1-sol`, medium): E1, no preservation/merge finding,
  no gate execution. Single native mode registry and workspace+label registration retained.
- Only this W3-U3 unit is owned; W3-U2 is not modified.

## Actual local checks (source22ef)

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-node.sh` |0|1376/1376, 27 summaries, fail/skip/cancel0; `node.log` |
| `pnpm --filter admin exec tsc --noEmit` |0|`tsc.log` |
| `bash scripts/dev/check-gates.sh` |0|82 documented modes, inventory1262, headers; `gates.log` |
| `go run ./scripts/dev/contractdrift` |0|**errors=0**, 362 existing baseline warnings; `contractdrift.log` |
| Ancestors / source-spec/BFF parity |0|Checks above; no actions, waiters, thresholds or timeouts edited |

These are E1 structural/static checks, **not** current E3 browser acceptance. Old candidate browser results
are historical and do not satisfy the new two-run requirement.

## CI gates — exact acceptance

Current `AGENT-PREAMBLE.md` and stored owner rule require every browser/full-foundation heavy gate
on GitHub, not this Mac. One asynchronous execution-location choice was presented to the integrator;
absent explicit local exception the existing GitHub rule remains. No heavy run or push was started.
Integrator owns branch push/PR/CI dispatch.

Use ONE fixed source commit (or evidence-only descendant whose apps/tests/scripts bytes equal this source):

1. `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-live-console`
2. After #1 finishes, run the **same original command again**, from the **same** source commit.
3. `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-inbox`
4. `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-click-sweep`
5. `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-visual-lint`

For GitHub, two distinct sequential workflow runs/IDs for the live-console mode are required;
a matrix with a duplicated string or one test run reported twice is not two-run evidence.
Within each original mode, keep all original operations/assertions/wait conditions/timeouts.
The console Go timeout stays780s; no grep/focus/filter/calibration is substituted for the complete mode.

Both console runs must each execute the original workspace and 9 label cases, including:
- `LCU2_404 real store grant revoked clears all view and selected buyer`;
- `LCU2_404 real store grant revoked clears private view and selected buyer`;
- private data/selection clearing, same valid session, no reads after scope loss, and grant restoration;
- unchanged A3 lost ACK same-key counts and native per-case click ledger.

Capture run IDs, exact head/source SHA, command/exit/counts, full artifact paths and ledger outcomes at launch;
watch through completion. Do not start a duplicate run or kill/delete someone else's PID/lock.
A failure preserves raw evidence; any product Go/DTO/contract change still requires a new ruling.

The semantically affected subset above does **not** reduce required PR CI. Actual unchanged
`node scripts/dev/pr-modes.mjs origin/r3/integration` emits53 modes for314 paths (conservative UI-path/tag
selection); the integrator's required plan remains authoritative.

## NOT_RUN / state

Current two full console runs, inbox, click-sweep and visual-lint: **NOT_RUN**, awaiting execution location/CI.
New full foundation, current independent K3/required PR CI, physical printing/provider LIVE/deploy: **NOT_RUN**.
W3-U3 is **not READY** until actual full results arrive. No push/deploy, no Go/SQL repair, no weakened tests.
Local sessions56325/65180/16811/86939 all ended; foreign `output/ext-agents/` remains untouched.
Exact coordination task `dbd03f8b-e0f2-4f2e-8ad9-a6f7e03efd0c`, agent `codex-w3-u3`.
Quiet `w3-u3-scope-404` heartbeat stays active until two-run handoff or explicit cancellation.
