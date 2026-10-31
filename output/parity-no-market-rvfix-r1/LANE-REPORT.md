# LANE-REPORT — RELAY 1 / RVFIX (PR #403, head 945849795)

Evidence level: E0 (no local runs; CI only).

| Finding / job | Class | What changed |
|---|---|---|
| RVFIX P1-1 (created_at heuristic) | already fixed in 945849795 | SettingsWizard.tsx:75-83 `pickMarket` (draft, active tw, sole active, else null); output/parity-no-market/RVFIX.md |
| RVFIX P1-2 (POST over existing tw) | already fixed in 945849795 | SettingsWizard.tsx:981-985 refuses POST, error `market_code_taken`; copy `twTaken` in settings-copy.ts |
| gate (bundle:06) fail, job 124595056230 | unclassified | Log not retrievable: run 41220653875 is still in_progress, so `gh run view --log-failed` refuses; direct job-log API call needs approval and was denied. No code change made. |

WAIT_RUN: 41220653875 (job 124595056230 = gate (bundle:06), failed at 15m48s; other gates pass or pending). Re-run `gh run view 41220653875 --log-failed --job 124595056230` once the run completes, then classify (RED assertion vs ownership drift vs other).

STATUS: BLOCKED gate (bundle:06) log unavailable until run 41220653875 completes; cannot classify failure without it
